package dto

import "time"

type UserInfo struct {
	ID          uint64     `json:"id"`
	Username    string     `json:"username"`
	RealName    string     `json:"real_name"`
	Role        string     `json:"role"`
	Roles       []string   `json:"roles"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	Region      string     `json:"region"`
	Balance     float64    `json:"balance"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type UserListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type UserListResponse struct {
	Items []UserInfo    `json:"items"`
	Meta  UserListMeta  `json:"meta"`
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
	ID          uint64    `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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

type LoginResponse struct {
	Token       string   `json:"token"`
	UserInfo    UserInfo `json:"user_info"`
	Permissions []string `json:"permissions"`
	Menus       []string `json:"menus"`
}

type APIResponse[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      T      `json:"data"`
	Timestamp int64  `json:"timestamp"`
}
