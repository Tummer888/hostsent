package server

// 开放平台（P6）装配：与 admin/uc 平级的独立模块，经 Bundle 单字段挂到 App。
// 本文件只做接线，不含业务逻辑（与 assembly.go 的具名装配函数约定一致）。

import (
	"go.uber.org/zap"
	"gorm.io/gorm"

	specrepo "hostsent/backend/internal/modules/admin/product/spec/repository"
	openhandler "hostsent/backend/internal/modules/open/handler"
	openrepo "hostsent/backend/internal/modules/open/repository"
	openservice "hostsent/backend/internal/modules/open/service"
	ucproductservice "hostsent/backend/internal/modules/uc/product/service"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/pricing"
)

// buildOpenBundle 装配开放平台处理器集合（网关 + 目录服务）。
func buildOpenBundle(
	cfg *config.Config,
	db *gorm.DB,
	ucProducts ucproductservice.ProductService,
	specAtoms specrepo.SpecContractRepository,
	pricePipeline *pricing.Service,
	logger *zap.Logger,
) *openhandler.Bundle {
	appRepo := openrepo.NewAppRepository(db)
	gw := openservice.NewGateway(openservice.GatewayDeps{
		AppRepo:    appRepo,
		LogRepo:    appRepo,
		EncryptKey: cfg.App.EncryptKey,
		Logger:     logger,
	})
	bundle := openhandler.NewBundle(gw, logger)
	bundle.SetCatalog(openservice.NewCatalogService(openservice.CatalogDeps{
		Atoms:    specAtoms,
		Products: ucProducts,
		Catalog:  openrepo.NewCatalogRepository(db),
		Pricing:  pricePipeline,
	}))
	return bundle
}
