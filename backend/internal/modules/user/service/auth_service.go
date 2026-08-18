package service

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/repository"
	appauth "hostsent/backend/internal/pkg/auth"
)

type AuthService interface {
	Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error)
	Me(ctx context.Context, userID uint64) (*dto.UserInfo, error)
}

type authService struct {
	repo      repository.UserRepository
	jwtIssuer *appauth.JWTIssuer
}

func NewAuthService(repo repository.UserRepository, jwtIssuer *appauth.JWTIssuer) AuthService {
	return &authService{repo: repo, jwtIssuer: jwtIssuer}
}

func (s *authService) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error) {
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

	token, err := s.jwtIssuer.Generate(&appauth.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		Roles:    user.Roles,
	})
	if err != nil {
		return nil, err
	}

	return &dto.LoginResponse{
		Token: token,
		UserInfo: dto.UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Role:     user.Role,
			Roles:    user.Roles,
			Email:    user.Email,
			Phone:    user.Phone,
			Status:   user.Status,
		},
		Permissions: []string{"user:me", "auth:login"},
		Menus:       []string{"dashboard", "profile"},
	}, nil
}

func (s *authService) Me(ctx context.Context, userID uint64) (*dto.UserInfo, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &dto.UserInfo{
		ID:       user.ID,
		Username: user.Username,
		Role:     user.Role,
		Roles:    user.Roles,
		Email:    user.Email,
		Phone:    user.Phone,
		Status:   user.Status,
	}, nil
}
