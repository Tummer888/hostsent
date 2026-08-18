package service

import (
	"context"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/repository"
)

type UserService interface {
	GetByID(ctx context.Context, id uint64) (*dto.UserInfo, error)
}

type userService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) GetByID(ctx context.Context, id uint64) (*dto.UserInfo, error) {
	user, err := s.repo.FindByID(ctx, id)
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
