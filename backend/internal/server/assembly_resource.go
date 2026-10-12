// 资源域装配：上游渠道/商品/同步引擎（批次四 4b）。

package server

import (
	"context"
	producthandler "hostsent/backend/internal/modules/admin/resource/product/handler"
	productrepo "hostsent/backend/internal/modules/admin/resource/product/repository"
	productservice "hostsent/backend/internal/modules/admin/resource/product/service"
	providerhandler "hostsent/backend/internal/modules/admin/resource/provider/handler"
	providerrepo "hostsent/backend/internal/modules/admin/resource/provider/repository"
	providerservice "hostsent/backend/internal/modules/admin/resource/provider/service"
	reconcilehandler "hostsent/backend/internal/modules/admin/resource/reconcile/handler"
	reconcilerepo "hostsent/backend/internal/modules/admin/resource/reconcile/repository"
	reconcileservice "hostsent/backend/internal/modules/admin/resource/reconcile/service"
	synchandler "hostsent/backend/internal/modules/admin/resource/sync/handler"
	syncrepo "hostsent/backend/internal/modules/admin/resource/sync/repository"
	syncservice "hostsent/backend/internal/modules/admin/resource/sync/service"
	taskqueuehandler "hostsent/backend/internal/modules/admin/resource/taskqueue/handler"
	taskqueuerepo "hostsent/backend/internal/modules/admin/resource/taskqueue/repository"
	taskqueueservice "hostsent/backend/internal/modules/admin/resource/taskqueue/service"
	"hostsent/backend/internal/pkg/upstream"

	"go.uber.org/zap"
)

// buildResource 构造上游渠道服务（含类型注册表与凭证归一副作用）、同步引擎与处理器。
func (a *asm) buildResource() {
	a.upstreamMgr = upstream.GetProviderManager()
	a.providerRepo = providerrepo.NewProviderRepository(a.database)
	a.poolRepo = providerrepo.NewPoolRepository(a.database)
	a.providerTypeRepo = providerrepo.NewProviderTypeRepository(a.database)
	a.productRepo = productrepo.NewProductRepository(a.database)
	a.syncRepo = syncrepo.NewSyncRepository(a.database)
	a.syncFwRepo = syncrepo.NewFrameworkRepository(a.database)
	a.providerService = providerservice.NewProviderService(a.providerRepo, a.poolRepo, a.providerTypeRepo, a.upstreamMgr, a.cfg.App.EncryptKey)
	// 渠道类型注册表（T2.2）：把已注册适配器的能力描述符回写 provider_types，
	// 使后台类型列表/动态表单以数据库为权威来源（地雷 L9 从硬编码 map 改为查表）。
	if err := a.providerService.SyncTypeRegistry(context.Background()); err != nil {
		a.logger.Warn("同步渠道类型注册表失败", zap.Error(err))
	}
	// 凭证归一（T2.3）：把迁移搬入 credentials 的旧列密文改为带 enc: 前缀的统一密文，
	// 使"解密失败必须报错"（L8）能可靠区分历史明文与密文。
	if err := a.providerService.NormalizeCredentials(context.Background()); err != nil {
		a.logger.Warn("归一渠道凭证失败", zap.Error(err))
	}
	a.productService = productservice.NewProductService(a.productRepo)
	a.syncEngine = syncservice.NewSyncEngine(a.upstreamMgr, a.providerService, a.productRepo, a.poolRepo, a.providerRepo, a.syncRepo, a.syncFwRepo, a.logger)
	a.syncService = syncservice.NewSyncService(a.syncRepo, a.syncEngine)
	a.scheduler = syncservice.NewScheduler(a.syncEngine, a.syncRepo, a.syncFwRepo, a.logger)
	a.syncFrameworkService = syncservice.NewFrameworkService(a.syncFwRepo, a.syncEngine, a.logger)
	a.providerHandler = providerhandler.NewProviderHandler(a.providerService)
	a.productHandler = producthandler.NewProductHandler(a.productService)
	a.syncHandler = synchandler.NewSyncHandler(a.syncService)
	a.syncFrameworkHandler = synchandler.NewFrameworkHandler(a.syncFrameworkService)
	// 任务队列（本轮 S3）：跨表只读聚合，仅依赖数据库。
	a.taskQueueHandler = taskqueuehandler.NewTaskQueueHandler(taskqueueservice.NewTaskQueueService(taskqueuerepo.NewRepository(a.database)))
	// 实例对账（本轮 S4）：instances 为事实表，只读比对，仅依赖数据库。
	a.reconcileHandler = reconcilehandler.NewReconcileHandler(reconcileservice.NewReconcileService(reconcilerepo.NewRepository(a.database)))
}
