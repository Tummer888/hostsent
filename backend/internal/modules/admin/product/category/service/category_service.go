// Package service 提供商品分类（category）子域的业务编排。
package service

import (
	"context"
	"errors"
	"math"
	"sort"

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
		CostRate:  normalizeCostRate(req.CostRate),
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
	item.CostRate = normalizeCostRate(req.CostRate)
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
			// CostRate 必须随树下发：分类管理页的编辑弹窗直接用树节点回显，
			// 漏掉这一项会让「编辑一次分类」把拿货成本率静默清零。
			CostRate: item.CostRate,
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
			// 父级不在结果里（被过滤/数据异常）：按顶级处理，避免分类凭空消失。
			roots = append(roots, node)
		}
	}
	// nodes 是 map，遍历顺序随机 —— 不排序的话同一份分类每次请求顺序都不同，
	// 下拉、树、折扣矩阵行的顺序会「每次刷新都不一样」，这是分类选择难用的根因。
	sortByOrder := func(items []*dto.CategoryInfo) {
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].SortOrder != items[j].SortOrder {
				return items[i].SortOrder < items[j].SortOrder
			}
			return items[i].ID < items[j].ID
		})
	}
	sortByOrder(roots)
	for _, node := range roots {
		if len(node.Children) > 0 {
			sortByOrder(node.Children)
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
		CostRate:  item.CostRate,
	}
}

// normalizeCostRate 归一成本率：只接受 (0, 1]，其余一律归 0（未配置）。
//
// 成本率是代理折扣的毛利校验基准，配错会把"亏本"判成"还有毛利"：
// 大于 1 的成本率意味着卖一单亏一单，直接归 0（跳过校验）比静默接受安全。
func normalizeCostRate(raw float64) float64 {
	if raw <= 0 || raw > 1 {
		return 0
	}
	return math.Round(raw*10000) / 10000
}
