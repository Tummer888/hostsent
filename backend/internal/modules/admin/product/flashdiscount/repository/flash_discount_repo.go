// Package repository 提供限时活动折扣的数据访问（doc108 §8J）。
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/product/flashdiscount/dto"
	"hostsent/backend/internal/modules/admin/product/flashdiscount/model"
)

// 业务错误（handler 映射 409/404）。
var (
	// ErrCodeTaken 活动编码重复（唯一索引 uk_flash_discounts_code）。
	ErrCodeTaken = errors.New("活动编码已存在")
	// ErrNotFound 活动不存在。
	ErrNotFound = errors.New("活动折扣不存在")
)

// Repository 限时活动折扣的持久化接口。
type Repository interface {
	List(ctx context.Context, query dto.Query) ([]model.FlashDiscount, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.FlashDiscount, error)
	// Create 活动 + 定向条目同事务写入。
	Create(ctx context.Context, item *model.FlashDiscount, items []model.FlashDiscountItem) error
	// Update 活动 +（可选）整体覆盖条目，同事务。
	Update(ctx context.Context, item *model.FlashDiscount, items []model.FlashDiscountItem, replaceItems bool) error
	Delete(ctx context.Context, id uint64) error
	Items(ctx context.Context, discountID uint64) ([]model.FlashDiscountItem, error)

	// —— 算价支撑 ——
	// ActiveAt 取指定时刻处于生效窗口内、且已启用的活动（含条目）。
	// 算价对每个商品调一次，因此这里一次查全（活动数量是运营手配的个位数）。
	ActiveAt(ctx context.Context, now time.Time) ([]ActiveDiscount, error)
	// CategoryOfProduct 商品的分类 ID（0 = 未归类）；供活动按分类命中。
	CategoryOfProduct(ctx context.Context, productID uint64) (uint64, error)

	// —— 名称解析（条目展示）——
	CategoryNames(ctx context.Context, ids []uint64) (map[uint64]string, error)
	ProductNames(ctx context.Context, ids []uint64) (map[uint64]string, error)
	// CategoryCostRate 分类成本率（0 = 未配置）。
	CategoryCostRate(ctx context.Context, id uint64) (float64, error)
	// ProductCostRate 商品成本率 = cost_price / price（0 = 无基准）。
	ProductCostRate(ctx context.Context, id uint64) (float64, error)
}

// ActiveDiscount 生效中的活动 + 其定向条目，供算价命中判定。
type ActiveDiscount struct {
	Discount model.FlashDiscount
	Items    []model.FlashDiscountItem
}

type flashRepository struct {
	db *gorm.DB
}

// NewRepository 创建活动折扣仓储。
func NewRepository(db *gorm.DB) Repository {
	return &flashRepository{db: db}
}

func (r *flashRepository) List(ctx context.Context, query dto.Query) ([]model.FlashDiscount, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	base := r.db.WithContext(ctx).Model(&model.FlashDiscount{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("name ILIKE ? OR code ILIKE ? OR description ILIKE ?", like, like, like)
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("status = ?", status)
	}
	// running=true：只看此刻在窗口内的活动（时间窗实时判断，status 只表达启停意图）。
	if strings.EqualFold(strings.TrimSpace(query.Running), "true") {
		now := time.Now()
		base = base.Where("status = ?", model.StatusActive).
			Where("(start_at IS NULL OR start_at <= ?)", now).
			Where("(end_at IS NULL OR end_at >= ?)", now)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.FlashDiscount
	// 行内带条目数：列表要显示「N 个目标」，逐行再查一次是 N+1。
	if err := base.
		Select("flash_discounts.*, COUNT(flash_discount_items.id) AS item_count").
		Joins("LEFT JOIN flash_discount_items ON flash_discount_items.discount_id = flash_discounts.id").
		Group("flash_discounts.id").
		Order("flash_discounts.id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *flashRepository) FindByID(ctx context.Context, id uint64) (*model.FlashDiscount, error) {
	var item model.FlashDiscount
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *flashRepository) Create(ctx context.Context, item *model.FlashDiscount, items []model.FlashDiscountItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return mapUniqueViolation(err)
		}
		return saveItems(tx, item.ID, items)
	})
}

func (r *flashRepository) Update(ctx context.Context, item *model.FlashDiscount, items []model.FlashDiscountItem, replaceItems bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(item).Error; err != nil {
			return mapUniqueViolation(err)
		}
		if !replaceItems {
			return nil
		}
		if err := tx.Where("discount_id = ?", item.ID).Delete(&model.FlashDiscountItem{}).Error; err != nil {
			return err
		}
		return saveItems(tx, item.ID, items)
	})
}

func (r *flashRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("discount_id = ?", id).Delete(&model.FlashDiscountItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.FlashDiscount{}, id).Error
	})
}

func (r *flashRepository) Items(ctx context.Context, discountID uint64) ([]model.FlashDiscountItem, error) {
	var items []model.FlashDiscountItem
	if err := r.db.WithContext(ctx).
		Where("discount_id = ?", discountID).
		Order("target_type asc, target_id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *flashRepository) ActiveAt(ctx context.Context, now time.Time) ([]ActiveDiscount, error) {
	var discounts []model.FlashDiscount
	if err := r.db.WithContext(ctx).
		Where("status = ?", model.StatusActive).
		Where("(start_at IS NULL OR start_at <= ?)", now).
		Where("(end_at IS NULL OR end_at >= ?)", now).
		Order("id asc").
		Find(&discounts).Error; err != nil {
		return nil, err
	}
	if len(discounts) == 0 {
		return nil, nil
	}
	ids := make([]uint64, 0, len(discounts))
	for _, d := range discounts {
		ids = append(ids, d.ID)
	}
	var items []model.FlashDiscountItem
	if err := r.db.WithContext(ctx).
		Where("discount_id IN ?", ids).
		Order("target_type asc, target_id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	byDiscount := make(map[uint64][]model.FlashDiscountItem, len(discounts))
	for _, it := range items {
		byDiscount[it.DiscountID] = append(byDiscount[it.DiscountID], it)
	}
	out := make([]ActiveDiscount, 0, len(discounts))
	for _, d := range discounts {
		out = append(out, ActiveDiscount{Discount: d, Items: byDiscount[d.ID]})
	}
	return out, nil
}

func (r *flashRepository) CategoryOfProduct(ctx context.Context, productID uint64) (uint64, error) {
	if productID == 0 {
		return 0, nil
	}
	var categoryID uint64
	if err := r.db.WithContext(ctx).Table("products").
		Select("COALESCE(category_id, 0)").Where("id = ?", productID).
		Scan(&categoryID).Error; err != nil {
		return 0, err
	}
	return categoryID, nil
}

func (r *flashRepository) CategoryNames(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	out := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID   uint64
		Name string
	}
	if err := r.db.WithContext(ctx).Table("product_categories").
		Select("id, name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.Name
	}
	return out, nil
}

func (r *flashRepository) ProductNames(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	out := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID   uint64
		Name string
	}
	if err := r.db.WithContext(ctx).Table("products").
		Select("id, name").Where("id IN ? AND deleted_at IS NULL", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.Name
	}
	return out, nil
}

func (r *flashRepository) CategoryCostRate(ctx context.Context, id uint64) (float64, error) {
	if id == 0 {
		return 0, nil
	}
	var rate float64
	if err := r.db.WithContext(ctx).Table("product_categories").
		Select("cost_rate").Where("id = ?", id).Scan(&rate).Error; err != nil {
		return 0, err
	}
	return rate, nil
}

func (r *flashRepository) ProductCostRate(ctx context.Context, id uint64) (float64, error) {
	if id == 0 {
		return 0, nil
	}
	var row struct {
		Price     float64
		CostPrice float64
	}
	if err := r.db.WithContext(ctx).Table("products").
		Select("price, cost_price").Where("id = ?", id).Scan(&row).Error; err != nil {
		return 0, err
	}
	if row.Price <= 0 || row.CostPrice <= 0 {
		return 0, nil
	}
	return row.CostPrice / row.Price, nil
}

func saveItems(tx *gorm.DB, discountID uint64, items []model.FlashDiscountItem) error {
	if len(items) == 0 {
		return nil
	}
	for i := range items {
		items[i].DiscountID = discountID
	}
	return tx.Create(&items).Error
}

func mapUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uk_flash_discounts_code" {
		return ErrCodeTaken
	}
	return err
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
