package model

import "time"

// 用户状态枚举（doc104 §4.3）。
//
// active 正常 / disabled 冻结 / pending 待审核 / cancelled 已注销。
// cancelled 此前只是声明从未使用；本次软删除落地后，它是「已注销」的权威状态，
// 不新增枚举值——注销 = deleted_at 非空 + status=cancelled（状态收敛）。
const (
	StatusActive    = "active"
	StatusDisabled  = "disabled"
	StatusPending   = "pending"
	StatusCancelled = "cancelled"
)

// IsValidStatus 判断状态是否在枚举内。DTO 层已用 oneof 校验，服务层再兜一层，
// 防止将来有别的调用方绕过 binding 直接调服务。
func IsValidStatus(s string) bool {
	switch s {
	case StatusActive, StatusDisabled, StatusPending, StatusCancelled:
		return true
	default:
		return false
	}
}

type User struct {
	ID uint64 `gorm:"primaryKey"`
	// Username / Email 的唯一性只在「未注销行」之间成立（doc104 §4.2）：
	// 部分唯一索引 uk_users_username_active / uk_users_email_active 带
	// WHERE deleted_at IS NULL，注销后账号名与邮箱可被重新注册。
	// 迁移 051 已把历史上的普通唯一索引替换为这两条部分唯一索引。
	Username      string `gorm:"size:64;not null;uniqueIndex:uk_users_username_active,WHERE:deleted_at IS NULL"`
	Email         string `gorm:"size:128;not null;uniqueIndex:uk_users_email_active,WHERE:deleted_at IS NULL"`
	Phone         string `gorm:"size:32"`
	PasswordHash  string `gorm:"column:password_hash;size:255;not null"`
	Status        string `gorm:"size:32;not null;default:active"`
	RealName      string `gorm:"column:real_name;size:64"`
	Region        string `gorm:"column:region;size:32"`
	OAuthProvider string `gorm:"column:oauth_provider;size:32"`
	OAuthOpenID   string `gorm:"column:oauth_openid;size:128"`
	// OAuthProviders 该用户已绑定的全部第三方 provider（由 user_oauth_bindings 聚合，
	// 非持久化）。管理端列表的第三方图标列消费本字段；此前前端读 oauth_providers
	// 而后端从不生产，图标恒为未绑定态（doc104 §6.1）。
	OAuthProviders []string `gorm:"-"`
	// 软删除（用户注销）。用 *time.Time 而非 gorm.DeletedAt：全仓库有大量
	// Table("users") 裸查询（等级仓储、工单前置条件等），gorm.DeletedAt 的隐式过滤
	// 会在这些路径上静默生效或静默失效，语义不可控；orders/products 用的也是本形态。
	// DeletedAt 非空即「已注销」，status 同时收敛为 cancelled。
	DeletedAt *time.Time `gorm:"column:deleted_at;index:idx_users_deleted_at"`
	// DeletedBy 执行注销的管理员 ID；0 表示系统或用户自助。
	DeletedBy uint64 `gorm:"column:deleted_by;not null;default:0"`
	// DeletedByName 注销人显示名；查询联表带出，非持久化（只读权限同上）。
	DeletedByName string `gorm:"->;-:migration"`
	// DeleteReason 注销原因（审计与恢复时展示）。
	DeleteReason string `gorm:"column:delete_reason;size:255;not null;default:''"`
	// StatusBeforeDelete 注销前的 status，恢复时精确还原（为空则回落 disabled）。
	StatusBeforeDelete string `gorm:"column:status_before_delete;size:32;not null;default:''"`
	// RealNameVerifiedAt 实名认证的**唯一信任信号**（doc104 §5.3）：
	// 只有审核通过（或三方核验通过）才会写入。users.real_name 从此只是展示名，
	// 不得再作为实名判据——原口径「real_name 非空即已实名」会让用户改一次昵称
	// 就被判定为已实名（资料表单的 name 会直写 real_name）。
	RealNameVerifiedAt *time.Time `gorm:"column:real_name_verified_at"`
	// RealNameVerifiedSource 实名来源：manual / alipay。
	RealNameVerifiedSource string `gorm:"column:real_name_verified_source;size:32;not null;default:''"`
	// Avatar 用户头像 URL；与用户中心模型（uc/auth/model.User）共用 users.avatar 列。
	Avatar string `gorm:"size:255"`
	// Tier 用户分层（free/pro…）；与用户中心模型共用 users.tier 列。
	Tier string `gorm:"size:32;not null;default:free"`
	// PhoneVerifiedAt / EmailVerifiedAt 手机与邮箱的验证时间，为空表示未验证（迁移 042 增列）。
	// 详情页据此渲染「已验证 / 未验证」徽章 —— 这两个列此前从未接出，属于"有数据但看不见"。
	PhoneVerifiedAt *time.Time `gorm:"column:phone_verified_at"`
	EmailVerifiedAt *time.Time `gorm:"column:email_verified_at"`
	Balance         float64    `gorm:"column:balance;type:decimal(15,2);not null;default:0"`
	UserGroupID     *uint64    `gorm:"column:user_group_id"`
	// UserGroupName 用户组名称；由列表/详情查询的 LEFT JOIN 别名带出，非持久化。
	// 只读权限（->）而非 gorm:"-"：后者会让 GORM 完全忽略该字段，联表别名无处可落，
	// 接口返回的 user_group_name 会恒为空。只读字段不参与写入，也不会被迁移建列。
	UserGroupName string `gorm:"->;-:migration"`
	// UserLevelID 当前用户等级；由消费升级服务按累计消费自动调整（只升不降）。
	UserLevelID *uint64 `gorm:"column:user_level_id"`
	// UserLevelName / UserLevelCode 列表查询联表带出，非持久化字段（同上，需只读权限）。
	UserLevelName string `gorm:"->;-:migration"`
	UserLevelCode string `gorm:"->;-:migration"`
	// TotalConsumeAmount 累计消费额（P3-01 起落列持久化，不再靠实时聚合子查询）。
	TotalConsumeAmount float64 `gorm:"column:total_consume_amount;type:decimal(15,2);not null;default:0"`
	// OwnerUserID 子账号归属的主账号 ID；主账号为 NULL（P4-01）。
	OwnerUserID *uint64 `gorm:"column:owner_user_id"`
	// IsSubAccount 是否子账号（成员）。主账号拥有全部客户侧权限。
	IsSubAccount bool `gorm:"column:is_sub_account;not null;default:false"`
	// SubAccountRemark 子账号备注，便于主账号区分成员。
	SubAccountRemark string `gorm:"column:sub_account_remark;size:64"`
	// InviteCode 用户专属邀请码（推广邀请注册用）；历史用户由迁移回填。
	InviteCode *string `gorm:"column:invite_code;size:32;uniqueIndex:uk_users_invite_code"`
	// InviterUserID 邀请人用户 ID；单级邀请，注册时一次性绑定。
	InviterUserID *uint64 `gorm:"column:inviter_user_id;index:idx_users_inviter_user_id"`
	// InvitedAt 绑定邀请关系的时间。
	InvitedAt *time.Time `gorm:"column:invited_at"`
	// InviterName 邀请人用户名；详情查询联表带出，非持久化（只读权限同上）。
	InviterName string `gorm:"->;-:migration"`
	// SalesAdminID 当前归属销售（admins.id），0 表示未归属；权威数据是 staff_sales_relations，
	// 本列只是快照（doc86 §1.3），归属变更时由销售模块回写。
	SalesAdminID uint64 `gorm:"column:sales_admin_id;not null;default:0;index:idx_users_sales_admin"`
	// SalesAdminName 归属销售名；列表/详情联表带出，非持久化（只读权限同上）。
	SalesAdminName string `gorm:"->;-:migration"`
	// OwnerName 子账号归属主账号用户名；列表/详情联表带出，非持久化（P4-10，只读权限同上）。
	OwnerName         string     `gorm:"->;-:migration"`
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
	// Deleted 已注销（软删除）用户数，供总览与回收站入口展示（doc104 §4）。
	Deleted int64 `json:"deleted"`
}

// UserDeletionCheck 注销前置校验结果（doc104 §4.4）。
// 任一 Blockers 非空即不可注销；Warnings 非空时需 force=true 才放行。
type UserDeletionCheck struct {
	UserID   uint64                `json:"user_id"`
	Username string                `json:"username"`
	Blockers []UserDeletionBlocker `json:"blockers"`
	Warnings []UserDeletionBlocker `json:"warnings"`
}

// UserDeletionBlocker 一条阻断/警告项。
type UserDeletionBlocker struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Count int64  `json:"count"`
}

// RegionStat 登录 IP 归属地分布聚合项。
type RegionStat struct {
	Region string `json:"region"`
	Count  int64  `json:"count"`
}

func (User) TableName() string {
	return "users"
}
