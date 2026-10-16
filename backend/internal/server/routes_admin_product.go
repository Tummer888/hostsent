// 管理端路由——Product域（自 routes_admin.go 拆分，批次三 C2）。

package server

import (
	"github.com/gin-gonic/gin"
)

func registerAdminProductRoutes(v1 *gin.RouterGroup, app *App) {
	productGroup := v1.Group("/product")
	productGroup.Use(app.adminAuth())
	{
		// 产品分类
		categories := productGroup.Group("/categories")
		{
			categories.GET("", app.perm("product:category"), app.prodCategoryHandler.List)
			categories.POST("", app.perm("product:category:create"), app.prodCategoryHandler.Create)
			categories.GET("/:id", app.perm("product:category"), app.prodCategoryHandler.Get)
			categories.PUT("/:id", app.perm("product:category:update"), app.prodCategoryHandler.Update)
			categories.DELETE("/:id", app.perm("product:category:delete"), app.prodCategoryHandler.Delete)
		}

		// 产品（商品管理 catalog 子域）
		prodProducts := productGroup.Group("/products")
		{
			prodProducts.GET("", app.perm("product:list"), app.prodCatalogHandler.List)
			prodProducts.POST("", app.perm("product:create"), app.prodCatalogHandler.Create)
			// 从上游商品克隆创建销售商品（对接魔方财务商品导入）。
			// 注意：静态路由需在 /:id 之前注册，避免被参数路由吞并。
			prodProducts.POST("/clone", app.perm("product:create"), app.prodCatalogHandler.CloneFromUpstream)
			prodProducts.POST("/clone/batch", app.perm("product:create"), app.prodCatalogHandler.BatchCloneFromUpstream)
			// 上游商品页调价：设定加价规则并重算其已导入商品的售价（静态路由须在 /:id 之前）。
			prodProducts.POST("/upstream-markup", app.perm("product:price:update"), app.prodCatalogHandler.ApplyUpstreamMarkup)
			// 批量操作（产品列表页多选）：逐条复用单条逻辑，失败逐条回报。
			// 上架门禁的显式放行项（仅透传）：单独开关 + 留痕，避免用改商品的其他字段顺带打开。
			prodProducts.POST("/:id/spec-passthrough", app.perm("product:update"), app.prodCatalogHandler.SetSpecPassthrough)
			prodProducts.POST("/batch-publish", app.perm("product:publish"), app.prodCatalogHandler.BatchPublishProducts)
			prodProducts.POST("/batch-unpublish", app.perm("product:publish"), app.prodCatalogHandler.BatchUnpublishProducts)
			prodProducts.POST("/batch-delete", app.perm("product:delete"), app.prodCatalogHandler.BatchDeleteProducts)
			prodProducts.GET("/:id", app.perm("product:list"), app.prodCatalogHandler.Get)
			prodProducts.PUT("/:id", app.perm("product:update"), app.prodCatalogHandler.Update)
			prodProducts.DELETE("/:id", app.perm("product:delete"), app.prodCatalogHandler.Delete)
			prodProducts.POST("/:id/publish", app.perm("product:publish"), app.prodCatalogHandler.Publish)
			prodProducts.POST("/:id/unpublish", app.perm("product:publish"), app.prodCatalogHandler.Unpublish)
			prodProducts.PUT("/:id/price", app.perm("product:price:update"), app.prodCatalogHandler.UpdatePrice)
			// 把商品当前周期价同步到已绑定续费价的实例（改价后让老机器也按新价续费）。
			// 影响已开通机器的计费口径，与调价同权限；支持 dry_run 试算与 clear 清除绑定。
			prodProducts.POST("/:id/sync-renewal-price", app.perm("product:price:update"), app.prodCatalogHandler.SyncRenewalPrices)
			prodProducts.POST("/:id/featured", app.perm("product:update"), app.prodCatalogHandler.SetFeatured)
			prodProducts.GET("/:id/history", app.perm("product:list"), app.prodCatalogHandler.ListHistory)
			prodProducts.GET("/:id/specs", app.perm("product:list"), app.prodCatalogHandler.ListSpecs)
			// SKU 规格变体维护（T4.1）：商品下挂规格矩阵，下单按 SKU 计价与开通。
			prodProducts.POST("/:id/specs", app.perm("product:update"), app.prodCatalogHandler.CreateSpec)
			// 自营链路打通：按规格模板生成 SKU 并同步落平台绑定（模板含平台写参数）。
			prodProducts.POST("/:id/specs/generate", app.perm("product:update"), app.prodCatalogHandler.GenerateSpecFromTemplate)
			prodProducts.PUT("/:id/specs/:specId", app.perm("product:update"), app.prodCatalogHandler.UpdateSpec)
			prodProducts.DELETE("/:id/specs/:specId", app.perm("product:update"), app.prodCatalogHandler.DeleteSpec)
			// 上游商品的 SKU 维度（T4.8）：选哪些上游配置项作为规格维度，按取值组合展开生成 SKU。
			// 只对 source_mode=upstream 有意义；自营商品的维度在规格模板的 option_selections 上。
			prodProducts.GET("/:id/upstream-axes", app.perm("product:list"), app.prodCatalogHandler.UpstreamAxisOptions)
			prodProducts.PUT("/:id/upstream-axes", app.perm("product:update"), app.prodCatalogHandler.SaveUpstreamAxisOptions)
			prodProducts.POST("/:id/upstream-axes/generate", app.perm("product:update"), app.prodCatalogHandler.GenerateUpstreamAxisSpecs)
			// 可配置项维护（T4.4）：source=upstream 走上游配置 id，source=self 直接下发平台参数名。
			prodProducts.GET("/:id/config-options", app.perm("product:list"), app.prodCatalogHandler.ListConfigOptions)
			prodProducts.PUT("/:id/config-options", app.perm("product:update"), app.prodCatalogHandler.SaveConfigOptions)
		}

		// 规格管理（spec 子域）
		specGroup := productGroup.Group("/spec")
		{
			specTemplates := specGroup.Group("/templates")
			{
				specTemplates.GET("", app.perm("spec:template:list"), app.specHandler.ListTemplates)
				specTemplates.POST("", app.perm("spec:template:update"), app.specHandler.CreateTemplate)
				specTemplates.GET("/:id", app.perm("spec:template:list"), app.specHandler.GetTemplate)
				specTemplates.PUT("/:id", app.perm("spec:template:update"), app.specHandler.UpdateTemplate)
				specTemplates.DELETE("/:id", app.perm("spec:template:update"), app.specHandler.DeleteTemplate)
			}
			specMappings := specGroup.Group("/mappings")
			{
				specMappings.GET("", app.perm("spec:mapping:list"), app.specHandler.ListMappings)
				specMappings.POST("", app.perm("spec:mapping:update"), app.specHandler.CreateMapping)
				specMappings.GET("/:id", app.perm("spec:mapping:list"), app.specHandler.GetMapping)
				specMappings.PUT("/:id", app.perm("spec:mapping:update"), app.specHandler.UpdateMapping)
				specMappings.POST("/:id/bind", app.perm("spec:mapping:update"), app.specHandler.BindMapping)
				specMappings.DELETE("/:id", app.perm("spec:mapping:update"), app.specHandler.DeleteMapping)
			}
			// 规格契约（P2/T2.5）：原子字典 / 校验 / 外部规格快照 / 绑定
			specGroup.GET("/atoms", app.perm("spec:contract:list"), app.specHandler.ListAtoms)
			specGroup.POST("/validate", app.perm("spec:contract:list"), app.specHandler.ValidateSpec)
			specGroup.GET("/external-specs", app.perm("spec:contract:list"), app.specHandler.ListExternalSpecs)
			specGroup.POST("/external-specs", app.perm("spec:contract:update"), app.specHandler.UpsertExternalSpec)
			specGroup.GET("/bindings", app.perm("spec:contract:list"), app.specHandler.ListBindings)
			specGroup.POST("/bindings", app.perm("spec:contract:update"), app.specHandler.UpsertBinding)
			specGroup.POST("/bindings/:id/confirm", app.perm("spec:contract:update"), app.specHandler.ConfirmBinding)

			// 平台配置项目录与取值库（T4.5 规格配置化）：
			// 按对接平台（魔方云）声明可配置项，运营改默认值/必选并维护可选值（含镜像批量入库）。
			// 静态路由须排在 /option-catalog/:id 之前，避免被参数路由吞并。
			specGroup.GET("/option-catalog", app.perm("spec:option:list"), app.specHandler.ListOptionCatalog)
			specGroup.POST("/option-catalog", app.perm("spec:option:update"), app.specHandler.CreateOptionSpec)
			specGroup.POST("/option-catalog/sync", app.perm("spec:option:update"), app.specHandler.SyncOptionCatalog)
			specGroup.PUT("/option-catalog/:id", app.perm("spec:option:update"), app.specHandler.UpdateOptionSpec)
			specGroup.DELETE("/option-catalog/:id", app.perm("spec:option:update"), app.specHandler.DeleteOptionSpec)
			specGroup.GET("/option-values", app.perm("spec:option:list"), app.specHandler.ListOptionValues)
			specGroup.POST("/option-values", app.perm("spec:option:update"), app.specHandler.UpsertOptionValue)
			specGroup.POST("/option-values/import", app.perm("spec:option:update"), app.specHandler.ImportOptionValues)
			specGroup.POST("/option-values/refresh", app.perm("spec:option:update"), app.specHandler.RefreshOptionValues)
			specGroup.DELETE("/option-values/:id", app.perm("spec:option:update"), app.specHandler.DeleteOptionValue)
		}

		// 定价与计费（pricing 子域）
		pricingGroup := productGroup.Group("/pricing")
		{
			pricingGroup.GET("", app.perm("pricing:list"), app.pricingHandler.List)
			pricingGroup.POST("", app.perm("pricing:update"), app.pricingHandler.Create)
			pricingGroup.GET("/:id", app.perm("pricing:list"), app.pricingHandler.Get)
			pricingGroup.PUT("/:id", app.perm("pricing:update"), app.pricingHandler.Update)
			pricingGroup.DELETE("/:id", app.perm("pricing:update"), app.pricingHandler.Delete)
		}

		// 周期价格矩阵（doc25）：商品 × 规格 × 周期，GET 取矩阵 / PUT 整表保存。
		prices := productGroup.Group("/prices")
		{
			prices.GET("", app.perm("pricing:list"), app.priceMatrixHandler.Get)
			prices.PUT("", app.perm("pricing:update"), app.priceMatrixHandler.Save)
		}

		// 折扣策略（discount 子域，P5-01/P5-06）：用户组/代理绑定的价格策略
		discountPolicies := productGroup.Group("/discount-policies")
		{
			discountPolicies.GET("", app.perm("pricing:list"), app.discountPolicyHandler.List)
			discountPolicies.POST("", app.perm("pricing:update"), app.discountPolicyHandler.Create)
			discountPolicies.GET("/:id", app.perm("pricing:list"), app.discountPolicyHandler.Get)
			discountPolicies.PUT("/:id", app.perm("pricing:update"), app.discountPolicyHandler.Update)
			discountPolicies.DELETE("/:id", app.perm("pricing:update"), app.discountPolicyHandler.Delete)
		}

		// 促销管理（promotion 子域）
		promoGroup := productGroup.Group("/promotion")
		{
			coupons := promoGroup.Group("/coupons")
			{
				coupons.GET("", app.perm("promotion:coupon:list"), app.promotionHandler.ListCoupons)
				coupons.POST("", app.perm("promotion:coupon:update"), app.promotionHandler.CreateCoupon)
				coupons.GET("/:id", app.perm("promotion:coupon:list"), app.promotionHandler.GetCoupon)
				coupons.PUT("/:id", app.perm("promotion:coupon:update"), app.promotionHandler.UpdateCoupon)
				coupons.DELETE("/:id", app.perm("promotion:coupon:update"), app.promotionHandler.DeleteCoupon)
			}
			couponGrants := promoGroup.Group("/coupon-grants")
			{
				couponGrants.GET("", app.perm("promotion:coupon:list"), app.promotionHandler.ListCouponGrants)
				couponGrants.POST("", app.perm("promotion:coupon:update"), app.promotionHandler.CreateCouponGrants)
			}
			promotions := promoGroup.Group("/promotions")
			{
				promotions.GET("", app.perm("promotion:activity:list"), app.promotionHandler.ListPromotions)
				promotions.POST("", app.perm("promotion:activity:update"), app.promotionHandler.CreatePromotion)
				promotions.GET("/:id", app.perm("promotion:activity:list"), app.promotionHandler.GetPromotion)
				promotions.PUT("/:id", app.perm("promotion:activity:update"), app.promotionHandler.UpdatePromotion)
				promotions.DELETE("/:id", app.perm("promotion:activity:update"), app.promotionHandler.DeletePromotion)
			}
			// 限时活动折扣（doc108 §8J）：面向**普通用户**的营销折扣，真实参与算价。
			// 与代理拿货折扣（agent-levels）是两条互不叠加的线：代理用户不参与活动折扣。
			flashDiscounts := promoGroup.Group("/flash-discounts")
			{
				flashDiscounts.GET("", app.perm("promotion:activity:list"), app.flashDiscountHandler.List)
				flashDiscounts.POST("", app.perm("promotion:activity:update"), app.flashDiscountHandler.Create)
				flashDiscounts.GET("/:id", app.perm("promotion:activity:list"), app.flashDiscountHandler.Get)
				flashDiscounts.PUT("/:id", app.perm("promotion:activity:update"), app.flashDiscountHandler.Update)
				flashDiscounts.DELETE("/:id", app.perm("promotion:activity:update"), app.flashDiscountHandler.Delete)
			}
		}
	}
}
