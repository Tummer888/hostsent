package service

import (
	"context"

	"hostsent/backend/internal/modules/user/dto"
)

type PermissionService interface {
	Tree(ctx context.Context) ([]dto.PermissionNode, error)
}

type permissionService struct{}

func NewPermissionService() PermissionService {
	return &permissionService{}
}

func (s *permissionService) Tree(ctx context.Context) ([]dto.PermissionNode, error) {
	return []dto.PermissionNode{{ID: 1, Name: "系统管理", Code: "system", Type: "directory"}}, nil
}
