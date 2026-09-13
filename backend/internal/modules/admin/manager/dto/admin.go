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
	// —— S1 员工体系扩展 ——
	RealName string `json:"real_name"`
	Phone    string `json:"phone"`
	// DepartmentID 归属部门（departments.id），0 表示未分配。
	DepartmentID uint64 `json:"department_id"`
	// DepartmentName 只读：部门名称快照，列表/详情直接展示，避免前端再拉一次部门表。
	DepartmentName string `json:"department_name"`
	// StaffType 员工类型（admin/sales/support/tech/ops/finance）。
	StaffType string `json:"staff_type"`
	// SalesEnabled 是否开启销售能力（可被分配客户）。
	SalesEnabled bool       `json:"sales_enabled"`
	JoinedAt     *time.Time `json:"joined_at,omitempty"`
	// ResignedAt 非空表示已离职（不再派单/归属），IsResigned 为其布尔投影。
	ResignedAt *time.Time `json:"resigned_at,omitempty"`
	IsResigned bool       `json:"is_resigned"`
	// ServiceGroupID 工单客服组（P2 自动派单用，迁移期兜底）
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
	// —— S1 员工体系扩展 ——
	RealName     string `json:"real_name"`
	Phone        string `json:"phone"`
	DepartmentID uint64 `json:"department_id"`
	StaffType    string `json:"staff_type"`
	SalesEnabled bool   `json:"sales_enabled"`
}

type AdminUpdateRequest struct {
	Email      string `json:"email" binding:"required"`
	Role       string `json:"role"`
	Department string `json:"department"`
	Position   string `json:"position"`
	Status     string `json:"status" binding:"required"`
	// —— S1 员工体系扩展 ——
	RealName     string `json:"real_name"`
	Phone        string `json:"phone"`
	DepartmentID uint64 `json:"department_id"`
	StaffType    string `json:"staff_type"`
	SalesEnabled bool   `json:"sales_enabled"`
}

// AdminResignRequest 员工离职（doc86 §2.1）。
// 离职后不再派单/分配客户，在途客户由 salesReleaser 转交（S4 装配后生效）。
type AdminResignRequest struct {
	// Reason 离职原因，写入审计日志备注。
	Reason string `json:"reason"`
	// TransferToAdminID 在途客户承接人；0 表示由服务端按部门主管兜底。
	TransferToAdminID uint64 `json:"transfer_to_admin_id"`
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
	// —— S1 员工体系扩展：员工列表按组织维度筛选 ——
	DepartmentID uint64 `form:"department_id"`
	StaffType    string `form:"staff_type"`
	// SalesEnabled 传 1/0 精确筛选销售能力开关，空串不过滤。
	SalesEnabled string `form:"sales_enabled"`
	// IsResigned 传 1 只看离职、0 只看在职，空串不过滤。
	IsResigned string `form:"is_resigned"`
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
