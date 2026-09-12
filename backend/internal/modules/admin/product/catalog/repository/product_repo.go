// Package repository 提供商品管理（catalog）子域的数据访问实现。
package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
	"hostsent/backend/internal/modules/admin/product/catalog/model"
)

// ProductRepository 定义商品数据访问能力。
type ProductRepository interface {
	List(ctx context.Context, query dto.ProductListQuery) ([]model.Product, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.Product, error)
	Create(ctx context.Context, item *model.Product) error
	Update(ctx context.Context, item *model.Product) error
	Delete(ctx context.Context, id uint64) error
	ListSpecs(ctx context.Context, productID uint64) ([]model.ProductSpec, error)
	// FindSpecByCode 按商品 + 规格编码取 SKU（下单/算价用，T4.1）。
	FindSpecByCode(ctx context.Context, productID uint64, specCode string) (*model.ProductSpec, error)
	// FindSpecByID 按主键取 SKU（改/删时校验归属用）。
	FindSpecByID(ctx context.Context, id uint64) (*model.ProductSpec, error)
	CreateSpec(ctx context.Context, item *model.ProductSpec) error
	UpdateSpec(ctx context.Context, item *model.ProductSpec) error
	DeleteSpec(ctx context.Context, id uint64) error
	// DecrementSpecStock 原子扣减库存（stock<0 表示不限，不扣减）；返回受影响行数，0 表示库存不足。
	DecrementSpecStock(ctx context.Context, specID uint64, qty int) (int64, error)
	// IncrementSpecStock 回补库存（下单失败补偿用；stock<0 表示不限，不回补）。
	IncrementSpecStock(ctx context.Context, specID uint64, qty int) (int64, error)
	AddHistory(ctx context.Context, history *model.ProductHistory) error
	ListHistory(ctx context.Context, productID uint64) ([]model.ProductHistory, error)
	// ListBySourceProductID 按上游资源商品 ID 查关联售出商品（T3.4 已确认调价落地用）。
	ListBySourceProductID(ctx context.Context, sourceProductID uint64) ([]model.Product, error)
	// SaveConfigOptions 保存商品的配置组（上游 config_groups）到子表（幂等：先删后插）。
	// 配置组内可带 source/source_key 覆盖来源（T4.4）；缺省按 upstream + upstream_id 派生。
	SaveConfigOptions(ctx context.Context, productID uint64, groups []interface{}) error
	// ConfigGroupsByProductID 读取商品的配置组，重建为 config_groups 结构（供上游下单使用）。
	ConfigGroupsByProductID(ctx context.Context, productID uint64) ([]interface{}, error)
	// AllConfigGroupsByProductID 读取商品全部配置组（含 source=self），供配置项管理页展示/编辑（T4.4）。
	AllConfigGroupsByProductID(ctx context.Context, productID uint64) ([]interface{}, error)
	// SelfConfigParams 读取 source=self 的配置项，返回"平台参数名 → 选中值"（T4.4 自营可配置项）。
	SelfConfigParams(ctx context.Context, productID uint64) (map[string]string, error)
}

type productRepository struct {
	db *gorm.DB
}

// NewProductRepository 创建商品仓储实现。
func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) List(ctx context.Context, query dto.ProductListQuery) ([]model.Product, int64, error) {
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

	base := r.db.WithContext(ctx).Model(&model.Product{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("name ILIKE ? OR code ILIKE ?", like, like)
	}
	if query.CategoryID > 0 {
		base = base.Where("category_id = ?", query.CategoryID)
	}
	if query.Status != nil {
		base = base.Where("status = ?", *query.Status)
	}
	if query.SourceMode != "" {
		base = base.Where("source_mode = ?", query.SourceMode)
	}
	if query.Featured != nil {
		base = base.Where("featured = ?", *query.Featured)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Product
	if err := base.Order("sort_order asc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *productRepository) FindByID(ctx context.Context, id uint64) (*model.Product, error) {
	var item model.Product
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *productRepository) Create(ctx context.Context, item *model.Product) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *productRepository) Update(ctx context.Context, item *model.Product) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *productRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Product{}, id).Error
}

func (r *productRepository) ListSpecs(ctx context.Context, productID uint64) ([]model.ProductSpec, error) {
	var items []model.ProductSpec
	if err := r.db.WithContext(ctx).Where("product_id = ?", productID).Order("sort_order asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *productRepository) FindSpecByCode(ctx context.Context, productID uint64, specCode string) (*model.ProductSpec, error) {
	var item model.ProductSpec
	if err := r.db.WithContext(ctx).
		Where("product_id = ? AND spec_code = ?", productID, specCode).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *productRepository) FindSpecByID(ctx context.Context, id uint64) (*model.ProductSpec, error) {
	var item model.ProductSpec
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *productRepository) CreateSpec(ctx context.Context, item *model.ProductSpec) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *productRepository) UpdateSpec(ctx context.Context, item *model.ProductSpec) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *productRepository) DeleteSpec(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.ProductSpec{}, id).Error
}

// DecrementSpecStock 条件更新扣减库存：stock < 0（不限）不扣减，视为成功；
// 有限库存时要求 stock >= qty，用 SQL 原子自减避免并发超卖。
func (r *productRepository) DecrementSpecStock(ctx context.Context, specID uint64, qty int) (int64, error) {
	if qty <= 0 {
		qty = 1
	}
	res := r.db.WithContext(ctx).Model(&model.ProductSpec{}).
		Where("id = ? AND (stock < 0 OR stock >= ?)", specID, qty).
		Updates(map[string]interface{}{
			"stock":      gorm.Expr("CASE WHEN stock < 0 THEN stock ELSE stock - ? END", qty),
			"updated_at": time.Now(),
		})
	return res.RowsAffected, res.Error
}

// IncrementSpecStock 回补库存（下单/开通失败补偿）；不限库存（stock<0）不处理。
func (r *productRepository) IncrementSpecStock(ctx context.Context, specID uint64, qty int) (int64, error) {
	if qty <= 0 {
		qty = 1
	}
	res := r.db.WithContext(ctx).Model(&model.ProductSpec{}).
		Where("id = ? AND stock >= 0", specID).
		Updates(map[string]interface{}{
			"stock":      gorm.Expr("stock + ?", qty),
			"updated_at": time.Now(),
		})
	return res.RowsAffected, res.Error
}

func (r *productRepository) AddHistory(ctx context.Context, history *model.ProductHistory) error {
	return r.db.WithContext(ctx).Create(history).Error
}

func (r *productRepository) ListHistory(ctx context.Context, productID uint64) ([]model.ProductHistory, error) {
	var items []model.ProductHistory
	if err := r.db.WithContext(ctx).Where("product_id = ?", productID).Order("created_at desc, id desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListBySourceProductID 返回绑定了该上游资源商品的售出商品（克隆/转售链路）。
func (r *productRepository) ListBySourceProductID(ctx context.Context, sourceProductID uint64) ([]model.Product, error) {
	var items []model.Product
	if err := r.db.WithContext(ctx).
		Where("source_product_id = ?", sourceProductID).
		Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// configGroup/configOption/configOptionSub 为 config_groups 的持久化/重建视图，
// JSON 结构与上游 (pkg/upstream/mofangfinance) 的 ConfigGroup/ConfigOption/ConfigSub 对齐，
// 使下游 buildCartConfigOption 等保持不变。
type configGroup struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Options     []configOption `json:"options"`
}

type configOption struct {
	ID         int64  `json:"id"`
	OptionName string `json:"option_name"`
	OptionType int    `json:"option_type"`
	QtyMinimum int    `json:"qty_minimum"`
	QtyMaximum int    `json:"qty_maximum"`
	UpstreamID int64  `json:"upstream_id"`
	// Source/SourceKey 配置项来源（T4.4）：upstream（默认）或 self；self 时 SourceKey 是平台参数名。
	Source    string      `json:"source,omitempty"`
	SourceKey string      `json:"source_key,omitempty"`
	Hidden    int         `json:"hidden"`
	SortOrder int         `json:"sort_order"`
	Sub       []configSub `json:"sub"`
}

type configSub struct {
	ID         int64         `json:"id"`
	OptionName string        `json:"option_name"`
	QtyMinimum int           `json:"qty_minimum"`
	QtyMaximum int           `json:"qty_maximum"`
	UpstreamID int64         `json:"upstream_id"`
	Source     string        `json:"source,omitempty"`
	SourceKey  string        `json:"source_key,omitempty"`
	Hidden     int           `json:"hidden"`
	SortOrder  int           `json:"sort_order"`
	Pricings   []configPrice `json:"pricings"`
}

// configPrice 上游 pricing 项（价格可能为字符串或数字），用 flexFloat 兼容。
type configPrice struct {
	Monthly   flexFloat `json:"monthly"`
	Annually  flexFloat `json:"annually"`
	Quarterly flexFloat `json:"quarterly"`
	Onetime   flexFloat `json:"onetime"`
}

// flexFloat 兼容字符串或数字的价格解析（上游 pricing 字段常为字符串如 "0.00"）。
type flexFloat float64

// UnmarshalJSON 接受 json 字符串或数字。
func (f *flexFloat) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*f = 0
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		v, _ := strconv.ParseFloat(s, 64)
		*f = flexFloat(v)
		return nil
	}
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexFloat(n)
		return nil
	}
	*f = 0
	return nil
}

// SaveConfigOptions 幂等保存商品的配置组：先删除该商品既有子表行，再按组/选项/子项写库。
// groups 为 config_groups 数组（来自 extractConfigGroups 或上游镜像结构，nil/空则清空）。
func (r *productRepository) SaveConfigOptions(ctx context.Context, productID uint64, groups []interface{}) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先删该商品全部配置项（依赖 FK 或显式子查询清理其子项）。
		sub := tx.Model(&model.ProductConfigOption{}).Select("id").Where("product_id = ?", productID)
		if err := tx.Where("option_id IN (?)", sub).Delete(&model.ProductConfigOptionSub{}).Error; err != nil {
			return err
		}
		if err := tx.Where("product_id = ?", productID).Delete(&model.ProductConfigOption{}).Error; err != nil {
			return err
		}
		if len(groups) == 0 {
			return nil
		}
		parsed, err := parseConfigGroups(groups)
		if err != nil {
			return err
		}
		for _, option := range buildConfigOptionRows(parsed, productID) {
			// Omit("Subs") 避免 GORM 全关联创建；子项单独落库以正确回填 OptionID。
			if err := tx.Omit("Subs").Create(option).Error; err != nil {
				return err
			}
			for i := range option.Subs {
				option.Subs[i].OptionID = option.ID
				if err := tx.Create(&option.Subs[i]).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// buildConfigOptionRows 把解析后的 config_groups 组织为商品配置项（含子项）行，便于落库。
// 来源规则（T4.4）：显式 source/source_key 优先；缺省按 upstream + upstream_id 派生。
func buildConfigOptionRows(parsed []configGroup, productID uint64) []*model.ProductConfigOption {
	rows := make([]*model.ProductConfigOption, 0)
	for _, g := range parsed {
		for _, opt := range g.Options {
			source, sourceKey := normalizeConfigSource(opt.Source, opt.SourceKey, opt.UpstreamID)
			option := &model.ProductConfigOption{
				ProductID:   productID,
				UpstreamKey: opt.UpstreamID,
				Source:      source,
				SourceKey:   sourceKey,
				OptionName:  opt.OptionName,
				OptionType:  intOr(opt.OptionType, 1),
				SortOrder:   opt.SortOrder,
			}
			for _, sub := range opt.Sub {
				price := firstPrice(sub.Pricings)
				subSource, subSourceKey := normalizeConfigSource(sub.Source, sub.SourceKey, sub.UpstreamID)
				option.Subs = append(option.Subs, model.ProductConfigOptionSub{
					UpstreamKey:    sub.UpstreamID,
					Source:         subSource,
					SourceKey:      subSourceKey,
					OptionName:     sub.OptionName,
					Hidden:         sub.Hidden,
					PriceMonthly:   float64(price.Monthly),
					PriceAnnually:  float64(price.Annually),
					PriceQuarterly: float64(price.Quarterly),
					PriceOnetime:   float64(price.Onetime),
					SortOrder:      sub.SortOrder,
				})
			}
			rows = append(rows, option)
		}
	}
	return rows
}

// normalizeConfigSource 归一配置项来源：source 仅接受 upstream/self，其余回落 upstream；
// source_key 为空时用上游 id 派生（自营项无上游 id 则留空，由 option_name 兜底）。
func normalizeConfigSource(source, sourceKey string, upstreamID int64) (string, string) {
	sourceKey = strings.TrimSpace(sourceKey)
	switch strings.TrimSpace(source) {
	case model.ConfigSourceSelf:
		return model.ConfigSourceSelf, sourceKey
	case model.ConfigSourceUpstream:
		if sourceKey == "" && upstreamID != 0 {
			sourceKey = strconv.FormatInt(upstreamID, 10)
		}
		return model.ConfigSourceUpstream, sourceKey
	}
	if upstreamID != 0 {
		if sourceKey == "" {
			sourceKey = strconv.FormatInt(upstreamID, 10)
		}
		return model.ConfigSourceUpstream, sourceKey
	}
	// 无上游 id 且未显式声明来源：视为自营平台参数项。
	return model.ConfigSourceSelf, sourceKey
}

// ConfigGroupsByProductID 读取商品配置子表，重建为 config_groups 数组（结构对齐上游 ConfigGroup）。
func (r *productRepository) ConfigGroupsByProductID(ctx context.Context, productID uint64) ([]interface{}, error) {
	var options []*model.ProductConfigOption
	if err := r.db.WithContext(ctx).
		Preload("Subs").
		Where("product_id = ? AND source = ?", productID, model.ConfigSourceUpstream).
		Order("sort_order asc, id asc").
		Find(&options).Error; err != nil {
		return nil, err
	}
	if len(options) == 0 {
		return nil, nil
	}
	return buildConfigGroups(options), nil
}

// AllConfigGroupsByProductID 读取商品全部配置组（含 source=self），供配置项管理页展示/编辑（T4.4）。
func (r *productRepository) AllConfigGroupsByProductID(ctx context.Context, productID uint64) ([]interface{}, error) {
	var options []*model.ProductConfigOption
	if err := r.db.WithContext(ctx).
		Preload("Subs").
		Where("product_id = ?", productID).
		Order("sort_order asc, id asc").
		Find(&options).Error; err != nil {
		return nil, err
	}
	if len(options) == 0 {
		return nil, nil
	}
	return buildConfigGroups(options), nil
}

// SelfConfigParams 读取 source=self 的配置项，返回"平台参数名 → 选中值"（T4.4）。
// 自营链路里 option_name 即目标平台写参数名（如 area/os/store），sub 的首个可见项为取值；
// source_key 非空时优先作为平台参数名，兼容"展示名 ≠ 平台参数名"的场景。
func (r *productRepository) SelfConfigParams(ctx context.Context, productID uint64) (map[string]string, error) {
	var options []*model.ProductConfigOption
	if err := r.db.WithContext(ctx).
		Preload("Subs").
		Where("product_id = ? AND source = ?", productID, model.ConfigSourceSelf).
		Order("sort_order asc, id asc").
		Find(&options).Error; err != nil {
		return nil, err
	}
	if len(options) == 0 {
		return nil, nil
	}
	out := map[string]string{}
	for _, opt := range options {
		key := strings.TrimSpace(opt.SourceKey)
		if key == "" {
			key = strings.TrimSpace(opt.OptionName)
		}
		if key == "" {
			continue
		}
		value := ""
		for _, sub := range opt.Subs {
			if sub.Hidden != 0 {
				continue
			}
			if sub.SourceKey != "" {
				value = sub.SourceKey
			} else {
				value = sub.OptionName
			}
			break
		}
		if value != "" {
			out[key] = value
		}
	}
	return out, nil
}

// parseConfigGroups 把 config_groups（接口切片，来自 extractConfigGroups 或上游镜像结构）解析为内部类型。
func parseConfigGroups(groups []interface{}) ([]configGroup, error) {
	var parsed []configGroup
	raw, err := json.Marshal(groups)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

// buildConfigGroups 把商品配置子表行重建为 config_groups 数组（结构对齐上游 ConfigGroup）。
func buildConfigGroups(options []*model.ProductConfigOption) []interface{} {
	if len(options) == 0 {
		return nil
	}
	group := &configGroup{Options: make([]configOption, 0, len(options))}
	for _, opt := range options {
		copt := configOption{
			OptionName: opt.OptionName,
			OptionType: opt.OptionType,
			UpstreamID: opt.UpstreamKey,
			Source:     opt.Source,
			SourceKey:  opt.SourceKey,
			Sub:        make([]configSub, 0, len(opt.Subs)),
		}
		for _, sub := range opt.Subs {
			copt.Sub = append(copt.Sub, configSub{
				OptionName: sub.OptionName,
				UpstreamID: sub.UpstreamKey,
				Source:     sub.Source,
				SourceKey:  sub.SourceKey,
				Hidden:     sub.Hidden,
				SortOrder:  sub.SortOrder,
				Pricings: []configPrice{{
					Monthly:   flexFloat(sub.PriceMonthly),
					Annually:  flexFloat(sub.PriceAnnually),
					Quarterly: flexFloat(sub.PriceQuarterly),
					Onetime:   flexFloat(sub.PriceOnetime),
				}},
			})
		}
		group.Options = append(group.Options, copt)
	}
	out := make([]interface{}, 0, 1)
	out = append(out, group)
	return out
}

// firstPrice 返回首个 pricing 项；为空返回全零。
func firstPrice(prices []configPrice) configPrice {
	if len(prices) == 0 {
		return configPrice{}
	}
	return prices[0]
}

// intOr 值为 0 时返回默认值（如 option_type 缺省按 1 下拉）。
func intOr(v int, def int) int {
	if v == 0 {
		return def
	}
	return v
}
