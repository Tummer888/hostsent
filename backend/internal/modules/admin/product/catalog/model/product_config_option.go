package model

import "time"

// ProductConfigOption 商品配置项（上游 configoption），对应 product_config_options 表。
// 由克隆/上游导入流程把 resource_products.raw_specs->'config_groups' 中的配置项落库，
// 替代对 JSON 文本的惰性解析（见 migration 015）。
type ProductConfigOption struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	ProductID   uint64    `gorm:"column:product_id;not null;index;constraint:OnDelete:CASCADE"` // 所属销售商品 ID
	UpstreamKey int64     `gorm:"column:upstream_key;default:0"`                                // 上游 configoption 键（option.upstream_id）
	OptionName  string    `gorm:"column:option_name;size:64"`                                   // cpu/memory/os/area/...
	OptionType  int       `gorm:"column:option_type;default:1"`                                 // 1 下拉 2 单选 3 开关 4 数量
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
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	OptionID       uint64    `gorm:"column:option_id;not null;index;constraint:OnDelete:CASCADE"` // 所属配置项 ID
	UpstreamKey    int64     `gorm:"column:upstream_key;default:0"`                               // 上游 configoption 值（sub.upstream_id）
	OptionName     string    `gorm:"column:option_name;size:255"`                                 // 如 "16|16核" / "HK" / "CentOS"
	Hidden         int       `gorm:"default:0"`
	PriceMonthly   float64   `gorm:"column:price_monthly;type:decimal(15,2);default:0"`
	PriceAnnually  float64   `gorm:"column:price_annually;type:decimal(15,2);default:0"`
	PriceQuarterly float64   `gorm:"column:price_quarterly;type:decimal(15,2);default:0"`
	PriceOnetime   float64   `gorm:"column:price_onetime;type:decimal(15,2);default:0"`
	SortOrder      int       `gorm:"column:sort_order;default:0"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (ProductConfigOptionSub) TableName() string {
	return "product_config_options_sub"
}
