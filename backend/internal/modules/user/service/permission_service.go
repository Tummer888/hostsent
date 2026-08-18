package service

import (
	"context"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/model"
	"hostsent/backend/internal/modules/user/repository"
)

type PermissionService interface {
	Tree(ctx context.Context) ([]dto.PermissionNode, error)
	Create(ctx context.Context, req dto.PermissionCreateRequest) (*dto.PermissionNode, error)
	Update(ctx context.Context, id uint64, req dto.PermissionUpdateRequest) (*dto.PermissionNode, error)
	Delete(ctx context.Context, id uint64) error
}

type permissionService struct {
	repo repository.PermissionRepository
}

func NewPermissionService(repo repository.PermissionRepository) PermissionService {
	return &permissionService{repo: repo}
}

func (s *permissionService) Tree(ctx context.Context) ([]dto.PermissionNode, error) {
	permissions, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	return buildPermissionTree(permissions), nil
}

func (s *permissionService) Create(ctx context.Context, req dto.PermissionCreateRequest) (*dto.PermissionNode, error) {
	permission := &model.Permission{
		ParentID:  req.ParentID,
		Name:      req.Name,
		Code:      req.Code,
		Type:      req.Type,
		Path:      req.Path,
		Component: req.Component,
		Icon:      req.Icon,
		SortOrder: req.SortOrder,
		Status:    req.Status,
	}
	if permission.Status == "" {
		permission.Status = "active"
	}
	if err := s.repo.Create(ctx, permission); err != nil {
		return nil, err
	}
	return ptrPermissionNode(*permission), nil
}

func (s *permissionService) Update(ctx context.Context, id uint64, req dto.PermissionUpdateRequest) (*dto.PermissionNode, error) {
	permission, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	permission.ParentID = req.ParentID
	permission.Name = req.Name
	permission.Code = req.Code
	permission.Type = req.Type
	permission.Path = req.Path
	permission.Component = req.Component
	permission.Icon = req.Icon
	permission.SortOrder = req.SortOrder
	permission.Status = req.Status
	if err := s.repo.Update(ctx, permission); err != nil {
		return nil, err
	}
	return ptrPermissionNode(*permission), nil
}

func (s *permissionService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

func buildPermissionTree(permissions []model.Permission) []dto.PermissionNode {
	childrenByParent := make(map[uint64][]model.Permission)
	for _, permission := range permissions {
		childrenByParent[permission.ParentID] = append(childrenByParent[permission.ParentID], permission)
	}
	return buildPermissionChildren(childrenByParent, 0)
}

func buildPermissionChildren(childrenByParent map[uint64][]model.Permission, parentID uint64) []dto.PermissionNode {
	permissions := childrenByParent[parentID]
	nodes := make([]dto.PermissionNode, 0, len(permissions))
	for _, permission := range permissions {
		node := toPermissionNode(permission)
		node.Children = buildPermissionChildren(childrenByParent, permission.ID)
		nodes = append(nodes, node)
	}
	return nodes
}

func toPermissionNode(permission model.Permission) dto.PermissionNode {
	return dto.PermissionNode{
		ID:       permission.ID,
		ParentID: permission.ParentID,
		Name:     permission.Name,
		Code:     permission.Code,
		Type:     permission.Type,
		Path:     permission.Path,
		Icon:     permission.Icon,
	}
}

func ptrPermissionNode(permission model.Permission) *dto.PermissionNode {
	node := toPermissionNode(permission)
	return &node
}
