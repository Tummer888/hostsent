package dto

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UserCreateRequest struct {
	Username string   `json:"username" binding:"required"`
	Email    string   `json:"email" binding:"required"`
	RoleIDs  []uint64 `json:"role_ids"`
}

type AssignRolesRequest struct {
	RoleIDs []uint64 `json:"role_ids" binding:"required"`
}

type AssignPermissionsRequest struct {
	PermissionIDs []uint64 `json:"permission_ids" binding:"required"`
}
