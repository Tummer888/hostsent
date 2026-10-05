// Package repository 提供代理等级模块的数据访问实现。
package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/agentlevel/dto"
	"hostsent/backend/internal/modules/admin/user/agentlevel/model"
)

// ErrLevelInUse 等级仍被用户占用（删除前置校验）。
var ErrLevelInUse = errors.New("该代理等级仍有用户绑定，请先调整用户等级")

// ErrCodeTaken 等级编码重复（唯一索引 uk_agent_levels_code 冲突）。
// 不把这个转成业务错误的话，运营看到的是 500 加一句英文 SQLSTATE 报文。
var ErrCodeTaken = errors.New("代理等级编码已存在")

// AgentLevelRepository 定义代理等级的持久化操作。
type AgentLevelRepository interface {
	List(ctx context.Context, query dto.ListQuery) ([]model.AgentLevel, int64, error)
	// ListAllActive 取全部启用中的等级，按权重降序（矩阵列顺序的唯一来源）。
	ListAllActive(ctx context.Context) ([]model.AgentLevel, error)
	FindByID(ctx context.Context, id uint64) (*model.AgentLevel, error)
	FindByCode(ctx context.Context, code string) (*model.AgentLevel, error)
	// CreateWithDiscounts 建等级 + 写矩阵，同一事务：矩阵写入失败不能留下半个等级
	// （否则运营看到"创建失败"，列表里却多了一个没有折扣的空等级）。
	CreateWithDiscounts(ctx context.Context, item *model.AgentLevel, discounts []model.AgentLevelDiscount) error
	// UpdateWithDiscounts 改等级 +（可选）整体覆盖矩阵，同一事务。
	// replace=true 时按 discounts 覆盖；false 表示只改等级本身（矩阵不动）。
	UpdateWithDiscounts(ctx context.Context, item *model.AgentLevel, discounts []model.AgentLevelDiscount, replace bool) error
	Delete(ctx context.Context, id uint64) error
	CountUsers(ctx context.Context, levelID uint64) (int64, error)

	// MemberCounts 各等级的代理用户数（一次聚合，避免列表页 N+1）。
	MemberCounts(ctx context.Context) (map[uint64]int64, error)

	// —— 折扣矩阵 ——
	Discounts(ctx context.Context, levelID uint64) ([]model.AgentLevelDiscount, error)
	AllDiscounts(ctx context.Context) ([]model.AgentLevelDiscount, error)
	// DeleteDiscount 清除单格（折扣率填 0 = 不打折，等价于删除该格）。
	DeleteDiscount(ctx context.Context, levelID uint64, targetType string, targetID uint64) error
	// ActiveCategories 全部启用中的商品分类（矩阵行 = 全站兜底 + 这些分类），
	// 让运营在矩阵上能直接给「还没配过任何折扣」的分类配折扣。
	ActiveCategories(ctx context.Context) ([]CategoryRef, error)
	ReplaceDiscounts(ctx context.Context, levelID uint64, items []model.AgentLevelDiscount) error
	// UpsertDiscount 单格写入（阶梯填充用：按 (level, target) 更新或插入）。
	UpsertDiscount(ctx context.Context, item *model.AgentLevelDiscount) error

	// —— 算价支撑 ——
	// LevelIDOfUser 读取用户的代理等级 ID；非代理返回 0（nil）。
	LevelIDOfUser(ctx context.Context, userID uint64) (uint64, error)

	// —— 目标名解析（分类/商品名，供展示）——
	CategoryNames(ctx context.Context, ids []uint64) (map[uint64]string, error)
	ProductNames(ctx context.Context, ids []uint64) (map[uint64]string, error)
	CategoryCostRates(ctx context.Context, ids []uint64) (map[uint64]float64, error)
	// ProductCostRates 读取商品的「成本率」(cost_price / price)；无成本或价
	// 格为 0 的商品不返回键，由调用方回落到分类成本率。
	ProductCostRates(ctx context.Context, ids []uint64) (map[uint64]float64, error)
	// ValidateTargetRefs 校验 category/product 目标确实存在；不存在返回错误，
	// 避免把折扣挂到已删商品上、算价时静默失效（运营以为配了折扣但价格没变）。
	ValidateTargetRefs(ctx context.Context, items []model.AgentLevelDiscount) error
}

// CategoryRef 矩阵行需要的分类信息。
type CategoryRef struct {
	ID       uint64
	Name     string
	CostRate float64
}

type agentLevelRepository struct {
	db *gorm.DB
}

// NewAgentLevelRepository 创建代理等级仓储。
func NewAgentLevelRepository(db *gorm.DB) AgentLevelRepository {
	return &agentLevelRepository{db: db}
}

func (r *agentLevelRepository) List(ctx context.Context, query dto.ListQuery) ([]model.AgentLevel, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	base := r.db.WithContext(ctx).Model(&model.AgentLevel{})
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
	var items []model.AgentLevel
	if err := base.Order("weight desc, id asc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	// 折扣格数由 service 层 toInfo 按明细条数填充（List 本就带明细），无需再聚合一次。
	return items, total, nil
}

func (r *agentLevelRepository) ListAllActive(ctx context.Context) ([]model.AgentLevel, error) {
	var items []model.AgentLevel
	if err := r.db.WithContext(ctx).
		Where("status = ?", "active").
		Order("weight desc, id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *agentLevelRepository) FindByID(ctx context.Context, id uint64) (*model.AgentLevel, error) {
	var item model.AgentLevel
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *agentLevelRepository) FindByCode(ctx context.Context, code string) (*model.AgentLevel, error) {
	var item model.AgentLevel
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *agentLevelRepository) CreateWithDiscounts(ctx context.Context, item *model.AgentLevel, discounts []model.AgentLevelDiscount) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return mapUniqueViolation(err)
		}
		if len(discounts) == 0 {
			return nil
		}
		for i := range discounts {
			discounts[i].AgentLevelID = item.ID
		}
		return tx.Create(&discounts).Error
	})
}

func (r *agentLevelRepository) UpdateWithDiscounts(ctx context.Context, item *model.AgentLevel, discounts []model.AgentLevelDiscount, replace bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(item).Error; err != nil {
			return mapUniqueViolation(err)
		}
		if !replace {
			return nil
		}
		if err := tx.Where("agent_level_id = ?", item.ID).Delete(&model.AgentLevelDiscount{}).Error; err != nil {
			return err
		}
		if len(discounts) == 0 {
			return nil
		}
		for i := range discounts {
			discounts[i].AgentLevelID = item.ID
		}
		return tx.Create(&discounts).Error
	})
}

// Delete 删除等级并清理其折扣矩阵（同一事务）。用户占用校验由 service 先做。
func (r *agentLevelRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("agent_level_id = ?", id).Delete(&model.AgentLevelDiscount{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.AgentLevel{}, id).Error
	})
}

// mapUniqueViolation 把 code 唯一键冲突翻成业务错误，其余原样返回。
func mapUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uk_agent_levels_code" {
		return ErrCodeTaken
	}
	return err
}

func (r *agentLevelRepository) CountUsers(ctx context.Context, levelID uint64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Table("users").Where("agent_level_id = ?", levelID).Count(&total).Error
	return total, err
}

func (r *agentLevelRepository) MemberCounts(ctx context.Context) (map[uint64]int64, error) {
	type row struct {
		AgentLevelID uint64
		Cnt          int64
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("users").
		Select("agent_level_id, COUNT(*) AS cnt").
		Where("agent_level_id IS NOT NULL AND deleted_at IS NULL").
		Group("agent_level_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[uint64]int64, len(rows))
	for _, item := range rows {
		counts[item.AgentLevelID] = item.Cnt
	}
	return counts, nil
}

func (r *agentLevelRepository) DeleteDiscount(ctx context.Context, levelID uint64, targetType string, targetID uint64) error {
	return r.db.WithContext(ctx).
		Where("agent_level_id = ? AND target_type = ? AND target_id = ?", levelID, targetType, targetID).
		Delete(&model.AgentLevelDiscount{}).Error
}

func (r *agentLevelRepository) ActiveCategories(ctx context.Context) ([]CategoryRef, error) {
	var rows []CategoryRef
	// product_categories.status：1 = 启用（历史 bigint，不用字符串比较）。
	if err := r.db.WithContext(ctx).
		Table("product_categories").
		Select("id, name, cost_rate").
		Where("status = 1").
		Order("sort_order asc, id asc").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *agentLevelRepository) Discounts(ctx context.Context, levelID uint64) ([]model.AgentLevelDiscount, error) {
	var items []model.AgentLevelDiscount
	if err := r.db.WithContext(ctx).
		Where("agent_level_id = ?", levelID).
		Order("target_type asc, target_id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *agentLevelRepository) AllDiscounts(ctx context.Context) ([]model.AgentLevelDiscount, error) {
	var items []model.AgentLevelDiscount
	if err := r.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ReplaceDiscounts 覆盖式写入某等级的折扣矩阵（先删后插，事务内一致）。
func (r *agentLevelRepository) ReplaceDiscounts(ctx context.Context, levelID uint64, items []model.AgentLevelDiscount) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("agent_level_id = ?", levelID).Delete(&model.AgentLevelDiscount{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
}

// UpsertDiscount 单格 upsert：唯一键为 (agent_level_id, target_type, target_id)。
func (r *agentLevelRepository) UpsertDiscount(ctx context.Context, item *model.AgentLevelDiscount) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.AgentLevelDiscount
		err := tx.Where("agent_level_id = ? AND target_type = ? AND target_id = ?",
			item.AgentLevelID, item.TargetType, item.TargetID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(item).Error
		}
		if err != nil {
			return err
		}
		existing.DiscountRate = item.DiscountRate
		return tx.Save(&existing).Error
	})
}

// LevelIDOfUser 取用户代理等级 ID；未设（非代理）返回 0，不报错。
func (r *agentLevelRepository) LevelIDOfUser(ctx context.Context, userID uint64) (uint64, error) {
	var levelID *uint64
	err := r.db.WithContext(ctx).
		Table("users").
		Select("agent_level_id").
		Where("id = ?", userID).
		Limit(1).
		Scan(&levelID).Error
	if err != nil {
		return 0, err
	}
	if levelID == nil {
		return 0, nil
	}
	return *levelID, nil
}

func (r *agentLevelRepository) CategoryNames(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	result := map[uint64]string{}
	if len(ids) == 0 {
		return result, nil
	}
	type row struct {
		ID   uint64
		Name string
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("product_categories").
		Select("id, name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, item := range rows {
		result[item.ID] = item.Name
	}
	return result, nil
}

func (r *agentLevelRepository) ProductNames(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	result := map[uint64]string{}
	if len(ids) == 0 {
		return result, nil
	}
	type row struct {
		ID   uint64
		Name string
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("products").
		Select("id, name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, item := range rows {
		result[item.ID] = item.Name
	}
	return result, nil
}

// CategoryCostRates 读取分类成本率（product_categories.cost_rate，0 = 未配置）。
func (r *agentLevelRepository) CategoryCostRates(ctx context.Context, ids []uint64) (map[uint64]float64, error) {
	result := map[uint64]float64{}
	if len(ids) == 0 {
		return result, nil
	}
	type row struct {
		ID       uint64
		CostRate float64
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("product_categories").
		Select("id, cost_rate").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, item := range rows {
		result[item.ID] = item.CostRate
	}
	return result, nil
}

// ProductCostRates 由 cost_price / price 推导商品的成本率（0 = 无基准，不返回键）。
//
// 商品本身不存"成本率"，只存两个金额；这里折算成比率与分类口径统一。
// 价格或成本为 0 时无法折算，交由调用方回落到分类成本率。
func (r *agentLevelRepository) ProductCostRates(ctx context.Context, ids []uint64) (map[uint64]float64, error) {
	result := map[uint64]float64{}
	if len(ids) == 0 {
		return result, nil
	}
	type row struct {
		ID        uint64
		Price     float64
		CostPrice float64
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("products").
		Select("id, price, cost_price").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, item := range rows {
		if item.Price > 0 && item.CostPrice > 0 {
			result[item.ID] = item.CostPrice / item.Price
		}
	}
	return result, nil
}

// CategoryOfProduct 取商品的分类 ID（0 = 未归类）。
func (r *agentLevelRepository) CategoryOfProduct(ctx context.Context, productIDs []uint64) (map[uint64]uint64, error) {
	result := map[uint64]uint64{}
	if len(productIDs) == 0 {
		return result, nil
	}
	type row struct {
		ID         uint64
		CategoryID uint64
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("products").
		Select("id, COALESCE(category_id, 0) AS category_id").Where("id IN ?", productIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, item := range rows {
		result[item.ID] = item.CategoryID
	}
	return result, nil
}

// ValidateTargetRefs 校验 category/product 目标存在。
//
// 为什么必须校验：折扣矩阵没有外键（沿用 price_policy_items 的松耦合约定），
// 把折扣挂到不存在的分类/商品上不会报错，只会在算价时永远命不中 ——
// 运营在界面上看到"已配置 85 折"，但用户下单仍是原价，排查成本极高。
func (r *agentLevelRepository) ValidateTargetRefs(ctx context.Context, items []model.AgentLevelDiscount) error {
	categoryIDs := make([]uint64, 0, len(items))
	productIDs := make([]uint64, 0, len(items))
	for _, item := range items {
		switch item.TargetType {
		case model.TargetCategory:
			if item.TargetID > 0 {
				categoryIDs = append(categoryIDs, item.TargetID)
			}
		case model.TargetProduct:
			if item.TargetID > 0 {
				productIDs = append(productIDs, item.TargetID)
			}
		}
	}
	if len(categoryIDs) > 0 {
		var found int64
		if err := r.db.WithContext(ctx).Table("product_categories").
			Where("id IN ?", categoryIDs).Count(&found).Error; err != nil {
			return err
		}
		if int(found) != len(dedupeUint64(categoryIDs)) {
			return ErrTargetNotFound
		}
	}
	if len(productIDs) > 0 {
		var found int64
		if err := r.db.WithContext(ctx).Table("products").
			Where("id IN ? AND deleted_at IS NULL", productIDs).Count(&found).Error; err != nil {
			return err
		}
		if int(found) != len(dedupeUint64(productIDs)) {
			return ErrTargetNotFound
		}
	}
	return nil
}

// ErrTargetNotFound 折扣目标（分类/商品）不存在。
var ErrTargetNotFound = errors.New("折扣目标不存在或已删除")

func dedupeUint64(ids []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(ids))
	out := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
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
