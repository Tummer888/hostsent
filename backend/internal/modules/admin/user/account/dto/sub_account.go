package dto

// 管理端子账号（成员）管理请求/响应（P4-10 管理面补全）。
//
// 为什么管理端需要这套能力：子账号管理原本只有客户侧入口（主账号自助），
// 管理端成员面板是纯只读的。于是客服遇到「帮我把这个成员权限调一下」这类
// 请求，只能让客户自己去操作，或者直接改库 —— 后者没有审计，比开接口更危险。
//
// 语义与客户侧完全一致（同一套权限枚举、同一套配额与安全规则），
// 差别只在归属判定：客户侧以登录主账号为归属，管理端以路径上的主账号 ID 为归属。

// AdminSubAccountCreateRequest 管理端新建子账号。
// Password 留空时由服务生成一次性密码，仅在响应中返回一次。
type AdminSubAccountCreateRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone"`
	Password string `json:"password" binding:"omitempty,min=6,max=64"`
	Remark   string `json:"remark"`
	// Permissions 客户侧权限码（固定枚举）；为空时按默认权限集授予。
	Permissions []string `json:"permissions"`
}

// AdminSubAccountUpdateRequest 管理端修改子账号备注/状态。
type AdminSubAccountUpdateRequest struct {
	Remark string `json:"remark"`
	// Status 留空表示不改；仅接受 active / disabled。
	Status string `json:"status"`
}

// AdminSubAccountPermissionsRequest 覆盖式设置子账号权限。
type AdminSubAccountPermissionsRequest struct {
	Permissions []string `json:"permissions"`
}

// AdminSubAccountResetPasswordRequest 重置子账号密码。
// Password 留空时生成一次性密码，仅在响应中返回一次。
type AdminSubAccountResetPasswordRequest struct {
	Password string `json:"password" binding:"omitempty,min=6,max=64"`
}

// AdminSubAccountResetPasswordResponse 重置子账号密码响应。
type AdminSubAccountResetPasswordResponse struct {
	Password string `json:"password"`
	// RevokedSessions 本次连带撤销的会话数（≥0）。
	RevokedSessions int `json:"revoked_sessions"`
}

// CreateSubAccountResult 新建子账号响应（一次性密码仅此返回）。
type CreateSubAccountResult struct {
	Member   SubAccountMemberInfo `json:"member"`
	Password string               `json:"password"`
}

// SubAccountLogListResponse 子账号操作日志列表。
type SubAccountLogListResponse struct {
	Items []SubAccountLogInfo `json:"items"`
	Total int64               `json:"total"`
}

// SubAccountLogInfo 子账号操作日志项。
type SubAccountLogInfo struct {
	ID        uint64 `json:"id"`
	ActorID   uint64 `json:"actor_user_id"`
	ActorName string `json:"actor_name"`
	Module    string `json:"module"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Detail    string `json:"detail"`
	IP        string `json:"ip"`
	CreatedAt string `json:"created_at"`
}

// SubAccountPermissionOption 可选客户侧权限项（含中文标签）。
type SubAccountPermissionOption struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}
