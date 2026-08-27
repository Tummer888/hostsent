package dto

import "time"

type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AdminInfo struct {
	ID          uint64     `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	Avatar      string     `json:"avatar"`
	Role        string     `json:"role"`
	Department  string     `json:"department"`
	Status      string     `json:"status"`
	LastLoginIP string     `json:"last_login_ip"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type AdminLoginResponse struct {
	Token       string    `json:"token"`
	UserInfo    AdminInfo `json:"user_info"`
	Permissions []string  `json:"permissions"`
	Menus       []string  `json:"menus"`
}

type AdminCreateRequest struct {
	Username   string `json:"username" binding:"required"`
	Email      string `json:"email" binding:"required"`
	Password   string `json:"password" binding:"required"`
	Role       string `json:"role"`
	Department string `json:"department"`
	Status     string `json:"status"`
}

type AdminUpdateRequest struct {
	Email      string `json:"email" binding:"required"`
	Role       string `json:"role" binding:"required"`
	Department string `json:"department"`
	Status     string `json:"status" binding:"required"`
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
