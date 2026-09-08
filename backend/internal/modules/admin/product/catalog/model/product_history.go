package model

import "time"

// 变更类型
const (
	ChangeTypePublish   string = "publish"   // 上架
	ChangeTypeUnpublish string = "unpublish" // 下架
	ChangeTypePrice     string = "price"     // 调价
	ChangeTypeCreate    string = "create"    // 创建
	ChangeTypeUpdate    string = "update"    // 编辑
	ChangeTypeDelete    string = "delete"    // 删除
)

// ProductHistory 产品变更历史
type ProductHistory struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	ProductID    uint64    `gorm:"column:product_id;not null;index"`
	ChangeType   string    `gorm:"column:change_type;size:20;not null"`
	OldValue     string    `gorm:"column:old_value;type:text"`
	NewValue     string    `gorm:"column:new_value;type:text"`
	OperatorID   uint64    `gorm:"column:operator_id"`
	OperatorName string    `gorm:"column:operator_name;size:50"`
	Remark       string    `gorm:"size:255"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (ProductHistory) TableName() string {
	return "product_history"
}
