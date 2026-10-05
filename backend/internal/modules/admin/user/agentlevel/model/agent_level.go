// Package model 定义代理等级模块的数据库实体。
//
// 代理等级是「拿货折扣」的载体（doc108）：等级越高权重越大、折扣越优
// （折扣率数值越小）。与用户等级（user_levels，消费升级、不打折）是两套
// 互不相干的阶梯 —— 不要把两者合并，一个回答"消费了多少"，一个回答"进货多少钱"。
package model

import "time"

// 折扣目标类型。命中优先级 product > category > all（与 price_policy_items 同约定）。
const (
	TargetAll      string = "all"
	TargetCategory string = "category"
	TargetProduct  string = "product"
)

// AgentLevel 代理等级（定价阶梯）。
type AgentLevel struct {
	ID   uint64 `gorm:"primaryKey;autoIncrement"`
	Name string `gorm:"size:64;not null"`
	Code string `gorm:"size:64;not null;uniqueIndex:uk_agent_levels_code"`
	// Weight 权重，越大越优先（折扣最优）。矩阵列按此降序展示。
	Weight int `gorm:"not null;default:0"`
	// Status active / disabled；禁用后该等级不再参与算价（RuleForUser 直接返回 nil）。
	Status      string `gorm:"size:32;not null;default:active"`
	Description string `gorm:"size:255"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`

	// DiscountCount 该等级已配置的折扣格数（联表统计，非持久化）。
	DiscountCount int64 `gorm:"->;-:migration"`
}

// TableName 指定表名。
func (AgentLevel) TableName() string { return "agent_levels" }

// AgentLevelDiscount 代理折扣矩阵的一格：等级 × 目标（全站/分类/商品）。
//
// 存「逐格值」而不是存「锚点 + 步长」：锚点+步长只是录入加速器
// （管理端"阶梯填充"），落库仍是可审计的具体数字。否则运营改了成本率后，
// 所有历史折扣会被静默重算，事后无法解释某笔订单为什么是这个价。
type AgentLevelDiscount struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement"`
	AgentLevelID uint64 `gorm:"column:agent_level_id;not null;uniqueIndex:uk_agent_level_discounts"`
	TargetType   string `gorm:"column:target_type;size:16;not null;uniqueIndex:uk_agent_level_discounts"`
	TargetID     uint64 `gorm:"column:target_id;not null;default:0;uniqueIndex:uk_agent_level_discounts"`
	// DiscountRate 折扣率（0.85 = 八五折，越小越优惠）；0 表示未配置（不打折）。
	DiscountRate float64 `gorm:"column:discount_rate;type:decimal(10,4);not null;default:0"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (AgentLevelDiscount) TableName() string { return "agent_level_discounts" }
