// Package model 定义用户中心成员（子账号）模块的数据库实体。
package model

import "time"

// Member 子账号（成员）视图，映射 users 表。
// 该表同时被多处模块复用，此处仅保留成员管理所需字段（R2 的第三份 users 映射）。
type Member struct {
	ID           uint64 `gorm:"primaryKey"`
	Username     string `gorm:"size:64;not null"`
	Email        string `gorm:"size:128;not null"`
	Phone        string `gorm:"size:32"`
	PasswordHash string `gorm:"column:password_hash;size:255;not null"`
	Status       string `gorm:"size:32;not null;default:active"`
	RealName     string `gorm:"column:real_name;size:64"`
	// OwnerUserID 归属主账号 ID（P4-01）。
	OwnerUserID *uint64 `gorm:"column:owner_user_id"`
	// IsSubAccount 固定为 true。
	IsSubAccount bool `gorm:"column:is_sub_account;not null;default:false"`
	// SubAccountRemark 主账号给成员打的备注。
	SubAccountRemark string     `gorm:"column:sub_account_remark;size:64"`
	LastLoginAt      *time.Time `gorm:"column:last_login_at"`
	LastLoginIP      string     `gorm:"column:last_login_ip;size:64"`
	CreatedAt        time.Time  `gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (Member) TableName() string {
	return "users"
}

// SubAccountPermission 子账号权限授予记录（P4-01）。
type SubAccountPermission struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	UserID         uint64    `gorm:"column:user_id;not null;uniqueIndex:uk_sub_account_perms"`
	PermissionCode string    `gorm:"column:permission_code;size:64;not null;uniqueIndex:uk_sub_account_perms"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名。
func (SubAccountPermission) TableName() string {
	return "sub_account_permissions"
}

// OperationLog 用户侧操作日志（P4-08）。
//
// 记录子账号的写操作，主账号可在成员详情查看「谁在什么时候做了什么」。
type OperationLog struct {
	ID uint64 `gorm:"primaryKey;autoIncrement"`
	// ActorUserID 真实操作人（子账号 ID 或主账号 ID）。
	ActorUserID uint64 `gorm:"column:actor_user_id;not null;index"`
	// ActorName 操作人用户名快照。
	ActorName string `gorm:"column:actor_name;size:64"`
	// AccountUserID 数据归属账号 ID（主账号）。
	AccountUserID uint64 `gorm:"column:account_user_id;not null;index"`
	// Module 业务模块，如 order/instance/finance/ticket。
	Module string `gorm:"size:32;not null"`
	// Action 动作，如 create/renew/power/reply。
	Action string `gorm:"size:32;not null"`
	// Target 操作对象描述（订单号、实例 ID 等）。
	Target string `gorm:"size:128"`
	// Detail 补充说明。
	Detail    string    `gorm:"size:255"`
	IP        string    `gorm:"column:ip;size:64"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名。
func (OperationLog) TableName() string {
	return "user_operation_logs"
}
