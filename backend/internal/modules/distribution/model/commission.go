package model

import "time"

type Commission struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement"`
	AgentID        uint64     `gorm:"column:agent_id;not null;index"`
	SubordinateID  *uint64    `gorm:"column:subordinate_id;index"`
	SettlementID   *uint64    `gorm:"column:settlement_id;index"`
	OrderNo        string     `gorm:"column:order_no;size:64;not null;uniqueIndex"`
	SourceType     string     `gorm:"column:source_type;size:32;not null;default:order"`
	CommissionType string     `gorm:"column:commission_type;size:32;not null;default:direct"`
	BaseAmount     float64    `gorm:"column:base_amount;type:decimal(15,2);not null;default:0"`
	Rate           float64    `gorm:"type:decimal(7,4);not null;default:0"`
	Amount         float64    `gorm:"type:decimal(15,2);not null;default:0"`
	Status         string     `gorm:"size:32;not null;default:pending"`
	FreezeUntil    *time.Time `gorm:"column:freeze_until"`
	SettledAt      *time.Time `gorm:"column:settled_at"`
	Remark         string     `gorm:"size:255"`
	CreatedAt      time.Time  `gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime"`
}

func (Commission) TableName() string {
	return "distribution_commissions"
}
