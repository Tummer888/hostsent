package dto

type UserListQuery struct {
	Page              int    `form:"page"`
	PageSize          int    `form:"page_size"`
	Status            string `form:"status"`
	Filter            string `form:"filter"`
	LastLoginIPRegion string `form:"last_login_ip_region"`
	Keyword           string `form:"keyword"`
	// UserLevelID 按用户等级筛选（P3-04，0 表示不筛选）。
	UserLevelID uint64 `form:"user_level_id"`
	// IsSubAccount 按主账号/子账号筛选（P4-10）："true" 仅子账号，"false" 仅主账号，空为全部。
	IsSubAccount string `form:"is_sub_account"`
}

type UserCreateRequest struct {
	ID          uint64   `json:"id"`
	Username    string   `json:"username" binding:"required"`
	Email       string   `json:"email" binding:"required"`
	Phone       string   `json:"phone" binding:"required,len=11,numeric"`
	Password    string   `json:"password" binding:"required"`
	Status      string   `json:"status"`
	RoleIDs     []uint64 `json:"role_ids"`
	UserGroupID *uint64  `json:"user_group_id"`
}

type UserUpdateRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Phone    string `json:"phone" binding:"required"`
	Status   string `json:"status" binding:"required"`
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

// RechargeRequest 用户充值（人工调账）请求
type RechargeRequest struct {
	Amount float64 `json:"amount" binding:"required"`
	Remark string  `json:"remark"`
}

// AdminCreateOrderRequest 为指定用户创建订单。
type AdminCreateOrderRequest struct {
	ProductID    uint64  `json:"product_id" binding:"required"`
	BillingCycle string  `json:"billing_cycle"` // monthly/quarterly/annually...
	Price        float64 `json:"price"`         // 覆盖价格（<=0 用商品默认价）
	PayMode      string  `json:"pay_mode"`      // balance=余额支付并开通; create=仅创建不支付(默认)
}
