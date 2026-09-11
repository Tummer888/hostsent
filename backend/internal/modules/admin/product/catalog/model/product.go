// Package model 提供商品管理（catalog）子域的数据模型。
package model

import "time"

// 产品状态
const (
	ProductStatusDraft     int = 0 // 草稿
	ProductStatusPublished int = 1 // 上架
	ProductStatusOffline   int = 2 // 下架
)

// 商品供货模式
const (
	ProvisionModeClone string = "clone" // 上游克隆：直接销售上游商品（对接魔方财务）
	ProvisionModeSelf  string = "self"  // 自营：自定义配置映射到上游（对接魔方云）
)

// 商品链路判据（双链路重构 D6，单一判据，禁止与 provision_mode 混用）
const (
	SourceModeSelf     string = "self"     // 自营链路
	SourceModeUpstream string = "upstream" // 上游转售链路
)

// 价格模型
const (
	PriceModelFixed   string = "fixed"   // 固定价
	PriceModelHourly  string = "hourly"  // 按小时
	PriceModelMonthly string = "monthly" // 按月
)

// ProductSpecItem 产品规格项（Specs 列中的 JSON 结构）
type ProductSpecItem struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value any    `json:"value"`
}

// Product 面向终端售卖的产品
type Product struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement"`
	Code             string     `gorm:"column:code;size:64;uniqueIndex;not null"`           // SKU 编码
	Name             string     `gorm:"size:100;not null"`                                  // 产品名称
	CategoryID       uint64     `gorm:"column:category_id;index"`                           // 分类 ID
	ProductType      string     `gorm:"column:product_type;size:20;default:'cloud_host'"`   // 产品类型
	Description      string     `gorm:"type:text"`                                          // 描述
	CoverImage       string     `gorm:"column:cover_image;size:255"`                        // 封面图 URL（官网产品卡/详情页展示，空则由前端用占位图兜底）
	Specs            string     `gorm:"type:text"`                                          // JSON 规格：自营/克隆商品的可编辑展示规格。克隆默认取 buildCloneBaseOptions(rp)（开通参数词汇：system_disk_size/bw/area/node）；与资源商品 ResourceProduct.Specs（StandardProductSpec 词汇：disk/bandwidth/region/zone）语义不同，见 docs/实施计划/00 §Phase4 T4.4
	PriceModel       string     `gorm:"column:price_model;size:20;default:'fixed'"`         // 价格模型
	Price            float64    `gorm:"column:price;type:decimal(10,2)"`                    // 终端售价
	CostPrice        float64    `gorm:"column:cost_price;type:decimal(10,2)"`               // 成本价
	SourceProductID  uint64     `gorm:"column:source_product_id;index"`                     // 克隆模式：关联【本地】resource_products.id（上游资源商品的本地主键，非上游 upstream_id）
	SourceProviderID uint64     `gorm:"column:source_provider_id;index"`                    // 关联上游提供商 ID（克隆模式的推送目标）
	ProvisionMode    string     `gorm:"column:provision_mode;size:20;default:'self';index"` // 供货模式：self 自营 / clone 上游克隆（保留只读一版，P8 删除）
	SourceMode       string     `gorm:"column:source_mode;size:16;index"`                   // 链路判据：self 自营 / upstream 上游转售（双链路重构 D6 单一判据）
	ConfigOptions    string     `gorm:"column:config_options;type:text"`                    // JSON 可配置项（自营模式映射到上游 /clouds 参数；克隆模式可覆盖规格）
	Featured         bool       `gorm:"column:featured;default:false;index"`                // 是否前台推荐（推荐位管理）
	Stock            int        `gorm:"default:-1"`                                         // 库存，-1 表示不限
	SortOrder        int        `gorm:"column:sort_order;default:0"`                        // 排序
	Status           int        `gorm:"default:0;index"`                                    // 状态
	CreatedAt        time.Time  `gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime"`
	DeletedAt        *time.Time `gorm:"index"`
}

// TableName 指定表名
func (Product) TableName() string {
	return "products"
}
