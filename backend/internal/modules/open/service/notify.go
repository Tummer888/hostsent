package service

// 事件回调（T6.5，doc16 §8.4）：
//   - EventPublisher 把业务事件落 open_notify_deliveries（pending）；
//   - NotifyDeliveryWorker 进程内工作池按退避投递到 app.notify_url，
//     2xx 视为成功，指数退避重试，耗尽转 dead（可经 openapp redeliver 人工重投）。
//
// 回调验签（与下游约定）：
//	headers: X-Open-Event / X-Open-Timestamp / X-Open-Delivery / X-Open-Signature
//	signature = hex( HMAC-SHA256( notify_secret, timestamp + "\n" + body ) )

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	openmodel "hostsent/backend/internal/modules/open/model"
	openrepo "hostsent/backend/internal/modules/open/repository"
	"hostsent/backend/internal/pkg/crypto"
)

// 事件名（标准契约，下游按事件分发）。
const (
	EventInstanceCreated       = "instance.created"
	EventInstanceStatusChanged = "instance.status_changed"
	EventInstanceRenewed       = "instance.renewed"
)

// EventPublisher 事件发布器：落库即返回，投递由工作池异步执行。
type EventPublisher struct {
	notifyRepo openrepo.NotifyRepository
	appRepo    openrepo.AppRepository
	logger     *zap.Logger
}

// NewEventPublisher 构造事件发布器（依赖仅仓储，装配期可早于通知中心构造）。
func NewEventPublisher(notifyRepo openrepo.NotifyRepository, appRepo openrepo.AppRepository, logger *zap.Logger) *EventPublisher {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &EventPublisher{notifyRepo: notifyRepo, appRepo: appRepo, logger: logger}
}

// PublishToOwner 向归属账号的全部可通知应用（启用且配置了 notify_url）发布事件。
// 单条落库失败只记日志，不影响业务主流程。
func (p *EventPublisher) PublishToOwner(ctx context.Context, ownerUserID uint64, event string, data map[string]any) {
	if p == nil {
		return
	}
	apps, err := p.appRepo.ListNotifiableByOwner(ctx, ownerUserID)
	if err != nil {
		p.logger.Warn("open: resolve notify targets failed", zap.Uint64("owner_user_id", ownerUserID), zap.Error(err))
		return
	}
	if len(apps) == 0 {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"event":       event,
		"occurred_at": time.Now().Format(time.RFC3339),
		"data":        data,
	})
	for _, app := range apps {
		row := &openmodel.OpenNotifyDelivery{
			AppID:       app.ID,
			Event:       event,
			Payload:     string(payload),
			Status:      openmodel.OpenNotifyPending,
			MaxAttempts: defaultMaxAttempts,
		}
		if err := p.notifyRepo.Create(ctx, row); err != nil {
			p.logger.Warn("open: enqueue notify delivery failed",
				zap.Uint64("app_id", app.ID), zap.String("event", event), zap.Error(err))
		}
	}
}

const defaultMaxAttempts = 8

// NotifyDeliveryWorker 回调投递工作池（单协程扫描，与 provision worker 同构）。
type NotifyDeliveryWorker struct {
	repo     openrepo.NotifyRepository
	appRepo  openrepo.AppRepository
	encrypt  string
	logger   *zap.Logger
	interval time.Duration
	client   *http.Client
}

// NotifyWorkerOptions 工作池参数。
type NotifyWorkerOptions struct {
	// Interval 扫描间隔，默认 30s。
	Interval time.Duration
	// Timeout 单次回调超时，默认 5s。
	Timeout time.Duration
}

// NewNotifyDeliveryWorker 构造回调投递工作池。
func NewNotifyDeliveryWorker(
	repo openrepo.NotifyRepository,
	appRepo openrepo.AppRepository,
	encryptKey string,
	logger *zap.Logger,
	opts NotifyWorkerOptions,
) *NotifyDeliveryWorker {
	if logger == nil {
		logger = zap.NewNop()
	}
	if opts.Interval <= 0 {
		opts.Interval = 30 * time.Second
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}
	return &NotifyDeliveryWorker{
		repo:     repo,
		appRepo:  appRepo,
		encrypt:  encryptKey,
		logger:   logger,
		interval: opts.Interval,
		client:   &http.Client{Timeout: opts.Timeout},
	}
}

// Start 启动扫描循环（阻塞，由 Server.Run 的 ctx 控制退出）。
func (w *NotifyDeliveryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *NotifyDeliveryWorker) runOnce(ctx context.Context) {
	rows, err := w.repo.ListDue(ctx, 50)
	if err != nil {
		w.logger.Warn("open: list due deliveries failed", zap.Error(err))
		return
	}
	for i := range rows {
		w.deliver(ctx, &rows[i])
	}
}

// deliver 投递单条事件：成功→success；失败→退避重试或转 dead。
func (w *NotifyDeliveryWorker) deliver(ctx context.Context, row *openmodel.OpenNotifyDelivery) {
	app, err := w.appRepo.GetByID(ctx, row.AppID)
	if err != nil {
		w.markFailure(ctx, row, fmt.Errorf("resolve app: %w", err))
		return
	}
	if app.NotifyURL == "" {
		// 回调地址被移除：转死信待人工处理，避免无限重试。
		_ = w.repo.MarkDead(ctx, row.ID, row.MaxAttempts, "notify_url removed")
		return
	}
	secret, err := crypto.Decrypt(app.NotifySecret, w.encrypt)
	if err != nil {
		w.markFailure(ctx, row, fmt.Errorf("decrypt notify secret: %w", err))
		return
	}

	body := []byte(row.Payload)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "\n" + string(body)))
	signature := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, app.NotifyURL, bytes.NewReader(body))
	if err != nil {
		w.markFailure(ctx, row, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Open-Event", row.Event)
	req.Header.Set("X-Open-Timestamp", ts)
	req.Header.Set("X-Open-Delivery", strconv.FormatUint(row.ID, 10))
	req.Header.Set("X-Open-Signature", signature)

	resp, err := w.client.Do(req)
	if err != nil {
		w.markFailure(ctx, row, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		w.markFailure(ctx, row, fmt.Errorf("non-2xx status %d", resp.StatusCode))
		return
	}
	if err := w.repo.MarkSuccess(ctx, row.ID); err != nil {
		w.logger.Warn("open: mark delivery success failed", zap.Uint64("delivery_id", row.ID), zap.Error(err))
	}
}

func (w *NotifyDeliveryWorker) markFailure(ctx context.Context, row *openmodel.OpenNotifyDelivery, cause error) {
	attempts := row.Attempts + 1
	if attempts >= row.MaxAttempts {
		if err := w.repo.MarkDead(ctx, row.ID, attempts, cause.Error()); err != nil {
			w.logger.Warn("open: mark delivery dead failed", zap.Uint64("delivery_id", row.ID), zap.Error(err))
		}
		return
	}
	next := time.Now().Add(retryBackoff(attempts))
	if err := w.repo.MarkFailed(ctx, row.ID, attempts, next, cause.Error()); err != nil {
		w.logger.Warn("open: mark delivery failed", zap.Uint64("delivery_id", row.ID), zap.Error(err))
	}
}

// retryBackoff 指数退避：1m, 2m, 4m, 8m, 16m, 32m, 64m（封顶 64m）。
func retryBackoff(attempts int) time.Duration {
	shift := attempts - 1 // attempts=1 → 1m
	if shift < 0 {
		shift = 0
	}
	minutes := 1 << uint(shift)
	if minutes > 64 {
		minutes = 64
	}
	return time.Duration(minutes) * time.Minute
}
