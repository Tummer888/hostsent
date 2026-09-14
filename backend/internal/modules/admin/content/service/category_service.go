package service

import (
	"context"
	"strings"

	"hostsent/backend/internal/modules/admin/content/dto"
	"hostsent/backend/internal/modules/admin/content/model"
	"hostsent/backend/internal/modules/admin/content/repository"
	"hostsent/backend/internal/pkg/revalidate"
)

// CategoryService 内容分类业务能力。
type CategoryService interface {
	List(ctx context.Context, req dto.CategoryListQuery) (*dto.CategoryListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.CategoryInfo, error)
	Create(ctx context.Context, req *dto.CategorySaveRequest) (*dto.CategoryInfo, error)
	Update(ctx context.Context, id uint64, req *dto.CategorySaveRequest) (*dto.CategoryInfo, error)
	Delete(ctx context.Context, id uint64) error
	// ListActiveTree 某类型下「启用中」的分类树，供门户公开接口读取（装配层注入 site 模块）。
	// 与 List 的差别：强制 status=active，运营下架的分栏/目录不会漏到公网。
	ListActiveTree(ctx context.Context, kind string) ([]dto.CategoryInfo, error)
}

type categoryService struct {
	repo repository.CategoryRepository
}

// NewCategoryService 创建分类业务服务。
func NewCategoryService(repo repository.CategoryRepository) CategoryService {
	return &categoryService{repo: repo}
}

func (s *categoryService) List(ctx context.Context, req dto.CategoryListQuery) (*dto.CategoryListResponse, error) {
	items, err := s.repo.List(ctx, repository.CategoryQuery{
		Kind:   strings.TrimSpace(req.Kind),
		Status: strings.TrimSpace(req.Status),
	})
	if err != nil {
		return nil, err
	}
	return &dto.CategoryListResponse{Items: buildCategoryTree(items)}, nil
}

func (s *categoryService) FindByID(ctx context.Context, id uint64) (*dto.CategoryInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrCategoryNotFound
	}
	info := toCategoryInfo(*item)
	return &info, nil
}

func (s *categoryService) Create(ctx context.Context, req *dto.CategorySaveRequest) (*dto.CategoryInfo, error) {
	kind := strings.TrimSpace(req.Kind)
	if !model.KindNeedsCategory(kind) {
		return nil, ErrInvalidKind
	}
	slug, err := s.resolveCategorySlug(ctx, kind, req.Slug, req.Name, 0)
	if err != nil {
		return nil, err
	}
	item := &model.Category{
		Kind:        kind,
		ParentID:    req.ParentID,
		Name:        strings.TrimSpace(req.Name),
		Slug:        slug,
		Description: strings.TrimSpace(req.Description),
		Icon:        strings.TrimSpace(req.Icon),
		SortOrder:   req.SortOrder,
		Status:      defaultCategoryStatus(req.Status),
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	// 分类树渲染在门户 /news 与 /help 的侧栏，改动要立刻可见。
	revalidate.Notify(ctx, revalidate.KeyArticle)
	return s.FindByID(ctx, item.ID)
}

func (s *categoryService) Update(ctx context.Context, id uint64, req *dto.CategorySaveRequest) (*dto.CategoryInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrCategoryNotFound
	}
	kind := strings.TrimSpace(req.Kind)
	if !model.KindNeedsCategory(kind) {
		return nil, ErrInvalidKind
	}
	// 不允许把分类挂到自己下面：会形成自环，树构建时整棵子树消失。
	if req.ParentID == id {
		req.ParentID = 0
	}
	slug, serr := s.resolveCategorySlug(ctx, kind, req.Slug, req.Name, item.ID)
	if serr != nil {
		return nil, serr
	}
	item.Kind = kind
	item.ParentID = req.ParentID
	item.Name = strings.TrimSpace(req.Name)
	item.Slug = slug
	item.Description = strings.TrimSpace(req.Description)
	item.Icon = strings.TrimSpace(req.Icon)
	item.SortOrder = req.SortOrder
	item.Status = defaultCategoryStatus(req.Status)
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	revalidate.Notify(ctx, revalidate.KeyArticle)
	return s.FindByID(ctx, item.ID)
}

func (s *categoryService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return ErrCategoryNotFound
	}
	exists, err := s.repo.ExistsChildren(ctx, id)
	if err != nil {
		return err
	}
	if exists {
		return ErrCategoryHasChild
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	revalidate.Notify(ctx, revalidate.KeyArticle)
	return nil
}

// ListActiveTree 门户用的启用分类树。
func (s *categoryService) ListActiveTree(ctx context.Context, kind string) ([]dto.CategoryInfo, error) {
	normalized := strings.TrimSpace(kind)
	if !model.KindNeedsCategory(normalized) {
		return []dto.CategoryInfo{}, nil
	}
	items, err := s.repo.ListActive(ctx, normalized)
	if err != nil {
		return nil, err
	}
	tree := buildCategoryTree(items)
	out := make([]dto.CategoryInfo, 0, len(tree))
	for _, node := range tree {
		out = append(out, *node)
	}
	return out, nil
}

func (s *categoryService) resolveCategorySlug(ctx context.Context, kind, raw, name string, excludeID uint64) (string, error) {
	base := slugify(raw)
	if base == "" {
		base = slugify(name)
	}
	if base == "" {
		base = "category"
	}
	base = truncateRunes(base, 48)

	candidate := base
	for i := 2; i <= 50; i++ {
		exists, err := s.repo.ExistsSlug(ctx, kind, candidate, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = truncateRunes(base, 44) + "-" + itoa(i)
	}
	return "", ErrSlugRequired
}

func defaultCategoryStatus(status string) string {
	switch strings.TrimSpace(status) {
	case model.CategoryStatusDisabled:
		return model.CategoryStatusDisabled
	default:
		return model.CategoryStatusActive
	}
}

// buildCategoryTree 把平铺列表组装成树。
//
// 父节点缺失（被删/被禁用过滤掉）时把该节点提升为根，而不是丢弃 ——
// 丢弃会让运营「改了一个父分类的状态，子分类就整个不见了」。
func buildCategoryTree(items []model.Category) []*dto.CategoryInfo {
	nodes := make(map[uint64]*dto.CategoryInfo, len(items))
	for _, item := range items {
		info := toCategoryInfo(item)
		nodes[item.ID] = &info
	}

	roots := make([]*dto.CategoryInfo, 0, len(items))
	for _, item := range items {
		node := nodes[item.ID]
		if item.ParentID == 0 {
			roots = append(roots, node)
			continue
		}
		if parent, ok := nodes[item.ParentID]; ok && parent != node {
			parent.Children = append(parent.Children, node)
			continue
		}
		roots = append(roots, node)
	}
	return roots
}

func toCategoryInfo(c model.Category) dto.CategoryInfo {
	return dto.CategoryInfo{
		ID:          c.ID,
		Kind:        c.Kind,
		ParentID:    c.ParentID,
		Name:        c.Name,
		Slug:        c.Slug,
		Description: c.Description,
		Icon:        c.Icon,
		SortOrder:   c.SortOrder,
		Status:      c.Status,
	}
}

// itoa 避免为几处小整数拼接引入 strconv。
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [8]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
