package dto

type UserListQuery struct {
	Page              int    `form:"page"`
	PageSize          int    `form:"page_size"`
	Status            string `form:"status"`
	Filter            string `form:"filter"`
	LastLoginIPRegion string `form:"last_login_ip_region"`
	Keyword           string `form:"keyword"`
	// IncludeDeleted 为 true 时连已注销用户一起返回（doc104 §4.3）；
	// 与 filter=deleted 互斥，前者优先级更高。
	IncludeDeleted bool `form:"include_deleted"`
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
	ID       uint64 `json:"id"`
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Phone    string `json:"phone" binding:"required,len=11,numeric"`
	// Password 建号密码。补最小长度校验：原实现只有 required，
	// 运营可以给用户建一个 1 位密码的账号（doc104 §3.1）。
	Password    string   `json:"password" binding:"required,min=8,max=64"`
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

// UserStatusRequest 用户状态变更。
//
// 补 oneof 枚举校验：原实现只有 required，实测 PATCH /users/:id/status 传 "banana"
// 会成功落库（doc104 §3.1 F3），与 UserUpdateRequest.Status 的校验强度不一致。
type UserStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active disabled pending cancelled"`
}

// ResetPasswordRequest 重置用户密码。
// Password 补最小长度：原实现只有 required（doc104 §3.1）。
type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=8,max=64"`
}

// AssignRolesRequest 覆盖用户角色。
//
// 用 min=1 而不是 required：validator 对 slice 的 required 只在 nil 上触发，
// 空数组 `{"role_ids":[]}` 会通过校验并静默清空用户全部角色（实测，doc104 §3.1 F4）。
// 服务层再兜一层非空判断，防止将来有调用方绕过 binding。
type AssignRolesRequest struct {
	RoleIDs []uint64 `json:"role_ids" binding:"required,min=1"`
}

// UserDeleteRequest 注销（软删除）用户。
type UserDeleteRequest struct {
	// Reason 注销原因，落库并进审计；必填以便事后追溯「为什么注销」。
	Reason string `json:"reason" binding:"required,max=255"`
	// Force 为 true 时允许绕过「余额非零 / 未结账单 / 未完成工单 / 未完成订单」四项警告。
	// 在管实例属硬阻断，force 也绕不过（有外键，且必须先释放）。
	Force bool `json:"force"`
}

// UserBatchDeleteRequest 批量注销。
type UserBatchDeleteRequest struct {
	IDs    []uint64 `json:"ids" binding:"required,min=1,max=100"`
	Reason string   `json:"reason" binding:"required,max=255"`
	Force  bool     `json:"force"`
}

// UserBatchRestoreRequest 批量恢复。
type UserBatchRestoreRequest struct {
	IDs []uint64 `json:"ids" binding:"required,min=1,max=100"`
}

// UserBatchResult 批量操作结果。
//
// 沿用既有批量接口的「成功数 + 跳过明细」形态（对齐 notification 的
// DeliveryRetryResponse），但带原因，便于运营在界面上逐条说明为什么没做成。
type UserBatchResult struct {
	Affected int                 `json:"affected"`
	Skipped  []UserBatchSkipItem `json:"skipped"`
}

// UserBatchSkipItem 一条被跳过的记录及原因。
type UserBatchSkipItem struct {
	ID     uint64 `json:"id"`
	Reason string `json:"reason"`
}

// UserPurgeRequest 手工触发留存期清理。
type UserPurgeRequest struct {
	// DryRun 为 true 时只统计将被删除的用户，不做任何写操作。
	DryRun bool `json:"dry_run"`
	// Limit 单轮上限，0 用服务端默认值（50）。
	Limit int `json:"limit"`
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
