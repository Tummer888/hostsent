package dto

import "time"

type UserInfo struct {
	ID                 uint64   `json:"id"`
	Username           string   `json:"username"`
	RealName           string   `json:"real_name"`
	Role               string   `json:"role"`
	Roles              []string `json:"roles"`
	Email              string   `json:"email"`
	Phone              string   `json:"phone"`
	UserGroupID        *uint64  `json:"user_group_id"`
	UserGroupName      string   `json:"user_group_name"`
	UserLevelID        *uint64  `json:"user_level_id"`
	UserLevelName      string   `json:"user_level_name"`
	UserLevelCode      string   `json:"user_level_code"`
	Region             string   `json:"region"`
	Avatar             string   `json:"avatar"`
	Tier               string   `json:"tier"`
	LastLoginIP        string   `json:"last_login_ip"`
	LastLoginIPRegion  string   `json:"last_login_ip_region"`
	OAuthProvider      string   `json:"oauth_provider"`
	OAuthOpenID        string   `json:"oauth_openid"`
	Balance            float64  `json:"balance"`
	TotalConsumeAmount float64  `json:"total_consume_amount"`
	Status             string   `json:"status"`
	// 手机/邮箱验证时间（nil = 未验证）。迁移 042 已落列，此前从未接出，
	// 详情页据此渲染认证徽章，而不是靠"字段非空"猜测。
	PhoneVerifiedAt *time.Time `json:"phone_verified_at"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	// 邀请关系（推广）：邀请码、邀请人 ID 与用户名、绑定时间。
	InviteCode    *string    `json:"invite_code"`
	InviterUserID *uint64    `json:"inviter_user_id"`
	InviterName   string     `json:"inviter_name"`
	InvitedAt     *time.Time `json:"invited_at"`
	// 子账号标识（P4-10）：是否子账号、归属主账号 ID 与用户名、成员备注。
	IsSubAccount     bool    `json:"is_sub_account"`
	OwnerUserID      *uint64 `json:"owner_user_id"`
	OwnerName        string  `json:"owner_name"`
	SubAccountRemark string  `json:"sub_account_remark"`
	// 归属销售（doc86 §4.1.10）：销售归属变更时由销售模块回写 users 快照列。
	SalesAdminID   uint64     `json:"sales_admin_id"`
	SalesAdminName string     `json:"sales_admin_name"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
	// —— 软删除（注销）与实名认证（doc104 §4/§5）——
	// DeletedAt 非空即已注销；回收站列表与详情页据此渲染「已注销」态。
	DeletedAt *time.Time `json:"deleted_at"`
	// DeletedBy / DeletedByName 执行注销的管理员；0 表示系统或用户自助。
	DeletedBy     uint64 `json:"deleted_by"`
	DeletedByName string `json:"deleted_by_name"`
	DeleteReason  string `json:"delete_reason"`
	// StatusBeforeDelete 注销前的状态，恢复时精确还原（空则回落 disabled）。
	StatusBeforeDelete string `json:"status_before_delete"`
	// RealNameVerifiedAt 实名认证的唯一信任信号（nil = 未实名）。
	// users.real_name 只是展示名，前端不得再用「real_name 非空」判断是否已实名。
	RealNameVerifiedAt     *time.Time `json:"real_name_verified_at"`
	RealNameVerifiedSource string     `json:"real_name_verified_source"`
	// OAuthProviders 已绑定的第三方渠道列表（微信/QQ/支付宝），由 user_oauth_bindings 聚合。
	OAuthProviders []string `json:"oauth_providers"`
}

// UserDeletionBlockerItem 一条注销阻断/警告项。
type UserDeletionBlockerItem struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Count int64  `json:"count"`
}

// UserDeletionCheckResponse 注销前置校验结果（doc104 §4.4）。
//
// Blockers 非空即硬阻断（force 也绕不过）；Warnings 非空需 force=true 才放行。
// CanDelete 由服务层算好直接给前端，避免前端自己复述一遍判定逻辑。
type UserDeletionCheckResponse struct {
	UserID    uint64                    `json:"user_id"`
	Username  string                    `json:"username"`
	CanDelete bool                      `json:"can_delete"`
	Blockers  []UserDeletionBlockerItem `json:"blockers"`
	Warnings  []UserDeletionBlockerItem `json:"warnings"`
}

// UserPurgePreviewItem 待清理用户的一条预览（dry_run 与手工触发共用）。
type UserPurgePreviewItem struct {
	ID        uint64     `json:"id"`
	Username  string     `json:"username"`
	DeletedAt *time.Time `json:"deleted_at"`
	Reason    string     `json:"reason"`
}

// UserPurgeResponse 留存期清理结果（doc104 §4.6）。
type UserPurgeResponse struct {
	// RetentionDays 本轮使用的留存天数（来自 user.deletion_retention_days）。
	RetentionDays int `json:"retention_days"`
	// Cutoff 早于该时刻注销的用户才进入清理范围。
	Cutoff time.Time `json:"cutoff"`
	// DryRun 为 true 时不做任何写操作，只返回 Candidates。
	DryRun bool `json:"dry_run"`
	// Candidates 预览列表（dry_run 时最多 Limit 条）。
	Candidates []UserPurgePreviewItem `json:"candidates"`
	// Purged 实际硬删除的用户数；dry_run 恒为 0。
	Purged int `json:"purged"`
	// Skipped 因仍有在管实例等原因跳过的用户。
	Skipped []UserBatchSkipItem `json:"skipped"`
	// HasMore 本轮取满上限，仍有积压待下一轮。
	HasMore bool `json:"has_more"`
}

// SubAccountMemberInfo 管理端成员 Tab 展示项（P4-10）。
type SubAccountMemberInfo struct {
	ID          uint64     `json:"id"`
	Username    string     `json:"username"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	Remark      string     `json:"remark"`
	Status      string     `json:"status"`
	Permissions []string   `json:"permissions"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// SubAccountMemberListResponse 某主账号下的成员列表响应（P4-10）。
type SubAccountMemberListResponse struct {
	Items []SubAccountMemberInfo `json:"items"`
	Total int64                  `json:"total"`
}

type UserListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type UserListResponse struct {
	Items []UserInfo   `json:"items"`
	Meta  UserListMeta `json:"meta"`
}

type UserStatsResponse struct {
	Total           int64   `json:"total"`
	TodayNew        int64   `json:"today_new"`
	Active          int64   `json:"active"`
	Disabled        int64   `json:"disabled"`
	PendingRealName int64   `json:"pending_real_name"`
	PendingReview   int64   `json:"pending_review"`
	TotalBalance    float64 `json:"total_balance"`
	PurchasedCount  int64   `json:"purchased_count"`
	// Deleted 已注销用户数（回收站入口徽标，doc104 §4）。
	Deleted int64 `json:"deleted"`
}

type RegionStatItem struct {
	Region string `json:"region"`
	Count  int64  `json:"count"`
}

type RegionStatsResponse struct {
	Items []RegionStatItem `json:"items"`
	Total int64            `json:"total"`
}

type RoleInfo struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Scope       string `json:"scope"`
	// Builtin 内置角色（如 super_admin）：不可删除、不可改 code，权限树只读。
	Builtin   bool      `json:"builtin"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PermissionNode struct {
	ID        uint64           `json:"id"`
	ParentID  uint64           `json:"parent_id"`
	Name      string           `json:"name"`
	Code      string           `json:"code"`
	Type      string           `json:"type"`
	Path      string           `json:"path,omitempty"`
	Component string           `json:"component,omitempty"`
	Icon      string           `json:"icon,omitempty"`
	SortOrder int              `json:"sort_order"`
	Status    string           `json:"status"`
	Children  []PermissionNode `json:"children,omitempty"`
}

type APIResponse[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      T      `json:"data"`
	Timestamp int64  `json:"timestamp"`
}

// ImpersonateResponse 代登录响应：返回用户端 token 与用户信息
type ImpersonateResponse struct {
	Token    string   `json:"token"`
	UserInfo UserInfo `json:"user_info"`
}

// AdminOrderBrief 为指定用户创建订单的返回摘要。
type AdminOrderBrief struct {
	ID           uint64  `json:"id"`
	OrderNo      string  `json:"order_no"`
	ProductName  string  `json:"product_name"`
	BillingCycle string  `json:"billing_cycle"`
	TotalAmount  float64 `json:"total_amount"`
	Status       string  `json:"status"`
	PayMethod    string  `json:"pay_method"`
}
