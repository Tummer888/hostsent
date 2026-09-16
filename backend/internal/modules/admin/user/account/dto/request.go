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
	// UserGroupID 按用户组筛选（0 表示不筛选）。
	UserGroupID uint64 `form:"user_group_id"`
	// IsSubAccount 按主账号/子账号筛选（P4-10）："true" 仅子账号，"false" 仅主账号，空为全部。
	IsSubAccount string `form:"is_sub_account"`
	// SalesAdminID 按归属销售筛选（doc86 §4.1.10，0 表示不筛选）。
	SalesAdminID uint64 `form:"sales_admin_id"`
	// UnassignedSales 为 "true" 时仅返回未归属销售的用户；与 SalesAdminID 互斥，未归属优先。
	UnassignedSales string `form:"unassigned_sales"`
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

// UserUpdateRequest 用户资料部分更新。
//
// 全部字段为指针：nil = 不修改，显式传空串 = 清空该字段。
// 原实现把 Username/Email/Phone/Status 全部标 required，而库中 40 个用户有 14 个
// 无手机号 —— 这些账号在详情页点保存必然 400。手机/邮箱改为「填了就校验、空则存空」。
type UserUpdateRequest struct {
	Username         *string `json:"username" binding:"omitempty,min=3,max=64"`
	RealName         *string `json:"real_name" binding:"omitempty,max=64"`
	Email            *string `json:"email" binding:"omitempty,email,max=128"`
	Phone            *string `json:"phone" binding:"omitempty,max=32"`
	Region           *string `json:"region" binding:"omitempty,max=32"`
	SubAccountRemark *string `json:"sub_account_remark" binding:"omitempty,max=64"`
	Status           *string `json:"status" binding:"omitempty,oneof=active disabled pending cancelled"`
	// UserGroupID 调整用户组：nil 表示不修改，0 表示移出分组（未分组），其余为组 ID。
	UserGroupID *uint64 `json:"user_group_id"`
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

// RechargeRequest 用户钱包人工调账请求。
// Amount 为正表示入账（充值/补偿），为负表示扣减（追回）；0 会被服务拒绝。
type RechargeRequest struct {
	Amount float64 `json:"amount" binding:"required"`
	Remark string  `json:"remark" binding:"max=255"`
}

// AdminCreateOrderRequest 为指定用户创建订单。
type AdminCreateOrderRequest struct {
	ProductID    uint64  `json:"product_id" binding:"required"`
	BillingCycle string  `json:"billing_cycle"` // monthly/quarterly/annually...
	Price        float64 `json:"price"`         // 覆盖价格（<=0 用商品默认价）
	PayMode      string  `json:"pay_mode"`      // balance=余额支付并开通; create=仅创建不支付(默认)
}
