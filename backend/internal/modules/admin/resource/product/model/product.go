// Package model 提供上游商品管理子域的数据模型。
package model

import "time"

// ResourceProduct 上游商品（标准化存储）
type ResourceProduct struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	ProviderID uint64    `gorm:"column:provider_id;not null;uniqueIndex:idx_provider_upstream"`
	UpstreamID string    `gorm:"column:upstream_id;size:64;not null;uniqueIndex:idx_provider_upstream"`
	Name       string    `gorm:"size:100;not null"`
	CPU        int       `gorm:"not null"`
	Memory     int       `gorm:"not null"`
	Disk       int       `gorm:"not null"`
	DiskType   string    `gorm:"column:disk_type;size:20"`
	Bandwidth  int       `gorm:"default:0"`
	OS         string    `gorm:"size:50"`
	Region     string    `gorm:"size:50"`
	Zone       string    `gorm:"size:50"`
	Specs      string    `gorm:"type:text"` // JSON 以 text 存储，避免 jsonb 空串解析崩溃
	RawSpecs   string    `gorm:"column:raw_specs;type:text"`
	CostPrice  float64   `gorm:"column:cost_price;type:decimal(10,2)"`
	SalePrice  float64   `gorm:"column:sale_price;type:decimal(10,2)"`
	Status     int       `gorm:"default:1;index"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (ResourceProduct) TableName() string {
	return "resource_products"
}
