package server

import (
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/user/handler"
	"hostsent/backend/internal/modules/user/repository"
	"hostsent/backend/internal/modules/user/service"
	"hostsent/backend/internal/pkg/config"
)

type Server struct {
	cfg    *config.Config
	logger *zap.Logger
	http   *http.Server
}

func New(cfg *config.Config, logger *zap.Logger) *Server {
	repo := repository.NewUserRepository()
	authService := service.NewAuthService(repo, cfg.Auth.MockToken)
	authHandler := handler.NewAuthHandler(authService)
	router := newRouter(cfg, authHandler, logger)

	addr := fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port)

	return &Server{
		cfg:    cfg,
		logger: logger,
		http: &http.Server{
			Addr:         addr,
			Handler:      router,
			ReadTimeout:  time.Duration(cfg.App.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.App.WriteTimeout) * time.Second,
		},
	}
}

func (s *Server) Run() error {
	s.logger.Info("server starting", zap.String("addr", s.http.Addr), zap.String("name", s.cfg.App.Name))
	return s.http.ListenAndServe()
}
