// Package model 定义分销模块的数据库实体。
package model

import "time"

type Settlement struct {
	ID              uint64     `gorm:"primaryKey;autoIncrement"`
	AgentID         uint64     `gorm:"column:agent_id;not null;index"`
	SettlementNo    string     `gorm:"column:settlement_no;size:64;not null;uniqueIndex"`
	PeriodStart     time.Time  `gorm:"column:period_start;not null;index"`
	PeriodEnd       time.Time  `gorm:"column:period_end;not null;index"`
	CommissionTotal float64    `gorm:"column:commission_total;type:decimal(15,2);not null;default:0"`
	DeductionTotal  float64    `gorm:"column:deduction_total;type:decimal(15,2);not null;default:0"`
	PayableTotal    float64    `gorm:"column:payable_total;type:decimal(15,2);not null;default:0"`
	Status          string     `gorm:"size:32;not null;default:draft;index"`
	ConfirmedBy     *uint64    `gorm:"column:confirmed_by"`
	ConfirmedAt     *time.Time `gorm:"column:confirmed_at"`
	PaidAt          *time.Time `gorm:"column:paid_at"`
	Remark          string     `gorm:"size:255"`
	CreatedAt       time.Time  `gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime"`
}

// TableName 返回结算单对应的数据表名。
func (Settlement) TableName() string {
	return "distribution_settlements"
}
