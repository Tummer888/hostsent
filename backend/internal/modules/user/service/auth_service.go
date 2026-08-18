package service

import (
	"context"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/repository"
)

type AuthService interface {
	Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error)
	Me(ctx context.Context, userID uint64) (*dto.UserInfo, error)
}

type authService struct {
	repo      repository.UserRepository
	mockToken string
}

func NewAuthService(repo repository.UserRepository, mockToken string) AuthService {
	return &authService{repo: repo, mockToken: mockToken}
}

func (s *authService) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error) {
	user, err := s.repo.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}

	return &dto.LoginResponse{
		Token: s.mockToken,
		UserInfo: dto.UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Role:     user.Role,
			Roles:    user.Roles,
			Email:    user.Email,
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
	}, nil
}
