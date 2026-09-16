// Package service 提供菜单模块的树构建业务。
package service

import (
	"context"

	"hostsent/backend/internal/modules/admin/menu/dto"
	"hostsent/backend/internal/modules/admin/menu/model"
	"hostsent/backend/internal/modules/admin/menu/repository"
)

// MenuService 菜单树构建能力。
//
// 只读：菜单由 seed（internal/pkg/db 的 SeedMenus）定义，管理端不提供写能力（doc102 M0）。
type MenuService interface {
	Tree(ctx context.Context, platform string) ([]dto.MenuNode, error)
}

type menuService struct {
	repo repository.MenuRepository
}

// NewMenuService 创建菜单服务实例。
func NewMenuService(repo repository.MenuRepository) MenuService {
	return &menuService{repo: repo}
}

func (s *menuService) Tree(ctx context.Context, platform string) ([]dto.MenuNode, error) {
	menus, err := s.repo.List(ctx, platform)
	if err != nil {
		return nil, err
	}
	return buildMenuTree(menus), nil
}

func buildMenuTree(menus []model.Menu) []dto.MenuNode {
	childrenByParent := make(map[uint64][]model.Menu, len(menus))
	for _, menu := range menus {
		childrenByParent[menu.ParentID] = append(childrenByParent[menu.ParentID], menu)
	}
	return buildMenuChildren(childrenByParent, 0)
}

func buildMenuChildren(childrenByParent map[uint64][]model.Menu, parentID uint64) []dto.MenuNode {
	children := childrenByParent[parentID]
	nodes := make([]dto.MenuNode, 0, len(children))
	for _, menu := range children {
		node := toMenuNode(menu)
		node.Children = buildMenuChildren(childrenByParent, menu.ID)
		nodes = append(nodes, node)
	}
	return nodes
}

func toMenuNode(menu model.Menu) dto.MenuNode {
	return dto.MenuNode{
		ID:        menu.ID,
		ParentID:  menu.ParentID,
		Platform:  menu.Platform,
		Name:      menu.Name,
		Type:      menu.Type,
		Path:      menu.Path,
		Component: menu.Component,
		Icon:      menu.Icon,
		SortOrder: menu.SortOrder,
		Status:    menu.Status,
	}
}
