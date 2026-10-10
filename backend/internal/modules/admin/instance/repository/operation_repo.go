package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/instance/dto"
	"hostsent/backend/internal/modules/admin/instance/model"
)

// OperationRepository 实例操作流水仓储接口。
type OperationRepository interface {
	Create(ctx context.Context, op *model.Operation) error
	List(ctx context.Context, instanceID uint64, page, pageSize int) ([]model.Operation, int64, error)
	// ListGlobal 跨实例操作流水分页（全局审计视图），支持按动作/操作人类型/结果/用户/关键词/时间筛。
	ListGlobal(ctx context.Context, q *dto.OperationLogQuery) ([]OperationLogRow, int64, error)
}

// OperationLogRow 全局流水行（联表实例标识与归属用户名，便于一屏阅读）。
type OperationLogRow struct {
	model.Operation
	Username string `gorm:"column:username"`
}

type operationRepository struct {
	db *gorm.DB
}

// NewOperationRepository 创建操作流水仓储。
func NewOperationRepository(db *gorm.DB) OperationRepository {
	return &operationRepository{db: db}
}

// Create 落库一条操作流水。
func (r *operationRepository) Create(ctx context.Context, op *model.Operation) error {
	return r.db.WithContext(ctx).Create(op).Error
}

// List 按实例倒序分页查询操作流水。
func (r *operationRepository) List(ctx context.Context, instanceID uint64, page, pageSize int) ([]model.Operation, int64, error) {
	page, pageSize = normalizePage(page, pageSize)

	base := r.db.WithContext(ctx).Model(&model.Operation{}).Where("instance_id = ?", instanceID)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Operation
	err := r.db.WithContext(ctx).Model(&model.Operation{}).
		Where("instance_id = ?", instanceID).
		Order("id DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListGlobal 跨实例操作流水分页（全局审计视图）。
func (r *operationRepository) ListGlobal(ctx context.Context, q *dto.OperationLogQuery) ([]OperationLogRow, int64, error) {
	page, pageSize := normalizePage(q.Page, q.PageSize)

	build := func() *gorm.DB {
		db := r.db.WithContext(ctx).Table("instance_operations AS o").
			Joins("LEFT JOIN users AS u ON u.id = o.user_id")
		if a := strings.TrimSpace(q.Action); a != "" {
			db = db.Where("o.action = ?", a)
		}
		if t := strings.TrimSpace(q.OperatorType); t != "" {
			db = db.Where("o.operator_type = ?", t)
		}
		if res := strings.TrimSpace(q.Result); res != "" {
			db = db.Where("o.result = ?", res)
		}
		if q.UserID > 0 {
			db = db.Where("o.user_id = ?", q.UserID)
		}
		if kw := strings.TrimSpace(q.Keyword); kw != "" {
			like := "%" + kw + "%"
			db = db.Where("o.instance_mark ILIKE ? OR o.operator_name ILIKE ? OR u.username ILIKE ?", like, like, like)
		}
		if q.StartTime != "" {
			db = db.Where("o.created_at >= ?", q.StartTime)
		}
		if q.EndTime != "" {
			db = db.Where("o.created_at <= ?", q.EndTime)
		}
		return db
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []OperationLogRow
	err := build().
		Select("o.*, COALESCE(u.username, '') AS username").
		Order("o.id DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
