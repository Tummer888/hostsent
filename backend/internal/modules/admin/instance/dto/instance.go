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
	// 到期处置退避状态（doc61 §8.4）：连续失败次数、下次重试时间、最近失败原因。
	EnforceAttempts  int    `json:"enforce_attempts"`
	EnforceNextAt    string `json:"enforce_next_at"`
	LastEnforceError string `json:"last_enforce_error"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
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
//
// 与 upstream.CapabilityDescriptor 的关系：描述符是**渠道级**声明（"这个平台能做什么"），
// 这里是**实例级**结果（当前实例的渠道实际能做什么），由类型断言 + 描述符共同得出。
type Capabilities struct {
	Power     bool `json:"power"`
	Console   bool `json:"console"`
	Resize    bool `json:"resize"`
	Destroy   bool `json:"destroy"`
	Reinstall bool `json:"reinstall"`
	// Suspend 是否支持暂停/恢复（T5.5）：平台暂停态或退化为关机/开机皆算支持。
	Suspend bool `json:"suspend"`
	// 维护类（实测平台接口存在，见 mofangyun/provider.go 头部映射）。
	ResetPassword bool `json:"reset_password"` // 重置登录密码
	Rescue        bool `json:"rescue"`         // 救援系统（进/出）
	Snapshot      bool `json:"snapshot"`       // 磁盘快照 / 备份
	Bandwidth     bool `json:"bandwidth"`      // 带宽直改
	AddIP         bool `json:"add_ip"`         // 增加 IP / IPv6
	AttachDisk    bool `json:"attach_disk"`    // 挂载数据盘
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

// ReinstallRequest 重装系统请求（可换镜像；FormatDataDisk 危险需显式传 true）。
type ReinstallRequest struct {
	OS string `json:"os" binding:"required"`
	// Port 重装后的自定义端口（>0 生效；SSH/RDP）。
	Port int `json:"port"`
	// FormatDataDisk 是否同时格式化数据盘（数据将丢失，默认 false）。
	FormatDataDisk bool `json:"format_data_disk"`
	// SystemDiskSize 目标系统盘大小（>0 生效）。
	SystemDiskSize int    `json:"system_disk_size"`
	Reason         string `json:"reason"`
}

// ReinstallResult 重装结果：平台为 新系统 生成的初始凭据。
//
// 只在此处一次性回传，不落库（本库 instances 表没有凭据列，开通时也不落盘）；
// 前端必须提示运营/客户立即保存，否则只能去平台面板重置。
type ReinstallResult struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ResetPasswordRequest 重置实例登录密码请求。
type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// RescueRequest 进入救援系统请求。
type RescueRequest struct {
	// System 救援系统类型（平台口径：魔方云 1/2）。
	System int `json:"system" binding:"required"`
	// TempPassword 救援系统的临时密码。
	TempPassword string `json:"temp_password" binding:"required"`
}

// SnapshotCreateRequest 创建快照/备份请求。
type SnapshotCreateRequest struct {
	// Type snap（快照）| backup（备份）；空视为 snap。
	Type string `json:"type"`
	// Name 快照名称；空由适配器生成。
	Name string `json:"name"`
	// DiskID 目标磁盘 ID（平台侧）；空则由服务层取实例的系统盘。
	DiskID string `json:"disk_id"`
}

// SnapshotRestoreRequest 用快照/备份恢复请求（高危：会覆盖当前系统盘）。
type SnapshotRestoreRequest struct {
	// SnapshotID 快照 ID。
	SnapshotID string `json:"snapshot_id" binding:"required"`
	// ConfirmMark 二次确认（须等于实例标识或记录 ID）。
	ConfirmMark string `json:"confirm_mark" binding:"required"`
}

// SnapshotInfo 快照/备份条目（下发给前端展示）。
type SnapshotInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Size       string `json:"size"`
	Status     int    `json:"status"`
	DiskID     string `json:"disk_id"`
	DiskName   string `json:"disk_name"`
	CreateTime string `json:"create_time"`
	Remarks    string `json:"remarks"`
}

// BandwidthRequest 带宽直改请求（Mbps；0 表示不改该方向）。
type BandwidthRequest struct {
	InBw   int    `json:"in_bw"`
	OutBw  int    `json:"out_bw"`
	Reason string `json:"reason"`
}

// AddIPRequest 增加 IP / IPv6 请求。
type AddIPRequest struct {
	// Version 4 或 6；空视为 4。
	Version int `json:"version"`
	// Num 增加数量（>0）。
	Num int `json:"num" binding:"required"`
	// IPGroup 平台 IP 分组 ID（可选，空由平台决定）。
	IPGroup string `json:"ip_group"`
}

// AttachDiskRequest 挂载数据盘请求。
type AttachDiskRequest struct {
	SizeGB int    `json:"size_gb" binding:"required"`
	Store  string `json:"store"`
	Reason string `json:"reason"`
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

// BatchActionRequest 批量运维请求（doc61 P1：列表页多选后一次性下发）。
//
// Action 取值与单实例接口一致：on/off/hard_off/reboot/hard_reboot（电源）、
// sync（回源刷新）、suspend/unsuspend（平台暂停态）。
type BatchActionRequest struct {
	Action string   `json:"action" binding:"required"`
	IDs    []uint64 `json:"ids" binding:"required"`
	Reason string   `json:"reason"`
}

// BatchActionItem 批量操作中的单实例结果。
//
// Status 三态：success（已下发）/ skipped（无需或不可能执行，如已在目标状态、上游缺能力）/
// failed（真实失败，如上游不可达）。skipped 不计入失败，避免运维被"已在运行中"这类
// 无害结果误导为事故。
type BatchActionItem struct {
	ID           uint64 `json:"id"`
	InstanceMark string `json:"instance_mark"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	BeforeStatus string `json:"before_status"`
	AfterStatus  string `json:"after_status"`
}

// BatchActionResponse 批量运维响应（逐台结果，供前端按行回显）。
type BatchActionResponse struct {
	Total     int               `json:"total"`
	Succeeded int               `json:"succeeded"`
	Failed    int               `json:"failed"`
	Skipped   int               `json:"skipped"`
	Items     []BatchActionItem `json:"items"`
}

// RemarkRequest 管理员备注请求。
type RemarkRequest struct {
	Remark string `json:"remark"`
}

// RefundInstanceRequest 退款联动处置请求（订单侧钩子 → 实例运维台内部调用）。
type RefundInstanceRequest struct {
	OrderID      uint64  `json:"order_id"`
	OrderNo      string  `json:"order_no"`
	UserID       uint64  `json:"user_id"`
	RefundNo     string  `json:"refund_no"`
	RefundAmount float64 `json:"refund_amount"`
	PaidAmount   float64 `json:"paid_amount"`
	// FullRefund 本次退款是否已覆盖订单全部实付：只有全额退款才谈得上处置实例。
	FullRefund bool `json:"full_refund"`
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

// OperationLogQuery 全局操作流水查询（跨实例审计视图）。
type OperationLogQuery struct {
	Keyword      string `form:"keyword" json:"keyword"`             // 实例标识 / 操作人 / 用户名
	Action       string `form:"action" json:"action"`               // 动作：power_on/power_off/.../suspend/destroy/stage
	OperatorType string `form:"operator_type" json:"operator_type"` // admin/user/system
	Result       string `form:"result" json:"result"`               // success/failed/skipped
	UserID       uint64 `form:"user_id" json:"user_id"`
	StartTime    string `form:"start_time" json:"start_time"`
	EndTime      string `form:"end_time" json:"end_time"`
	Page         int    `form:"page" json:"page"`
	PageSize     int    `form:"page_size" json:"page_size"`
}

// OperationLogItem 全局操作流水项。
type OperationLogItem struct {
	OperationItem
	InstanceName string `json:"instance_name"`
	Username     string `json:"username"`
}

// OperationLogListResponse 全局操作流水响应。
type OperationLogListResponse struct {
	Items []OperationLogItem `json:"items"`
	Meta  ListMeta           `json:"meta"`
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
