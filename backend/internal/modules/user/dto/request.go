package dto

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AdminImpersonateRequest struct {
	UserID uint64 `json:"user_id" binding:"required"`
}

type UserListQuery struct {
	Page              int    `form:"page"`
	PageSize          int    `form:"page_size"`
	Status            string `form:"status"`
	Filter            string `form:"filter"`
	LastLoginIPRegion string `form:"last_login_ip_region"`
	Keyword           string `form:"keyword"`
}

type UserCreateRequest struct {
	ID       uint64   `json:"id"`
	Username string   `json:"username" binding:"required"`
	Email    string   `json:"email" binding:"required"`
	Phone    string   `json:"phone" binding:"required,len=11,numeric"`
	Password string   `json:"password" binding:"required"`
	Status   string   `json:"status"`
	RoleIDs  []uint64 `json:"role_ids"`
}

type UserUpdateRequest struct {
	Username string   `json:"username" binding:"required"`
	Email    string   `json:"email" binding:"required"`
	Phone    string   `json:"phone" binding:"required"`
	Status   string   `json:"status" binding:"required"`
}

type UserStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

type AssignRolesRequest struct {
	RoleIDs []uint64 `json:"role_ids" binding:"required"`
}

type RoleCreateRequest struct {
	Name   string `json:"name" binding:"required"`
	Code   string `json:"code" binding:"required"`
	Status string `json:"status"`
}

type RoleUpdateRequest struct {
	Name   string `json:"name" binding:"required"`
	Code   string `json:"code" binding:"required"`
	Status string `json:"status" binding:"required"`
}

type AssignPermissionsRequest struct {
	PermissionIDs []uint64 `json:"permission_ids" binding:"required"`
}

type PermissionCreateRequest struct {
	ParentID  uint64 `json:"parent_id"`
	Name      string `json:"name" binding:"required"`
	Code      string `json:"code" binding:"required"`
	Type      string `json:"type" binding:"required"`
	Path      string `json:"path"`
	Component string `json:"component"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
	Status    string `json:"status"`
}

type PermissionUpdateRequest struct {
	ParentID  uint64 `json:"parent_id"`
	Name      string `json:"name" binding:"required"`
	Code      string `json:"code" binding:"required"`
	Type      string `json:"type" binding:"required"`
	Path      string `json:"path"`
	Component string `json:"component"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
	Status    string `json:"status" binding:"required"`
}
