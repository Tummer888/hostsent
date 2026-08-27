// Package model 定义配额模块的数据库实体。
package model

import "time"

// QuotaAdjustmentLog 记录一次手工或自动的配额变更事件。
type QuotaAdjustmentLog struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	UserID         uint64    `gorm:"column:user_id;not null;index"`
	Username       string    `gorm:"size:64;not null"`
	QuotaCode      string    `gorm:"column:quota_code;size:64;not null;index"`
	QuotaName      string    `gorm:"column:quota_name;size:64;not null"`
	BeforeValue    float64   `gorm:"column:before_value;type:decimal(15,2);not null;default:0"`
	AfterValue     float64   `gorm:"column:after_value;type:decimal(15,2);not null;default:0"`
	DeltaValue     float64   `gorm:"column:delta_value;type:decimal(15,2);not null;default:0"`
	AdjustmentType string    `gorm:"column:adjustment_type;size:32;not null"`
	Source         string    `gorm:"size:32;not null;default:manual"`
	TemplateID     *uint64   `gorm:"column:template_id"`
	LevelID        *uint64   `gorm:"column:level_id"`
	OperatorID     uint64    `gorm:"column:operator_id;not null;default:0;index"`
	OperatorName   string    `gorm:"column:operator_name;size:64;not null"`
	Reason         string    `gorm:"size:255"`
	TicketNo       string    `gorm:"column:ticket_no;size:64"`
	BatchNo        string    `gorm:"column:batch_no;size:64"`
	CreatedAt      time.Time `gorm:"autoCreateTime;index"`
}

func (QuotaAdjustmentLog) TableName() string {
	return "quota_adjustment_logs"
}
