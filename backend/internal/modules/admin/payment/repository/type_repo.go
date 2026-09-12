package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/payment/model"
)

// TypeRepository 支付渠道类型注册表数据访问。
type TypeRepository interface {
	Upsert(ctx context.Context, t *model.PaymentType) error
	FindByType(ctx context.Context, providerType string) (*model.PaymentType, error)
	List(ctx context.Context) ([]model.PaymentType, error)
}

type typeRepository struct {
	db *gorm.DB
}

// NewTypeRepository 创建渠道类型仓储。
func NewTypeRepository(db *gorm.DB) TypeRepository {
	return &typeRepository{db: db}
}

func (r *typeRepository) Upsert(ctx context.Context, t *model.PaymentType) error {
	return r.db.WithContext(ctx).Where("type = ?", t.Type).
		Assign(map[string]interface{}{
			"name":            t.Name,
			"mode":            t.Mode,
			"descriptor":      t.DescriptorJSON,
			"adapter_version": t.AdapterVersion,
		}).FirstOrCreate(t).Error
}

func (r *typeRepository) FindByType(ctx context.Context, providerType string) (*model.PaymentType, error) {
	var t model.PaymentType
	if err := r.db.WithContext(ctx).Where("type = ?", providerType).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *typeRepository) List(ctx context.Context) ([]model.PaymentType, error) {
	var items []model.PaymentType
	err := r.db.WithContext(ctx).Order("sort_order asc, id asc").Find(&items).Error
	return items, err
}

// ReconRepository 渠道对账记录数据访问。
type ReconRepository interface {
	Create(ctx context.Context, rec *model.PaymentReconRecord) error
	List(ctx context.Context, page, pageSize int) ([]model.PaymentReconRecord, int64, error)
}

type reconRepository struct {
	db *gorm.DB
}

// NewReconRepository 创建对账记录仓储。
func NewReconRepository(db *gorm.DB) ReconRepository {
	return &reconRepository{db: db}
}

func (r *reconRepository) Create(ctx context.Context, rec *model.PaymentReconRecord) error {
	return r.db.WithContext(ctx).Create(rec).Error
}

func (r *reconRepository) List(ctx context.Context, page, pageSize int) ([]model.PaymentReconRecord, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.PaymentReconRecord{})
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.PaymentReconRecord
	err := base.Order("id desc").
		Offset((normalizePage(page) - 1) * normalizePageSize(pageSize)).
		Limit(normalizePageSize(pageSize)).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
