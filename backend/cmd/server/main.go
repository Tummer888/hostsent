// Command server 是 HostSent 后端唯一的生产服务入口。
//
// 职责仅限：加载配置 → 初始化日志/数据库 → 装配 internal/server 并监听。
// 业务模块一律在 internal/server 装配层接入，不要在 main 里写业务构造。
// 辅助工具入口见 cmd/openapp、cmd/ucgatewaymock，说明见仓库根 README。
//
// 接口文档主声明（swag v2 原生 OpenAPI 3.1，docs/swagger.json 由 handler 注解生成）：
//
//	~/go/bin/swag init -g cmd/server/main.go -o docs -ot json --v3.1 --parseDependency --parseInternal
//
// @Title HostSent Backend API
// @Version 2.1.0
// @Description HostSent 后端接口文档（原生 OpenAPI 3，由 handler 注解生成）。三条独立前缀：/api/v1/admin/*（管理端，Bearer 管理员 JWT）、/api/v1/uc/*（用户控制台，Bearer 用户 JWT）、/open/v1/*（开放平台，签名鉴权）。除导出类接口外一律返回 {code, message, data, timestamp} 信封，code==0 为成功。
// @BasePath /
// OpenSignature 由 scripts/patch_openapi.py 在生成后注入（swag v2.0.0 OAS3 模式仍只注册第一个 securitydefinitions）
// @securitydefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @securitydefinitions.apikey OpenSignature
// @in header
// @name X-Open-Signature
package main

import (
	"log"

	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/logger"
	"hostsent/backend/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logg, err := logger.New(cfg.App.Env)
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	defer logg.Sync() //nolint:errcheck

	srv, err := server.New(cfg, logg)
	if err != nil {
		logg.Fatal("init server", logger.Error(err))
	}
	if err := srv.Run(); err != nil {
		logg.Fatal("server exited", logger.Error(err))
	}
}
