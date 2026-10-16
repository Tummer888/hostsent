// 管理端路由——Resource域（自 routes_admin.go 拆分，批次三 C2）。

package server

import (
	"github.com/gin-gonic/gin"
)

func registerAdminResourceRoutes(v1 *gin.RouterGroup, app *App) {
	resourceGroup := v1.Group("/resource")
	resourceGroup.Use(app.adminAuth())
	{
		// 上游提供商
		providers := resourceGroup.Group("/providers")
		{
			providers.GET("", app.perm("resource:provider"), app.providerHandler.List)
			providers.POST("", app.perm("provider:create"), app.providerHandler.Create)
			providers.GET("/types", app.perm("resource:provider"), app.providerHandler.ListTypes)
			providers.GET("/:id", app.perm("resource:provider"), app.providerHandler.Get)
			providers.PUT("/:id", app.perm("provider:update"), app.providerHandler.Update)
			providers.DELETE("/:id", app.perm("provider:delete"), app.providerHandler.Delete)
			// 关联数据详情：删除前弹窗列出该渠道下的商品与资源池，让运营看清删的是什么。
			providers.GET("/:id/associations", app.perm("provider:delete"), app.providerHandler.Associations)
			// 连带删除（渠道 + 未被销售商品引用的商品 + 资源池）；有引用时拒绝并说明是哪些。
			providers.DELETE("/:id/cascade", app.perm("provider:delete"), app.providerHandler.DeleteCascade)
			providers.POST("/:id/test", app.perm("provider:test"), app.providerHandler.TestConnection)
			providers.POST("/:id/sync/resume", app.perm("provider:update"), app.providerHandler.ResumeSync)
			// 同步总开关：只动 sync_enabled 一个字段（通用 PUT 是整行替换，不能用来单切开关）。
			providers.PUT("/:id/sync/enabled", app.perm("provider:update"), app.providerHandler.SetSyncEnabled)
			// 平台可售资源目录（区域/节点/存储/镜像）：自营规格模板配置平台参数用。
			providers.GET("/:id/resources", app.perm("resource:provider"), app.providerHandler.PlatformResources)
		}

		// 资源池
		pools := resourceGroup.Group("/pools")
		{
			pools.GET("", app.perm("resource:provider"), app.providerHandler.ListPools)
			pools.GET("/:id", app.perm("resource:provider"), app.providerHandler.GetPool)
			// 删除用于清理演示/历史脏数据；在线渠道的池会在下次同步重新出现，前端已提示。
			pools.DELETE("/:id", app.perm("resource:provider"), app.providerHandler.DeletePool)
		}

		// 上游商品
		products := resourceGroup.Group("/products")
		{
			products.GET("", app.perm("resource:product"), app.productHandler.List)
			products.GET("/:id", app.perm("resource:product"), app.productHandler.Get)
			products.PUT("/:id/price", app.perm("product:update_price"), app.productHandler.UpdatePrice)
			products.POST("/sync", app.perm("product:sync"), app.productHandler.Sync)
		}

		// 同步管理
		sync := resourceGroup.Group("/sync")
		{
			sync.POST("", app.perm("sync:create"), app.syncHandler.CreateTask)
			sync.GET("/tasks", app.perm("resource:sync"), app.syncHandler.ListTasks)
			sync.GET("/tasks/:id", app.perm("resource:sync"), app.syncHandler.GetTask)
			sync.GET("/logs", app.perm("sync:log"), app.syncHandler.ListLogs)
			// ---- P3 同步框架（T3.2/T3.4/T3.5）----
			sync.GET("/scopes", app.perm("resource:sync"), app.syncFrameworkHandler.ScopeMeta)
			sync.GET("/schedules", app.perm("resource:sync"), app.syncFrameworkHandler.ListSchedules)
			sync.PUT("/schedules/:id", app.perm("sync:schedule"), app.syncFrameworkHandler.UpdateSchedule)
			sync.GET("/price-changes", app.perm("sync:price"), app.syncFrameworkHandler.ListPriceChanges)
			sync.POST("/price-changes/handle", app.perm("sync:price:confirm"), app.syncFrameworkHandler.HandlePriceChanges)
			sync.GET("/diffs", app.perm("resource:sync"), app.syncFrameworkHandler.ListDiffs)
			sync.GET("/diffs/summary", app.perm("resource:sync"), app.syncFrameworkHandler.DiffSummary)
		}

		// 实例管理
		instances := resourceGroup.Group("/instances")
		{
			instances.GET("", app.perm("resource:instance"), app.syncHandler.ListInstances)
			instances.GET("/:id", app.perm("resource:instance"), app.syncHandler.GetInstance)
		}

		// 任务队列（本轮 S3）：开通履约 / 实例动作 / 续费 / 上游同步四类平台动作的「是否到达上游」。
		// 只读聚合视图，无写接口；/categories 静态段先于其它段注册。
		taskQueue := resourceGroup.Group("/task-queue")
		{
			taskQueue.GET("", app.perm("resource:sync"), app.taskQueueHandler.List)
			taskQueue.GET("/categories", app.perm("resource:sync"), app.taskQueueHandler.Categories)
		}

		// 实例对账（本轮 S4）：已开通的上游链路实例，本地售价/到期 vs 上游成本/到期。
		// 只读比对视图，修正动作仍在商品/实例详情页完成。
		resourceGroup.GET("/reconcile", app.perm("resource:instance"), app.reconcileHandler.List)
		// 对账差异台账（doc120）：把上面的比对结论持久化，供预警与持续时长追踪。
		// /findings 固定段先于可能的通配段注册。
		reconcileGroup := resourceGroup.Group("/reconcile/findings")
		{
			reconcileGroup.GET("", app.perm("resource:instance"), app.reconcileFindingHandler.ListFindings)
			reconcileGroup.PUT("/:id", app.perm("resource:instance"), app.reconcileFindingHandler.UpdateFinding)
		}
	}
	lcInstances := v1.Group("/instances")
	lcInstances.Use(app.adminAuth())
	{
		lcInstances.GET("/expiring", app.perm("lifecycle:expiring"), app.expiringHandler.List)
		lcInstances.POST("/:id/renew", app.perm("lifecycle:renew"), app.lifecycleAdminHandler.RenewAdmin)
	}
	instanceOps := v1.Group("/instances")
	instanceOps.Use(app.adminAuth())
	{
		instanceOps.GET("", app.perm("resource:instance"), app.instanceOpsHandler.List)
		instanceOps.GET("/stats", app.perm("resource:instance"), app.instanceOpsHandler.Stats)
		// 产品线导航（类型清单 + 各自实例数）：静态段须先于 /:id。
		instanceOps.GET("/types", app.perm("resource:instance"), app.instanceOpsHandler.InstanceTypes)
		// 全局操作流水（跨实例审计视图）：静态段须先于 /:id。
		instanceOps.GET("/operations", app.perm("resource:instance"), app.instanceOpsHandler.OperationLogs)
		instanceOps.GET("/:id", app.perm("resource:instance"), app.instanceOpsHandler.Detail)
		instanceOps.GET("/:id/operations", app.perm("resource:instance"), app.instanceOpsHandler.Operations)
		instanceOps.GET("/:id/related", app.perm("resource:instance"), app.instanceOpsHandler.Related)
		instanceOps.POST("/:id/sync", app.perm("instance:action"), app.instanceOpsHandler.Sync)
		// 重新开通（待开通占位实例专用）：重投来源订单的履约任务，
		// 上游充值后点一次即可复用已生成账单完成开通，不会重复下单。
		instanceOps.POST("/:id/provision-retry", app.perm("instance:action"), app.instanceOpsHandler.RetryProvision)
		instanceOps.POST("/:id/power", app.perm("instance:action"), app.instanceOpsHandler.Power)
		// 批量运维（doc61 P1）：静态段 /batch 须先于 /:id 注册。
		instanceOps.POST("/batch", app.perm("instance:action"), app.instanceOpsHandler.BatchAction)
		// 批量销毁：独立权限码 + 二次验证（与单实例销毁同场景），
		// 且服务层要求逐台确认文本 —— "勾了全选"不能变成"一键删机器"。
		instanceOps.POST("/batch-destroy", app.perm("instance:destroy"), app.adminRequireVerification("instance_destroy"), app.instanceOpsHandler.BatchDestroy)
		// 批量删除记录（物理删除已销毁的行）：与单实例删除同一权限与二次验证口径，
		// 服务层逐台确认 + 只接受终态行 —— 批量不能变成"绕过销毁的一次性抹除"。
		instanceOps.POST("/batch-purge", app.perm("instance:destroy"), app.adminRequireVerification("instance_destroy"), app.instanceOpsHandler.BatchPurge)
		instanceOps.POST("/:id/suspend", app.perm("instance:action"), app.instanceOpsHandler.Suspend)
		instanceOps.POST("/:id/unsuspend", app.perm("instance:action"), app.instanceOpsHandler.Unsuspend)
		// 管理员修改服务状态（服务中/已暂停/已取消）：面向客户可见的状态变更，
		// 且"违规暂停"会直接影响客户，因此复用 instance:action 权限；
		// 服务层按目标状态分派到既有的上游动作路径（流水/事件齐全）。
		instanceOps.POST("/:id/state", app.perm("instance:action"), app.instanceOpsHandler.SetServiceState)
		instanceOps.PUT("/:id/remark", app.perm("instance:action"), app.instanceOpsHandler.SetRemark)
		instanceOps.POST("/:id/vnc", app.perm("instance:console"), app.instanceOpsHandler.VNC)
		instanceOps.POST("/:id/resize", app.perm("instance:resize"), app.adminRequireVerification("instance_resize"), app.instanceOpsHandler.Resize)
		// 维护类操作（平台接口实测存在）：各拆独立权限码，避免"能重启就能重装"。
		// 会丢失数据的动作（重装格式化 / 快照恢复）另加二次验证。
		instanceOps.POST("/:id/reinstall", app.perm("instance:reinstall"), app.adminRequireVerification("instance_reinstall"), app.instanceOpsHandler.Reinstall)
		instanceOps.POST("/:id/reset-password", app.perm("instance:password"), app.instanceOpsHandler.ResetPassword)
		instanceOps.POST("/:id/rescue", app.perm("instance:rescue"), app.instanceOpsHandler.Rescue)
		instanceOps.POST("/:id/exit-rescue", app.perm("instance:rescue"), app.instanceOpsHandler.ExitRescue)
		instanceOps.GET("/:id/snapshots", app.perm("instance:snapshot"), app.instanceOpsHandler.Snapshots)
		instanceOps.POST("/:id/snapshots", app.perm("instance:snapshot"), app.instanceOpsHandler.CreateSnapshot)
		instanceOps.DELETE("/:id/snapshots/:snapshotId", app.perm("instance:snapshot"), app.instanceOpsHandler.DeleteSnapshot)
		instanceOps.POST("/:id/snapshots/restore", app.perm("instance:snapshot"), app.adminRequireVerification("instance_snapshot_restore"), app.instanceOpsHandler.RestoreSnapshot)
		instanceOps.PUT("/:id/bandwidth", app.perm("instance:hardware"), app.instanceOpsHandler.SetBandwidth)
		instanceOps.POST("/:id/ips", app.perm("instance:hardware"), app.instanceOpsHandler.AddIP)
		instanceOps.POST("/:id/disks", app.perm("instance:hardware"), app.instanceOpsHandler.AttachDataDisk)
		// 账期自定义（本地口径，不推送上游——上游无改到期时间/周期接口）。
		// 高危：改动直接影响续费/到期处置，需二次验证（复用 instance_resize 场景以外的独立场景码）。
		instanceOps.PUT("/:id/billing", app.perm("instance:billing"), app.adminRequireVerification("instance_billing"), app.instanceOpsHandler.AdjustBilling)
		// 账期调整上下文：可选周期以上游能力为准（避免选中上游不卖的周期）+ 各周期上游价/我方价。
		instanceOps.GET("/:id/billing-context", app.perm("instance:billing"), app.instanceOpsHandler.AdjustContext)
		// 重装可选镜像：自营取渠道镜像目录、对接取上游商品的 os 配置子项
		//（此前前端只读渠道镜像目录，对接链路必然为空 —— 重装无法选镜像）。
		instanceOps.GET("/:id/reinstall-images", app.perm("instance:reinstall"), app.instanceOpsHandler.ReinstallImages)
		// 按上游实例号重新关联（上游换机/换号）：改上游号 = 改变后续所有上游动作的作用对象，
		// 因此需独立权限码 + 二次验证，与销毁同档。
		instanceOps.POST("/:id/relink", app.perm("instance:relink"), app.adminRequireVerification("instance_relink"), app.instanceOpsHandler.RelinkUpstream)
		// 更换绑定的本地商品 / 上游镜像（上游换产品）：直接决定续费算价与配置口径，与账期同档。
		instanceOps.POST("/:id/rebind-product", app.perm("instance:billing"), app.instanceOpsHandler.RebindProduct)
		// 更换候选（选框式 UI）：本地商品全量 + 同渠道上游镜像，并给出上下架/渠道提示。
		instanceOps.GET("/:id/rebind-candidates", app.perm("instance:billing"), app.instanceOpsHandler.RebindCandidates)
		// 查看明文密码：只读但属敏感信息访问，独立权限码便于收窄。
		instanceOps.POST("/:id/credential", app.perm("instance:credential"), app.instanceOpsHandler.RevealCredential)
		instanceOps.DELETE("/:id", app.perm("instance:destroy"), app.adminRequireVerification("instance_destroy"), app.instanceOpsHandler.Destroy)
		// 删除实例记录（物理删除已销毁的行）：比销毁更彻底，因此拆成独立接口 + 二次验证。
		// 只接受终态行（服务层守卫）：在跑的机器必须先走销毁，否则会绕过"通知渠道终止机器"。
		instanceOps.POST("/:id/purge", app.perm("instance:destroy"), app.adminRequireVerification("instance_destroy"), app.instanceOpsHandler.Purge)
	}
	lcRenewals := v1.Group("/renewals")
	lcRenewals.Use(app.adminAuth())
	{
		lcRenewals.GET("", app.perm("lifecycle:renewals"), app.lifecycleAdminHandler.ListRenewals)
		lcRenewals.GET("/:id", app.perm("lifecycle:renewals"), app.lifecycleAdminHandler.GetRenewal)
	}
	lifecycleGroup := v1.Group("/lifecycle")
	lifecycleGroup.Use(app.adminAuth())
	{
		lifecycleGroup.GET("/policy", app.perm("lifecycle:policy"), app.lifecycleAdminHandler.GetPolicy)
		lifecycleGroup.PUT("/policy", app.perm("lifecycle:policy:update"), app.lifecycleAdminHandler.UpdatePolicy)
		lifecycleGroup.POST("/scan", app.perm("lifecycle:policy:update"), app.lifecycleAdminHandler.ScanOnce)
		// 到期处置（doc61 §8.4）：预演只读；手动单实例执行属高危写，需二次验证。
		// 复用既有 instance_destroy 场景（处置可能含销毁），不新增验证场景码。
		lifecycleGroup.GET("/enforcement/preview", app.perm("lifecycle:enforce"), app.lifecycleAdminHandler.PreviewEnforcement)
		lifecycleGroup.POST("/enforcement/:id/run", app.perm("lifecycle:enforce"), app.adminRequireVerification("instance_destroy"), app.lifecycleAdminHandler.EnforceInstance)
	}
}
