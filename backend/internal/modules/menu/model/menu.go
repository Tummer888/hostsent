// Package model 定义菜单模块的数据库实体。
package model

import "time"

const (
	// PlatformAdmin 标识管理员后台菜单。
	PlatformAdmin = "admin"
	// PlatformUser 标识用户中心菜单。
	PlatformUser = "user"

	// TypeDirectory 表示目录节点。
	TypeDirectory = "directory"
	// TypeMenu 表示菜单节点。
	TypeMenu = "menu"

	// StatusActive 表示启用状态。
	StatusActive = "active"
	// StatusDisabled 表示禁用状态。
	StatusDisabled = "disabled"
)

// Menu 表示菜单表中的一条记录。
type Menu struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ParentID  uint64    `gorm:"not null;default:0;index:idx_menu_platform" json:"parent_id"`
	Platform  string    `gorm:"size:32;not null;index:idx_menu_platform" json:"platform"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	Type      string    `gorm:"size:32;not null;default:menu" json:"type"`
	Path      string    `gorm:"size:255" json:"path"`
	Component string    `gorm:"size:255" json:"component"`
	Icon      string    `gorm:"size:128" json:"icon"`
	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	Status    string    `gorm:"size:32;not null;default:active" json:"status"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Menu) TableName() string {
	return "menus"
}
