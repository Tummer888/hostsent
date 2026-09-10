package model

import "time"

// 角色作用域：隔离后台员工角色与客户侧角色。
const (
	RoleScopeAdmin = "admin"
	RoleScopeUser  = "user"
)

type Role struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"size:64;not null;uniqueIndex"`
	Code        string `gorm:"size:64;not null;uniqueIndex"`
	Description string `gorm:"column:description;size:255"`
	// Scope 隔离后台角色与客户角色：admin（默认，进后台权限树）/ user（客户侧，本方案不使用）。
	Scope     string    `gorm:"size:16;not null;default:admin"`
	Status    string    `gorm:"size:32;not null;default:active"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (Role) TableName() string {
	return "roles"
}
