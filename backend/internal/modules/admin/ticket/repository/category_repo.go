package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/ticket/model"
)

// CategoryRepository 定义工单分类数据访问能力。
type CategoryRepository interface {
	List(ctx context.Context, includeDisabled bool) ([]model.TicketCategory, error)
	FindByID(ctx context.Context, id uint64) (*model.TicketCategory, error)
	FindByCode(ctx context.Context, code string) (*model.TicketCategory, error)
	Create(ctx context.Context, item *model.TicketCategory) error
	Update(ctx context.Context, item *model.TicketCategory) error
	Delete(ctx context.Context, id uint64) error
}

type categoryRepository struct {
	db *gorm.DB
}

// NewCategoryRepository 创建工单分类仓储实现。
func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) List(ctx context.Context, includeDisabled bool) ([]model.TicketCategory, error) {
	query := r.db.WithContext(ctx).Model(&model.TicketCategory{})
	if !includeDisabled {
		query = query.Where("status = ?", model.CategoryStatusActive)
	}
	var items []model.TicketCategory
	if err := query.Order("sort_order asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *categoryRepository) FindByID(ctx context.Context, id uint64) (*model.TicketCategory, error) {
	var item model.TicketCategory
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *categoryRepository) FindByCode(ctx context.Context, code string) (*model.TicketCategory, error) {
	var item model.TicketCategory
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *categoryRepository) Create(ctx context.Context, item *model.TicketCategory) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *categoryRepository) Update(ctx context.Context, item *model.TicketCategory) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *categoryRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.TicketCategory{}, id).Error
}
