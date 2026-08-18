package dto

type UserInfo struct {
	ID       uint64   `json:"id"`
	Username string   `json:"username"`
	Role     string   `json:"role"`
	Roles    []string `json:"roles"`
	Email    string   `json:"email"`
	Phone    string   `json:"phone"`
	Status   string   `json:"status"`
}

type RoleInfo struct {
	ID     uint64 `json:"id"`
	Name   string `json:"name"`
	Code   string `json:"code"`
	Status string `json:"status"`
}

type PermissionNode struct {
	ID       uint64           `json:"id"`
	ParentID uint64           `json:"parent_id"`
	Name     string           `json:"name"`
	Code     string           `json:"code"`
	Type     string           `json:"type"`
	Path     string           `json:"path,omitempty"`
	Icon     string           `json:"icon,omitempty"`
	Children []PermissionNode `json:"children,omitempty"`
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
