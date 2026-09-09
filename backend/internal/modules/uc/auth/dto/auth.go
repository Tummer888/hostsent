package dto

// LoginRequest 用户中心登录请求参数。
// 与前端 frontend-user/src/api/auth.ts 的 LoginParams 对齐。
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest 用户中心注册请求参数。
// 与前端 frontend-user/src/api/auth.ts 的 RegisterParams 对齐。
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone"`
}

// UserInfo 用户中心用户信息，响应结构对齐前端
// frontend-user/src/api/auth.ts 中 LoginResponse.user 的字段定义。
type UserInfo struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"` // 显示名：优先取 real_name，为空时回退为 username
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Avatar   string `json:"avatar"`
	Role     string `json:"role"` // 用户中心角色，固定为 "user"
	Tier     string `json:"tier"` // 用户等级（标准用户 standard / 企业用户 business / 免费 free）
	Status   string `json:"status"`
}

// LoginResponse 用户中心登录响应，结构对齐前端 LoginResponse：
// { token, user: { id, username, name, email, phone, avatar, role } }
type LoginResponse struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}

// UpdateProfileRequest 用户资料更新请求参数。
// 仅允许更新展示类字段（显示名、邮箱、手机、头像），用户名与密码走独立接口。
type UpdateProfileRequest struct {
	Name   string `json:"name"` // 显示名/真实姓名，写入 real_name 字段
	Email  string `json:"email" binding:"omitempty,email"`
	Phone  string `json:"phone"`
	Avatar string `json:"avatar"`
}

// ChangePasswordRequest 修改密码请求参数。
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`       // 旧密码，用于身份校验
	NewPassword string `json:"new_password" binding:"required,min=6"` // 新密码，最短 6 位
}
