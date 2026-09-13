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
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	notifyhandler "hostsent/backend/internal/modules/admin/notification/handler"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
	notifyservice "hostsent/backend/internal/modules/admin/notification/service"
	"hostsent/backend/internal/pkg/cache"
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
