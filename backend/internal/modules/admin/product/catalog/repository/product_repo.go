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
	// FindByCode 按商品编码取商品（建品查重 / 批量导入跳过已导入项用）。
	// products.code 上有唯一索引（idx_catalog_products_code）：先查后插才能给出
	// "编码已被占用"的可读提示，否则撞唯一约束会把 SQLSTATE 23505 原文抛给运营。
	FindByCode(ctx context.Context, code string) (*model.Product, error)
	Create(ctx context.Context, item *model.Product) error
	Update(ctx context.Context, item *model.Product) error
	Delete(ctx context.Context, id uint64) error
	// CountInstancesByProduct 统计该商品已开通/待开通的实例数。
	// 删除守卫用：商品被实例引用时不能删——实例的续费算价与详情页商品信息都靠这行，
	// 删掉商品会让那些实例后续无法续费（界面上表现为"来源商品已删除"）。
	CountInstancesByProduct(ctx context.Context, productID uint64) (int64, error)
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
	// DecrementProductStock / IncrementProductStock 商品级库存增减。
	// 上游转售商品没有 SKU，库存挂在商品行上（products.stock），
	// 与 SKU 库存一样需要原子条件更新，否则并发下单会超卖上游限量商品。
	DecrementProductStock(ctx context.Context, productID uint64, qty int) (int64, error)
	IncrementProductStock(ctx context.Context, productID uint64, qty int) (int64, error)
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
	// AllConfigOptionRows 读取商品全部配置项原始行（含子项，upstream + self）。
	// 对接商品的多值上游配置项（操作系统/网络类型）也要能被客户选择，
	// 单取自营行会让这些项永远渲染不出来。
	AllConfigOptionRows(ctx context.Context, productID uint64) ([]*model.ProductConfigOption, error)
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

// FindByCode 按编码查商品；不存在返回 gorm.ErrRecordNotFound。
//
// 这是建品查重的依据。注意它只做"尽力而为"的前置检查：真正的唯一性由
// idx_catalog_products_code 保证，并发下仍可能冲突，上层另有兜底文案。
func (r *productRepository) FindByCode(ctx context.Context, code string) (*model.Product, error) {
	var item model.Product
	if err := r.db.WithContext(ctx).Where("code = ?", strings.TrimSpace(code)).First(&item).Error; err != nil {
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

// CountInstancesByProduct 统计引用该商品的实例数（instances.sell_product_id）。
//
// 用原生表名而不是 import 实例域模型：商品域反向依赖实例域会形成循环，
// 且这里只读一个计数值，不值得为此拆包。
func (r *productRepository) CountInstancesByProduct(ctx context.Context, productID uint64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("instances").
		Where("sell_product_id = ?", productID).Count(&n).Error
	return n, err
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

// DecrementProductStock 商品级库存条件更新扣减（口径同 SKU 库存：stock<0 不限量）。
// 上游转售商品没有 SKU，上游限量是否售罄只能落在商品行上。
func (r *productRepository) DecrementProductStock(ctx context.Context, productID uint64, qty int) (int64, error) {
	if qty <= 0 {
		qty = 1
	}
	res := r.db.WithContext(ctx).Model(&model.Product{}).
		Where("id = ? AND (stock < 0 OR stock >= ?)", productID, qty).
		Updates(map[string]interface{}{
			"stock":      gorm.Expr("CASE WHEN stock < 0 THEN stock ELSE stock - ? END", qty),
			"updated_at": time.Now(),
		})
	return res.RowsAffected, res.Error
}

// IncrementProductStock 回补商品级库存（下单/开通失败与关单补偿用）。
func (r *productRepository) IncrementProductStock(ctx context.Context, productID uint64, qty int) (int64, error) {
	if qty <= 0 {
		qty = 1
	}
	res := r.db.WithContext(ctx).Model(&model.Product{}).
		Where("id = ? AND stock >= 0", productID).
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
	// IsQuantityTier 上游数量型选项的"价档"标记（落库后回读时用），
	// 区分「这是按量计价的档位」与「这是一个可选取值」。
	IsQuantityTier bool   `json:"is_quantity_tier,omitempty"`
	UpstreamID     int64  `json:"upstream_id"`
	Source         string `json:"source,omitempty"`
	SourceKey      string `json:"source_key,omitempty"`
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

// quantityOptionType 数量型配置项的 option_type 集合。
//
// 口径直接对齐上游 common.php::judgeQuantity（4/7/9/11/14/15/16/17/18/19）。
// 为什么必须按这张表判：上游对数量型选项**不收子项 id 而收数量本身**
// （ConfigOptions::filterConfigOptions 里 judgeQuantity 为真时直接把数值夹到
// qty_minimum..qty_maximum 后作为取值下发），前端也把它渲染成拉条/输入框。
// 若按普通下拉处理，客户只能从几个价档里挑一个 id，既选不了"100G 系统盘"，
// 下发的 id 也会被上游当成数量解析成错误的值。
func quantityOptionType(optionType int) bool {
	switch optionType {
	case 4, 7, 9, 11, 14, 15, 16, 17, 18, 19:
		return true
	}
	return false
}

// isQuantityWidget 判断落库的选项是否为数量型（回读时用）。
//
// 以 option_type 为准而不是 widget 字符串：widget 是本地渲染口径，历史行可能是
// select；option_type 来自上游镜像，是"这项到底怎么计价"的权威标记。
func isQuantityWidget(opt *model.ProductConfigOption) bool {
	return opt != nil && opt.Source == model.ConfigSourceUpstream && quantityOptionType(opt.OptionType)
}

// quantityTierFreeMax 数量型选项的免费额度：零价档覆盖到的最大数量。
//
// 上游把"档位内含量"表达成一个价格为 0 的价档（如系统盘 80~80 价 0），
// 客户按该额度开通时加价为 0；超过部分才按有价档计价。
func quantityTierFreeMax(subs []configSub) int {
	maxFree := 0
	for _, s := range subs {
		if s.Hidden != 0 {
			continue
		}
		if float64(firstPrice(s.Pricings).Monthly) > 0 {
			continue
		}
		if s.QtyMaximum > maxFree {
			maxFree = s.QtyMaximum
		}
	}
	return maxFree
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
			// 数量型选项（系统盘/数据盘/带宽/IP…）：上游给的是「可填的数量区间」，
			// 不是候选值。落库成带 min/max 的 qty 控件，客户才能填 100G 这样的具体值。
			//
			// 只对上游项做这层推导：自营项（source=self）本来就由建品页显式写入
			// min/max/default 与 widget，这里不该再动它，否则会覆盖运营的配置。
			isQty := source == model.ConfigSourceUpstream && quantityOptionType(opt.OptionType)
			minValue := opt.MinValue
			maxValue := opt.MaxValue
			defaultValue := opt.DefaultValue
			widget := opt.Widget
			if isQty {
				// 控件必须显式落成 qty：列默认值是 select，留空会让客户侧
				// 按下拉渲染（只有几个价档、根本填不出 100G），必须在这里定死。
				widget = model.ConfigWidgetQty
				if opt.QtyMinimum != 0 || minValue == nil {
					lo := float64(opt.QtyMinimum)
					minValue = &lo
				}
				if opt.QtyMaximum > 0 {
					hi := float64(opt.QtyMaximum)
					maxValue = &hi
				}
				// 默认值 = 免费额度（零价档覆盖到的最大数量）：客户不填就是"档位内含量"，
				// 加价为 0。缺省时按上游口径回落最小值（filterConfigOptions 里
				// configoptions 取 0/空时用 qty_minimum）。
				if strings.TrimSpace(defaultValue) == "" {
					free := quantityTierFreeMax(opt.Sub)
					if free == 0 {
						free = opt.QtyMinimum
					}
					defaultValue = strconv.Itoa(free)
				}
			}
			option := &model.ProductConfigOption{
				ProductID:    productID,
				UpstreamKey:  upstreamCartKey(opt.ID, opt.UpstreamID),
				Source:       source,
				SourceKey:    sourceKey,
				OptionName:   opt.OptionName,
				OptionType:   intOr(opt.OptionType, 1),
				ProviderType: opt.ProviderType,
				OptionKey:    firstNonEmptyStr(opt.OptionKey, sourceKey),
				Widget:       widget,
				Required:     opt.Required,
				DefaultValue: defaultValue,
				WidgetGroup:  opt.WidgetGroup,
				MinValue:     minValue,
				MaxValue:     maxValue,
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
					QtyMinimum:     sub.QtyMinimum,
					QtyMaximum:     sub.QtyMaximum,
					IsQuantityTier: isQty,
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
//
// Subs 必须显式排序：履约侧 pickConfigSub 对"没选的项"取首个可见子项，
// 而用户/后台界面按 sort_order 展示并预选第一个候选值。两处顺序若不一致，
// 界面显示"默认开 VPC 网络"而实际开出经典网络 —— 排序在这里定死，
// 保证「界面所见」与「履约默认」是同一个值。
func (r *productRepository) ConfigGroupsByProductID(ctx context.Context, productID uint64) ([]interface{}, error) {
	var options []*model.ProductConfigOption
	if err := r.db.WithContext(ctx).
		Preload("Subs", orderSubsBySort).
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

// orderSubsBySort 配置项子表的标准排序：运营调整过的顺序（sort_order）+ 创建顺序兜底。
func orderSubsBySort(db *gorm.DB) *gorm.DB {
	return db.Order("sort_order asc, id asc")
}

// AllConfigGroupsByProductID 读取商品全部配置组（含 source=self），供配置项管理页展示/编辑（T4.4）。
func (r *productRepository) AllConfigGroupsByProductID(ctx context.Context, productID uint64) ([]interface{}, error) {
	var options []*model.ProductConfigOption
	if err := r.db.WithContext(ctx).
		Preload("Subs", orderSubsBySort).
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
		Preload("Subs", orderSubsBySort).
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
	return r.configOptionRows(ctx, productID, model.ConfigSourceSelf)
}

// AllConfigOptionRows 读取商品全部配置项原始行（含子项，upstream + self）。
//
// 对接商品（source_mode=upstream）的多值上游配置项（操作系统/网络类型）是客户
// 真正可选的参数，只读 upstream 行会漏掉运营自加的开通项（主机名），只读 self 行
// 则会让上游可选值永远渲染不出来 —— 用户侧选配必须看全量。
func (r *productRepository) AllConfigOptionRows(ctx context.Context, productID uint64) ([]*model.ProductConfigOption, error) {
	return r.configOptionRows(ctx, productID, "")
}

// configOptionRows 读取配置项原始行；source 为空表示不过滤来源。
func (r *productRepository) configOptionRows(ctx context.Context, productID uint64, source string) ([]*model.ProductConfigOption, error) {
	query := r.db.WithContext(ctx).
		Preload("Subs", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order asc, id asc")
		}).
		Where("product_id = ?", productID)
	if source != "" {
		query = query.Where("source = ?", source)
	}
	var options []*model.ProductConfigOption
	if err := query.Order("sort_order asc, id asc").Find(&options).Error; err != nil {
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
		// 数量型选项的可填区间回读成 qty_minimum/qty_maximum：适配器按这两个字段
		// 判定"这是数量变量"（与上游同名同义），丢了就会当成普通可选项下发子项 id。
		if isQuantityWidget(opt) {
			if opt.MinValue != nil {
				copt.QtyMinimum = int(*opt.MinValue)
			}
			if opt.MaxValue != nil {
				copt.QtyMaximum = int(*opt.MaxValue)
			}
		}
		for _, sub := range opt.Subs {
			copt.Sub = append(copt.Sub, configSub{
				ID:             sub.UpstreamKey,
				OptionName:     sub.OptionName,
				QtyMinimum:     sub.QtyMinimum,
				QtyMaximum:     sub.QtyMaximum,
				IsQuantityTier: sub.IsQuantityTier,
				UpstreamID:     parseIDOrZero(sub.SourceKey),
				Source:         sub.Source,
				SourceKey:      sub.SourceKey,
				GroupLabel:     sub.GroupLabel,
				Help:           sub.Help,
				IsDefault:      sub.IsDefault,
				Hidden:         sub.Hidden,
				SortOrder:      sub.SortOrder,
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
