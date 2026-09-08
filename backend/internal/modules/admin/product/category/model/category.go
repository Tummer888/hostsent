// Package model 提供商品分类（category）子域的数据模型。
package model

import "time"

// 分类状态
const (
	CategoryStatusDisabled int = 0 // 停用
	CategoryStatusEnabled  int = 1 // 启用
)

// ProductCategory 产品分类
type ProductCategory struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	ParentID  uint64    `gorm:"column:parent_id;default:0;index"` // 父分类 ID，0 为根节点
	Name      string    `gorm:"size:50;not null"`
	Icon      string    `gorm:"size:255"`
	SortOrder int       `gorm:"column:sort_order;default:0"`
	Status    int       `gorm:"default:1"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (ProductCategory) TableName() string {
	return "product_categories"
}
