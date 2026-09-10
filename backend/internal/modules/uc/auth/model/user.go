package model

import "time"

// User 用户中心用户模型，映射 users 表。
// 该表同时被后台管理模块（admin/user/account）使用，此处仅保留用户中心
// 自助场景所需的字段；avatar / tier 为新增列，启动时由 AutoMigrate 补齐。
type User struct {
	ID           uint64 `gorm:"primaryKey"`
	Username     string `gorm:"size:64;not null"`
	Email        string `gorm:"size:128;not null"`
	Phone        string `gorm:"size:32"`
	PasswordHash string `gorm:"column:password_hash;size:255;not null"`
	Status       string `gorm:"size:32;not null;default:active"`
	RealName     string `gorm:"column:real_name;size:64"`
	Avatar       string `gorm:"size:255"`                      // 用户头像 URL
	Tier         string `gorm:"size:32;not null;default:free"` // 用户等级
	// UserLevelID 当前用户等级 ID（消费升级服务维护）。
	UserLevelID *uint64 `gorm:"column:user_level_id"`
	// UserGroupID 所属用户组 ID；注册时兜底归入默认组，未配置默认组则为 NULL。
	UserGroupID *uint64 `gorm:"column:user_group_id"`
	// TotalConsumeAmount 累计消费额，用于等级判定与展示。
	TotalConsumeAmount float64 `gorm:"column:total_consume_amount;type:decimal(15,2);not null;default:0"`
	// OwnerUserID 子账号归属的主账号 ID；主账号为 NULL（P4-01）。
	OwnerUserID *uint64 `gorm:"column:owner_user_id"`
	// IsSubAccount 是否子账号（成员）。
	IsSubAccount bool `gorm:"column:is_sub_account;not null;default:false"`
	// SubAccountRemark 子账号备注。
	SubAccountRemark string `gorm:"column:sub_account_remark;size:64"`
	// InviteCode 用户专属邀请码（推广邀请注册用）。
	InviteCode *string `gorm:"column:invite_code;size:32;uniqueIndex:uk_users_invite_code"`
	// InviterUserID 邀请人用户 ID；单级邀请，注册时一次性绑定。
	InviterUserID *uint64 `gorm:"column:inviter_user_id;index:idx_users_inviter_user_id"`
	// InvitedAt 绑定邀请关系的时间。
	InvitedAt         *time.Time `gorm:"column:invited_at"`
	LastLoginAt       *time.Time `gorm:"column:last_login_at"`                 // 最近登录时间
	LastLoginIP       string     `gorm:"column:last_login_ip;size:64"`         // 最近登录 IP
	LastLoginIPRegion string     `gorm:"column:last_login_ip_region;size:128"` // 最近登录 IP 归属地
	CreatedAt         time.Time  `gorm:"autoCreateTime"`                       // 创建时间
	UpdatedAt         time.Time  `gorm:"autoUpdateTime"`                       // 更新时间
}

func (User) TableName() string {
	return "users"
}
