// Package model 提供定价与计费（pricing）子域的数据模型。
package model

import "time"

// 计费模式
const (
	BillingModeHourly  string = "hourly"  // 按量计费（按小时/分钟）
	BillingModeMonthly string = "monthly" // 包年包月
	BillingModeSpot    string = "spot"    // 竞价实例
)

// 价格策略状态
const (
	PricingDisabled int = 0 // 停用
	PricingEnabled  int = 1 // 启用
)

// ProductPricing 定价表：针对单个商品配置计费模式、单价、阶梯定价与等级折扣。
// TierPricing 为 JSON 数组，形如 [{"threshold":50,"price":1.08}]（阶梯用量阈值与单价）。
// TierDiscount 为 JSON 数组，形如 [{"level":"standard","percent":5}]（用户等级与折扣百分比）。
type ProductPricing struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	ProductID      uint64    `gorm:"column:product_id;not null;uniqueIndex"`   // 商品 ID（一商品多计费模式时拆多条）
	BillingMode    string    `gorm:"column:billing_mode;size:20;not null"`     // 计费模式
	UnitPrice      float64   `gorm:"column:unit_price;type:decimal(10,2)"`     // 单价
	MinBillingUnit int       `gorm:"column:min_billing_unit;default:1"`        // 最小计费单位（小时/分钟）
	TierPricing    string    `gorm:"column:tier_pricing;type:text"`            // 阶梯定价 JSON
	TierDiscount   string    `gorm:"column:tier_discount;type:text"`           // 等级折扣 JSON
	Status         int       `gorm:"default:1"`                                // 状态
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (ProductPricing) TableName() string {
	return "product_pricing"
}
