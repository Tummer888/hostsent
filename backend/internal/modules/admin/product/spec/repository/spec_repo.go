// Package repository 提供规格管理（spec）子域的数据访问实现。
package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/product/spec/dto"
	"hostsent/backend/internal/modules/admin/product/spec/model"
)

// SpecTemplateRepository 定义规格模板数据访问能力。
type SpecTemplateRepository interface {
	List(ctx context.Context, query dto.SpecTemplateQuery) ([]model.SpecTemplate, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.SpecTemplate, error)
	Create(ctx context.Context, item *model.SpecTemplate) error
	Update(ctx context.Context, item *model.SpecTemplate) error
	Delete(ctx context.Context, id uint64) error
}

type specTemplateRepository struct {
	db *gorm.DB
}

func NewSpecTemplateRepository(db *gorm.DB) SpecTemplateRepository {
	return &specTemplateRepository{db: db}
}

func (r *specTemplateRepository) List(ctx context.Context, query dto.SpecTemplateQuery) ([]model.SpecTemplate, int64, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	base := r.db.WithContext(ctx).Model(&model.SpecTemplate{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("name ILIKE ?", like)
	}
	if query.SpecFamily != "" {
		base = base.Where("spec_family = ?", query.SpecFamily)
	}
	if query.Status != 0 {
		base = base.Where("status = ?", query.Status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.SpecTemplate
	if err := base.Order("spec_family asc, sort_order asc, id asc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *specTemplateRepository) FindByID(ctx context.Context, id uint64) (*model.SpecTemplate, error) {
	var item model.SpecTemplate
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *specTemplateRepository) Create(ctx context.Context, item *model.SpecTemplate) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *specTemplateRepository) Update(ctx context.Context, item *model.SpecTemplate) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *specTemplateRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.SpecTemplate{}, id).Error
}

// SpecMappingRepository 定义规格映射数据访问能力。
type SpecMappingRepository interface {
	List(ctx context.Context, query dto.SpecMappingQuery) ([]model.SpecMapping, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.SpecMapping, error)
	Create(ctx context.Context, item *model.SpecMapping) error
	Update(ctx context.Context, item *model.SpecMapping) error
	Delete(ctx context.Context, id uint64) error
}

type specMappingRepository struct {
	db *gorm.DB
}

func NewSpecMappingRepository(db *gorm.DB) SpecMappingRepository {
	return &specMappingRepository{db: db}
}

func (r *specMappingRepository) List(ctx context.Context, query dto.SpecMappingQuery) ([]model.SpecMapping, int64, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	base := r.db.WithContext(ctx).Model(&model.SpecMapping{})
	if query.ProviderType != "" {
		base = base.Where("provider_type = ?", query.ProviderType)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("upstream_spec_id ILIKE ? OR upstream_name ILIKE ? OR platform_name ILIKE ?", like, like, like)
	}
	if query.Status != 0 {
		base = base.Where("status = ?", query.Status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.SpecMapping
	if err := base.Order("provider_type asc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *specMappingRepository) FindByID(ctx context.Context, id uint64) (*model.SpecMapping, error) {
	var item model.SpecMapping
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *specMappingRepository) Create(ctx context.Context, item *model.SpecMapping) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *specMappingRepository) Update(ctx context.Context, item *model.SpecMapping) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *specMappingRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.SpecMapping{}, id).Error
}
