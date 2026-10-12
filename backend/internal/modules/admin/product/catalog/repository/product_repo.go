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
	categorymodel "hostsent/backend/internal/modules/admin/product/category/model"
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
	// ListImportedRefs 批量取「已导入的上游商品 → 销售商品」引用（上游商品页/导入弹窗标注已导入）。
	// 返回 resource_products.id → 销售商品（同一上游商品正常只导入一次，重复时取最早的一条）。
	ListImportedRefs(ctx context.Context, resourceProductIDs []uint64) (map[uint64]model.Product, error)
	// SaveConfigOptions 保存商品的配置组（上游 config_groups）到子表（幂等：先删后插）。
	// 配置组内可带 source/source_key 覆盖来源（T4.4）；缺省按 upstream + upstream_id 派生。
	SaveConfigOptions(ctx context.Context, productID uint64, groups []interface{}) error
	// ConfigGroupsByProductID 读取商品的配置组，重建为 config_groups 结构（供上游下单使用）。
	ConfigGroupsByProductID(ctx context.Context, productID uint64) ([]interface{}, error)
	// SaveSelfOptionRows 幂等保存 source=self 的客户选配项：只重建自营行，上游镜像行原样保留。
	// 用途：导入对接商品时默认注入「主机名」等开通选项，而不动上游配置项。
	SaveSelfOptionRows(ctx context.Context, productID uint64, rows []*model.ProductConfigOption) error
	// AllConfigGroupsByProductID 读取商品全部配置组（含 source=self），供配置项管理页展示/编辑（T4.4）。
	AllConfigGroupsByProductID(ctx context.Context, productID uint64) ([]interface{}, error)
	// SelfConfigParams 读取 source=self 的配置项，返回"平台参数名 → 选中值"（T4.4 自营可配置项）。
	SelfConfigParams(ctx context.Context, productID uint64) (map[string]string, error)
	// SelfConfigOptionRows 读取 source=self 的配置项原始行（含子项），供用户侧选配
	// 渲染与选项计价使用（T4.5）：需要全部可选值与加价，而不是首个取值。
	SelfConfigOptionRows(ctx context.Context, productID uint64) ([]*model.ProductConfigOption, error)
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
		// 分类导航点父节点时应看到整棵子树的货：把所选分类展开为「自身 + 全部子孙」再过滤。
		// 分类表量级很小（几十行），一次读入内存构建父子映射即可，不值得递归 SQL。
		ids, err := r.categorySubtreeIDs(ctx, query.CategoryID)
		if err != nil {
			return nil, 0, err
		}
		base = base.Where("category_id IN ?", ids)
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

// categorySubtreeIDs 返回给定分类及其全部子孙的 ID（产品列表按分类导航用）。
func (r *productRepository) categorySubtreeIDs(ctx context.Context, rootID uint64) ([]uint64, error) {
	var cats []categorymodel.ProductCategory
	if err := r.db.WithContext(ctx).Select("id", "parent_id").Find(&cats).Error; err != nil {
		return nil, err
	}
	children := make(map[uint64][]uint64, len(cats))
	for _, c := range cats {
		children[c.ParentID] = append(children[c.ParentID], c.ID)
	}
	ids := []uint64{rootID}
	queue := []uint64{rootID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, child := range children[cur] {
			ids = append(ids, child)
			queue = append(queue, child)
		}
	}
	return ids, nil
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

// ListImportedRefs 批量取已导入引用；入参为空直接返回空表（避免 IN () 语法错误）。
func (r *productRepository) ListImportedRefs(ctx context.Context, resourceProductIDs []uint64) (map[uint64]model.Product, error) {
	out := make(map[uint64]model.Product, len(resourceProductIDs))
	if len(resourceProductIDs) == 0 {
		return out, nil
	}
	var items []model.Product
	if err := r.db.WithContext(ctx).
		Where("source_product_id IN ?", resourceProductIDs).
		Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.SourceProductID == 0 {
			continue
		}
		if _, exists := out[item.SourceProductID]; exists {
			continue // 重复导入时保留最早一条
		}
		out[item.SourceProductID] = item
	}
	return out, nil
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
// JSON 结构与上游 (pkg/upstream/zjmf) 的 ConfigGroup/ConfigOption/ConfigSub 对齐，
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
	Source    string `json:"source,omitempty"`
	SourceKey string `json:"source_key,omitempty"`
	// ---- T4.5 客户选配渲染与计价（自营配置档生成）----
	ProviderType string   `json:"provider_type,omitempty"`
	OptionKey    string   `json:"option_key,omitempty"` // 平台参数名（下发给魔方云的键）
	Widget       string   `json:"widget,omitempty"`     // select|radio|qty|bool
	Label        string   `json:"label,omitempty"`      // 中文名（CPU/操作系统）
	Required     bool     `json:"required,omitempty"`   // 客户必选
	DefaultValue string   `json:"default_value,omitempty"`
	WidgetGroup  string   `json:"widget_group,omitempty"` // 分组标签
	MinValue     *float64 `json:"min_value,omitempty"`
	MaxValue     *float64 `json:"max_value,omitempty"`
	Unit         string   `json:"unit,omitempty"`
	Help         string   `json:"help,omitempty"`
	// OptionsHelp 取值含义表（JSON 对象文本：值 → 含义），解释特殊值。
	OptionsHelp string      `json:"options_help,omitempty"`
	Hidden      int         `json:"hidden"`
	SortOrder   int         `json:"sort_order"`
	Sub         []configSub `json:"sub"`
}

type configSub struct {
	ID         int64  `json:"id"`
	OptionName string `json:"option_name"`
	QtyMinimum int    `json:"qty_minimum"`
	QtyMaximum int    `json:"qty_maximum"`
	UpstreamID int64  `json:"upstream_id"`
	Source     string `json:"source,omitempty"`
	SourceKey  string `json:"source_key,omitempty"`
	// GroupLabel/Help/IsDefault 用户侧分组下拉、取值含义与默认选中（T4.5）。
	GroupLabel string        `json:"group_label,omitempty"`
	Help       string        `json:"help,omitempty"`
	IsDefault  bool          `json:"is_default,omitempty"`
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

// SaveSelfOptionRows 幂等保存 source=self 的客户选配项：只删自营行再插入，上游镜像行不受影响。
// 与 SaveConfigOptions 的区别：后者是"整表覆盖"（管理端配置项编辑页语义），
// 本方法供导入流程注入默认开通选项（如主机名）使用，不能碰上游 configoption 镜像行。
func (r *productRepository) SaveSelfOptionRows(ctx context.Context, productID uint64, rows []*model.ProductConfigOption) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sub := tx.Model(&model.ProductConfigOption{}).Select("id").
			Where("product_id = ? AND source = ?", productID, model.ConfigSourceSelf)
		if err := tx.Where("option_id IN (?)", sub).Delete(&model.ProductConfigOptionSub{}).Error; err != nil {
			return err
		}
		if err := tx.Where("product_id = ? AND source = ?", productID, model.ConfigSourceSelf).
			Delete(&model.ProductConfigOption{}).Error; err != nil {
			return err
		}
		for _, row := range rows {
			if row == nil {
				continue
			}
			copyRow := *row
			copyRow.ID = 0
			copyRow.ProductID = productID
			copyRow.Source = model.ConfigSourceSelf
			subs := copyRow.Subs
			copyRow.Subs = nil
			if err := tx.Omit("Subs").Create(&copyRow).Error; err != nil {
				return err
			}
			for i := range subs {
				subs[i].ID = 0
				subs[i].OptionID = copyRow.ID
				if err := tx.Create(&subs[i]).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// buildConfigOptionRows 把解析后的 config_groups 组织为商品配置项（含子项）行，便于落库。
//
// upstream_key 落库口径（2026-10-10 修正）：存**上游自身 id**（option.id / sub.id），
// 它是向上游 cart 下单的 configoption 键（上游 ConfigOptions::filterConfigOptions 只认自身 id）。
// 上游的 upstream_id（上游再上一层的 id）改存 source_key，仅作链路参考。
// 历史实现把 upstream_id 当键存，导致：原生商品（upstream_id=0）参数整体丢失，
// 转售商品（upstream_id≠0）键错被上游过滤 → 按上游默认配置开通。见 provider.go::buildCartConfigOption。
func buildConfigOptionRows(parsed []configGroup, productID uint64) []*model.ProductConfigOption {
	rows := make([]*model.ProductConfigOption, 0)
	for _, g := range parsed {
		for _, opt := range g.Options {
			source, sourceKey := normalizeConfigSource(opt.Source, opt.SourceKey, opt.UpstreamID, opt.ID)
			option := &model.ProductConfigOption{
				ProductID:    productID,
				UpstreamKey:  upstreamCartKey(opt.ID, opt.UpstreamID),
				Source:       source,
				SourceKey:    sourceKey,
				OptionName:   opt.OptionName,
				OptionType:   intOr(opt.OptionType, 1),
				ProviderType: opt.ProviderType,
				OptionKey:    firstNonEmptyStr(opt.OptionKey, sourceKey),
				Widget:       opt.Widget,
				Required:     opt.Required,
				DefaultValue: opt.DefaultValue,
				WidgetGroup:  opt.WidgetGroup,
				MinValue:     opt.MinValue,
				MaxValue:     opt.MaxValue,
				Unit:         opt.Unit,
				Help:         opt.Help,
				OptionsHelp:  nullableJSONText(opt.OptionsHelp),
				SortOrder:    opt.SortOrder,
			}
			for _, sub := range opt.Sub {
				price := firstPrice(sub.Pricings)
				subSource, subSourceKey := normalizeConfigSource(sub.Source, sub.SourceKey, sub.UpstreamID, sub.ID)
				option.Subs = append(option.Subs, model.ProductConfigOptionSub{
					UpstreamKey:    upstreamCartKey(sub.ID, sub.UpstreamID),
					Source:         subSource,
					SourceKey:      subSourceKey,
					OptionName:     sub.OptionName,
					Hidden:         sub.Hidden,
					PriceMonthly:   float64(price.Monthly),
					PriceAnnually:  float64(price.Annually),
					PriceQuarterly: float64(price.Quarterly),
					PriceOnetime:   float64(price.Onetime),
					GroupLabel:     sub.GroupLabel,
					Help:           sub.Help,
					IsDefault:      sub.IsDefault,
					SortOrder:      sub.SortOrder,
				})
			}
			rows = append(rows, option)
		}
	}
	return rows
}

// nullableJSONText 归一写入 jsonb 列的文本：空串/{} 返回 nil（空串不是合法 jsonb，落库报 22P02）。
func nullableJSONText(s string) *string {
	t := strings.TrimSpace(s)
	if t == "" || t == "null" || t == "{}" {
		return nil
	}
	return &t
}

// jsonTextOrEmpty 读 jsonb 列时转回纯文本（NULL → 空串），供 config_groups 视图使用。
func jsonTextOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// normalizeConfigSource 归一配置项来源：source 仅接受 upstream/self，其余按下列顺序推断：
//  1. 有上游自身 id 或 upstream_id → upstream（来自上游镜像的配置项，
//     原生商品 upstream_id 为 0 但有自身 id，历史实现据此误判成 self 而整套丢参）；
//  2. 否则视为 self（运营在商品详情页手工新增的平台参数项）。
//
// source_key：explicit 优先；upstream 且未指定时取 upstream_id（上游再上一层的 id，仅作链路参考）。
func normalizeConfigSource(source, sourceKey string, upstreamID, ownID int64) (string, string) {
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
	if upstreamID != 0 || ownID != 0 {
		if sourceKey == "" && upstreamID != 0 {
			sourceKey = strconv.FormatInt(upstreamID, 10)
		}
		return model.ConfigSourceUpstream, sourceKey
	}
	// 无上游 id 且未显式声明来源：视为自营平台参数项。
	return model.ConfigSourceSelf, sourceKey
}

// upstreamCartKey 取向上游 cart 下单的配置项键：优先上游自身 id，缺失退回 upstream_id。
func upstreamCartKey(ownID, upstreamID int64) int64 {
	if ownID != 0 {
		return ownID
	}
	return upstreamID
}

// parseIDOrZero 把 source_key（可能存的是数字 id，也可能是平台参数名）解析为 id，非数字返回 0。
func parseIDOrZero(s string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return v
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

// SelfConfigOptionRows 读取 source=self 的配置项原始行（含子项）。
// 与 SelfConfigParams 的区别：后者只取"首个可见取值"用于开通兜底，前者要完整
// 可选值与加价，供用户侧选配界面渲染与选项计价（T4.5）。
func (r *productRepository) SelfConfigOptionRows(ctx context.Context, productID uint64) ([]*model.ProductConfigOption, error) {
	var options []*model.ProductConfigOption
	if err := r.db.WithContext(ctx).
		Preload("Subs", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order asc, id asc")
		}).
		Where("product_id = ? AND source = ?", productID, model.ConfigSourceSelf).
		Order("sort_order asc, id asc").
		Find(&options).Error; err != nil {
		return nil, err
	}
	return options, nil
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
//
// id / upstream_id 双字段口径（2026-10-10 修正）：
//   - id          = 上游自身配置项 id（cart configoption 的键，下发给上游必须用它）；
//   - upstream_id = 上游再上一层的 id（仅链路参考，来自 source_key）。
//
// upstream_key 列存的是前者，source_key 列存的是后者，故重建时各归其位。
func buildConfigGroups(options []*model.ProductConfigOption) []interface{} {
	if len(options) == 0 {
		return nil
	}
	group := &configGroup{Options: make([]configOption, 0, len(options))}
	for _, opt := range options {
		copt := configOption{
			ID:           opt.UpstreamKey,
			OptionName:   opt.OptionName,
			OptionType:   opt.OptionType,
			UpstreamID:   parseIDOrZero(opt.SourceKey),
			Source:       opt.Source,
			SourceKey:    opt.SourceKey,
			ProviderType: opt.ProviderType,
			OptionKey:    opt.OptionKey,
			Widget:       opt.Widget,
			Label:        opt.OptionName,
			Required:     opt.Required,
			DefaultValue: opt.DefaultValue,
			WidgetGroup:  opt.WidgetGroup,
			MinValue:     opt.MinValue,
			MaxValue:     opt.MaxValue,
			Unit:         opt.Unit,
			Help:         opt.Help,
			OptionsHelp:  jsonTextOrEmpty(opt.OptionsHelp),
			Sub:          make([]configSub, 0, len(opt.Subs)),
		}
		for _, sub := range opt.Subs {
			copt.Sub = append(copt.Sub, configSub{
				ID:         sub.UpstreamKey,
				OptionName: sub.OptionName,
				UpstreamID: parseIDOrZero(sub.SourceKey),
				Source:     sub.Source,
				SourceKey:  sub.SourceKey,
				GroupLabel: sub.GroupLabel,
				Help:       sub.Help,
				IsDefault:  sub.IsDefault,
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

// firstNonEmptyStr 返回第一个非空（去空白后）字符串。
func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
