// 管理端路由——System域（自 routes_admin.go 拆分，批次三 C2）。

package server

import (
	"github.com/gin-gonic/gin"
)

func registerAdminSystemRoutes(v1 *gin.RouterGroup, app *App) {
	systemConfigGroup := v1.Group("/system/configs")
	systemConfigGroup.Use(app.adminAuth())
	{
		systemConfigGroup.GET("", app.perm("system:config:view"), app.configHandler.List)
		systemConfigGroup.POST("", app.perm("system:config:create"), app.configHandler.Create)
		// 按分组读取（固定段 /group/:group 需先于 /:key 匹配，且二者不同段数不冲突）
		systemConfigGroup.GET("/group/:group", app.perm("system:config:view"), app.configHandler.ListByGroup)
		// 分组批量 upsert
		systemConfigGroup.POST("/batch", app.perm("system:config:update"), app.configHandler.BatchUpsert)
		systemConfigGroup.GET("/:key", app.perm("system:config:view"), app.configHandler.GetByKey)
		systemConfigGroup.PUT("/:id", app.perm("system:config:update"), app.configHandler.Update)
		systemConfigGroup.DELETE("/:id", app.perm("system:config:delete"), app.configHandler.Delete)
	}
	annGroup := v1.Group("/announcements")
	annGroup.Use(app.adminAuth())
	{
		annGroup.GET("", app.perm("notify:announcement"), app.notifyAdminHandler.ListAnnouncements)
		annGroup.POST("", app.perm("notify:manage"), app.notifyAdminHandler.CreateAnnouncement)
		annGroup.PUT("/:id", app.perm("notify:manage"), app.notifyAdminHandler.UpdateAnnouncement)
		annGroup.POST("/:id/publish", app.perm("notify:manage"), app.notifyAdminHandler.PublishAnnouncement)
		annGroup.POST("/:id/offline", app.perm("notify:manage"), app.notifyAdminHandler.OfflineAnnouncement)
		annGroup.DELETE("/:id", app.perm("notify:manage"), app.notifyAdminHandler.DeleteAnnouncement)
	}
	if app.content != nil {
		contentArticles := v1.Group("/content/articles")
		contentArticles.Use(app.adminAuth(), app.adminAudit())
		{
			contentArticles.GET("", app.perm("content:article:list"), app.content.articleHandler.List)
			contentArticles.POST("", app.perm("content:article:manage"), app.content.articleHandler.Create)
			contentArticles.GET("/:id", app.perm("content:article:list"), app.content.articleHandler.Get)
			contentArticles.PUT("/:id", app.perm("content:article:manage"), app.content.articleHandler.Update)
			contentArticles.POST("/:id/publish", app.perm("content:article:manage"), app.content.articleHandler.Publish)
			contentArticles.POST("/:id/offline", app.perm("content:article:manage"), app.content.articleHandler.Offline)
			contentArticles.DELETE("/:id", app.perm("content:article:manage"), app.content.articleHandler.Delete)
		}

		contentCategories := v1.Group("/content/categories")
		contentCategories.Use(app.adminAuth(), app.adminAudit())
		{
			contentCategories.GET("", app.perm("content:category:list"), app.content.categoryHandler.List)
			contentCategories.POST("", app.perm("content:category:manage"), app.content.categoryHandler.Create)
			contentCategories.PUT("/:id", app.perm("content:category:manage"), app.content.categoryHandler.Update)
			contentCategories.DELETE("/:id", app.perm("content:category:manage"), app.content.categoryHandler.Delete)
		}

		contentLinks := v1.Group("/content/links")
		contentLinks.Use(app.adminAuth(), app.adminAudit())
		{
			contentLinks.GET("", app.perm("content:link:list"), app.content.linkHandler.List)
			contentLinks.POST("", app.perm("content:link:manage"), app.content.linkHandler.Create)
			contentLinks.PUT("/:id", app.perm("content:link:manage"), app.content.linkHandler.Update)
			contentLinks.DELETE("/:id", app.perm("content:link:manage"), app.content.linkHandler.Delete)
		}
	}
	notifyGroup := v1.Group("/notifications")
	notifyGroup.Use(app.adminAuth())
	{
		notifyGroup.GET("", app.perm("notify:record"), app.notifyAdminHandler.ListRecords)
		notifyGroup.GET("/unread-count", app.perm("notify:record"), app.notifyAdminHandler.AdminUnreadCount)
		notifyGroup.POST("/mail-test", app.perm("notify:view"), app.notifyAdminHandler.SendMailTest)
		notifyGroup.POST("/:id/resend", app.perm("notify:view"), app.notifyAdminHandler.Resend)
	}
	tplGroup := v1.Group("/notification-templates")
	tplGroup.Use(app.adminAuth())
	{
		tplGroup.GET("", app.perm("notify:template"), app.notifyAdminHandler.ListTemplates)
		tplGroup.PUT("/:id", app.perm("notify:manage"), app.notifyAdminHandler.UpdateTemplate)
	}
	captchaGroup := v1.Group("/captcha")
	captchaGroup.Use(app.adminAuth())
	{
		captchaGroup.GET("/providers/types", app.perm("captcha:config"), app.captcha.adminHandler.ListProviderTypes)
		captchaGroup.GET("/providers", app.perm("captcha:config"), app.captcha.adminHandler.ListProviders)
		captchaGroup.POST("/providers", app.perm("captcha:config:manage"), app.captcha.adminHandler.CreateProvider)
		captchaGroup.PUT("/providers/:id", app.perm("captcha:config:manage"), app.captcha.adminHandler.UpdateProvider)
		captchaGroup.DELETE("/providers/:id", app.perm("captcha:config:manage"), app.captcha.adminHandler.DeleteProvider)
		captchaGroup.POST("/providers/:id/test", app.perm("captcha:config"), app.captcha.adminHandler.TestProvider)
		captchaGroup.GET("/policies", app.perm("captcha:config"), app.captcha.adminHandler.ListPolicies)
		captchaGroup.PUT("/policies/:scene", app.perm("captcha:config:manage"), app.captcha.adminHandler.UpdatePolicy)
		captchaGroup.GET("/stats", app.perm("captcha:config"), app.captcha.adminHandler.Stats)
	}
	adminNotify := v1.Group("/notification")
	adminNotify.Use(app.adminAuth())
	{
		// 渠道类型与渠道实例
		adminNotify.GET("/channel-types", app.perm("notify:channel"), app.notify.channelHandler.ListTypes)
		adminNotify.GET("/channels", app.perm("notify:channel"), app.notify.channelHandler.List)
		adminNotify.POST("/channels", app.perm("notify:channel:manage"), app.notify.channelHandler.Create)
		adminNotify.GET("/channels/:id", app.perm("notify:channel"), app.notify.channelHandler.Get)
		adminNotify.PUT("/channels/:id", app.perm("notify:channel:manage"), app.notify.channelHandler.Update)
		adminNotify.PATCH("/channels/:id/status", app.perm("notify:channel:manage"), app.notify.channelHandler.UpdateStatus)
		adminNotify.POST("/channels/:id/test", app.perm("notify:channel"), app.notify.channelHandler.Test)
		// 短信 / 邮箱测试发送（统一入口，频控 10 次/分钟/管理员）
		adminNotify.POST("/test-send", app.perm("notify:channel"), app.notify.channelHandler.TestSend)

		// 短信模板
		adminNotify.POST("/sms-templates/preview", app.perm("notify:sms-template"), app.notify.smsTemplateHandler.Preview)
		adminNotify.GET("/sms-templates", app.perm("notify:sms-template"), app.notify.smsTemplateHandler.List)
		adminNotify.POST("/sms-templates", app.perm("notify:sms-template:manage"), app.notify.smsTemplateHandler.Create)
		adminNotify.GET("/sms-templates/:id", app.perm("notify:sms-template"), app.notify.smsTemplateHandler.Get)
		adminNotify.PUT("/sms-templates/:id", app.perm("notify:sms-template:manage"), app.notify.smsTemplateHandler.Update)
		adminNotify.DELETE("/sms-templates/:id", app.perm("notify:sms-template:manage"), app.notify.smsTemplateHandler.Delete)

		// 模板变量注册表（D7）
		adminNotify.GET("/template-vars", app.perm("notify:template"), app.notify.smsTemplateHandler.ListVars)
		adminNotify.POST("/template-vars", app.perm("notify:template"), app.notify.smsTemplateHandler.CreateVar)
		adminNotify.PUT("/template-vars/:id", app.perm("notify:template"), app.notify.smsTemplateHandler.UpdateVar)
		adminNotify.DELETE("/template-vars/:id", app.perm("notify:template"), app.notify.smsTemplateHandler.DeleteVar)

		// 消息群发（目标检索 / 预览 / 发送）
		adminNotify.GET("/broadcast/targets", app.perm("notify:broadcast"), app.notify.broadcastHandler.Targets)
		adminNotify.POST("/broadcast/preview", app.perm("notify:broadcast"), app.notify.broadcastHandler.Preview)
		adminNotify.POST("/broadcast", app.perm("notify:broadcast"), app.notify.broadcastHandler.Send)

		// 发送日志（投递记录）：batch-retry 必须注册在 /:id/retry 之前
		adminNotify.POST("/deliveries/batch-retry", app.perm("notify:delivery"), app.notify.deliveryHandler.BatchRetry)
		adminNotify.GET("/deliveries", app.perm("notify:delivery"), app.notify.deliveryHandler.List)
		adminNotify.GET("/deliveries/:id", app.perm("notify:delivery"), app.notify.deliveryHandler.Get)
		adminNotify.POST("/deliveries/:id/retry", app.perm("notify:delivery"), app.notify.deliveryHandler.Retry)
	}
}
