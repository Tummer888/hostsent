// Package model 定义分销模块的数据库实体。
package model

import "time"

type Commission struct {
	ID uint64 `gorm:"primaryKey;autoIncrement"`
	// OrderID 关联订单 ID（P6-02）：与 AgentID 组成唯一键，保证订单完成自动计提幂等。
	// 手工录入的历史佣金无关联订单，保持 NULL（Postgres 唯一索引对 NULL 不去重）。
	OrderID        *uint64    `gorm:"column:order_id;uniqueIndex:uk_commissions_order_agent"`
	AgentID        uint64     `gorm:"column:agent_id;not null;index;uniqueIndex:uk_commissions_order_agent"`
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

// TableName 返回佣金记录对应的数据表名。
func (Commission) TableName() string {
	return "distribution_commissions"
}
