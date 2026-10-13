// 工单域装配（批次四 4b）。

package server

import (
	tickethandler "hostsent/backend/internal/modules/admin/ticket/handler"
	ticketrepo "hostsent/backend/internal/modules/admin/ticket/repository"
	ticketservice "hostsent/backend/internal/modules/admin/ticket/service"
	pushhandler "hostsent/backend/internal/modules/upstreampush/handler"
	pushrepo "hostsent/backend/internal/modules/upstreampush/repository"
	pushservice "hostsent/backend/internal/modules/upstreampush/service"
	"hostsent/backend/internal/pkg/storage"

	"go.uber.org/zap"
)

// buildTicket 构造工单服务/附件存储/分类与两端处理器。
func (a *asm) buildTicket() {
	// 工单支持域
	a.ticketRepo = ticketrepo.NewTicketRepository(a.database)
	a.ticketReplyRepo = ticketrepo.NewReplyRepository(a.database)
	a.ticketCategoryRepo = ticketrepo.NewCategoryRepository(a.database)
	a.ticketAttachmentRepo = ticketrepo.NewAttachmentRepository(a.database)
	a.ticketPreconditionRepo = ticketrepo.NewPreconditionRepository(a.database)
	a.ticketService = ticketservice.NewTicketService(a.ticketRepo, a.ticketReplyRepo, a.ticketCategoryRepo, nil)
	a.ticketService.SetPreconditionChecker(a.ticketPreconditionRepo)
	// 附件本地存储：根目录来自配置（容器内挂卷）；初始化失败不阻断启动，附件接口会明确报错。
	var storeErr error
	a.attachmentStore, storeErr = storage.NewLocalStore(a.cfg.Storage.Root)
	if storeErr != nil {
		a.logger.Warn("工单附件存储初始化失败，附件功能不可用", zap.String("root", a.cfg.Storage.Root), zap.Error(storeErr))
		a.attachmentStore = nil
	}
	a.ticketAttachmentService = ticketservice.NewAttachmentService(a.ticketAttachmentRepo, a.ticketRepo, a.attachmentStore)
	a.ticketService.SetAttachmentService(a.ticketAttachmentService)
	a.ticketCategoryService = ticketservice.NewCategoryService(a.ticketCategoryRepo, a.ticketRepo, a.ticketPreconditionRepo)
	a.ticketHandler = tickethandler.NewTicketHandler(a.ticketService, a.ticketAttachmentService, a.ticketCategoryService)
	a.ticketCategoryHandler = tickethandler.NewCategoryHandler(a.ticketCategoryService)
	a.userTicketHandler = tickethandler.NewUserTicketHandler(a.ticketService, a.ticketCategoryService, a.ticketAttachmentService)
	// 上游推送接收端（doc120）：/api/host/sync 与 /api/ticket_reply/sync。
	// 免登录，凭 body 内的 signature + orders.downstream_token 验签；
	// 不依赖工单服务，直接用窄仓储（推送只做"按上游单号反查 + 落一条回复"）。
	// notifier 在 buildBridges 里后置注入（通知服务晚于本域装配）。
	a.upstreamPushService = pushservice.NewService(pushrepo.NewRepository(a.database), nil, a.logger)
	a.upstreamPushHandler = pushhandler.NewHandler(a.upstreamPushService, a.logger)
}
