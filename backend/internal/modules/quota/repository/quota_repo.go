package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/quota/dto"
	"hostsent/backend/internal/modules/quota/model"
)

// ResourceQuotaWithUser joins a resource quota with its username for list views.
type ResourceQuotaWithUser struct {
	model.ResourceQuota
	Username string `gorm:"column:username"`
}

// UserLevelWithTemplate 表示带默认模板名称的用户等级查询结果。
type UserLevelWithTemplate struct {
	model.UserLevel
	DefaultTemplateName string `gorm:"column:default_template_name"`
}

type ResourceQuotaRepository interface {
	List(ctx context.Context, query dto.QuotaListQuery) ([]ResourceQuotaWithUser, int64, error)
	FindByID(ctx context.Context, id uint64) (*ResourceQuotaWithUser, error)
	FindByUserID(ctx context.Context, userID uint64) ([]ResourceQuotaWithUser, error)
	Update(ctx context.Context, item *model.ResourceQuota) error
}

type QuotaTemplateRepository interface {
	List(ctx context.Context, query dto.QuotaTemplateListQuery) ([]model.QuotaTemplate, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.QuotaTemplate, error)
	FindItems(ctx context.Context, templateID uint64) ([]model.QuotaTemplateItem, error)
	Create(ctx context.Context, item *model.QuotaTemplate, items []model.QuotaTemplateItem) error
	Update(ctx context.Context, item *model.QuotaTemplate, items []model.QuotaTemplateItem) error
	Delete(ctx context.Context, id uint64) error
	FindAll(ctx context.Context) ([]model.QuotaTemplate, error)
}

// UserLevelRepository defines persistence operations for user levels.
type UserLevelRepository interface {
	List(ctx context.Context, query dto.UserLevelListQuery) ([]UserLevelWithTemplate, int64, error)
	FindByID(ctx context.Context, id uint64) (*UserLevelWithTemplate, error)
	Create(ctx context.Context, item *model.UserLevel) error
	Update(ctx context.Context, item *model.UserLevel) error
	Delete(ctx context.Context, id uint64) error
}

type QuotaAdjustmentRepository interface {
	List(ctx context.Context, query dto.QuotaAdjustmentListQuery) ([]model.QuotaAdjustmentLog, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.QuotaAdjustmentLog, error)
	Create(ctx context.Context, item *model.QuotaAdjustmentLog) error
}

type resourceQuotaRepository struct {
	db *gorm.DB
}

type quotaTemplateRepository struct {
	db *gorm.DB
}

type userLevelRepository struct {
	db *gorm.DB
}

type quotaAdjustmentRepository struct {
	db *gorm.DB
}

func NewResourceQuotaRepository(db *gorm.DB) ResourceQuotaRepository {
	return &resourceQuotaRepository{db: db}
}

// NewQuotaTemplateRepository 创建一个基于 GORM 的配额模板仓储。
func NewQuotaTemplateRepository(db *gorm.DB) QuotaTemplateRepository {
	return &quotaTemplateRepository{db: db}
}

func NewUserLevelRepository(db *gorm.DB) UserLevelRepository {
	return &userLevelRepository{db: db}
}

// NewQuotaAdjustmentRepository 创建一个由 GORM 支持的配额调整仓储。
func NewQuotaAdjustmentRepository(db *gorm.DB) QuotaAdjustmentRepository {
	return &quotaAdjustmentRepository{db: db}
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func (r *resourceQuotaRepository) List(ctx context.Context, query dto.QuotaListQuery) ([]ResourceQuotaWithUser, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	base := r.db.WithContext(ctx).Model(&model.ResourceQuota{}).
		Select("resource_quotas.*, users.username").
		Joins("left join users on users.id = resource_quotas.user_id")
	if query.UserID > 0 {
		base = base.Where("resource_quotas.user_id = ?", query.UserID)
	}
	if username := strings.TrimSpace(query.Username); username != "" {
		base = base.Where("users.username ILIKE ?", "%"+username+"%")
	}
	if quotaType := strings.TrimSpace(query.QuotaType); quotaType != "" {
		base = base.Where("resource_quotas.quota_type = ?", quotaType)
	}
	if source := strings.TrimSpace(query.Source); source != "" {
		base = base.Where("resource_quotas.source = ?", source)
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("resource_quotas.status = ?", status)
	}
	if flag := strings.TrimSpace(query.IsOverallocated); flag != "" {
		base = base.Where("resource_quotas.is_overallocated = ?", flag == "true")
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("users.username ILIKE ? OR resource_quotas.quota_name ILIKE ? OR resource_quotas.quota_code ILIKE ?", like, like, like)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []ResourceQuotaWithUser
	if err := base.Order("resource_quotas.last_adjusted_at desc, resource_quotas.id desc").Offset((page-1)*pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *resourceQuotaRepository) FindByID(ctx context.Context, id uint64) (*ResourceQuotaWithUser, error) {
	var item ResourceQuotaWithUser
	if err := r.db.WithContext(ctx).Model(&model.ResourceQuota{}).
		Select("resource_quotas.*, users.username").
		Joins("left join users on users.id = resource_quotas.user_id").
		Where("resource_quotas.id = ?", id).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *resourceQuotaRepository) FindByUserID(ctx context.Context, userID uint64) ([]ResourceQuotaWithUser, error) {
	var items []ResourceQuotaWithUser
	if err := r.db.WithContext(ctx).Model(&model.ResourceQuota{}).
		Select("resource_quotas.*, users.username").
		Joins("left join users on users.id = resource_quotas.user_id").
		Where("resource_quotas.user_id = ?", userID).
		Order("resource_quotas.quota_type asc, resource_quotas.id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *resourceQuotaRepository) Update(ctx context.Context, item *model.ResourceQuota) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *quotaTemplateRepository) List(ctx context.Context, query dto.QuotaTemplateListQuery) ([]model.QuotaTemplate, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	base := r.db.WithContext(ctx).Model(&model.QuotaTemplate{})
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("status = ?", status)
	}
	if scope := strings.TrimSpace(query.Scope); scope != "" {
		base = base.Where("scope = ?", scope)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("name ILIKE ? OR code ILIKE ? OR description ILIKE ?", like, like, like)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.QuotaTemplate
	if err := base.Order("updated_at desc, id desc").Offset((page-1)*pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *quotaTemplateRepository) FindByID(ctx context.Context, id uint64) (*model.QuotaTemplate, error) {
	var item model.QuotaTemplate
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *quotaTemplateRepository) FindItems(ctx context.Context, templateID uint64) ([]model.QuotaTemplateItem, error) {
	var items []model.QuotaTemplateItem
	if err := r.db.WithContext(ctx).Where("template_id = ?", templateID).Order("sort asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *quotaTemplateRepository) Create(ctx context.Context, item *model.QuotaTemplate, items []model.QuotaTemplateItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		if len(items) > 0 {
			for i := range items {
				items[i].TemplateID = item.ID
			}
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *quotaTemplateRepository) Update(ctx context.Context, item *model.QuotaTemplate, items []model.QuotaTemplateItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(item).Error; err != nil {
			return err
		}
		if err := tx.Where("template_id = ?", item.ID).Delete(&model.QuotaTemplateItem{}).Error; err != nil {
			return err
		}
		if len(items) > 0 {
			for i := range items {
				items[i].TemplateID = item.ID
			}
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *quotaTemplateRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("template_id = ?", id).Delete(&model.QuotaTemplateItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.QuotaTemplate{}, id).Error
	})
}

func (r *quotaTemplateRepository) FindAll(ctx context.Context) ([]model.QuotaTemplate, error) {
	var items []model.QuotaTemplate
	if err := r.db.WithContext(ctx).Order("name asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *userLevelRepository) List(ctx context.Context, query dto.UserLevelListQuery) ([]UserLevelWithTemplate, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	base := r.db.WithContext(ctx).Model(&model.UserLevel{}).
		Select("user_levels.*, quota_templates.name as default_template_name").
		Joins("left join quota_templates on quota_templates.id = user_levels.default_template_id")
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("user_levels.status = ?", status)
	}
	if query.DefaultTemplateID > 0 {
		base = base.Where("user_levels.default_template_id = ?", query.DefaultTemplateID)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("user_levels.name ILIKE ? OR user_levels.code ILIKE ? OR user_levels.description ILIKE ?", like, like, like)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []UserLevelWithTemplate
	if err := base.Order("user_levels.weight desc, user_levels.id desc").Offset((page-1)*pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *userLevelRepository) FindByID(ctx context.Context, id uint64) (*UserLevelWithTemplate, error) {
	var item UserLevelWithTemplate
	if err := r.db.WithContext(ctx).Model(&model.UserLevel{}).
		Select("user_levels.*, quota_templates.name as default_template_name").
		Joins("left join quota_templates on quota_templates.id = user_levels.default_template_id").
		Where("user_levels.id = ?", id).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *userLevelRepository) Create(ctx context.Context, item *model.UserLevel) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *userLevelRepository) Update(ctx context.Context, item *model.UserLevel) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *userLevelRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.UserLevel{}, id).Error
}

func (r *quotaAdjustmentRepository) List(ctx context.Context, query dto.QuotaAdjustmentListQuery) ([]model.QuotaAdjustmentLog, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	base := r.db.WithContext(ctx).Model(&model.QuotaAdjustmentLog{})
	if query.UserID > 0 {
		base = base.Where("user_id = ?", query.UserID)
	}
	if username := strings.TrimSpace(query.Username); username != "" {
		base = base.Where("username ILIKE ?", "%"+username+"%")
	}
	if quotaCode := strings.TrimSpace(query.QuotaCode); quotaCode != "" {
		base = base.Where("quota_code = ?", quotaCode)
	}
	if adjustmentType := strings.TrimSpace(query.AdjustmentType); adjustmentType != "" {
		base = base.Where("adjustment_type = ?", adjustmentType)
	}
	if source := strings.TrimSpace(query.Source); source != "" {
		base = base.Where("source = ?", source)
	}
	if operatorName := strings.TrimSpace(query.OperatorName); operatorName != "" {
		base = base.Where("operator_name ILIKE ?", "%"+operatorName+"%")
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.QuotaAdjustmentLog
	if err := base.Order("created_at desc, id desc").Offset((page-1)*pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *quotaAdjustmentRepository) FindByID(ctx context.Context, id uint64) (*model.QuotaAdjustmentLog, error) {
	var item model.QuotaAdjustmentLog
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *quotaAdjustmentRepository) Create(ctx context.Context, item *model.QuotaAdjustmentLog) error {
	return r.db.WithContext(ctx).Create(item).Error
}
