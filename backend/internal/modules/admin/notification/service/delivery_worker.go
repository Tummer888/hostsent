package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
	"hostsent/backend/internal/pkg/jobrun"
	"hostsent/backend/internal/pkg/notifier"
)

// DeliveryWorkerOptions Worker 运行参数。
type DeliveryWorkerOptions struct {
	Interval    time.Duration // 轮询间隔，默认 15s
	Batch       int           // 单轮领取条数，默认 50
	MaxAttempts int           // 最大尝试次数，默认 5
	// OnDead 死信回调（告警用）；为空只记日志。
	OnDead func(ctx context.Context, d *notifymodel.NotificationDelivery)
}

// DeliveryWorker 投递队列工作器：领取到期任务 → 解析渠道 → 发送 → 回写状态。
//
// 退避：delay = min(30s * 2^attempts, 30min)；attempts >= max_attempts 转 dead。
// skipped 与 failed 的区别必须守住：skipped = 配置类问题（不重试），
// failed = 网络/服务商问题（重试），混在一起会导致「没配渠道」被反复重试到死信。
type DeliveryWorker struct {
	repo notifyrepo.DeliveryRepository
	// resolver 渠道解析（Enqueue 之外唯一的外部依赖）。
	resolver ChannelResolver
	// switchReader 读通道总开关原文（键不存在返回 ok=false）；nil 时按未配置处理。
	switchReader func(ctx context.Context, key string) (string, bool, error)
	opts         DeliveryWorkerOptions
	logger       *zap.Logger
}

// DeliveryWorkerDeps Worker 依赖。
type DeliveryWorkerDeps struct {
	Repo         notifyrepo.DeliveryRepository
	Resolver     ChannelResolver
	SwitchReader func(ctx context.Context, key string) (string, bool, error)
	Options      DeliveryWorkerOptions
	Logger       *zap.Logger
}

// NewDeliveryWorker 创建投递工作器。
func NewDeliveryWorker(deps DeliveryWorkerDeps) *DeliveryWorker {
	opts := deps.Options
	if opts.Interval <= 0 {
		opts.Interval = 15 * time.Second
	}
	if opts.Batch <= 0 {
		opts.Batch = 50
	}
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = notifymodel.DefaultMaxAttempts
	}
	return &DeliveryWorker{
		repo: deps.Repo, resolver: deps.Resolver, switchReader: deps.SwitchReader,
		opts: opts, logger: deps.Logger,
	}
}

// Start 阻塞循环，由 Server.Run 的 ctx 控制生命周期。
func (w *DeliveryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.opts.Interval)
	defer ticker.Stop()
	w.logger.Info("notification delivery worker started",
		zap.Duration("interval", w.opts.Interval), zap.Int("batch", w.opts.Batch))
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("notification delivery worker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

// RunOnce 暴露单轮执行（测试与手动触发用）。
func (w *DeliveryWorker) RunOnce(ctx context.Context) { w.runOnce(ctx) }

func (w *DeliveryWorker) runOnce(ctx context.Context) {
	// 高频任务（15 秒一轮）：留痕侧空轮不落库（doc92 §7.2）。
	jobrun.RunErr(ctx, "notify_delivery", "notify", jobrun.TriggerScheduled,
		func(ctx context.Context) (int, int, map[string]any, error) {
			rows, err := w.repo.ClaimDue(ctx, w.opts.Batch, notifymodel.DeliveryLockTTL)
			if err != nil {
				w.logger.Warn("notification delivery: claim due failed", zap.Error(err))
				return 0, 0, nil, err
			}
			for i := range rows {
				w.deliver(ctx, &rows[i])
			}
			return len(rows), len(rows), map[string]any{"picked": len(rows)}, nil
		})
}

func (w *DeliveryWorker) deliver(ctx context.Context, row *notifymodel.NotificationDelivery) {
	switch row.Channel {
	case notifymodel.ChannelInbox:
		// 站内信落库即送达（notifications 行已在 Publish 时写入），标记 sent 即可。
		if err := w.repo.MarkSent(ctx, row.ID, "", "", 0); err != nil {
			w.logger.Warn("notification delivery: mark inbox sent failed", zap.Uint64("id", row.ID), zap.Error(err))
		}
		return
	}

	// 发送总开关：sms_channel_enabled=false 时短信直接跳过（与既有 mail_channel_enabled 语义一致）。
	if row.Channel == notifymodel.ChannelSMS && !w.channelSwitchEnabled(ctx, "sms_channel_enabled") {
		_ = w.repo.MarkSkipped(ctx, row.ID, "短信通道总开关未开启")
		return
	}
	if row.Channel == notifymodel.ChannelMail && !w.channelSwitchEnabled(ctx, "mail_channel_enabled") {
		_ = w.repo.MarkSkipped(ctx, row.ID, "邮件通道总开关未开启")
		return
	}

	cfg, err := w.resolver.Resolve(ctx, row.Channel, notifyCategoryScene(row.Channel))
	if err != nil {
		if errors.Is(err, ErrChannelNotConfigured) {
			_ = w.repo.MarkSkipped(ctx, row.ID, "未配置可用渠道")
			return
		}
		w.markFailure(ctx, row, err)
		return
	}
	sender, err := notifier.New(cfg.Type, *cfg)
	if err != nil {
		if errors.Is(err, notifier.ErrAdapterNotImplemented) {
			_ = w.repo.MarkSkipped(ctx, row.ID, "该服务商适配器待接入")
			return
		}
		w.markFailure(ctx, row, err)
		return
	}
	msg := notifier.Message{
		Category:     row.Channel,
		Recipient:    row.Recipient,
		Subject:      row.Title,
		Body:         row.Content,
		Format:       firstNonEmptyString(row.ContentFormat, notifier.FormatText),
		TemplateID:   cfg.TemplateCode,
		SignName:     cfg.SignName,
		Vars:         decodeVars(row.Vars),
		SourceModule: row.SourceModule,
		SourceID:     row.SourceID,
	}
	res, err := sender.Send(ctx, *cfg, msg)
	if err != nil {
		w.markFailure(ctx, row, err)
		return
	}
	// 记录实际使用的渠道实例，便于发送日志与计费核对。
	if err := w.repo.MarkSentWithChannel(ctx, row.ID, cfg.ChannelID, res.ProviderMsgID, res.ProviderCode, res.CostFen); err != nil {
		w.logger.Warn("notification delivery: mark sent failed", zap.Uint64("id", row.ID), zap.Error(err))
	}
}

func (w *DeliveryWorker) markFailure(ctx context.Context, row *notifymodel.NotificationDelivery, sendErr error) {
	attempts := row.Attempts + 1
	maxAttempts := row.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = w.opts.MaxAttempts
	}
	if attempts >= maxAttempts {
		if err := w.repo.MarkDead(ctx, row.ID, sendErr.Error()); err != nil {
			w.logger.Warn("notification delivery: mark dead failed", zap.Uint64("id", row.ID), zap.Error(err))
		}
		w.logger.Error("notification delivery: dead after max attempts",
			zap.Uint64("id", row.ID), zap.String("channel", row.Channel),
			zap.Int("attempts", attempts), zap.Error(sendErr))
		if w.opts.OnDead != nil {
			w.opts.OnDead(ctx, row)
		}
		return
	}
	next := time.Now().Add(backoffDelay(attempts))
	if err := w.repo.MarkFailure(ctx, row.ID, attempts, &next, sendErr.Error()); err != nil {
		w.logger.Warn("notification delivery: mark failure failed", zap.Uint64("id", row.ID), zap.Error(err))
	}
}

// channelSwitchEnabled 读取通道总开关；读不到时按「未配置」处理：
// 只有在系统里显式开启才发送，避免升级后突然开始外发短信/邮件。
//
// 例外：mail 沿用历史键 mail_channel_enabled，历史行为是「配置了 SMTP 即启用」，
// 因此读不到时对 mail 放行（保持升级前行为不变），短信读不到则跳过。
func (w *DeliveryWorker) channelSwitchEnabled(ctx context.Context, key string) bool {
	if w.switchReader == nil {
		return key == "mail_channel_enabled"
	}
	v, ok, err := w.switchReader(ctx, key)
	if err != nil || !ok {
		return key == "mail_channel_enabled"
	}
	return v == "true" || v == "1"
}

// backoffDelay 退避：30s * 2^(attempts-1)，上限 30min。
func backoffDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	d := 30 * time.Second
	for i := 1; i < attempts; i++ {
		d *= 2
		if d >= 30*time.Minute {
			return 30 * time.Minute
		}
	}
	return d
}

// notifyCategoryScene 通道 → 渠道场景。业务通知走 notify，验证码走 otp。
func notifyCategoryScene(channel string) string {
	if channel == notifymodel.ChannelSMS || channel == notifymodel.ChannelMail {
		return notifymodel.ChannelSceneNotify
	}
	return notifymodel.ChannelSceneNotify
}

func decodeVars(raw string) map[string]string {
	if raw == "" || raw == "null" {
		return nil
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func encodeVars(vars map[string]string) string {
	if len(vars) == 0 {
		return "{}"
	}
	raw, err := json.Marshal(vars)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// deliveryErrorSummary 组装友好错误摘要（保留上游原文以便排查）。
func deliveryErrorSummary(op string, err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", op, err.Error())
}
