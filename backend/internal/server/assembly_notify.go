package server

// 消息中心装配（doc90 N1–N5）：
//
//   - 渠道类型/实例（descriptor 驱动前端动态表单，凭证字段级加密）
//   - 短信模板 + 模板变量注册表
//   - 投递队列（worker 领取 → 解析渠道 → 发送 → 回写状态）
//   - 消息群发（目标解析 → 分批写投递记录）
//
// 依赖方向：notification 模块不 import 任何业务模块；收件地址经
// repository.RecipientResolver 端口外接（只读 users 表），群发目标也只读
// users/user_groups，避免为群发引入 admin/user 模块的权限类型。

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	reconmodel "hostsent/backend/internal/modules/admin/resource/reconcile/model"
	reconcilerepo "hostsent/backend/internal/modules/admin/resource/reconcile/repository"
	notifydto "hostsent/backend/internal/modules/admin/system/notification/dto"
	notifyhandler "hostsent/backend/internal/modules/admin/system/notification/handler"
	notifymodel "hostsent/backend/internal/modules/admin/system/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/system/notification/repository"
	notifyservice "hostsent/backend/internal/modules/admin/system/notification/service"
	openrepo "hostsent/backend/internal/modules/open/repository"
	openservice "hostsent/backend/internal/modules/open/service"
	pushservice "hostsent/backend/internal/modules/upstreampush/service"
	"hostsent/backend/internal/pkg/cache"
	"hostsent/backend/internal/pkg/safego"
)

// notifyBundle 消息中心处理器与后台工作器集合。
type notifyBundle struct {
	channelHandler     *notifyhandler.ChannelHandler
	smsTemplateHandler *notifyhandler.SmsTemplateHandler
	broadcastHandler   *notifyhandler.BroadcastHandler
	deliveryHandler    *notifyhandler.DeliveryHandler
	// resolver 发送侧渠道路由：投递队列与 OTP 下发（doc91 §5.1 切换点）共用。
	resolver notifyservice.ChannelResolver
	// worker 投递队列工作器（由 Server.Run 的 ctx 控制生命周期）。
	worker *notifyservice.DeliveryWorker
}

// notifyBundleDeps 装配依赖。
type notifyBundleDeps struct {
	DB         *gorm.DB
	EncryptKey string
	// NotifySvc 既有通知服务（装配后注入投递队列依赖）。
	NotifySvc notifyservice.NotificationService
	// NotifyRepo 站内信仓储（通知记录页「重发」转发用）。
	NotifyRepo notifyrepo.NotificationRepository
	// ConfigReader 读 system_configs 原文（键不存在返回 ok=false，不报错）。
	ConfigReader func(ctx context.Context, key string) (string, bool, error)
	// Counter 频控计数器（Redis；不可用时降级放行）。
	Counter notifyservice.TestRateCounter
	Logger  *zap.Logger
}

// buildNotifyBundle 装配消息中心。
func buildNotifyBundle(deps notifyBundleDeps) *notifyBundle {
	channelRepo := notifyrepo.NewChannelRepository(deps.DB)
	channelTypeRepo := notifyrepo.NewChannelTypeRepository(deps.DB)
	smsTplRepo := notifyrepo.NewSmsTemplateRepository(deps.DB)
	varRepo := notifyrepo.NewTemplateVarRepository(deps.DB)
	deliveryRepo := notifyrepo.NewDeliveryRepository(deps.DB)
	recipientRepo := notifyrepo.NewRecipientResolver(deps.DB)
	broadcastRepo := notifyrepo.NewBroadcastRepository(deps.DB)

	channelSvc := notifyservice.NewChannelService(channelRepo, channelTypeRepo, deps.EncryptKey)
	channelResolver := notifyservice.NewChannelResolver(channelRepo, channelTypeRepo, deps.EncryptKey)
	smsTplSvc := notifyservice.NewSmsTemplateService(smsTplRepo, varRepo)
	varSvc := notifyservice.NewTemplateVarService(varRepo)
	testSendSvc := notifyservice.NewTestSendService(
		channelResolver, channelRepo, smsTplRepo, varRepo,
		deliveryRepo, nil, deps.Counter, deps.Logger,
	)
	broadcastSvc := notifyservice.NewBroadcastService(
		broadcastRepo, deliveryRepo, deps.NotifyRepo, smsTplRepo,
		deps.ConfigReader, deps.Logger,
	)
	deliverySvc := notifyservice.NewDeliveryService(deliveryRepo, deps.NotifyRepo)

	// 通知服务注入投递队列依赖：此后 Publish 的外发通道走队列而非直发。
	if deps.NotifySvc != nil {
		deps.NotifySvc.SetDeliveryDeps(deliveryRepo, smsTplRepo, recipientRepo)
	}

	// 启动时把注册表描述符回写 notification_channel_types（幂等 upsert），
	// 新增 provider 无需手写 seed SQL。
	go func() {
		defer safego.Recover(deps.Logger)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := channelSvc.SyncTypeRegistry(ctx); err != nil {
			deps.Logger.Warn("notify: sync channel type registry failed", zap.Error(err))
		}
	}()

	worker := notifyservice.NewDeliveryWorker(notifyservice.DeliveryWorkerDeps{
		Repo:         deliveryRepo,
		Resolver:     channelResolver,
		SwitchReader: deps.ConfigReader,
		Logger:       deps.Logger,
	})

	return &notifyBundle{
		channelHandler:     notifyhandler.NewChannelHandler(channelSvc, testSendSvc),
		smsTemplateHandler: notifyhandler.NewSmsTemplateHandler(smsTplSvc, varSvc),
		broadcastHandler:   notifyhandler.NewBroadcastHandler(broadcastSvc),
		deliveryHandler:    notifyhandler.NewDeliveryHandler(deliverySvc),
		resolver:           channelResolver,
		worker:             worker,
	}
}

// notifyRateCounter 适配 cache.Client 到测试发送频控计数端口。
type notifyRateCounter struct {
	cache *cache.Client
}

// Enabled 报告缓存是否可用；false 时测试发送频控降级放行。
func (c *notifyRateCounter) Enabled() bool { return c.cache != nil && c.cache.Enabled() }

func (c *notifyRateCounter) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	if c.cache == nil {
		return 0, errors.New("notify: cache unavailable")
	}
	return c.cache.Incr(ctx, key, ttl)
}

// buildNotify 构造通知中心服务、处理器、投递工作器与开放平台回调工作池（批次四 4b）。
// buildNotify 构造通知中心服务、处理器、投递工作器与开放平台回调工作池。
func (a *asm) buildNotify() {
	// 通知与消息中心（doc70）
	a.notifyRepo = notifyrepo.NewNotificationRepository(a.database)
	a.notifyTplRepo = notifyrepo.NewTemplateRepository(a.database)
	a.notifyPrefRepo = notifyrepo.NewPreferenceRepository(a.database)
	a.notifyAnnRepo = notifyrepo.NewAnnouncementRepository(a.database)
	a.notifyMailChannel = notifyservice.NewMailChannel(a.database, a.configRepo, a.notifyRepo, a.logger)
	a.notifySvc = notifyservice.NewNotificationService(a.database, a.notifyRepo, a.notifyTplRepo, a.notifyPrefRepo, a.notifyAnnRepo, a.notifyMailChannel, a.logger)
	a.announceSvc = notifyservice.NewAnnouncementService(a.notifyAnnRepo)
	a.templateSvc = notifyservice.NewTemplateService(a.notifyTplRepo)
	a.preferenceSvc = notifyservice.NewPreferenceService(a.notifyPrefRepo, a.notifyTplRepo)
	a.notifyAdminHandler = notifyhandler.NewAdminHandler(a.notifySvc, a.announceSvc, a.templateSvc, a.preferenceSvc)
	a.notifyUserHandler = notifyhandler.NewUserHandler(a.notifySvc, a.announceSvc, a.preferenceSvc)
	// 消息中心（doc90）：渠道/短信模板/群发/发送日志 + 投递队列工作器。
	// 该装配会把投递仓储注入 notifySvc，此后 Publish 的邮件/短信走队列而非直发。
	a.notifyBundleInst = buildNotifyBundle(notifyBundleDeps{
		DB:           a.database,
		EncryptKey:   a.cfg.App.EncryptKey,
		NotifySvc:    a.notifySvc,
		NotifyRepo:   a.notifyRepo,
		ConfigReader: a.configValueReader,
		Counter:      &notifyRateCounter{cache: a.cacheClient},
		Logger:       a.logger,
	})
	// 开放平台回调投递工作池（P6/T6.5）：指数退避，耗尽转死信。
	a.notifyWorker = openservice.NewNotifyDeliveryWorker(
		openrepo.NewNotifyRepository(a.database),
		openrepo.NewAppRepository(a.database),
		a.cfg.App.EncryptKey,
		a.logger,
		openservice.NotifyWorkerOptions{},
	)
}

func (b *lifecycleNotifierBridge) Publish(ctx context.Context, userID uint64, title, content string) error {
	return b.notifySvc.Publish(ctx, notifydto.PublishInput{
		Event:        notifymodel.EventInstanceExpiring,
		UserID:       userID,
		Target:       notifymodel.TargetUser,
		Vars:         map[string]string{"title": title, "content": content},
		SourceModule: "lifecycle",
		SourceID:     fmt.Sprintf("remind-%d", userID),
	})
}

// ticketNotifierBridge 桥接工单模块的 Notifier 接口到通知中心（P2-04）。
// target=user 时通知工单提交人，target=admin 时通知被指派员工。

// lifecycleNotifierBridge 桥接生命周期模块的 Notifier 接口到通知中心。
// 生命周期提醒（到期/续费结果）通过 Publish 发布到通知中心。
type lifecycleNotifierBridge struct {
	notifySvc notifyservice.NotificationService
	logger    *zap.Logger
}

func (b *ticketNotifierBridge) Publish(ctx context.Context, event, target string, recipientID uint64, vars map[string]string, sourceID string) error {
	if target == "" {
		target = notifymodel.TargetUser
	}
	err := b.notifySvc.Publish(ctx, notifydto.PublishInput{
		Event:        event,
		UserID:       recipientID,
		Target:       target,
		Vars:         vars,
		SourceModule: "ticket",
		SourceID:     sourceID,
	})
	if err != nil {
		b.logger.Warn("ticket: publish notification failed",
			zap.String("event", event), zap.Uint64("recipient_id", recipientID), zap.Error(err))
	}
	return err
}

// priceChangeNotifierBridge 桥接同步引擎的调价提醒到通知中心（T3.4）。
// 上游改价超阈值进入待确认队列时，向管理端发一条站内信（模板 event=sync_failed 之外
// 复用 system 事件，避免为一个提示新增模板）。

// ticketNotifierBridge 桥接工单模块的 Notifier 接口到通知中心（P2-04）。
// target=user 时通知工单提交人，target=admin 时通知被指派员工。
type ticketNotifierBridge struct {
	notifySvc notifyservice.NotificationService
	logger    *zap.Logger
}

func (b *priceChangeNotifierBridge) NotifyPriceChangePending(ctx context.Context, providerID uint64, providerName string, pending int) error {
	if pending <= 0 {
		return nil
	}
	err := b.notifySvc.Publish(ctx, notifydto.PublishInput{
		Event:  notifymodel.EventSystem,
		Target: notifymodel.TargetAdmin,
		Vars: map[string]string{
			"title":   "上游调价待确认",
			"content": fmt.Sprintf("提供商 %s（#%d）有 %d 条成本价变动超过阈值，售价未自动调整，请前往「同步状态运维控制台 → 待确认调价」处理。", providerName, providerID, pending),
		},
		SourceModule: "sync",
		SourceID:     fmt.Sprintf("price-pending-%d-%d", providerID, time.Now().Unix()/3600),
	})
	if err != nil {
		b.logger.Warn("publish price change notification failed",
			zap.Uint64("provider_id", providerID), zap.Error(err))
	}
	return err
}

// reconcileFindingNotifierBridge 桥接对账差异台账的预警到通知中心（doc120）。
//
// 事件按差异类型分流：售价低于成本、到期不同步、状态不同步各有专属模板，
// 管理员一眼能从标题判断是哪类问题、要不要马上看。
//
// 幂等键带 seq：notify 的唯一索引是 (source_module, source_id, channel, target_type, user_id)，
// 一条差异被 resolve 后复现必须换 source_id 才能再提醒一次，故把通知序号拼进去。
// 通知只发给管理员（TargetAdmin），不去打扰终端用户——定价与账期是我方自己的账。
type reconcileFindingNotifierBridge struct {
	notifySvc notifyservice.NotificationService
	logger    *zap.Logger
}

// reconcileEventOf 差异类型 → 通知事件。
func reconcileEventOf(kind string) string {
	switch kind {
	case reconmodel.KindPriceBelowCost, reconmodel.KindPriceThinMargin:
		return notifymodel.EventReconcileBelowCost
	case reconmodel.KindExpireEarly, reconmodel.KindExpireLate:
		return notifymodel.EventReconcileExpire
	default:
		return notifymodel.EventReconcileStatus
	}
}

func (b *reconcileFindingNotifierBridge) NotifyReconcileFinding(
	ctx context.Context, finding *reconmodel.ReconcileFinding, label reconcilerepo.InstanceLabel, seq int,
) error {
	if finding == nil {
		return nil
	}
	vars := map[string]string{
		"instance_mark":  firstNonEmptyText(label.Name, label.InstanceID, fmt.Sprintf("实例 #%d", finding.InstanceID)),
		"provider_name":  firstNonEmptyText(label.ProviderName, fmt.Sprintf("渠道 #%d", finding.ProviderID)),
		"local_value":    finding.LocalValue,
		"upstream_value": finding.UpstreamValue,
		"kind_name":      reconmodel.KindLabel(finding.Kind),
	}
	if finding.Delta != nil {
		vars["diff_days"] = fmt.Sprintf("%.1f", *finding.Delta)
		// 售价低于成本时模板里的 {loss} 是"每期亏损额"（毛利为负，取绝对值展示）。
		if finding.Kind == reconmodel.KindPriceBelowCost {
			loss := -*finding.Delta
			if loss < 0 {
				loss = *finding.Delta
			}
			vars["loss"] = fmt.Sprintf("￥%.2f", loss)
		}
	}
	err := b.notifySvc.Publish(ctx, notifydto.PublishInput{
		Event:        reconcileEventOf(finding.Kind),
		Target:       notifymodel.TargetAdmin,
		Vars:         vars,
		SourceModule: "reconcile",
		// 序号从 1 起：0 从未通知过；带 seq 保证"复现后重新提醒"能穿过去重索引。
		SourceID: fmt.Sprintf("reconcile-%d-%s-%d", finding.InstanceID, finding.Kind, seq),
	})
	if err != nil {
		b.logger.Warn("publish reconcile finding notification failed",
			zap.Uint64("instance_id", finding.InstanceID), zap.String("kind", finding.Kind), zap.Error(err))
	}
	return err
}

// firstNonEmptyText 取第一个非空字符串。
func firstNonEmptyText(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// buildUpstreamPushNotifier 桥接上游推送接收端到通知中心（doc120）：
// 上游把工单回复推过来并落库后，通知工单提交人。
//
// 为什么走 event=ticket_replied 而不是新事件：对用户而言这就是"工单有新回复"，
// 与客服在本站回复没有任何区别；用新事件会导致用户既有的通知偏好对上游回复不生效。
// 幂等键由推送服务给出（带 reply_id），保证同一条上游回复只通知一次。
func buildUpstreamPushNotifier(notifySvc notifyservice.NotificationService, logger *zap.Logger) pushservice.Notifier {
	return &upstreamPushNotifier{notifySvc: notifySvc, logger: logger}
}

type upstreamPushNotifier struct {
	notifySvc notifyservice.NotificationService
	logger    *zap.Logger
}

func (b *upstreamPushNotifier) Publish(
	ctx context.Context, event, target string, recipientID uint64, vars map[string]string, sourceID string,
) error {
	if target == "" {
		target = notifymodel.TargetUser
	}
	err := b.notifySvc.Publish(ctx, notifydto.PublishInput{
		Event:        event,
		UserID:       recipientID,
		Target:       target,
		Vars:         vars,
		SourceModule: "upstream_push",
		SourceID:     sourceID,
	})
	if err != nil {
		b.logger.Warn("upstream push: publish notification failed",
			zap.String("event", event), zap.Uint64("recipient_id", recipientID), zap.Error(err))
	}
	return err
}

// buildProductSourceModeResolver 返回商品链路判据解析（T5.1 任务留痕用）：
// 读不到商品时返回空串，不影响开通投递。

// priceChangeNotifierBridge 桥接同步引擎的调价提醒到通知中心（T3.4）。
// 上游改价超阈值进入待确认队列时，向管理端发一条站内信（模板 event=sync_failed 之外
// 复用 system 事件，避免为一个提示新增模板）。
type priceChangeNotifierBridge struct {
	notifySvc notifyservice.NotificationService
	logger    *zap.Logger
}

// Notify 发布"开通失败转人工"管理端站内信；发布失败只记日志，不影响工作池。
func (b *provisionManualNotifier) Notify(ctx context.Context, task *ordermodel.ProvisionTask) {
	if task == nil {
		return
	}
	err := b.notifySvc.Publish(ctx, notifydto.PublishInput{
		Event:  notifymodel.EventSystem,
		Target: notifymodel.TargetAdmin,
		Vars: map[string]string{
			"title": "订单开通失败待人工处理",
			"content": fmt.Sprintf("订单 #%d（用户 #%d）连续 %d 次开通失败，已转入人工队列：%s。请前往「订单管理」重试或联系上游排查。",
				task.OrderID, task.UserID, task.Attempts, task.LastError),
		},
		SourceModule: "order",
		SourceID:     fmt.Sprintf("provision-manual-%d", task.OrderID),
	})
	if err != nil {
		b.logger.Warn("publish provision manual notification failed",
			zap.Uint64("order_id", task.OrderID), zap.Error(err))
	}
}

// buildStageCapabilityNotifier 阶段推进遇上游缺能力时的显式告警（T5.4/T5.5）。
//
// 语义：平台确实不支持暂停/销毁时，推进器只落阶段并告警，必须让运营可见，
// 避免"看起来已暂停"的静默假完成。告警复用 system 事件，不新增模板。

// provisionManualNotifier 桥接开通任务连续失败告警到通知中心（T5.1）。
// 复用 system 事件避免为一个提示新增模板（与调价待确认一致）。
type provisionManualNotifier struct {
	notifySvc notifyservice.NotificationService
	logger    *zap.Logger
}

// Notify 发布"开通失败转人工"管理端站内信；发布失败只记日志，不影响工作池。
