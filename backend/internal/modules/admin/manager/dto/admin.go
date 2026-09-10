package dto

import "time"

type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AdminInfo struct {
	ID         uint64   `json:"id"`
	Username   string   `json:"username"`
	Email      string   `json:"email"`
	Avatar     string   `json:"avatar"`
	Role       string   `json:"role"` // 兼容/展示：主角色 code
	Roles      []string `json:"roles"`
	Department string   `json:"department"`
	Position   string   `json:"position"`
	Status     string   `json:"status"`
	// ServiceGroupID 工单客服组（P2 自动派单用）
	ServiceGroupID *uint64 `json:"service_group_id,omitempty"`
	// MustChangePassword 首次登录/重置后需强制改密
	MustChangePassword bool       `json:"must_change_password"`
	LastLoginIP        string     `json:"last_login_ip"`
	LastLoginAt        *time.Time `json:"last_login_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	// Permissions 仅登录/me 返回，用于前端刷新后恢复按钮级权限。
	Permissions []string `json:"permissions,omitempty"`
}

type AdminLoginResponse struct {
	Token       string    `json:"token"`
	UserInfo    AdminInfo `json:"user_info"`
	Permissions []string  `json:"permissions"`
	Roles       []string  `json:"roles"`
	Menus       []string  `json:"menus"`
	// MustChangePassword 为 true 时前端强制跳转改密页
	MustChangePassword bool `json:"must_change_password"`
}

type AdminCreateRequest struct {
	Username   string   `json:"username" binding:"required"`
	Email      string   `json:"email" binding:"required"`
	Password   string   `json:"password" binding:"required"`
	Role       string   `json:"role"`     // 兼容：单角色 code
	RoleIDs    []uint64 `json:"role_ids"` // 多角色（D2）
	Department string   `json:"department"`
	Position   string   `json:"position"`
	Status     string   `json:"status"`
}

type AdminUpdateRequest struct {
	Email      string `json:"email" binding:"required"`
	Role       string `json:"role"`
	Department string `json:"department"`
	Position   string `json:"position"`
	Status     string `json:"status" binding:"required"`
}

// AdminAssignRolesRequest 覆盖式设置员工角色。
type AdminAssignRolesRequest struct {
	RoleIDs []uint64 `json:"role_ids"`
}

// AdminChangePasswordRequest 管理员自助修改密码。
type AdminChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type AdminStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type AdminResetPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

type AdminListQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Role     string `form:"role"`
	Status   string `form:"status"`
	Keyword  string `form:"keyword"`
}

type AdminListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type AdminListResponse struct {
	Items []AdminInfo   `json:"items"`
	Meta  AdminListMeta `json:"meta"`
}

// AdminAuditLogQuery 管理端操作审计查询（P2-06）。
type AdminAuditLogQuery struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"page_size"`
	AdminID      uint64 `form:"admin_id"`
	ResourceType string `form:"resource_type"`
	Action       string `form:"action"`
	Keyword      string `form:"keyword"` // 操作人 / 路径
	StartTime    string `form:"start_time"`
	EndTime      string `form:"end_time"`
}

// AdminAuditLogInfo 管理端操作审计日志项。
type AdminAuditLogInfo struct {
	ID            uint64 `json:"id"`
	AdminID       uint64 `json:"admin_id"`
	AdminName     string `json:"admin_name"`
	Module        string `json:"module"`
	ResourceType  string `json:"resource_type"`
	ResourceID    string `json:"resource_id"`
	Action        string `json:"action"`
	RequestMethod string `json:"request_method"`
	RequestPath   string `json:"request_path"`
	ResponseCode  int    `json:"response_code"`
	IP            string `json:"ip"`
	UserAgent     string `json:"user_agent"`
	Detail        string `json:"detail"`
	CreatedAt     string `json:"created_at"`
}

// AdminAuditLogResponse 管理端操作审计分页响应。
type AdminAuditLogResponse struct {
	Items []AdminAuditLogInfo `json:"items"`
	Meta  AdminListMeta       `json:"meta"`
}
