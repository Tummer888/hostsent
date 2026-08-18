package main

import (
	"log"

	_ "hostsent/backend/docs/swagger"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/logger"
	"hostsent/backend/internal/server"
)

// @title HostSent Backend API
// @version 1.0
// @description HostSent 后端接口文档
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
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
