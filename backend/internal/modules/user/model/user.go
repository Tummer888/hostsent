package model

import "time"

type User struct {
	ID                uint64     `gorm:"primaryKey"`
	Username          string     `gorm:"size:64;not null;uniqueIndex"`
	Email             string     `gorm:"size:128;not null;uniqueIndex"`
	Phone             string     `gorm:"size:32"`
	PasswordHash      string     `gorm:"column:password_hash;size:255;not null"`
	Status            string     `gorm:"size:32;not null;default:active"`
	RealName          string     `gorm:"column:real_name;size:64"`
	Region            string     `gorm:"column:region;size:32"`
	OAuthProvider     string     `gorm:"column:oauth_provider;size:32"`
	OAuthOpenID       string     `gorm:"column:oauth_openid;size:128"`
	Balance           float64    `gorm:"column:balance;type:decimal(15,2);not null;default:0"`
	UserGroupID       *uint64    `gorm:"column:user_group_id"`
	UserGroupName     string     `gorm:"-"`
	TotalConsumeAmount float64   `gorm:"-"`
	LastLoginAt       *time.Time `gorm:"column:last_login_at"`
	LastLoginIP       string     `gorm:"column:last_login_ip;size:64"`
	LastLoginIPRegion string     `gorm:"column:last_login_ip_region;size:128"`
	Role              string     `gorm:"-"`
	Roles             []string   `gorm:"-"`
	CreatedAt         time.Time  `gorm:"autoCreateTime"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime"`
}

// UserStats 用户统计聚合结果，字段与 users 表对齐，不映射单独表。
type UserStats struct {
	Total           int64   `json:"total"`
	TodayNew        int64   `json:"today_new"`
	Active          int64   `json:"active"`
	Disabled        int64   `json:"disabled"`
	PendingRealName int64   `json:"pending_real_name"`
	PendingReview   int64   `json:"pending_review"`
	TotalBalance    float64 `json:"total_balance"`
	PurchasedCount  int64   `json:"purchased_count"`
}

// RegionStat 登录 IP 归属地分布聚合项。
type RegionStat struct {
	Region string `json:"region"`
	Count  int64  `json:"count"`
}

func (User) TableName() string {
	return "users"
}
