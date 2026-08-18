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
