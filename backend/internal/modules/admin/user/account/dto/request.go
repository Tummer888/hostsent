package dto

type UserListQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
	Filter   string `form:"filter"`
	Keyword  string `form:"keyword"`
	// Sort 列表排序方向，按 ID 排："asc" 正序，"desc" 倒序（默认）。
	// 只认这两个值，其余一律回落到 desc；排序列固定为 users.id，
	// 不接受外部传入列名，避免列名拼接进 ORDER BY。
	Sort string `form:"sort"`
	// IncludeDeleted 为 true 时连已注销用户一起返回（doc104 §4.3）；
	// 与 filter=deleted 互斥，前者优先级更高。
	IncludeDeleted bool `form:"include_deleted"`
	// UserLevelID 按用户等级筛选（P3-04，0 表示不筛选）。
	UserLevelID uint64 `form:"user_level_id"`
	// UserGroupID 按用户组筛选（0 表示不筛选）。
	UserGroupID uint64 `form:"user_group_id"`
	// AgentLevelID 按代理等级筛选（0 表示不筛选）（doc108）。
	AgentLevelID uint64 `form:"agent_level_id"`
	// IsAgent 按代理身份筛选："true" 仅代理，"false" 仅非代理，空为全部。
	IsAgent string `form:"is_agent"`
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
	// UserLevelID 建号时的用户等级；不传则由服务层给起始等级（最低权重的一级）。
	UserLevelID *uint64 `json:"user_level_id"`
	// AgentLevelID 建号时的代理等级（doc108）；不传 = 非代理（普通客户）。
	// 建号默认不授予代理身份 —— 代理是运营显式授予的商务关系，不该被"顺手带出来"。
	AgentLevelID *uint64 `json:"agent_level_id"`
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
	// UserLevelID 调整用户等级（会员体系）：nil 表示不修改，其余为等级 ID。
	// 与用户组不同，0 不是合法取值 —— 每个客户账号都应当有等级，
	// 「清空等级」会让子账号上限读到 0（=不限制）且用户端显示无等级。
	UserLevelID *uint64 `json:"user_level_id"`
	// AgentLevelID 调整代理等级（doc108，折扣的唯一来源）：nil 不修改，
	// 0 表示取消代理身份（回到普通客户），其余为代理等级 ID。
	// 与用户等级相反，0 在这里是合法且有意义的取值。
	AgentLevelID *uint64 `json:"agent_level_id"`
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

// RechargeRequest 用户钱包人工充值 / 调账请求。
// Amount 为正表示充值（生成充值单并入账，同时归集当期账单），为负表示扣减（追回）；
// 0 会被服务拒绝。
type RechargeRequest struct {
	Amount float64 `json:"amount" binding:"required"`
	Remark string  `json:"remark" binding:"max=255"`
}

// RechargeResponse 人工充值/调账结果：回可追溯单号，运营据此去财务页面核对。
//
// 充值回充值单号（财务「充值管理」页可查、用户端「我的充值单」可见），
// 扣减回资金流水号（「资金流水」页可查）。此前接口固定返回字符串 "ok"，
// 运营充完钱没有任何可核对的凭据。
type RechargeResponse struct {
	RechargeNo string `json:"recharge_no,omitempty"`
	TxNo       string `json:"tx_no,omitempty"`
}

// AdminCreateOrderRequest 为指定用户创建订单。
type AdminCreateOrderRequest struct {
	ProductID    uint64  `json:"product_id" binding:"required"`
	BillingCycle string  `json:"billing_cycle"` // monthly/quarterly/annually...
	Price        float64 `json:"price"`         // 覆盖价格（<=0 用商品默认价）
	PayMode      string  `json:"pay_mode"`      // balance=余额支付并开通; create=仅创建不支付(默认)
	// SpecCode 所选 SKU 编码（T4.1）。商品挂了规格时必填，与用户端下单同口径。
	SpecCode string `json:"spec_code"`
	// Quantity 购买数量，<=0 视为 1。
	Quantity int `json:"quantity"`
	// ConfigSelections 客户选配项选择（T4.5，平台参数名/上游选项 id → 取值）。
	ConfigSelections map[string]string `json:"config_selections"`
	// RenewalPrice 绑定该实例后续续费的单价（元/周期）；nil 或 <=0 表示不绑定，
	// 续费按商品定价实时算。与 Price（仅首次开通的成交价）分开，
	// 用于「首次 30 元、续费 18 元」这类单机价格承诺（一经绑定长期有效）。
	RenewalPrice *float64 `json:"renewal_price"`
}

// AdminOrderConfigResponse 后台代下单的可选配置项（供客服按商品选规格/周期/选配）。
//
// 与用户端商品详情同源：SKU 列表、可售周期、客户选配项都由商品目录与价格矩阵给出，
// 后台只做展示，不另维护一套口径（否则代下单开通出来的配置会与用户自助购买不一致）。
type AdminOrderConfigResponse struct {
	ProductID  uint64                    `json:"product_id"`
	Name       string                    `json:"name"`
	PriceModel string                    `json:"price_model"`
	Price      float64                   `json:"price"`
	SourceMode string                    `json:"source_mode"`
	Cycles     []string                  `json:"cycles"`
	Specs      []AdminOrderConfigSpec    `json:"specs"`
	Options    []AdminOrderConfigOption  `json:"options"`
	Upstream   *AdminOrderConfigUpstream `json:"upstream,omitempty"`
}

// AdminOrderConfigSpec 一个可售 SKU。
type AdminOrderConfigSpec struct {
	SpecCode   string   `json:"spec_code"`
	Name       string   `json:"name"`
	Price      float64  `json:"price"`
	PriceModel string   `json:"price_model"`
	Stock      int      `json:"stock"` // -1 表示不限
	Cycles     []string `json:"cycles"`
	// Specs 该 SKU 的原子取值 JSON（compute.cpu / compute.memory(MB) /
	// storage.system.size 等）。下发给后台是为了让规格以「2核 4G 50G」卡片展示，
	// 客服不必凭 SKU 名称猜配置；解析失败时前端只显示名称，不影响下单。
	Specs string `json:"specs"`
}

// AdminOrderQuoteRequest 代下单算价预览入参：与创建订单同口径（少掉支付方式）。
//
// 存在的意义是让客服在填自定义价前看到「默认要收多少」：此前弹窗只能显示商品基础价，
// 一换规格/周期/选配就与真实成交价脱节，客服只能凭经验填数。
type AdminOrderQuoteRequest struct {
	ProductID        uint64            `json:"product_id" binding:"required"`
	BillingCycle     string            `json:"billing_cycle"`
	SpecCode         string            `json:"spec_code"`
	Quantity         int               `json:"quantity"`
	ConfigSelections map[string]string `json:"config_selections"`
}

// AdminOrderQuoteResponse 代下单算价明细（原价/优惠/实付 + 选配加价小计）。
type AdminOrderQuoteResponse struct {
	OriginalAmount float64 `json:"original_amount"`
	DiscountAmount float64 `json:"discount_amount"`
	FinalAmount    float64 `json:"final_amount"`
	OptionAmount   float64 `json:"option_amount"`
	DiscountSource string  `json:"discount_source"`
	Currency       string  `json:"currency"`
}

// AdminOrderConfigOption 一个客户可选配置项（widget 决定前端渲染方式）。
type AdminOrderConfigOption struct {
	OptionKey    string   `json:"option_key"`
	Name         string   `json:"name"`
	Widget       string   `json:"widget"`
	Required     bool     `json:"required"`
	DefaultValue string   `json:"default_value"`
	Unit         string   `json:"unit"`
	Help         string   `json:"help"`
	MinValue     *float64 `json:"min_value"`
	MaxValue     *float64 `json:"max_value"`
	UnitPrice    float64  `json:"unit_price"`
	// 数量型选项的季度 / 年度 / 一次性单位加价；0 表示该周期未单独给价（回落月度）。
	// 代下单弹窗按当前所选周期挑对应档位估算，只给月度值会让年付预览远低于实付。
	UnitPriceQuarterly float64                     `json:"unit_price_quarterly"`
	UnitPriceAnnually  float64                     `json:"unit_price_annually"`
	UnitPriceOnetime   float64                     `json:"unit_price_onetime"`
	Values             []AdminOrderConfigOptionVal `json:"values"`
	// Locked 上游固定配置项（只有一个可见取值）：只读展示，不可下发覆盖。
	Locked bool `json:"locked"`
	// Creatable 允许填候选之外的取值（平台目录 manual 来源，如端口可填具体端口号）。
	// 前端据此把下拉换成可输入形态；不允许时只能用候选值。
	Creatable bool `json:"creatable"`
}

// AdminOrderConfigOptionVal 一个可选值（含四周期加价）。
type AdminOrderConfigOptionVal struct {
	Value          string  `json:"value"`
	Label          string  `json:"label"`
	Group          string  `json:"group_label,omitempty"`
	Help           string  `json:"help,omitempty"`
	Default        bool    `json:"is_default"`
	PriceMonthly   float64 `json:"price_monthly"`
	PriceQuarterly float64 `json:"price_quarterly"`
	PriceAnnually  float64 `json:"price_annually"`
	PriceOnetime   float64 `json:"price_onetime"`
}

// AdminOrderConfigUpstream 对接商品的上游固定配置（只读展示；客户可选项已在 Options 里）。
type AdminOrderConfigUpstream struct {
	FixedConfig bool                             `json:"fixed_config"`
	Renewable   bool                             `json:"renewable"`
	RenewNote   string                           `json:"renew_note"`
	Options     []AdminOrderConfigUpstreamOption `json:"options"`
}

// AdminOrderConfigUpstreamOption 上游配置项及锁定状态。
type AdminOrderConfigUpstreamOption struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Locked bool   `json:"locked"`
	Value  string `json:"value"`
}
