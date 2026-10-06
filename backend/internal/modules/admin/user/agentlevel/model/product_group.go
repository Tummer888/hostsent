// Package model 定义商品分组与折扣组（doc108 §8I：商品分组 × 代理等级的二维结构）。
//
// 这两张表是折扣的「录入与组织层」：商品分组 = 分类/商品的命名集合，
// 折扣组 = 绑定一个商品分组 + 每个代理等级一个折扣率。通过「应用」展开写入
// agent_level_discounts 逐格矩阵 —— 算价只认逐格矩阵，这里不产生第二真相。
package model

import "time"

// 商品分组成员粒度。
const (
	GroupTargetCategory = "category"
	GroupTargetProduct  = "product"
)

// ProductGroup 商品分组。
type ProductGroup struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"size:64;not null"`
	Code        string `gorm:"size:64;not null;uniqueIndex:uk_product_groups_code"`
	Description string `gorm:"size:255"`
	Status      string `gorm:"size:32;not null;default:active"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`

	// ItemCount 成员数（分类+商品，联表统计，非持久化）。
	ItemCount int64 `gorm:"->;-:migration"`
}

// TableName 指定表名。
func (ProductGroup) TableName() string { return "product_groups" }

// ProductGroupItem 商品分组的成员：一个分类或一个商品。
type ProductGroupItem struct {
	ID      uint64 `gorm:"primaryKey;autoIncrement"`
	GroupID uint64 `gorm:"column:group_id;not null;uniqueIndex:uk_product_group_items"`
	// TargetType category / product。
	TargetType string `gorm:"column:target_type;size:16;not null;uniqueIndex:uk_product_group_items"`
	TargetID   uint64 `gorm:"column:target_id;not null;uniqueIndex:uk_product_group_items"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名。
func (ProductGroupItem) TableName() string { return "product_group_items" }

// DiscountScheme 折扣组：绑定一个商品分组 + 每个代理等级一个折扣率。
type DiscountScheme struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"size:64;not null"`
	Code        string `gorm:"size:64;not null;uniqueIndex:uk_discount_schemes_code"`
	Description string `gorm:"size:255"`
	// ProductGroupID 绑定的商品分组（可空 = 未绑定，应用前必须绑定）。
	ProductGroupID *uint64 `gorm:"column:product_group_id"`
	Status         string  `gorm:"size:32;not null;default:active"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`

	// ItemCount 等级费率条数（联表统计，非持久化）。
	ItemCount int64 `gorm:"->;-:migration"`
}

// TableName 指定表名。
func (DiscountScheme) TableName() string { return "discount_schemes" }

// DiscountSchemeItem 折扣组的等级费率：应用时展开为该等级在分组内全部目标的格子。
type DiscountSchemeItem struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement"`
	SchemeID     uint64 `gorm:"column:scheme_id;not null;uniqueIndex:uk_discount_scheme_items"`
	AgentLevelID uint64 `gorm:"column:agent_level_id;not null;uniqueIndex:uk_discount_scheme_items"`
	// DiscountRate 折扣率（0.85 = 八五折）；0 = 该等级在此方案不打折（应用时跳过该等级）。
	DiscountRate float64 `gorm:"column:discount_rate;type:decimal(10,4);not null;default:0"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (DiscountSchemeItem) TableName() string { return "discount_scheme_items" }
