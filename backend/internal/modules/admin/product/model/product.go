// Package model 提供产品管理子域的数据模型。
package model

import "time"

// 产品状态
const (
	ProductStatusDraft     int = 0 // 草稿
	ProductStatusPublished int = 1 // 上架
	ProductStatusOffline   int = 2 // 下架
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
	Code             string     `gorm:"column:code;size:64;uniqueIndex;not null"`                       // SKU 编码
	Name             string     `gorm:"size:100;not null"`                                              // 产品名称
	CategoryID       uint64     `gorm:"column:category_id;index"`                                       // 分类 ID
	ProductType      string     `gorm:"column:product_type;size:20;default:'cloud_host'"`               // 产品类型
	Description      string     `gorm:"type:text"`                                                      // 描述
	Specs            string     `gorm:"type:text"`                                                      // JSON 规格
	PriceModel       string     `gorm:"column:price_model;size:20;default:'fixed'"`                     // 价格模型
	Price            float64    `gorm:"column:price;type:decimal(10,2)"`                                // 终端售价
	CostPrice        float64    `gorm:"column:cost_price;type:decimal(10,2)"`                           // 成本价
	SourceProductID  uint64     `gorm:"column:source_product_id;index"`                                 // 关联上游资源商品 ID（可选）
	SourceProviderID uint64     `gorm:"column:source_provider_id;index"`                                // 关联上游提供商 ID（可选）
	Stock            int        `gorm:"default:-1"`                                                     // 库存，-1 表示不限
	SortOrder        int        `gorm:"column:sort_order;default:0"`                                    // 排序
	Status           int        `gorm:"default:0;index"`                                                // 状态
	CreatedAt        time.Time  `gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime"`
	DeletedAt        *time.Time `gorm:"index"`
}

// TableName 指定表名
func (Product) TableName() string {
	return "products"
}
