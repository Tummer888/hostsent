package server

import (
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/user/handler"
	"hostsent/backend/internal/modules/user/repository"
	"hostsent/backend/internal/modules/user/service"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/db"
)

type Server struct {
	cfg    *config.Config
	logger *zap.Logger
	http   *http.Server
}

func New(cfg *config.Config, logger *zap.Logger) (*Server, error) {
	database, err := db.New(cfg.Database)
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(database); err != nil {
		return nil, err
	}
	if err := db.Seed(database); err != nil {
		return nil, err
	}

	jwtIssuer := appauth.NewJWTIssuer(cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer, time.Duration(cfg.Auth.JWTExpireHours)*time.Hour)
	repo := repository.NewUserRepository(database)
	authService := service.NewAuthService(repo, jwtIssuer)
	authHandler := handler.NewAuthHandler(authService)
	router := newRouter(cfg, authHandler, logger, jwtIssuer)

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
	}, nil
}

func (s *Server) Run() error {
	s.logger.Info("server starting", zap.String("addr", s.http.Addr), zap.String("name", s.cfg.App.Name))
	return s.http.ListenAndServe()
}
