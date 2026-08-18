package dto

type UserInfo struct {
	ID       uint64   `json:"id"`
	Username string   `json:"username"`
	Role     string   `json:"role"`
	Roles    []string `json:"roles"`
	Email    string   `json:"email"`
}

type LoginResponse struct {
	Token       string   `json:"token"`
	UserInfo    UserInfo `json:"user_info"`
	Permissions []string `json:"permissions"`
	Menus       []string `json:"menus"`
}
