package model

import "time"

// 规格状态
const (
	ProductSpecDisabled int = 0 // 停用
	ProductSpecEnabled  int = 1 // 启用
)

// ProductSpec 产品规格变体（套餐版本）
type ProductSpec struct {
	ID         uint64  `gorm:"primaryKey;autoIncrement"`
	ProductID  uint64  `gorm:"column:product_id;not null;index;uniqueIndex:idx_product_spec"`  // 所属产品 ID
	SpecCode   string  `gorm:"column:spec_code;size:64;not null;uniqueIndex:idx_product_spec"` // 规格编码
	Name       string  `gorm:"size:100;not null"`                                              // 规格名称，如"入门版"
	Specs      string  `gorm:"type:text"`                                                      // JSON 规格：原子 key → 取值（spec_atoms 字典口径，T4.1）
	PriceModel string  `gorm:"column:price_model;size:20;default:'fixed'"`                     // 价格模型
	Price      float64 `gorm:"column:price;type:decimal(10,2)"`                                // 规格售价
	CostPrice  float64 `gorm:"column:cost_price;type:decimal(10,2)"`                           // 规格成本
	Stock      int     `gorm:"default:-1"`                                                     // 库存，-1 不限
	// SpecTemplateID 引用的标准规格模板（product_spec_templates.id）；0 表示未绑定模板（T4.2）。
	SpecTemplateID uint64    `gorm:"column:spec_template_id;index"`
	SortOrder      int       `gorm:"column:sort_order;default:0"`
	Status         int       `gorm:"default:1"` // 1 启用 0 停用
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (ProductSpec) TableName() string {
	return "product_specs"
}
