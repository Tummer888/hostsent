// Package model 提供折扣策略（price policy）子域的数据模型。
//
// 与 product/pricing 的边界（P5-02）：
//   - product_pricing 决定「这个商品基础多少钱」；
//   - price_policies  决定「这个客户打几折」。
package model

import "time"

// 折扣类型
const (
	DiscountTypeRate   string = "rate"   // 折扣率：0.85 = 85 折
	DiscountTypeAmount string = "amount" // 直减：减固定金额
)

// 作用域
const (
	ScopeAll      string = "all"      // 全站商品
	ScopeCategory string = "category" // 指定分类
	ScopeProduct  string = "product"  // 指定商品
)

// 条目目标类型
const (
	TargetCategory string = "category"
	TargetProduct  string = "product"
)

// PricePolicy 折扣策略（P5-01）。
type PricePolicy struct {
	ID            uint64     `gorm:"primaryKey;autoIncrement"`
	Name          string     `gorm:"size:64;not null"`
	Code          string     `gorm:"size:64;not null;uniqueIndex:uk_price_policies_code"`
	DiscountType  string     `gorm:"column:discount_type;size:16;not null"`
	DiscountValue float64    `gorm:"column:discount_value;type:decimal(10,4);not null"` // 0.85 = 85 折
	Scope         string     `gorm:"size:16;not null;default:all"`                      // all / category / product
	Priority      int        `gorm:"not null;default:0"`                                // 数值越大优先级越高
	EffectiveFrom *time.Time `gorm:"column:effective_from"`                             // 生效开始，NULL 表示不限
	EffectiveTo   *time.Time `gorm:"column:effective_to"`                               // 生效结束，NULL 表示不限
	Status        string     `gorm:"size:32;not null;default:active"`                   // active / disabled
	Remark        string     `gorm:"size:255"`
	CreatedAt     time.Time  `gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (PricePolicy) TableName() string { return "price_policies" }

// PricePolicyItem 策略条目：在 scope=category/product 时按目标覆盖折扣（P5-01）。
type PricePolicyItem struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement"`
	PolicyID      uint64    `gorm:"column:policy_id;not null;uniqueIndex:uk_pp_items"`
	TargetType    string    `gorm:"column:target_type;size:16;not null;uniqueIndex:uk_pp_items"` // category / product
	TargetID      uint64    `gorm:"column:target_id;not null;uniqueIndex:uk_pp_items"`
	DiscountType  string    `gorm:"column:discount_type;size:16;not null"`
	DiscountValue float64   `gorm:"column:discount_value;type:decimal(10,4);not null"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名。
func (PricePolicyItem) TableName() string { return "price_policy_items" }
