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
	ID        uint64 `gorm:"primaryKey;autoIncrement"`
	ParentID  uint64 `gorm:"column:parent_id;default:0;index"` // 父分类 ID，0 为根节点
	Name      string `gorm:"size:50;not null"`
	Icon      string `gorm:"size:255"`
	SortOrder int    `gorm:"column:sort_order;default:0"`
	Status    int    `gorm:"default:1"`
	// CostRate 分类级拿货折扣率（doc108）：如 0.60 = 六折进货。
	// 它是代理折扣的毛利校验基准 —— 折扣率低于它会亏本卖。
	// 0 表示未配置，保存代理折扣时跳过该校验（存量分类都未配）。
	CostRate  float64   `gorm:"column:cost_rate;type:decimal(10,4);not null;default:0"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (ProductCategory) TableName() string {
	return "product_categories"
}
