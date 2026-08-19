package model

import "time"

// PlatformAdmin 标识管理员后台菜单，PlatformUser 标识用户中心菜单。
// 两类菜单共用 menus 单表，通过 Platform 字段区分。
const (
	PlatformAdmin = "admin"
	PlatformUser  = "user"

	TypeDirectory = "directory"
	TypeMenu      = "menu"

	StatusActive   = "active"
	StatusDisabled = "disabled"
)

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
