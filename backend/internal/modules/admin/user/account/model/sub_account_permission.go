package model

import "time"

// SubAccountPermission 子账号（成员）权限授予记录（P4-01）。
//
// 覆盖式设置：每次保存先清空该 user_id 的记录再写入，唯一索引兜底防重。
// 权限码为内置枚举，见 internal/pkg/auth/user_permission.go。
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
