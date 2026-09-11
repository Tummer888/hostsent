package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
)

// InstanceRepository 开放实例只读查询（按 owner_user_id 隔离，D1）。
type InstanceRepository interface {
	// ListByUser 归属账号实例分页。
	ListByUser(ctx context.Context, userID uint64, status string, page, pageSize int) ([]syncmodel.Instance, int64, error)
	// FindByIDForUser 按 ID 读取实例并校验归属；不归属返回 ErrInstanceNotFound。
	FindByIDForUser(ctx context.Context, id, userID uint64) (*syncmodel.Instance, error)
}

// ErrInstanceNotFound 实例不存在或不归属该账号。
var ErrInstanceNotFound = errors.New("open: instance not found")

type instanceRepository struct {
	db *gorm.DB
}

// NewInstanceRepository 构造开放实例仓储。
func NewInstanceRepository(db *gorm.DB) InstanceRepository {
	return &instanceRepository{db: db}
}

func (r *instanceRepository) ListByUser(ctx context.Context, userID uint64, status string, page, pageSize int) ([]syncmodel.Instance, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	q := r.db.WithContext(ctx).Model(&syncmodel.Instance{}).Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []syncmodel.Instance
	err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	return items, total, err
}

func (r *instanceRepository) FindByIDForUser(ctx context.Context, id, userID uint64) (*syncmodel.Instance, error) {
	var row syncmodel.Instance
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}
	return &row, nil
}
