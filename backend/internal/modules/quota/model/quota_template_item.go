package model

import "time"

// QuotaTemplateItem 表示模板中的一条配额规则。
type QuotaTemplateItem struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	TemplateID uint64    `gorm:"column:template_id;not null;index;uniqueIndex:idx_quota_template_items_template_code"`
	QuotaCode  string    `gorm:"column:quota_code;size:64;not null;uniqueIndex:idx_quota_template_items_template_code"`
	QuotaName  string    `gorm:"column:quota_name;size:64;not null"`
	QuotaType  string    `gorm:"column:quota_type;size:32;not null"`
	LimitValue float64   `gorm:"column:limit_value;type:decimal(15,2);not null;default:0"`
	Unit       string    `gorm:"size:32;not null;default:count"`
	Sort       int       `gorm:"not null;default:0"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime"`
}

func (QuotaTemplateItem) TableName() string {
	return "quota_template_items"
}
