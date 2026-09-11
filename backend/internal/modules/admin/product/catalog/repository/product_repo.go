// Package repository 提供商品管理（catalog）子域的数据访问实现。
package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

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
	AddHistory(ctx context.Context, history *model.ProductHistory) error
	ListHistory(ctx context.Context, productID uint64) ([]model.ProductHistory, error)
	// SaveConfigOptions 保存商品的配置组（上游 config_groups）到子表（幂等：先删后插）。
	SaveConfigOptions(ctx context.Context, productID uint64, groups []interface{}) error
	// ConfigGroupsByProductID 读取商品的配置组，重建为 config_groups 结构（供上游下单使用）。
	ConfigGroupsByProductID(ctx context.Context, productID uint64) ([]interface{}, error)
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
	if query.Status != 0 {
		base = base.Where("status = ?", query.Status)
	}
	if query.ProvisionMode != "" {
		base = base.Where("provision_mode = ?", query.ProvisionMode)
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
	ID         int64       `json:"id"`
	OptionName string      `json:"option_name"`
	OptionType int         `json:"option_type"`
	QtyMinimum int         `json:"qty_minimum"`
	QtyMaximum int         `json:"qty_maximum"`
	UpstreamID int64       `json:"upstream_id"`
	Hidden     int         `json:"hidden"`
	SortOrder  int         `json:"sort_order"`
	Sub        []configSub `json:"sub"`
}

type configSub struct {
	ID         int64         `json:"id"`
	OptionName string        `json:"option_name"`
	QtyMinimum int           `json:"qty_minimum"`
	QtyMaximum int           `json:"qty_maximum"`
	UpstreamID int64         `json:"upstream_id"`
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
func buildConfigOptionRows(parsed []configGroup, productID uint64) []*model.ProductConfigOption {
	rows := make([]*model.ProductConfigOption, 0)
	for _, g := range parsed {
		for _, opt := range g.Options {
			option := &model.ProductConfigOption{
				ProductID:   productID,
				UpstreamKey: opt.UpstreamID,
				OptionName:  opt.OptionName,
				OptionType:  intOr(opt.OptionType, 1),
				SortOrder:   opt.SortOrder,
			}
			for _, sub := range opt.Sub {
				price := firstPrice(sub.Pricings)
				option.Subs = append(option.Subs, model.ProductConfigOptionSub{
					UpstreamKey:    sub.UpstreamID,
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

// ConfigGroupsByProductID 读取商品配置子表，重建为 config_groups 数组（结构对齐上游 ConfigGroup）。
func (r *productRepository) ConfigGroupsByProductID(ctx context.Context, productID uint64) ([]interface{}, error) {
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
			Sub:        make([]configSub, 0, len(opt.Subs)),
		}
		for _, sub := range opt.Subs {
			copt.Sub = append(copt.Sub, configSub{
				OptionName: sub.OptionName,
				UpstreamID: sub.UpstreamKey,
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
