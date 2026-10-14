// 管理端路由——Order域（自 routes_admin.go 拆分，批次三 C2）。

package server

import (
	"github.com/gin-gonic/gin"
)

func registerAdminOrderRoutes(v1 *gin.RouterGroup, app *App) {
	orderGroup := v1.Group("/orders")
	orderGroup.Use(app.adminAuth())
	{
		orderGroup.GET("", app.perm("order:list"), app.orderHandler.List)
		orderGroup.GET("/stats", app.perm("order:stats"), app.orderHandler.Stats)
		// 退款策略（时限/免审按商品来源+分类配置）
		orderGroup.GET("/refund-policies", app.perm("order:refund:policy"), app.refundPolicyHandler.List)
		orderGroup.POST("/refund-policies", app.perm("order:refund:policy"), app.refundPolicyHandler.Create)
		orderGroup.PUT("/refund-policies/:id", app.perm("order:refund:policy"), app.refundPolicyHandler.Update)
		orderGroup.DELETE("/refund-policies/:id", app.perm("order:refund:policy"), app.refundPolicyHandler.Delete)
		// 批量动作静态段必须先于 /:id 注册，否则被通配路径吞掉（同 ticket /reviews 先例）。
		orderGroup.POST("/batch-cancel", app.perm("order:cancel"), app.orderHandler.BatchCancel)
		orderGroup.POST("/batch-activate", app.perm("order:activate"), app.orderHandler.BatchActivate)
		orderGroup.POST("/batch-verify", app.perm("order:verify"), app.orderHandler.BatchVerify)
		orderGroup.POST("/batch-delete", app.perm("order:delete"), app.orderHandler.BatchDelete)
		orderGroup.GET("/:id", app.perm("order:list"), app.orderHandler.Get)
		orderGroup.POST("/:id/cancel", app.perm("order:cancel"), app.orderHandler.Cancel)
		orderGroup.PUT("/:id/remark", app.perm("order:remark"), app.orderHandler.UpdateRemark)
		orderGroup.POST("/:id/refund", app.perm("order:refund"), app.orderHandler.CreateRefund)
		orderGroup.POST("/:id/activate", app.perm("order:activate"), app.orderHandler.Activate)
		orderGroup.POST("/:id/verify", app.perm("order:verify"), app.orderHandler.Verify)
	}
	ticketGroup := v1.Group("/tickets")
	ticketGroup.Use(app.adminAuth())
	{
		ticketGroup.GET("", app.perm("ticket:list"), app.ticketHandler.List)
		// 复核队列必须先于 /:id 注册，否则被通配路径吞掉。
		ticketGroup.GET("/reviews", app.perm("ticket:review"), app.ticketHandler.ListReviews)
		ticketGroup.GET("/stats", app.perm("ticket:stats"), app.ticketHandler.Stats)
		ticketGroup.GET("/attachments/:id/download", app.perm("ticket:view"), app.ticketHandler.DownloadAttachment)
		ticketGroup.POST("/replies/:replyId/review", app.perm("ticket:review"), app.ticketHandler.ReviewReply)
		ticketGroup.GET("/:id", app.perm("ticket:view"), app.ticketHandler.Get)
		ticketGroup.POST("/:id/reply", app.perm("ticket:reply"), app.ticketHandler.Reply)
		ticketGroup.POST("/:id/attachments", app.perm("ticket:reply"), app.ticketHandler.UploadAttachment)
		ticketGroup.PUT("/:id/assign", app.perm("ticket:assign"), app.ticketHandler.Assign)
		// 认领开放给有工单列表权限的员工（未分配池自助认领）
		ticketGroup.POST("/:id/claim", app.perm("ticket:list"), app.ticketHandler.Claim)
		ticketGroup.PUT("/:id/transfer", app.perm("ticket:assign"), app.ticketHandler.Transfer)
		ticketGroup.PUT("/:id/status", app.perm("ticket:update"), app.ticketHandler.UpdateStatus)
		ticketGroup.POST("/:id/close", app.perm("ticket:close"), app.ticketHandler.Close)
	}
	ticketCategoryGroup := v1.Group("/ticket-categories")
	ticketCategoryGroup.Use(app.adminAuth())
	{
		ticketCategoryGroup.GET("", app.perm("ticket:category"), app.ticketCategoryHandler.List)
		ticketCategoryGroup.POST("", app.perm("ticket:manage"), app.ticketCategoryHandler.Create)
		ticketCategoryGroup.PUT("/:id", app.perm("ticket:manage"), app.ticketCategoryHandler.Update)
		ticketCategoryGroup.DELETE("/:id", app.perm("ticket:manage"), app.ticketCategoryHandler.Delete)
	}
	refundGroup := v1.Group("/refunds")
	refundGroup.Use(app.adminAuth())
	{
		refundGroup.GET("", app.perm("order:refunds"), app.refundHandler.List)
		// 批量审核静态段先于 /:id 注册。
		refundGroup.POST("/batch-approve", app.perm("order:refund:audit"), app.refundHandler.BatchApprove)
		refundGroup.POST("/batch-reject", app.perm("order:refund:audit"), app.refundHandler.BatchReject)
		// 凭据附件下载静态段先于 /:id 注册。
		refundGroup.GET("/attachments/:id/download", app.perm("order:refunds"), app.refundHandler.DownloadAttachment)
		refundGroup.GET("/:id", app.perm("order:refunds"), app.refundHandler.Get)
		refundGroup.POST("/:id/approve", app.perm("order:refund:audit"), app.refundHandler.Approve)
		refundGroup.POST("/:id/reject", app.perm("order:refund:audit"), app.refundHandler.Reject)
		// 事后复核（免审核自动退款后的最后一道人工确认）
		refundGroup.POST("/:id/review", app.perm("order:refund:audit"), app.refundHandler.Review)
		// 审核凭据附件上传
		refundGroup.POST("/:id/attachments", app.perm("order:refund:audit"), app.refundHandler.UploadAttachment)
	}
}
