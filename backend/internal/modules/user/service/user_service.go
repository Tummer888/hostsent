package service

import (
	"context"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/model"
	"hostsent/backend/internal/modules/user/repository"
)

type UserService interface {
	List(ctx context.Context) ([]dto.UserInfo, error)
	Create(ctx context.Context, req dto.UserCreateRequest) (*dto.UserInfo, error)
	AssignRoles(ctx context.Context, userID uint64, roleIDs []uint64) error
}

type userService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) List(ctx context.Context) ([]dto.UserInfo, error) {
	users, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.UserInfo, 0, len(users))
	for _, user := range users {
		resp = append(resp, dto.UserInfo{ID: user.ID, Username: user.Username, Role: user.Role, Roles: user.Roles, Email: user.Email})
	}
	return resp, nil
}

func (s *userService) Create(ctx context.Context, req dto.UserCreateRequest) (*dto.UserInfo, error) {
	user := &model.User{Username: req.Username, Email: req.Email, Status: "active"}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	if len(req.RoleIDs) > 0 {
		if err := s.repo.UpsertRoles(ctx, user.ID, req.RoleIDs); err != nil {
			return nil, err
		}
	}
	return &dto.UserInfo{ID: user.ID, Username: user.Username, Role: "admin", Roles: []string{"admin"}, Email: user.Email}, nil
}

func (s *userService) AssignRoles(ctx context.Context, userID uint64, roleIDs []uint64) error {
	return s.repo.UpsertRoles(ctx, userID, roleIDs)
}
