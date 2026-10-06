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
	// CreateScheme 折扣组 + 等级费率 + 绑定的商品分组，同一事务。
	CreateScheme(ctx context.Context, scheme *model.DiscountScheme, items []model.DiscountSchemeItem, groupIDs []uint64) error
	// UpdateScheme 折扣组 +（可选）整体覆盖费率 +（可选）整体覆盖绑定分组，同一事务。
	// replaceGroups=false 时绑定关系不动（只改名称/费率）。
	UpdateScheme(ctx context.Context, scheme *model.DiscountScheme, items []model.DiscountSchemeItem, replaceItems bool, groupIDs []uint64, replaceGroups bool) error
	DeleteScheme(ctx context.Context, id uint64) error
	SchemeItems(ctx context.Context, schemeID uint64) ([]model.DiscountSchemeItem, error)
	// SchemeGroupIDs 该折扣组绑定的商品分组 ID 列表（有序）。
	SchemeGroupIDs(ctx context.Context, schemeID uint64) ([]uint64, error)
	// GroupsOfSchemes 批量取「折扣组 → 绑定的分组 ID」，避免列表页 N+1。
	GroupsOfSchemes(ctx context.Context, schemeIDs []uint64) (map[uint64][]uint64, error)
	// SchemeIDsOfGroups 批量反查「商品分组 → 绑定它的折扣组 ID」（用于归属校验与冲突检测）。
	SchemeIDsOfGroups(ctx context.Context, groupIDs []uint64) (map[uint64]uint64, error)
	// SchemeTargets 该折扣组绑定分组展开后的**去重**目标集合（应用时写入的对象）。
	// 去重是这里的职责：同一单品可能同时被绑定的两个分组包含（一个按分类、一个按单品）。
	SchemeTargets(ctx context.Context, schemeID uint64) ([]model.ProductGroupItem, error)
	// SchemeTargetSets 全部折扣组各自的去重目标集合（跨折扣组冲突检测用，一次查完）。
	SchemeTargetSets(ctx context.Context) (map[uint64][]TargetRef, error)
	// SchemeNames 折扣组 ID → 名称（冲突提示里要报出「被哪个折扣组占了」）。
	SchemeNames(ctx context.Context) (map[uint64]string, error)

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

// TargetRef 一个折扣目标，并带上是哪个折扣组（经哪个商品分组）引入的。
// 跨折扣组冲突检测要靠它报出「哪两组抢了同一个目标」。
type TargetRef struct {
	TargetType string
	TargetID   uint64
	// GroupID 引入该目标的分组；SchemeID 该分组当前归属的折扣组（0 = 未绑定）。
	GroupID  uint64
	SchemeID uint64
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

// CountSchemesOnGroup 绑定到该商品分组的折扣组数量（排除 excludeSchemeID，传 0 表示不排除）。
func (r *schemeRepo) CountSchemesOnGroup(ctx context.Context, groupID uint64, excludeSchemeID uint64) (int64, error) {
	query := r.db.WithContext(ctx).Model(&model.DiscountSchemeGroup{}).Where("group_id = ?", groupID)
	if excludeSchemeID > 0 {
		query = query.Where("scheme_id <> ?", excludeSchemeID)
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

func (r *schemeRepo) CreateScheme(ctx context.Context, scheme *model.DiscountScheme, items []model.DiscountSchemeItem, groupIDs []uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(scheme).Error; err != nil {
			return mapSchemeUniqueViolation(err)
		}
		if err := saveSchemeItems(tx, scheme.ID, items); err != nil {
			return err
		}
		return saveSchemeGroups(tx, scheme.ID, groupIDs)
	})
}

func (r *schemeRepo) UpdateScheme(ctx context.Context, scheme *model.DiscountScheme, items []model.DiscountSchemeItem, replaceItems bool, groupIDs []uint64, replaceGroups bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(scheme).Error; err != nil {
			return mapSchemeUniqueViolation(err)
		}
		if replaceItems {
			if err := tx.Where("scheme_id = ?", scheme.ID).Delete(&model.DiscountSchemeItem{}).Error; err != nil {
				return err
			}
			if err := saveSchemeItems(tx, scheme.ID, items); err != nil {
				return err
			}
		}
		if !replaceGroups {
			return nil
		}
		if err := tx.Where("scheme_id = ?", scheme.ID).Delete(&model.DiscountSchemeGroup{}).Error; err != nil {
			return err
		}
		return saveSchemeGroups(tx, scheme.ID, groupIDs)
	})
}

// saveSchemeGroups 写入绑定关系。唯一索引 uk_discount_scheme_groups_group 会拦住
// 「同一分组被两个折扣组绑定」，冲突时翻成业务错误而不是 500。
func saveSchemeGroups(tx *gorm.DB, schemeID uint64, groupIDs []uint64) error {
	if len(groupIDs) == 0 {
		return nil
	}
	rows := make([]model.DiscountSchemeGroup, 0, len(groupIDs))
	for _, id := range groupIDs {
		rows = append(rows, model.DiscountSchemeGroup{SchemeID: schemeID, GroupID: id})
	}
	if err := tx.Create(&rows).Error; err != nil {
		return mapSchemeUniqueViolation(err)
	}
	return nil
}

func (r *schemeRepo) SchemeGroupIDs(ctx context.Context, schemeID uint64) ([]uint64, error) {
	var ids []uint64
	if err := r.db.WithContext(ctx).Model(&model.DiscountSchemeGroup{}).
		Where("scheme_id = ?", schemeID).
		Order("group_id asc").
		Pluck("group_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *schemeRepo) GroupsOfSchemes(ctx context.Context, schemeIDs []uint64) (map[uint64][]uint64, error) {
	out := make(map[uint64][]uint64, len(schemeIDs))
	if len(schemeIDs) == 0 {
		return out, nil
	}
	var rows []model.DiscountSchemeGroup
	if err := r.db.WithContext(ctx).
		Where("scheme_id IN ?", schemeIDs).
		Order("scheme_id asc, group_id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.SchemeID] = append(out[row.SchemeID], row.GroupID)
	}
	return out, nil
}

func (r *schemeRepo) SchemeIDsOfGroups(ctx context.Context, groupIDs []uint64) (map[uint64]uint64, error) {
	out := make(map[uint64]uint64, len(groupIDs))
	if len(groupIDs) == 0 {
		return out, nil
	}
	var rows []model.DiscountSchemeGroup
	if err := r.db.WithContext(ctx).
		Where("group_id IN ?", groupIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.GroupID] = row.SchemeID
	}
	return out, nil
}

// SchemeTargets 该折扣组绑定分组展开后的去重目标。
//
// 去重必须做：同一单品可能被两个分组各包含一次（分组 A 含分类「对象存储」，
// 分组 B 含单品「对象存储 100GB」），展开后是同一条 (product, 100)。
// 不去重会撞 agent_level_discounts 的唯一索引，让整次应用失败。
func (r *schemeRepo) SchemeTargets(ctx context.Context, schemeID uint64) ([]model.ProductGroupItem, error) {
	var rows []TargetRef
	// DISTINCT 而不是 GROUP BY：同一目标被多个绑定分组同时纳入时只保留一条。
	// 这里不再回带 scheme_id（TargetRef 的该字段仅用于跨组比对），避免多余的参数绑定。
	if err := r.db.WithContext(ctx).
		Table("product_group_items AS pgi").
		Select("DISTINCT pgi.target_type, pgi.target_id").
		Joins("JOIN discount_scheme_groups AS dsg ON dsg.group_id = pgi.group_id").
		Where("dsg.scheme_id = ?", schemeID).
		Order("pgi.target_type asc, pgi.target_id asc").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]model.ProductGroupItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, model.ProductGroupItem{TargetType: row.TargetType, TargetID: row.TargetID})
	}
	return items, nil
}

// SchemeTargetSets 全部折扣组各自的去重目标集合。
func (r *schemeRepo) SchemeTargetSets(ctx context.Context) (map[uint64][]TargetRef, error) {
	var rows []TargetRef
	if err := r.db.WithContext(ctx).
		Table("product_group_items AS pgi").
		Select("DISTINCT pgi.target_type, pgi.target_id, dsg.group_id, dsg.scheme_id").
		Joins("JOIN discount_scheme_groups AS dsg ON dsg.group_id = pgi.group_id").
		Order("dsg.scheme_id asc, pgi.target_type asc, pgi.target_id asc").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uint64][]TargetRef)
	for _, row := range rows {
		out[row.SchemeID] = append(out[row.SchemeID], row)
	}
	return out, nil
}

// SchemeNames 折扣组 ID → 名称。
func (r *schemeRepo) SchemeNames(ctx context.Context) (map[uint64]string, error) {
	var rows []struct {
		ID   uint64
		Name string
	}
	if err := r.db.WithContext(ctx).Model(&model.DiscountScheme{}).
		Select("id, name").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uint64]string, len(rows))
	for _, row := range rows {
		out[row.ID] = row.Name
	}
	return out, nil
}

func (r *schemeRepo) DeleteScheme(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("scheme_id = ?", id).Delete(&model.DiscountSchemeItem{}).Error; err != nil {
			return err
		}
		if err := tx.Where("scheme_id = ?", id).Delete(&model.DiscountSchemeGroup{}).Error; err != nil {
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

// mapSchemeUniqueViolation 把本域的唯一键冲突翻成业务错误（其余原样返回）。
func mapSchemeUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "uk_product_groups_code":
			return ErrGroupCodeTaken
		case "uk_discount_schemes_code":
			return ErrSchemeCodeTaken
		case "uk_discount_scheme_groups_group":
			// 一个商品分组至多被一个折扣组绑定：并发下也可能撞这条，翻成同一句业务话。
			return ErrGroupBound
		}
	}
	return err
}

// saveSchemeItems 写入等级费率（schemeID 回填）。
func saveSchemeItems(tx *gorm.DB, schemeID uint64, items []model.DiscountSchemeItem) error {
	if len(items) == 0 {
		return nil
	}
	for i := range items {
		items[i].SchemeID = schemeID
	}
	return tx.Create(&items).Error
}

// normalizeSchemeStatus 归一状态值。
func normalizeSchemeStatus(status string) string {
	if strings.TrimSpace(status) == "" {
		return "active"
	}
	return status
}
