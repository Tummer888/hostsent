package service

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/model"
	"hostsent/backend/internal/modules/user/repository"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/netutil"
)

type AuthService interface {
	Login(ctx context.Context, req dto.LoginRequest, ip string) (*dto.LoginResponse, error)
	Impersonate(ctx context.Context, adminUserID, targetUserID uint64, ip string) (*dto.LoginResponse, error)
	Me(ctx context.Context, userID uint64) (*dto.UserInfo, error)
}

type authService struct {
	repo             repository.UserRepository
	jwtIssuer        *appauth.JWTIssuer
	ipRegionResolver netutil.IPRegionResolver
}

func NewAuthService(repo repository.UserRepository, jwtIssuer *appauth.JWTIssuer, ipRegionResolver netutil.IPRegionResolver) AuthService {
	return &authService{repo: repo, jwtIssuer: jwtIssuer, ipRegionResolver: ipRegionResolver}
}

func (s *authService) Login(ctx context.Context, req dto.LoginRequest, ip string) (*dto.LoginResponse, error) {
	user, err := s.repo.FindByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户名或密码错误")
		}
		return nil, err
	}

	if user.Status != "active" {
		return nil, errors.New("用户已被禁用")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	if err := s.updateLoginProfile(ctx, user.ID, ip); err != nil {
		return nil, err
	}
	user, err = s.repo.FindByID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return s.buildLoginResponse(*user), nil
}

func (s *authService) Impersonate(ctx context.Context, adminUserID, targetUserID uint64, ip string) (*dto.LoginResponse, error) {
	admin, err := s.repo.FindByID(ctx, adminUserID)
	if err != nil {
		return nil, err
	}
	if !containsRole(admin.Roles, "super_admin") && !containsRole(admin.Roles, "admin") {
		return nil, errors.New("无权限代登录该用户")
	}

	target, err := s.repo.FindByID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("目标用户不存在")
		}
		return nil, err
	}
	if target.Status != "active" {
		return nil, errors.New("目标用户不可登录")
	}

	if err := s.updateLoginProfile(ctx, target.ID, ip); err != nil {
		return nil, err
	}
	target, err = s.repo.FindByID(ctx, target.ID)
	if err != nil {
		return nil, err
	}

	return s.buildLoginResponse(*target), nil
}

func (s *authService) Me(ctx context.Context, userID uint64) (*dto.UserInfo, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	info := toAuthUserInfo(*user)
	return &info, nil
}

func (s *authService) buildLoginResponse(user model.User) *dto.LoginResponse {
	token, err := s.jwtIssuer.Generate(&appauth.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		Roles:    user.Roles,
	})
	if err != nil {
		return nil
	}
	return &dto.LoginResponse{
		Token:       token,
		UserInfo:    toAuthUserInfo(user),
		Permissions: []string{"user:me", "auth:login"},
		Menus:       []string{"dashboard", "profile"},
	}
}

func (s *authService) updateLoginProfile(ctx context.Context, userID uint64, ip string) error {
	ipRegion := ""
	if s.ipRegionResolver != nil && ip != "" {
		ipRegion = s.ipRegionResolver.Resolve(ctx, ip)
	}
	return s.repo.UpdateLoginProfile(ctx, userID, ip, ipRegion, time.Now())
}

func toAuthUserInfo(user model.User) dto.UserInfo {
	return dto.UserInfo{
		ID:                user.ID,
		Username:          user.Username,
		Role:              user.Role,
		Roles:             user.Roles,
		Email:             user.Email,
		Phone:             user.Phone,
		Status:            user.Status,
		RealName:          user.RealName,
		Region:            user.Region,
		LastLoginIP:       user.LastLoginIP,
		LastLoginIPRegion: user.LastLoginIPRegion,
		CreatedAt:         user.CreatedAt,
		LastLoginAt:       user.LastLoginAt,
	}
}

func containsRole(roles []string, target string) bool {
	for _, role := range roles {
		if role == target {
			return true
		}
	}
	return false
}
