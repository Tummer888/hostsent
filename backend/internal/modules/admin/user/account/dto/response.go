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
	LastLoginIP        string   `json:"last_login_ip"`
	LastLoginIPRegion  string   `json:"last_login_ip_region"`
	OAuthProvider      string   `json:"oauth_provider"`
	Balance            float64  `json:"balance"`
	TotalConsumeAmount float64  `json:"total_consume_amount"`
	Status             string   `json:"status"`
	// 子账号标识（P4-10）：是否子账号、归属主账号 ID 与用户名、成员备注。
	IsSubAccount     bool    `json:"is_sub_account"`
	OwnerUserID      *uint64 `json:"owner_user_id"`
	OwnerName        string  `json:"owner_name"`
	SubAccountRemark string  `json:"sub_account_remark"`
	// 归属销售（doc86 §4.1.10）：销售归属变更时由销售模块回写 users 快照列。
	SalesAdminID   uint64     `json:"sales_admin_id"`
	SalesAdminName string     `json:"sales_admin_name"`
	CreatedAt      time.Time  `json:"created_at"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
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
