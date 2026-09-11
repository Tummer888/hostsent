// Package dto 定义实例运维管理台的接口出入参。
package dto

// 到期状态筛选项。
const (
	ExpireStateAll      = "all"      // 全部
	ExpireStateExpiring = "expiring" // 临期（expire_within_days 天内）
	ExpireStateExpired  = "expired"  // 已过期
	ExpireStateNone     = "none"     // 未设置到期时间
	ExpireStateNormal   = "normal"   // 到期时间充裕（列表项展示用，非筛选项）
)

// ListMeta 通用分页元信息。
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// ListQuery 跨用户实例列表查询。
type ListQuery struct {
	Keyword          string `form:"keyword" json:"keyword"`                       // 实例标识 / 实例名
	UserKeyword      string `form:"user_keyword" json:"user_keyword"`             // 用户名 / 邮箱 / 手机号
	UserID           uint64 `form:"user_id" json:"user_id"`                       // 精确用户
	ProviderID       uint64 `form:"provider_id" json:"provider_id"`               // 服务商
	Status           string `form:"status" json:"status"`                         // 服务状态
	SourceMode       string `form:"source_mode" json:"source_mode"`               // self/upstream（链路筛选，T1.3）
	ExpireState      string `form:"expire_state" json:"expire_state"`             // all/expiring/expired/none
	ExpireWithinDays int    `form:"expire_within_days" json:"expire_within_days"` // 临期天数，默认 7
	Page             int    `form:"page" json:"page"`
	PageSize         int    `form:"page_size" json:"page_size"`
}

// InstanceItem 实例列表项（实例 + 归属用户 + 服务商）。
type InstanceItem struct {
	ID           uint64 `json:"id"`
	InstanceID   string `json:"instance_id"`
	ProviderID   uint64 `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	ProviderType string `json:"provider_type"`
	UserID       uint64 `json:"user_id"`
	Username     string `json:"username"`
	UserEmail    string `json:"user_email"`
	UserPhone    string `json:"user_phone"`
	ProductID    uint64 `json:"product_id"`
	OrderID      uint64 `json:"order_id"`
	// 双链路语义（P1/T1.4）：来源判据与两条链路各自的产品引用。
	SourceMode         string `json:"source_mode"`
	SellProductID      uint64 `json:"sell_product_id"`
	UpstreamProductID  uint64 `json:"upstream_product_id"`
	ProviderInstanceID string `json:"provider_instance_id"`
	LifecycleStage     string `json:"lifecycle_stage"`
	Name               string `json:"name"`
	CPU                int    `json:"cpu"`
	Memory             int    `json:"memory"`
	Disk               int    `json:"disk"`
	DiskType           string `json:"disk_type"`
	Bandwidth          int    `json:"bandwidth"`
	OS                 string `json:"os"`
	Region             string `json:"region"`
	Zone               string `json:"zone"`
	Status             string `json:"status"`
	PowerStatus        string `json:"power_status"`
	PrivateIP          string `json:"private_ip"`
	PublicIP           string `json:"public_ip"`
	BillingMode        string `json:"billing_mode"`
	ActorUserID        uint64 `json:"actor_user_id"`
	ActorName          string `json:"actor_name"`
	Remark             string `json:"remark"`
	ExpireAt           string `json:"expire_at"`
	DaysLeft           int    `json:"days_left"`
	ExpireState        string `json:"expire_state"`
	LastSyncedAt       string `json:"last_synced_at"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

// ListResponse 实例列表响应。
type ListResponse struct {
	Items []InstanceItem `json:"items"`
	Meta  ListMeta       `json:"meta"`
}

// StatsQuery 概览统计查询。
type StatsQuery struct {
	ExpireWithinDays int `form:"expire_within_days" json:"expire_within_days"`
}

// StatsResponse 概览统计（全局口径，不受列表筛选影响）。
type StatsResponse struct {
	Total    int64 `json:"total"`
	Running  int64 `json:"running"`
	Stopped  int64 `json:"stopped"`
	Creating int64 `json:"creating"`
	Error    int64 `json:"error"`
	Expiring int64 `json:"expiring"`
	Expired  int64 `json:"expired"`
}

// Capabilities 上游对当前实例支持的操作能力（前端据此置灰按钮）。
type Capabilities struct {
	Power     bool `json:"power"`
	Console   bool `json:"console"`
	Resize    bool `json:"resize"`
	Destroy   bool `json:"destroy"`
	Reinstall bool `json:"reinstall"`
	// Suspend 是否支持暂停/恢复（T5.5）：平台暂停态或退化为关机/开机皆算支持。
	Suspend bool `json:"suspend"`
}

// DetailInfo 实例详情。
type DetailInfo struct {
	InstanceItem
	Capabilities    Capabilities `json:"capabilities"`
	CapabilityError string       `json:"capability_error"`
}

// PowerRequest 电源操作请求。
type PowerRequest struct {
	Action string `json:"action" binding:"required"`
}

// VNCResult 远程控制台结果。
type VNCResult struct {
	URL      string `json:"url"`
	Password string `json:"password"`
	External bool   `json:"external"`
}

// ResizeRequest 变配请求（仅提交需变更的字段）。
type ResizeRequest struct {
	CPU      int    `json:"cpu"`
	Memory   int    `json:"memory"`
	Disk     int    `json:"disk"`
	DiskType string `json:"disk_type"`
	Reason   string `json:"reason"`
}

// DestroyRequest 销毁请求；ConfirmMark 必须与实例标识完全一致才算二次确认。
type DestroyRequest struct {
	ConfirmMark string `json:"confirm_mark" binding:"required"`
	Reason      string `json:"reason"`
}

// SuspendRequest 暂停请求（T5.5；reason 透传上游暂停原因并落审计）。
type SuspendRequest struct {
	Reason string `json:"reason"`
}

// RemarkRequest 管理员备注请求。
type RemarkRequest struct {
	Remark string `json:"remark"`
}

// OperationItem 操作流水项。
type OperationItem struct {
	ID           uint64 `json:"id"`
	InstanceID   uint64 `json:"instance_id"`
	InstanceMark string `json:"instance_mark"`
	UserID       uint64 `json:"user_id"`
	OperatorType string `json:"operator_type"`
	OperatorID   uint64 `json:"operator_id"`
	OperatorName string `json:"operator_name"`
	Action       string `json:"action"`
	Params       string `json:"params"`
	BeforeStatus string `json:"before_status"`
	AfterStatus  string `json:"after_status"`
	Result       string `json:"result"`
	ErrorMessage string `json:"error_message"`
	CreatedAt    string `json:"created_at"`
}

// OperationListQuery 操作流水分页查询。
type OperationListQuery struct {
	Page     int `form:"page" json:"page"`
	PageSize int `form:"page_size" json:"page_size"`
}

// OperationListResponse 操作流水响应。
type OperationListResponse struct {
	Items []OperationItem `json:"items"`
	Meta  ListMeta        `json:"meta"`
}

// RelatedOrderItem 关联订单项。
type RelatedOrderItem struct {
	ID          uint64  `json:"id"`
	OrderNo     string  `json:"order_no"`
	Status      string  `json:"status"`
	ProductName string  `json:"product_name"`
	TotalAmount float64 `json:"total_amount"`
	PaidAmount  float64 `json:"paid_amount"`
	PayMethod   string  `json:"pay_method"`
	CreatedAt   string  `json:"created_at"`
}

// RelatedRenewalItem 关联续费记录项。
type RelatedRenewalItem struct {
	ID           uint64  `json:"id"`
	RenewalNo    string  `json:"renewal_no"`
	Source       string  `json:"source"`
	Status       string  `json:"status"`
	PeriodCount  int     `json:"period_count"`
	Amount       float64 `json:"amount"`
	ExpireBefore string  `json:"expire_before"`
	ExpireAfter  string  `json:"expire_after"`
	CreatedAt    string  `json:"created_at"`
}

// RelatedTicketItem 关联工单项。
type RelatedTicketItem struct {
	ID           uint64 `json:"id"`
	TicketNo     string `json:"ticket_no"`
	Title        string `json:"title"`
	Priority     string `json:"priority"`
	Status       string `json:"status"`
	AssignedName string `json:"assigned_name"`
	CreatedAt    string `json:"created_at"`
}

// RelatedInfo 实例关联的业务记录（订单/续费/工单）。
type RelatedInfo struct {
	Orders   []RelatedOrderItem   `json:"orders"`
	Renewals []RelatedRenewalItem `json:"renewals"`
	Tickets  []RelatedTicketItem  `json:"tickets"`
}
