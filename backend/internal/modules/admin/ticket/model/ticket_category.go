package model

import "time"

// TicketCategory 工单分类
type TicketCategory struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	Name        string    `gorm:"size:64;not null"`                     // 显示名称
	Code        string    `gorm:"size:64;not null;uniqueIndex"`         // 分类编码（如 presales）
	Description string    `gorm:"size:255"`                             // 描述
	SortOrder   int       `gorm:"column:sort_order;not null;default:0"` // 排序值（越小越靠前）
	Status      string    `gorm:"size:32;not null;default:active"`      // 状态：active / disabled
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (TicketCategory) TableName() string {
	return "ticket_categories"
}
