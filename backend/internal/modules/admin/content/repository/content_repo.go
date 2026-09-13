// Package repository 提供内容中心的数据访问实现。
package repository

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/content/model"
)

// 列表查询上限：防止管理端传 page_size=100000 把整表拉进内存。
const maxPageSize = 100

// ArticleQuery 文章列表过滤条件（仓储层结构，由 service 从 dto 映射而来）。
type ArticleQuery struct {
	Kind       string
	CategoryID uint64
	Status     string
	Keyword    string
	Page       int
	PageSize   int
}

// CategoryQuery 分类过滤条件。
type CategoryQuery struct {
	Kind   string
	Status string
}

// LinkQuery 友情链接过滤条件。
type LinkQuery struct {
	Status   string
	Keyword  string
	Page     int
	PageSize int
}

// ArticleRepository 内容文章数据访问能力。
type ArticleRepository interface {
	List(ctx context.Context, q ArticleQuery) ([]model.Article, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.Article, error)
	// FindBySlug 按类型 + slug 取单篇（门户详情用）。
	FindBySlug(ctx context.Context, kind, slug string) (*model.Article, error)
	// ListPublished 取已发布且未下线的内容，按 pinned DESC, sort_order ASC, publish_at DESC 排序。
	ListPublished(ctx context.Context, kind string, categoryID uint64, limit int) ([]model.Article, error)
	// FindPublishedBySlug 取单篇已发布内容；不存在返回 nil（门户据此判 404）。
	FindPublishedBySlug(ctx context.Context, kind, slug string) (*model.Article, error)
	// FindPublishedSingleton 取该类型唯一一篇已发布内容（条款/隐私用，这类只应有一篇）。
	FindPublishedSingleton(ctx context.Context, kind string) (*model.Article, error)
	// ExistsSlug 判断同类型下 slug 是否已被占用（排除 excludeID 自身）。
	ExistsSlug(ctx context.Context, kind, slug string, excludeID uint64) (bool, error)
	Create(ctx context.Context, item *model.Article) error
	Update(ctx context.Context, item *model.Article) error
	Delete(ctx context.Context, id uint64) error
}

// CategoryRepository 内容分类数据访问能力。
type CategoryRepository interface {
	List(ctx context.Context, q CategoryQuery) ([]model.Category, error)
	FindByID(ctx context.Context, id uint64) (*model.Category, error)
	// FindByIDs 批量取分类名（列表页展示分类名，避免 N+1）。
	FindByIDs(ctx context.Context, ids []uint64) (map[uint64]model.Category, error)
	ExistsSlug(ctx context.Context, kind, slug string, excludeID uint64) (bool, error)
	ExistsChildren(ctx context.Context, id uint64) (bool, error)
	Create(ctx context.Context, item *model.Category) error
	Update(ctx context.Context, item *model.Category) error
	Delete(ctx context.Context, id uint64) error
}

// LinkRepository 友情链接数据访问能力。
type LinkRepository interface {
	List(ctx context.Context, q LinkQuery) ([]model.FriendlyLink, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.FriendlyLink, error)
	ListActive(ctx context.Context) ([]model.FriendlyLink, error)
	Create(ctx context.Context, item *model.FriendlyLink) error
	Update(ctx context.Context, item *model.FriendlyLink) error
	Delete(ctx context.Context, id uint64) error
}

/* ----------------------------- 文章 ----------------------------- */

type articleRepository struct{ db *gorm.DB }

// NewArticleRepository 创建文章仓储实现。
func NewArticleRepository(db *gorm.DB) ArticleRepository {
	return &articleRepository{db: db}
}

func (r *articleRepository) List(ctx context.Context, q ArticleQuery) ([]model.Article, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.Article{})
	if q.Kind != "" {
		base = base.Where("kind = ?", q.Kind)
	}
	if q.CategoryID > 0 {
		base = base.Where("category_id = ?", q.CategoryID)
	}
	if q.Status != "" {
		base = base.Where("status = ?", q.Status)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		base = base.Where("title ILIKE ?", "%"+escapeLike(kw)+"%")
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := normalizePage(q.Page, q.PageSize)
	var items []model.Article
	if err := base.
		Order("pinned DESC, sort_order ASC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *articleRepository) FindByID(ctx context.Context, id uint64) (*model.Article, error) {
	var item model.Article
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *articleRepository) FindBySlug(ctx context.Context, kind, slug string) (*model.Article, error) {
	var item model.Article
	if err := r.db.WithContext(ctx).
		Where("kind = ? AND slug = ?", kind, slug).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *articleRepository) ListPublished(ctx context.Context, kind string, categoryID uint64, limit int) ([]model.Article, error) {
	q := r.db.WithContext(ctx).Model(&model.Article{}).
		Where("status = ?", model.StatusPublished).
		Where("offline_at IS NULL")
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	if categoryID > 0 {
		q = q.Where("category_id = ?", categoryID)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	var items []model.Article
	if err := q.Order("pinned DESC, sort_order ASC, publish_at DESC, id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *articleRepository) FindPublishedBySlug(ctx context.Context, kind, slug string) (*model.Article, error) {
	var item model.Article
	err := r.db.WithContext(ctx).
		Where("kind = ? AND slug = ? AND status = ? AND offline_at IS NULL", kind, slug, model.StatusPublished).
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // 门户据此判 404，而不是当故障
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *articleRepository) FindPublishedSingleton(ctx context.Context, kind string) (*model.Article, error) {
	items, err := r.ListPublished(ctx, kind, 0, 1)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (r *articleRepository) ExistsSlug(ctx context.Context, kind, slug string, excludeID uint64) (bool, error) {
	q := r.db.WithContext(ctx).Model(&model.Article{}).Where("kind = ? AND slug = ?", kind, slug)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *articleRepository) Create(ctx context.Context, item *model.Article) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *articleRepository) Update(ctx context.Context, item *model.Article) error {
	// 用 Select 精确指定字段：offline_at 需要在「重新发布」时被置回 NULL，
	// GORM 的 Save 对 nil 指针不生成 SET NULL 语句，会残留下线标记导致内容发布后仍不可见。
	return r.db.WithContext(ctx).Model(item).
		Select("kind", "category_id", "slug", "title", "summary", "body", "body_format",
			"cover", "tags", "status", "pinned", "sort_order", "version",
			"publish_at", "offline_at", "operator_id").
		Updates(item).Error
}

func (r *articleRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Article{}, id).Error
}

/* ----------------------------- 分类 ----------------------------- */

type categoryRepository struct{ db *gorm.DB }

// NewCategoryRepository 创建分类仓储实现。
func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) List(ctx context.Context, q CategoryQuery) ([]model.Category, error) {
	base := r.db.WithContext(ctx).Model(&model.Category{})
	if q.Kind != "" {
		base = base.Where("kind = ?", q.Kind)
	}
	if q.Status != "" {
		base = base.Where("status = ?", q.Status)
	}
	var items []model.Category
	if err := base.Order("parent_id ASC, sort_order ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *categoryRepository) FindByID(ctx context.Context, id uint64) (*model.Category, error) {
	var item model.Category
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *categoryRepository) FindByIDs(ctx context.Context, ids []uint64) (map[uint64]model.Category, error) {
	out := make(map[uint64]model.Category, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var items []model.Category
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, err
	}
	for _, item := range items {
		out[item.ID] = item
	}
	return out, nil
}

func (r *categoryRepository) ExistsSlug(ctx context.Context, kind, slug string, excludeID uint64) (bool, error) {
	q := r.db.WithContext(ctx).Model(&model.Category{}).Where("kind = ? AND slug = ?", kind, slug)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *categoryRepository) ExistsChildren(ctx context.Context, id uint64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.Category{}).Where("parent_id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *categoryRepository) Create(ctx context.Context, item *model.Category) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *categoryRepository) Update(ctx context.Context, item *model.Category) error {
	return r.db.WithContext(ctx).Model(item).
		Select("kind", "parent_id", "name", "slug", "description", "icon", "sort_order", "status").
		Updates(item).Error
}

func (r *categoryRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Category{}, id).Error
}

/* --------------------------- 友情链接 --------------------------- */

type linkRepository struct{ db *gorm.DB }

// NewLinkRepository 创建友情链接仓储实现。
func NewLinkRepository(db *gorm.DB) LinkRepository {
	return &linkRepository{db: db}
}

func (r *linkRepository) List(ctx context.Context, q LinkQuery) ([]model.FriendlyLink, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.FriendlyLink{})
	if q.Status != "" {
		base = base.Where("status = ?", q.Status)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		base = base.Where("name ILIKE ?", "%"+escapeLike(kw)+"%")
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := normalizePage(q.Page, q.PageSize)
	var items []model.FriendlyLink
	if err := base.Order("sort_order ASC, id ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *linkRepository) FindByID(ctx context.Context, id uint64) (*model.FriendlyLink, error) {
	var item model.FriendlyLink
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *linkRepository) ListActive(ctx context.Context) ([]model.FriendlyLink, error) {
	var items []model.FriendlyLink
	if err := r.db.WithContext(ctx).
		Where("status = ?", model.CategoryStatusActive).
		Order("sort_order ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *linkRepository) Create(ctx context.Context, item *model.FriendlyLink) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *linkRepository) Update(ctx context.Context, item *model.FriendlyLink) error {
	return r.db.WithContext(ctx).Model(item).
		Select("name", "url", "logo", "description", "sort_order", "open_in_new", "status").
		Updates(item).Error
}

func (r *linkRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.FriendlyLink{}, id).Error
}

/* ----------------------------- 工具 ----------------------------- */

// normalizePage 归一化分页参数：页码从 1 起，单页上限 maxPageSize。
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > maxPageSize {
		pageSize = 20
	}
	return page, pageSize
}

// escapeLike 转义 LIKE 通配符，避免用户输入的 % 变成「匹配任意内容」。
func escapeLike(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(s)
}
