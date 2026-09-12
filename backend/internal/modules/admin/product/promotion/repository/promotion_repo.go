// Package repository 提供促销管理（promotion）子域的数据访问实现。
package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/product/promotion/dto"
	"hostsent/backend/internal/modules/admin/product/promotion/model"
)

// CouponRepository 定义优惠券数据访问能力。
type CouponRepository interface {
	List(ctx context.Context, query dto.CouponQuery) ([]model.Coupon, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.Coupon, error)
	Create(ctx context.Context, item *model.Coupon) error
	Update(ctx context.Context, item *model.Coupon) error
	Delete(ctx context.Context, id uint64) error
	AddClaimed(ctx context.Context, id uint64, delta int) error
}

type couponRepository struct {
	db *gorm.DB
}

func NewCouponRepository(db *gorm.DB) CouponRepository {
	return &couponRepository{db: db}
}

func (r *couponRepository) List(ctx context.Context, query dto.CouponQuery) ([]model.Coupon, int64, error) {
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
	base := r.db.WithContext(ctx).Model(&model.Coupon{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("name ILIKE ? OR coupon_code ILIKE ?", like, like)
	}
	if query.CouponType != "" {
		base = base.Where("coupon_type = ?", query.CouponType)
	}
	if query.Status != nil {
		base = base.Where("status = ?", *query.Status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.Coupon
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *couponRepository) FindByID(ctx context.Context, id uint64) (*model.Coupon, error) {
	var item model.Coupon
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *couponRepository) Create(ctx context.Context, item *model.Coupon) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *couponRepository) Update(ctx context.Context, item *model.Coupon) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *couponRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Coupon{}, id).Error
}

func (r *couponRepository) AddClaimed(ctx context.Context, id uint64, delta int) error {
	return r.db.WithContext(ctx).Model(&model.Coupon{}).Where("id = ?", id).
		UpdateColumn("claimed_count", gorm.Expr("claimed_count + ?", delta)).Error
}

// CouponGrantRepository 定义优惠券发放记录数据访问能力。
type CouponGrantRepository interface {
	List(ctx context.Context, query dto.CouponGrantQuery) ([]model.CouponGrant, int64, error)
	Create(ctx context.Context, item *model.CouponGrant) error
}

type couponGrantRepository struct {
	db *gorm.DB
}

func NewCouponGrantRepository(db *gorm.DB) CouponGrantRepository {
	return &couponGrantRepository{db: db}
}

func (r *couponGrantRepository) List(ctx context.Context, query dto.CouponGrantQuery) ([]model.CouponGrant, int64, error) {
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
	base := r.db.WithContext(ctx).Model(&model.CouponGrant{})
	if query.CouponID > 0 {
		base = base.Where("coupon_id = ?", query.CouponID)
	}
	if query.Status != "" {
		base = base.Where("status = ?", query.Status)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("user_name ILIKE ?", like)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.CouponGrant
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *couponGrantRepository) Create(ctx context.Context, item *model.CouponGrant) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// PromotionRepository 定义促销活动数据访问能力。
type PromotionRepository interface {
	List(ctx context.Context, query dto.PromotionQuery) ([]model.Promotion, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.Promotion, error)
	Create(ctx context.Context, item *model.Promotion) error
	Update(ctx context.Context, item *model.Promotion) error
	Delete(ctx context.Context, id uint64) error
}

type promotionRepository struct {
	db *gorm.DB
}

func NewPromotionRepository(db *gorm.DB) PromotionRepository {
	return &promotionRepository{db: db}
}

func (r *promotionRepository) List(ctx context.Context, query dto.PromotionQuery) ([]model.Promotion, int64, error) {
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
	base := r.db.WithContext(ctx).Model(&model.Promotion{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("name ILIKE ?", like)
	}
	if query.PromotionType != "" {
		base = base.Where("promotion_type = ?", query.PromotionType)
	}
	if query.Status != nil {
		base = base.Where("status = ?", *query.Status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.Promotion
	if err := base.Order("sort_order asc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *promotionRepository) FindByID(ctx context.Context, id uint64) (*model.Promotion, error) {
	var item model.Promotion
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *promotionRepository) Create(ctx context.Context, item *model.Promotion) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *promotionRepository) Update(ctx context.Context, item *model.Promotion) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *promotionRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Promotion{}, id).Error
}
