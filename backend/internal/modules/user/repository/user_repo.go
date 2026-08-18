package repository

import (
	"context"

	"hostsent/backend/internal/modules/user/model"
)

type UserRepository interface {
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindByID(ctx context.Context, id uint64) (*model.User, error)
}

type userRepository struct{}

func NewUserRepository() UserRepository {
	return &userRepository{}
}

func (r *userRepository) FindByUsername(_ context.Context, username string) (*model.User, error) {
	return &model.User{
		ID:       1,
		Username: username,
		Email:    username + "@example.com",
		Role:     "admin",
		Roles:    []string{"admin"},
	}, nil
}

func (r *userRepository) FindByID(_ context.Context, id uint64) (*model.User, error) {
	return &model.User{
		ID:       id,
		Username: "admin",
		Email:    "admin@example.com",
		Role:     "admin",
		Roles:    []string{"admin"},
	}, nil
}
