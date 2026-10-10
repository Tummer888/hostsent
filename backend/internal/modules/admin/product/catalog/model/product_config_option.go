package model

import "time"

// 配置项来源（T4.4 / 16 §7.5）。两条链路共用一套表与渲染/校验逻辑。
const (
	ConfigSourceUpstream = "upstream" // source_key 是上游配置项 id（代理链路）
	ConfigSourceSelf     = "self"     // source_key 是平台参数名（自营链路，如 area/os/store）
)

// ProductConfigOption 商品配置项（上游 configoption），对应 product_config_options 表。
// 由克隆/上游导入流程把 resource_products.raw_specs->'config_groups' 中的配置项落库，
// 替代对 JSON 文本的惰性解析（见 migration 015）。
//
// T4.5 起同时承载"客户可选配置项"：自营商品按配置档生成时，为每个多值参数
// 写一行（SourceKey=平台参数名），Widget/Required/DefaultValue 等驱动用户侧控件渲染。
type ProductConfigOption struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	ProductID   uint64 `gorm:"column:product_id;not null;index;constraint:OnDelete:CASCADE"` // 所属销售商品 ID
	UpstreamKey int64  `gorm:"column:upstream_key;default:0"`                                // 旧列（P8 清理）：上游 configoption 键
	Source      string `gorm:"column:source;size:16;not null;default:upstream"`              // 配置项来源：upstream / self（T4.4）
	SourceKey   string `gorm:"column:source_key;size:128"`                                   // 来源键：upstream=上游选项 id，self=平台参数名
	OptionName  string `gorm:"column:option_name;size:64"`                                   // cpu/memory/os/area/...
	OptionType  int    `gorm:"column:option_type;default:1"`                                 // 1 下拉 2 单选 3 开关 4 数量
	// ---- T4.5 客户选配渲染与计价字段 ----
	ProviderType string `gorm:"column:provider_type;size:64;not null;default:''"` // 归属平台（mofangyun）
	OptionKey    string `gorm:"column:option_key;size:64;not null;default:''"`    // 平台参数名（与 source_key 同义，选配专用）
	Widget       string `gorm:"column:widget;size:32;not null;default:'select'"`  // select|radio|qty|bool
	Required     bool   `gorm:"column:required;not null;default:false"`           // 客户下单必选
	DefaultValue string `gorm:"column:default_value;size:255;not null;default:''"`
	// WidgetGroup 同组下拉的分组标签（操作系统家族 Ubuntu/Windows），空则平铺。
	WidgetGroup string    `gorm:"column:widget_group;size:128;not null;default:''"`
	MinValue    *float64  `gorm:"column:min_value;type:numeric(16,4)"` // 数量型下限
	MaxValue    *float64  `gorm:"column:max_value;type:numeric(16,4)"` // 数量型上限
	Unit        string    `gorm:"column:unit;size:16;not null;default:''"`
	Help        string    `gorm:"column:help;type:text;not null;default:''"`
	SortOrder   int       `gorm:"column:sort_order;default:0"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`

	Subs []ProductConfigOptionSub `gorm:"foreignKey:OptionID;references:ID"` // 子项（可选值）
}

// TableName 指定表名
func (ProductConfigOption) TableName() string {
	return "product_config_options"
}

// ProductConfigOptionSub 配置项可选值（sub），对应 product_config_options_sub 表。
type ProductConfigOptionSub struct {
	ID             uint64  `gorm:"primaryKey;autoIncrement"`
	OptionID       uint64  `gorm:"column:option_id;not null;index;constraint:OnDelete:CASCADE"` // 所属配置项 ID
	UpstreamKey    int64   `gorm:"column:upstream_key;default:0"`                               // 旧列（P8 清理）：上游 configoption 值
	Source         string  `gorm:"column:source;size:16;not null;default:upstream"`             // 子项来源：upstream / self（T4.4）
	SourceKey      string  `gorm:"column:source_key;size:128"`                                  // 子项来源键（上游子项 id 或平台取值）
	OptionName     string  `gorm:"column:option_name;size:255"`                                 // 如 "16|16核" / "HK" / "CentOS"
	Hidden         int     `gorm:"default:0"`
	PriceMonthly   float64 `gorm:"column:price_monthly;type:decimal(15,2);default:0"`
	PriceAnnually  float64 `gorm:"column:price_annually;type:decimal(15,2);default:0"`
	PriceQuarterly float64 `gorm:"column:price_quarterly;type:decimal(15,2);default:0"`
	PriceOnetime   float64 `gorm:"column:price_onetime;type:decimal(15,2);default:0"`
	// GroupLabel 分组标签（Ubuntu/Windows/CentOS），用户侧分组下拉用。
	GroupLabel string `gorm:"column:group_label;size:128;not null;default:''"`
	// IsDefault 该值是否为客户默认选中项（每配置项宜只有一个）。
	IsDefault bool      `gorm:"column:is_default;not null;default:false"`
	SortOrder int       `gorm:"column:sort_order;default:0"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// 配置项控件类型（T4.5，用户侧选配控件选型）。
const (
	ConfigWidgetSelect = "select" // 下拉（可分组）
	ConfigWidgetRadio  = "radio"  // 单选（按钮组）
	ConfigWidgetQty    = "qty"    // 数量（步进器）
	ConfigWidgetBool   = "bool"   // 开关
)

// TableName 指定表名
func (ProductConfigOptionSub) TableName() string {
	return "product_config_options_sub"
}
