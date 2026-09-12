package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/product/pricing/model"
)

// PriceMatrixRepository 周期价格矩阵数据访问（doc25 §3.1）。
type PriceMatrixRepository interface {
	// List 取某商品（可指定 SKU，0=商品级）在指定币种下的全部档位。
	List(ctx context.Context, productID, specID uint64, currency string) ([]model.ProductPrice, error)
	// FindActive 取指定 (商品, SKU, 周期) 启用中的价格行；无则返回 nil。
	FindActive(ctx context.Context, productID, specID uint64, cycle, currency string) (*model.ProductPrice, error)
	// ListByProductIDs 批量取商品级价格行（列表展示/批量算价用）。
	ListByProductIDs(ctx context.Context, productIDs []uint64, currency string) ([]model.ProductPrice, error)
	// Upsert 按 (商品, SKU, 周期, 币种) 唯一键写入或更新。
	Upsert(ctx context.Context, item *model.ProductPrice) error
	// DeleteMissing 删除某商品（或 SKU）下不在 keepCycles 中的档位，返回删除行数。
	DeleteMissing(ctx context.Context, productID, specID uint64, currency string, keepCycles []string) (int64, error)
	// DeleteByProduct 删除商品全部档位（商品删除时级联清理）。
	DeleteByProduct(ctx context.Context, productID uint64) error
}

type priceMatrixRepository struct {
	db *gorm.DB
}

// NewPriceMatrixRepository 创建周期价格矩阵仓储。
func NewPriceMatrixRepository(db *gorm.DB) PriceMatrixRepository {
	return &priceMatrixRepository{db: db}
}

func (r *priceMatrixRepository) List(ctx context.Context, productID, specID uint64, currency string) ([]model.ProductPrice, error) {
	q := r.db.WithContext(ctx).Where("product_id = ? AND product_spec_id = ?", productID, specID)
	if currency != "" {
		q = q.Where("currency = ?", currency)
	}
	var items []model.ProductPrice
	if err := q.Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *priceMatrixRepository) FindActive(ctx context.Context, productID, specID uint64, cycle, currency string) (*model.ProductPrice, error) {
	// 先按 SKU 精确匹配；调用方未指定 SKU 时 specID=0 即商品级。
	var item model.ProductPrice
	err := r.db.WithContext(ctx).
		Where("product_id = ? AND product_spec_id = ? AND cycle = ? AND status = ?",
			productID, specID, cycle, PriceStatusEnabled).
		Where("currency = ?", currency).
		Order("id asc").
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *priceMatrixRepository) ListByProductIDs(ctx context.Context, productIDs []uint64, currency string) ([]model.ProductPrice, error) {
	if len(productIDs) == 0 {
		return nil, nil
	}
	q := r.db.WithContext(ctx).Where("product_id IN ?", productIDs)
	if currency != "" {
		q = q.Where("currency = ?", currency)
	}
	var items []model.ProductPrice
	if err := q.Order("product_id asc, product_spec_id asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Upsert 依 (product_id, product_spec_id, cycle, currency) 唯一键写入或更新。
func (r *priceMatrixRepository) Upsert(ctx context.Context, item *model.ProductPrice) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "product_id"}, {Name: "product_spec_id"}, {Name: "cycle"}, {Name: "currency"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"price", "cost_price", "setup_fee", "cost_setup_fee", "source", "status", "remark", "updated_at",
		}),
	}).Create(item).Error
}

func (r *priceMatrixRepository) DeleteMissing(ctx context.Context, productID, specID uint64, currency string, keepCycles []string) (int64, error) {
	q := r.db.WithContext(ctx).
		Where("product_id = ? AND product_spec_id = ? AND currency = ?", productID, specID, currency)
	if len(keepCycles) > 0 {
		q = q.Where("cycle NOT IN ?", keepCycles)
	}
	res := q.Delete(&model.ProductPrice{})
	return res.RowsAffected, res.Error
}

func (r *priceMatrixRepository) DeleteByProduct(ctx context.Context, productID uint64) error {
	return r.db.WithContext(ctx).Where("product_id = ?", productID).Delete(&model.ProductPrice{}).Error
}

// PriceStatusEnabled / PriceStatusDisabled 矩阵档位启停（与 model 同口径，避免调用方依赖 model 包）。
const (
	PriceStatusDisabled = 0
	PriceStatusEnabled  = 1
)
