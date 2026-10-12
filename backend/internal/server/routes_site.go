package server

import (
	"github.com/gin-gonic/gin"
)

// registerSiteRoutes 注册门户公开只读接口（/api/v1/public）：站点内容与验证码公开接口。
// 自 router.go 原样拆出（批次四 4a），组内注册顺序与拆分前保持一致。
func registerSiteRoutes(r *gin.Engine, app *App) {
	// 官网门户公开只读接口（internal/modules/site，无需登录，字段已脱敏）
	// 与 uc 分开的原因：这些接口服务的是「未登录的公网访客与搜索引擎」，响应可共享缓存；
	// 用户中心接口一律带用户态，两者不能放同一模块混用缓存策略。
	publicSite := r.Group("/api/v1/public")
	{
		publicSite.GET("/announcements", app.siteHandler.Announcements)               // 已发布公告
		publicSite.GET("/announcements/:id", app.siteHandler.AnnouncementDetail)      // 公告详情（门户查看）
		publicSite.GET("/site-content", app.siteHandler.SiteContent)                  // 品牌与站点配置（白名单）
		publicSite.GET("/articles", app.siteHandler.Articles)                         // 内容列表（news/help/terms/privacy）
		publicSite.GET("/articles/:kind/:slug", app.siteHandler.ArticleDetail)        // 内容详情（按 slug）
		publicSite.GET("/article-singletons/:kind", app.siteHandler.SingletonArticle) // 条款/隐私（每类型一篇）
		publicSite.GET("/article-categories", app.siteHandler.ArticleCategories)      // 分类树（新闻分栏/帮助目录）
		publicSite.GET("/friendly-links", app.siteHandler.FriendlyLinks)              // 友情链接
	}

	// 验证码公开接口（doc91 §3.3）：登录前使用，无需鉴权。
	// auth-config 是前端「要不要渲染验证码」的唯一依据：没有它前端只能写死。
	if app.captcha != nil {
		publicCaptcha := r.Group("/api/v1/public")
		{
			publicCaptcha.GET("/auth-config", app.captcha.publicHandler.AuthConfig)
			publicCaptcha.GET("/captcha/image", app.captcha.publicHandler.ImageChallenge)
			publicCaptcha.POST("/verify-code/send", app.captcha.publicHandler.SendCode)
		}
	}
}
