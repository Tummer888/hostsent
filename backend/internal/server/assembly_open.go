package server

// 开放平台（P6/T6.1）装配：与 admin/uc 平级的独立模块，经 Bundle 单字段挂到 App。
// 本文件只做接线，不含业务逻辑（与 assembly.go 的具名装配函数约定一致）。

import (
	"go.uber.org/zap"
	"gorm.io/gorm"

	openhandler "hostsent/backend/internal/modules/open/handler"
	openrepo "hostsent/backend/internal/modules/open/repository"
	openservice "hostsent/backend/internal/modules/open/service"
	"hostsent/backend/internal/pkg/config"
)

// buildOpenBundle 装配开放平台处理器集合（网关 + 业务服务）。
func buildOpenBundle(cfg *config.Config, db *gorm.DB, logger *zap.Logger) *openhandler.Bundle {
	appRepo := openrepo.NewAppRepository(db)
	gw := openservice.NewGateway(openservice.GatewayDeps{
		AppRepo:    appRepo,
		LogRepo:    appRepo,
		EncryptKey: cfg.App.EncryptKey,
		Logger:     logger,
	})
	return openhandler.NewBundle(gw, logger)
}
