// 管理端路由——Log域（自 routes_admin.go 拆分，批次三 C2）。

package server

import (
	"github.com/gin-gonic/gin"
)

func registerAdminLogRoutes(v1 *gin.RouterGroup, app *App) {
	if app.logcenter != nil {
		logs := v1.Group("/logs")
		logs.Use(app.adminAuth())
		{
			// 目录 / 统计 / 查询（固定段）
			logs.GET("/catalog", app.perm("log:center"), app.logcenter.handler.Catalog)
			logs.GET("/stats", app.perm("log:center"), app.logcenter.handler.Stats)
			logs.GET("/query", app.perm("log:center"), app.logcenter.handler.Query)

			// 导出（导出即留痕；下载同权限，写操作由全局审计中间件记录）
			logs.POST("/export", app.perm("log:export"), app.logcenter.handler.Export)
			logs.GET("/export-files", app.perm("log:center"), app.logcenter.handler.ListExportFiles)
			logs.GET("/export-files/:id/download", app.perm("log:export"), app.logcenter.handler.DownloadExportFile)
			logs.DELETE("/export-files/:id", app.perm("log:cleanup"), app.logcenter.handler.DeleteExportFile)

			// 保留策略
			logs.GET("/policies", app.perm("log:policy"), app.logcenter.handler.ListPolicies)
			logs.PUT("/policies/:source", app.perm("log:cleanup"), app.logcenter.handler.UpdatePolicy)

			// 清理：预演与执行分离，执行需 DELETE 确认文本
			logs.POST("/cleanup/preview", app.perm("log:cleanup"), app.logcenter.handler.PreviewCleanup)
			logs.POST("/cleanup", app.perm("log:cleanup"), app.logcenter.handler.RunCleanup)
			logs.GET("/cleanup-jobs", app.perm("log:center"), app.logcenter.handler.ListCleanupJobs)

			// 通配段收尾注册
			logs.GET("/cleanup-jobs/:id", app.perm("log:center"), app.logcenter.handler.GetCleanupJob)
			logs.POST("/cleanup-jobs/:id/cancel", app.perm("log:cleanup"), app.logcenter.handler.CancelCleanupJob)
			logs.GET("/:source/detail/:id", app.perm("log:center"), app.logcenter.handler.Detail)
		}
	}
}
