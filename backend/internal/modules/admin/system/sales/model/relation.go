// Package model 提供销售归属与提成子域的数据模型。
//
// 边界（见 docs/实施计划/86-员工体系与销售提成及工单协同实施文档.md §1.3–1.5）：
//   - 客户归属以 staff_sales_relations 为唯一权威，users.sales_admin_id 只是当前归属的快照列；
//   - 订单归属 orders.sales_admin_id 是「成交那一刻」的不可变快照，退款/换归属都不回改；
//   - 提成账本 sales_commission_* 与返现账本 referral_* 物理隔离，主体是 admins 而非 users。
package model

import "time"

// 归属关系状态。
const (
	RelationStatusActive   string = "active"   // 现役归属
	RelationStatusReleased string = "released" // 已释放（被改派或人工释放）
)

// StaffSalesRelation 员工-客户归属关系。同一客户同一时刻只允许一条 active 记录
// （部分唯一索引 uk_ssr_user_active 兜底）。
type StaffSalesRelation struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement"`
	AdminID  uint64 `gorm:"column:admin_id;not null;index:idx_ssr_admin"`
	UserID   uint64 `gorm:"column:user_id;not null;index:idx_ssr_user"`
	Status   string `gorm:"size:20;not null;default:active"`
	Reason   string `gorm:"size:255;not null;default:''"`
	Operator uint64 `gorm:"column:operator_id;not null;default:0"` // 操作人（admins.id，0=系统）
	// ProtectUntil 保护期截止：保护期内不允许改派（保护销售已付出的跟进成本）。
	ProtectUntil *time.Time `gorm:"column:protect_until"`
	EffectiveAt  time.Time  `gorm:"column:effective_at;not null;default:now()"`
	ReleasedAt   *time.Time `gorm:"column:released_at"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (StaffSalesRelation) TableName() string { return "staff_sales_relations" }
