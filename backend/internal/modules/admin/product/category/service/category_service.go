// Package service 提供商品分类（category）子域的业务编排。
package service

import (
	"context"
	"errors"

	"hostsent/backend/internal/modules/admin/product/category/dto"
	"hostsent/backend/internal/modules/admin/product/category/model"
	"hostsent/backend/internal/modules/admin/product/category/repository"
)

// 业务错误
var (
	// ErrCategoryHasChildren 存在子分类，禁止删除
	ErrCategoryHasChildren = errors.New("分类下存在子分类，无法删除")
)

// CategoryService 定义商品分类业务能力。
type CategoryService interface {
	List(ctx context.Context, req dto.CategoryListRequest) (*dto.CategoryListResponse, error)
	Create(ctx context.Context, req dto.CategoryCreateRequest) (*dto.CategoryInfo, error)
	FindByID(ctx context.Context, id uint64) (*dto.CategoryInfo, error)
	Update(ctx context.Context, id uint64, req dto.CategoryUpdateRequest) (*dto.CategoryInfo, error)
	Delete(ctx context.Context, id uint64) error
}

type categoryService struct {
	repo repository.CategoryRepository
}

// NewCategoryService 创建分类业务服务。
func NewCategoryService(repo repository.CategoryRepository) CategoryService {
	return &categoryService{repo: repo}
}

func (s *categoryService) List(ctx context.Context, req dto.CategoryListRequest) (*dto.CategoryListResponse, error) {
	// 未传 status 时取全部（含停用，保证树结构完整）；传了则精确筛选。
	items, err := s.repo.List(ctx, req.Status)
	if err != nil {
		return nil, err
	}
	tree := buildCategoryTree(items)
	return &dto.CategoryListResponse{Items: tree}, nil
}

func (s *categoryService) Create(ctx context.Context, req dto.CategoryCreateRequest) (*dto.CategoryInfo, error) {
	item := &model.ProductCategory{
		ParentID:  req.ParentID,
		Name:      req.Name,
		Icon:      req.Icon,
		SortOrder: req.SortOrder,
		Status:    req.Status,
	}
	// 默认启用
	if item.Status == 0 {
		item.Status = model.CategoryStatusEnabled
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *categoryService) FindByID(ctx context.Context, id uint64) (*dto.CategoryInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info := buildCategoryInfo(*item)
	return &info, nil
}

func (s *categoryService) Update(ctx context.Context, id uint64, req dto.CategoryUpdateRequest) (*dto.CategoryInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.ParentID = req.ParentID
	item.Name = req.Name
	item.Icon = req.Icon
	item.SortOrder = req.SortOrder
	item.Status = req.Status
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *categoryService) Delete(ctx context.Context, id uint64) error {
	// 存在子分类时禁止删除，避免悬挂节点
	exists, err := s.repo.ExistsChildren(ctx, id)
	if err != nil {
		return err
	}
	if exists {
		return ErrCategoryHasChildren
	}
	return s.repo.Delete(ctx, id)
}

func buildCategoryTree(items []model.ProductCategory) []*dto.CategoryInfo {
	nodes := make(map[uint64]*dto.CategoryInfo, len(items))
	for _, item := range items {
		nodes[item.ID] = &dto.CategoryInfo{
			ID:        item.ID,
			ParentID:  item.ParentID,
			Name:      item.Name,
			Icon:      item.Icon,
			SortOrder: item.SortOrder,
			Status:    item.Status,
		}
	}

	var roots []*dto.CategoryInfo
	for _, node := range nodes {
		if node.ParentID == 0 {
			roots = append(roots, node)
			continue
		}
		if parent, ok := nodes[node.ParentID]; ok {
			parent.Children = append(parent.Children, node)
		} else {
			roots = append(roots, node)
		}
	}
	return roots
}

func buildCategoryInfo(item model.ProductCategory) dto.CategoryInfo {
	return dto.CategoryInfo{
		ID:        item.ID,
		ParentID:  item.ParentID,
		Name:      item.Name,
		Icon:      item.Icon,
		SortOrder: item.SortOrder,
		Status:    item.Status,
	}
}
