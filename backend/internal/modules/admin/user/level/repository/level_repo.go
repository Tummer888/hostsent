// Package repository 提供用户等级模块的数据访问实现。
package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/level/dto"
	"hostsent/backend/internal/modules/admin/user/level/model"
)

// UserLevelRepository 定义用户等级的持久化操作。
type UserLevelRepository interface {
	List(ctx context.Context, query dto.ListQuery) ([]model.UserLevel, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.UserLevel, error)
	FindByCode(ctx context.Context, code string) (*model.UserLevel, error)
	Create(ctx context.Context, item *model.UserLevel) error
	Update(ctx context.Context, item *model.UserLevel) error
	Delete(ctx context.Context, id uint64) error

	// —— 消费升级支撑（P3-03）——
	// FindBestByThreshold 取「升级门槛 ≤ amount 且启用中」的最高权重等级。
	FindBestByThreshold(ctx context.Context, amount float64) (*model.UserLevel, error)
	// GetUserTier 读取用户当前等级 ID 与累计消费额。
	GetUserTier(ctx context.Context, userID uint64) (*uint64, float64, error)
	// AddUserConsume 原子累加累计消费额，返回累加后的值。
	AddUserConsume(ctx context.Context, userID uint64, delta float64) (float64, error)
	// SetUserTier 更新用户当前等级 ID。
	SetUserTier(ctx context.Context, userID uint64, levelID uint64) error
	// CreateChangeLog 写入等级变更日志。
	CreateChangeLog(ctx context.Context, item *model.UserLevelChangeLog) error
}

type userLevelRepository struct {
	db *gorm.DB
}

// NewUserLevelRepository 创建用户等级仓储实现。
func NewUserLevelRepository(db *gorm.DB) UserLevelRepository {
	return &userLevelRepository{db: db}
}

func (r *userLevelRepository) List(ctx context.Context, query dto.ListQuery) ([]model.UserLevel, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	base := r.db.WithContext(ctx).Model(&model.UserLevel{})
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("name ILIKE ? OR code ILIKE ? OR description ILIKE ?", like, like, like)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.UserLevel
	if err := base.Order("weight desc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *userLevelRepository) FindByID(ctx context.Context, id uint64) (*model.UserLevel, error) {
	var item model.UserLevel
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *userLevelRepository) FindByCode(ctx context.Context, code string) (*model.UserLevel, error) {
	var item model.UserLevel
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&item).Error; err != nil {
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

// FindBestByThreshold 取满足门槛的最高权重等级；没有满足条件的等级时返回 gorm.ErrRecordNotFound。
func (r *userLevelRepository) FindBestByThreshold(ctx context.Context, amount float64) (*model.UserLevel, error) {
	var item model.UserLevel
	if err := r.db.WithContext(ctx).
		Where("status = ? AND upgrade_threshold <= ?", "active", amount).
		Order("weight desc, upgrade_threshold desc, id desc").
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// GetUserTier 读取 users 表的 user_level_id 与 total_consume_amount。
func (r *userLevelRepository) GetUserTier(ctx context.Context, userID uint64) (*uint64, float64, error) {
	var row struct {
		UserLevelID        *uint64
		TotalConsumeAmount float64
	}
	if err := r.db.WithContext(ctx).
		Table("users").
		Select("user_level_id, total_consume_amount").
		Where("id = ?", userID).
		Take(&row).Error; err != nil {
		return nil, 0, err
	}
	return row.UserLevelID, row.TotalConsumeAmount, nil
}

// AddUserConsume 原子累加累计消费额，并返回累加后的最新值。
func (r *userLevelRepository) AddUserConsume(ctx context.Context, userID uint64, delta float64) (float64, error) {
	if err := r.db.WithContext(ctx).
		Table("users").
		Where("id = ?", userID).
		UpdateColumn("total_consume_amount", gorm.Expr("total_consume_amount + ?", delta)).Error; err != nil {
		return 0, err
	}
	_, total, err := r.GetUserTier(ctx, userID)
	return total, err
}

// SetUserTier 更新用户当前等级。
func (r *userLevelRepository) SetUserTier(ctx context.Context, userID uint64, levelID uint64) error {
	return r.db.WithContext(ctx).
		Table("users").
		Where("id = ?", userID).
		UpdateColumn("user_level_id", levelID).Error
}

// CreateChangeLog 写入等级变更日志。
func (r *userLevelRepository) CreateChangeLog(ctx context.Context, item *model.UserLevelChangeLog) error {
	return r.db.WithContext(ctx).Create(item).Error
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
