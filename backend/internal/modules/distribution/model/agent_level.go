package model

import "time"

type AgentLevel struct {
	ID                     uint64    `gorm:"primaryKey;autoIncrement"`
	Name                   string    `gorm:"size:64;not null;uniqueIndex"`
	Code                   string    `gorm:"size:64;not null;uniqueIndex"`
	Weight                 int       `gorm:"not null;default:0"`
	DirectCommissionRate   float64   `gorm:"column:direct_commission_rate;type:decimal(7,4);not null;default:0"`
	IndirectCommissionRate float64   `gorm:"column:indirect_commission_rate;type:decimal(7,4);not null;default:0"`
	RenewalCommissionRate  float64   `gorm:"column:renewal_commission_rate;type:decimal(7,4);not null;default:0"`
	UpgradeRewardAmount    float64   `gorm:"column:upgrade_reward_amount;type:decimal(15,2);not null;default:0"`
	SelfPurchaseRebateRate float64   `gorm:"column:self_purchase_rebate_rate;type:decimal(7,4);not null;default:0"`
	AllowManualPrice       bool      `gorm:"column:allow_manual_price;not null;default:false"`
	AllowSubAgent          bool      `gorm:"column:allow_sub_agent;not null;default:false"`
	MaxSubAgentDepth       int       `gorm:"column:max_sub_agent_depth;not null;default:0"`
	Status                 string    `gorm:"size:32;not null;default:active"`
	Description            string    `gorm:"size:255"`
	CreatedAt              time.Time `gorm:"autoCreateTime"`
	UpdatedAt              time.Time `gorm:"autoUpdateTime"`
}

func (AgentLevel) TableName() string {
	return "agent_levels"
}
