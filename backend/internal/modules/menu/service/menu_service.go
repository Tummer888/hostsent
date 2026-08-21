// Package service 提供菜单模块的树构建与增删改查业务。
package service

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/menu/dto"
	"hostsent/backend/internal/modules/menu/model"
	"hostsent/backend/internal/modules/menu/repository"
)

// MenuService 定义菜单树构建与增删改查所需的业务能力。
type MenuService interface {
	Tree(ctx context.Context, platform string) ([]dto.MenuNode, error)
	Create(ctx context.Context, req dto.MenuCreateRequest) (*dto.MenuNode, error)
	Update(ctx context.Context, id uint64, req dto.MenuUpdateRequest) (*dto.MenuNode, error)
	Delete(ctx context.Context, id uint64) error
}

type menuService struct {
	repo repository.MenuRepository
}

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

func (s *menuService) Create(ctx context.Context, req dto.MenuCreateRequest) (*dto.MenuNode, error) {
	menu := &model.Menu{
		ParentID:  req.ParentID,
		Platform:  req.Platform,
		Name:      req.Name,
		Type:      defaultType(req.Type),
		Path:      req.Path,
		Component: req.Component,
		Icon:      req.Icon,
		SortOrder: req.SortOrder,
		Status:    defaultStatus(req.Status),
	}
	if err := s.repo.Create(ctx, menu); err != nil {
		return nil, err
	}
	return ptrMenuNode(*menu), nil
}

func (s *menuService) Update(ctx context.Context, id uint64, req dto.MenuUpdateRequest) (*dto.MenuNode, error) {
	menu, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	menu.ParentID = req.ParentID
	menu.Platform = req.Platform
	menu.Name = req.Name
	menu.Type = defaultType(req.Type)
	menu.Path = req.Path
	menu.Component = req.Component
	menu.Icon = req.Icon
	menu.SortOrder = req.SortOrder
	menu.Status = req.Status
	if err := s.repo.Update(ctx, menu); err != nil {
		return nil, err
	}
	return ptrMenuNode(*menu), nil
}

func (s *menuService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		return err
	}
	return s.repo.Delete(ctx, id)
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

func ptrMenuNode(menu model.Menu) *dto.MenuNode {
	node := toMenuNode(menu)
	return &node
}

func defaultType(t string) string {
	if t == "" {
		return model.TypeMenu
	}
	return t
}

func defaultStatus(s string) string {
	if s == "" {
		return model.StatusActive
	}
	return s
}
