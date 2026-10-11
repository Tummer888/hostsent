// Command server 是 HostSent 后端唯一的生产服务入口。
//
// 职责仅限：加载配置 → 初始化日志/数据库 → 装配 internal/server 并监听。
// 业务模块一律在 internal/server 装配层接入，不要在 main 里写业务构造。
// 辅助工具入口见 cmd/openapp、cmd/ucgatewaymock，说明见仓库根 README。
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
