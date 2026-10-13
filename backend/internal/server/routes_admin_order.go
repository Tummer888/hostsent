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
		orderGroup.GET("/:id", app.perm("order:list"), app.orderHandler.Get)
		orderGroup.POST("/:id/cancel", app.perm("order:cancel"), app.orderHandler.Cancel)
		orderGroup.PUT("/:id/remark", app.perm("order:remark"), app.orderHandler.UpdateRemark)
		orderGroup.POST("/:id/refund", app.perm("order:refund"), app.orderHandler.CreateRefund)
		orderGroup.POST("/:id/activate", app.perm("order:activate"), app.orderHandler.Activate)
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
		refundGroup.GET("/:id", app.perm("order:refunds"), app.refundHandler.Get)
		refundGroup.POST("/:id/approve", app.perm("order:refund:audit"), app.refundHandler.Approve)
		refundGroup.POST("/:id/reject", app.perm("order:refund:audit"), app.refundHandler.Reject)
	}
}
