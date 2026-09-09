package model

import "time"

// User 用户中心用户模型，映射 users 表。
// 该表同时被后台管理模块（admin/user/account）使用，此处仅保留用户中心
// 自助场景所需的字段；avatar / tier 为新增列，启动时由 AutoMigrate 补齐。
type User struct {
	ID                uint64     `gorm:"primaryKey"`
	Username          string     `gorm:"size:64;not null"`
	Email             string     `gorm:"size:128;not null"`
	Phone             string     `gorm:"size:32"`
	PasswordHash      string     `gorm:"column:password_hash;size:255;not null"`
	Status            string     `gorm:"size:32;not null;default:active"`
	RealName          string     `gorm:"column:real_name;size:64"`
	Avatar            string     `gorm:"size:255"`                             // 用户头像 URL
	Tier              string     `gorm:"size:32;not null;default:free"`        // 用户等级
	LastLoginAt       *time.Time `gorm:"column:last_login_at"`                 // 最近登录时间
	LastLoginIP       string     `gorm:"column:last_login_ip;size:64"`         // 最近登录 IP
	LastLoginIPRegion string     `gorm:"column:last_login_ip_region;size:128"` // 最近登录 IP 归属地
	CreatedAt         time.Time  `gorm:"autoCreateTime"`                       // 创建时间
	UpdatedAt         time.Time  `gorm:"autoUpdateTime"`                       // 更新时间
}

func (User) TableName() string {
	return "users"
}
