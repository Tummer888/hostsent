package service

import (
	"context"

	"hostsent/backend/internal/modules/user/dto"
)

type RoleService interface {
	List(ctx context.Context) ([]dto.RoleInfo, error)
}

type roleService struct{}

func NewRoleService() RoleService {
	return &roleService{}
}

func (s *roleService) List(ctx context.Context) ([]dto.RoleInfo, error) {
	return []dto.RoleInfo{{ID: 1, Name: "管理员", Code: "admin", Status: "active"}}, nil
}
