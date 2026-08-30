package model

import "time"

// OrderItem 订单项（一单多产品）
type OrderItem struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	OrderID     uint64    `gorm:"column:order_id;not null;index"`
	ProductID   uint64    `gorm:"column:product_id;index"`
	ProductName string    `gorm:"column:product_name;size:128;not null"`
	SpecCode    string    `gorm:"column:spec_code;size:64"`
	Specs       string    `gorm:"type:text"`
	Price       float64   `gorm:"type:decimal(15,2);not null;default:0"`
	Quantity    int       `gorm:"default:1"`
	Amount      float64   `gorm:"type:decimal(15,2);not null;default:0"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (OrderItem) TableName() string {
	return "order_items"
}
