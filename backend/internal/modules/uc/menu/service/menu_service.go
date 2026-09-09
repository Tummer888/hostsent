// Package service 提供用户中心菜单模块的树构建业务。
package service

import (
	"context"

	menumodel "hostsent/backend/internal/modules/admin/menu/model"
	"hostsent/backend/internal/modules/uc/menu/dto"
	"hostsent/backend/internal/modules/uc/menu/repository"
)

// MenuService 用户中心菜单服务接口。
type MenuService interface {
	// Tree 按平台构建菜单树，platform 为空时默认取用户端（user）。
	Tree(ctx context.Context, platform string) ([]dto.MenuNode, error)
}

type menuService struct {
	repo repository.MenuRepository
}

// NewMenuService 创建用户中心菜单服务实例。
func NewMenuService(repo repository.MenuRepository) MenuService {
	return &menuService{repo: repo}
}

// Tree 查询菜单记录并递归组装为树形结构。
func (s *menuService) Tree(ctx context.Context, platform string) ([]dto.MenuNode, error) {
	if platform == "" {
		platform = menumodel.PlatformUser
	}
	menus, err := s.repo.ListByPlatform(ctx, platform)
	if err != nil {
		return nil, err
	}
	return buildTree(menus), nil
}

// buildTree 按 parent_id 分组并递归构建菜单树。
func buildTree(menus []menumodel.Menu) []dto.MenuNode {
	childrenByParent := make(map[uint64][]menumodel.Menu, len(menus))
	for _, menu := range menus {
		childrenByParent[menu.ParentID] = append(childrenByParent[menu.ParentID], menu)
	}
	return buildChildren(childrenByParent, 0)
}

// buildChildren 递归构建指定父节点下的子节点列表。
func buildChildren(childrenByParent map[uint64][]menumodel.Menu, parentID uint64) []dto.MenuNode {
	children := childrenByParent[parentID]
	nodes := make([]dto.MenuNode, 0, len(children))
	for _, menu := range children {
		node := dto.MenuNode{
			ID:       menu.ID,
			ParentID: menu.ParentID,
			Name:     menu.Name,
			Type:     menu.Type,
			Path:     menu.Path,
			Icon:     menu.Icon,
		}
		node.Children = buildChildren(childrenByParent, menu.ID)
		nodes = append(nodes, node)
	}
	return nodes
}
