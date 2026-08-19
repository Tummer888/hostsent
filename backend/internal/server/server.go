package server

import (
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	menuhandler "hostsent/backend/internal/modules/menu/handler"
	menurepo "hostsent/backend/internal/modules/menu/repository"
	menuservice "hostsent/backend/internal/modules/menu/service"
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
	userRepo := repository.NewUserRepository(database)
	userDetailRepo := repository.NewUserDetailRepository(database)
	userGroupRepo := repository.NewUserGroupRepository(database)
	roleRepo := repository.NewRoleRepository(database)
	permissionRepo := repository.NewPermissionRepository(database)
	menuRepo := menurepo.NewMenuRepository(database)
	authService := service.NewAuthService(userRepo, jwtIssuer)
	userService := service.NewUserService(userRepo)
	userDetailService := service.NewUserDetailService(userRepo, userDetailRepo)
	userGroupService := service.NewUserGroupService(userGroupRepo)
	roleService := service.NewRoleService(roleRepo)
	permissionService := service.NewPermissionService(permissionRepo)
	menuService := menuservice.NewMenuService(menuRepo)
	authHandler := handler.NewAuthHandler(authService)
	userHandler := handler.NewUserHandler(userService)
	userDetailHandler := handler.NewUserDetailHandler(userDetailService)
	userGroupHandler := handler.NewUserGroupHandler(userGroupService)
	roleHandler := handler.NewRoleHandler(roleService)
	permissionHandler := handler.NewPermissionHandler(permissionService)
	menuHandler := menuhandler.NewMenuHandler(menuService)
	router := newRouter(cfg, authHandler, userHandler, userDetailHandler, userGroupHandler, roleHandler, permissionHandler, menuHandler, logger, jwtIssuer)

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
