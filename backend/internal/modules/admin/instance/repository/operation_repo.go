package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/instance/model"
)

// OperationRepository 实例操作流水仓储接口。
type OperationRepository interface {
	Create(ctx context.Context, op *model.Operation) error
	List(ctx context.Context, instanceID uint64, page, pageSize int) ([]model.Operation, int64, error)
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
