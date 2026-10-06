// Package repository 提供商品分组与折扣组的持久化（doc108 §8I）。
package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/agentlevel/model"
)

// 折扣组/商品分组域的业务错误。
var (
	// ErrGroupInUse 商品分组仍被折扣组绑定。
	ErrGroupInUse = errors.New("该商品分组仍被折扣组绑定，请先解除绑定")
	// ErrGroupCodeTaken 商品分组编码重复。
	ErrGroupCodeTaken = errors.New("商品分组编码已存在")
	// ErrSchemeCodeTaken 折扣组编码重复。
	ErrSchemeCodeTaken = errors.New("折扣组编码已存在")
	// ErrGroupBound 折扣组要绑定的商品分组已被其它折扣组绑定。
	ErrGroupBound = errors.New("该商品分组已被其它折扣组绑定")
)

// schemeRepo 商品分组 + 折扣组共用一个仓储实现（两表强耦合）。
type schemeRepo struct {
	db *gorm.DB
}

// NewSchemeRepository 创建商品分组/折扣组仓储。
func NewSchemeRepository(db *gorm.DB) SchemeRepository {
	return &schemeRepo{db: db}
}

// SchemeRepository 商品分组与折扣组的持久化接口。
type SchemeRepository interface {
	// —— 商品分组 ——
	ListGroups(ctx context.Context) ([]model.ProductGroup, error)
	FindGroupByID(ctx context.Context, id uint64) (*model.ProductGroup, error)
	CreateGroup(ctx context.Context, group *model.ProductGroup, items []model.ProductGroupItem) error
	UpdateGroup(ctx context.Context, group *model.ProductGroup, items []model.ProductGroupItem, replaceItems bool) error
	DeleteGroup(ctx context.Context, id uint64) error
	GroupItems(ctx context.Context, groupID uint64) ([]model.ProductGroupItem, error)
	// CountSchemesOnGroup 绑定到该商品分组的折扣组数量（排除 excludeSchemeID，传 0 表示不排除）。
	CountSchemesOnGroup(ctx context.Context, groupID uint64, excludeSchemeID uint64) (int64, error)

	// —— 折扣组 ——
	ListSchemes(ctx context.Context) ([]model.DiscountScheme, error)
	FindSchemeByID(ctx context.Context, id uint64) (*model.DiscountScheme, error)
	CreateScheme(ctx context.Context, scheme *model.DiscountScheme, items []model.DiscountSchemeItem) error
	UpdateScheme(ctx context.Context, scheme *model.DiscountScheme, items []model.DiscountSchemeItem, replaceItems bool) error
	DeleteScheme(ctx context.Context, id uint64) error
	SchemeItems(ctx context.Context, schemeID uint64) ([]model.DiscountSchemeItem, error)

	// —— 名称解析（成员/等级展示）——
	CategoryNames(ctx context.Context, ids []uint64) (map[uint64]string, error)
	ProductNames(ctx context.Context, ids []uint64) (map[uint64]string, error)
	// LevelsBasic 代理等级基础信息（折扣组列头与费率明细回显用）。
	LevelsBasic(ctx context.Context) ([]LevelBasic, error)
}

// LevelBasic 代理等级基础信息。
type LevelBasic struct {
	ID     uint64
	Name   string
	Weight int
	Status string
}

func (r *schemeRepo) ListGroups(ctx context.Context) ([]model.ProductGroup, error) {
	var items []model.ProductGroup
	if err := r.db.WithContext(ctx).
		Model(&model.ProductGroup{}).
		Select("product_groups.*, COUNT(product_group_items.id) AS item_count").
		Joins("LEFT JOIN product_group_items ON product_group_items.group_id = product_groups.id").
		Group("product_groups.id").
		Order("product_groups.id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *schemeRepo) FindGroupByID(ctx context.Context, id uint64) (*model.ProductGroup, error) {
	var group model.ProductGroup
	if err := r.db.WithContext(ctx).First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *schemeRepo) GroupItems(ctx context.Context, groupID uint64) ([]model.ProductGroupItem, error) {
	var items []model.ProductGroupItem
	if err := r.db.WithContext(ctx).
		Where("group_id = ?", groupID).
		Order("target_type asc, target_id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *schemeRepo) CreateGroup(ctx context.Context, group *model.ProductGroup, items []model.ProductGroupItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(group).Error; err != nil {
			return mapSchemeUniqueViolation(err)
		}
		return saveGroupItems(tx, group.ID, items)
	})
}

func (r *schemeRepo) UpdateGroup(ctx context.Context, group *model.ProductGroup, items []model.ProductGroupItem, replaceItems bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(group).Error; err != nil {
			return mapSchemeUniqueViolation(err)
		}
		if !replaceItems {
			return nil
		}
		if err := tx.Where("group_id = ?", group.ID).Delete(&model.ProductGroupItem{}).Error; err != nil {
			return err
		}
		return saveGroupItems(tx, group.ID, items)
	})
}

func saveGroupItems(tx *gorm.DB, groupID uint64, items []model.ProductGroupItem) error {
	if len(items) == 0 {
		return nil
	}
	for i := range items {
		items[i].GroupID = groupID
	}
	return tx.Create(&items).Error
}

func (r *schemeRepo) DeleteGroup(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", id).Delete(&model.ProductGroupItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.ProductGroup{}, id).Error
	})
}

func (r *schemeRepo) CountSchemesOnGroup(ctx context.Context, groupID uint64, excludeSchemeID uint64) (int64, error) {
	query := r.db.WithContext(ctx).Model(&model.DiscountScheme{}).Where("product_group_id = ?", groupID)
	if excludeSchemeID > 0 {
		query = query.Where("id <> ?", excludeSchemeID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *schemeRepo) ListSchemes(ctx context.Context) ([]model.DiscountScheme, error) {
	var items []model.DiscountScheme
	if err := r.db.WithContext(ctx).
		Model(&model.DiscountScheme{}).
		Select("discount_schemes.*, COUNT(discount_scheme_items.id) AS item_count").
		Joins("LEFT JOIN discount_scheme_items ON discount_scheme_items.scheme_id = discount_schemes.id").
		Group("discount_schemes.id").
		Order("discount_schemes.id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *schemeRepo) FindSchemeByID(ctx context.Context, id uint64) (*model.DiscountScheme, error) {
	var scheme model.DiscountScheme
	if err := r.db.WithContext(ctx).First(&scheme, id).Error; err != nil {
		return nil, err
	}
	return &scheme, nil
}

func (r *schemeRepo) SchemeItems(ctx context.Context, schemeID uint64) ([]model.DiscountSchemeItem, error) {
	var items []model.DiscountSchemeItem
	if err := r.db.WithContext(ctx).
		Where("scheme_id = ?", schemeID).
		Order("agent_level_id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *schemeRepo) CreateScheme(ctx context.Context, scheme *model.DiscountScheme, items []model.DiscountSchemeItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(scheme).Error; err != nil {
			return mapSchemeUniqueViolation(err)
		}
		return saveSchemeItems(tx, scheme.ID, items)
	})
}

func (r *schemeRepo) UpdateScheme(ctx context.Context, scheme *model.DiscountScheme, items []model.DiscountSchemeItem, replaceItems bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(scheme).Error; err != nil {
			return mapSchemeUniqueViolation(err)
		}
		if !replaceItems {
			return nil
		}
		if err := tx.Where("scheme_id = ?", scheme.ID).Delete(&model.DiscountSchemeItem{}).Error; err != nil {
			return err
		}
		return saveSchemeItems(tx, scheme.ID, items)
	})
}

func saveSchemeItems(tx *gorm.DB, schemeID uint64, items []model.DiscountSchemeItem) error {
	if len(items) == 0 {
		return nil
	}
	for i := range items {
		items[i].SchemeID = schemeID
	}
	return tx.Create(&items).Error
}

func (r *schemeRepo) DeleteScheme(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("scheme_id = ?", id).Delete(&model.DiscountSchemeItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.DiscountScheme{}, id).Error
	})
}

func (r *schemeRepo) CategoryNames(ctx context.Context, ids []uint64) (map[uint64]string, error) {
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

func (r *schemeRepo) ProductNames(ctx context.Context, ids []uint64) (map[uint64]string, error) {
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

func (r *schemeRepo) LevelsBasic(ctx context.Context) ([]LevelBasic, error) {
	var rows []LevelBasic
	if err := r.db.WithContext(ctx).Table("agent_levels").
		Select("id, name, weight, status").
		Order("weight desc, id asc").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// mapSchemeUniqueViolation 把本域两条唯一键冲突翻成业务错误（其余原样返回）。
func mapSchemeUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "uk_product_groups_code":
			return ErrGroupCodeTaken
		case "uk_discount_schemes_code":
			return ErrSchemeCodeTaken
		}
	}
	return err
}

// normalizeSchemeStatus 归一状态值。
func normalizeSchemeStatus(status string) string {
	if strings.TrimSpace(status) == "" {
		return "active"
	}
	return status
}
