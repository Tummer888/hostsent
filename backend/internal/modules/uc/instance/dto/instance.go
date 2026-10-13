// Package dto 定义用户中心主机模块数据结构。
package dto

// InstanceInfo 用户可见主机信息。
type InstanceInfo struct {
	ID           uint64 `json:"id"`
	InstanceID   string `json:"instance_id"`
	ProviderType string `json:"provider_type"`
	ProviderID   uint64 `json:"provider_id"`
	Name         string `json:"name"`
	Status       string `json:"status"`       // 服务状态：running/stopped/creating/...
	PowerStatus  string `json:"power_status"` // 电源状态：on/off/unknown
	OS           string `json:"os"`
	CPU          int    `json:"cpu"`
	Memory       int    `json:"memory"`
	Disk         int    `json:"disk"`
	DiskType     string `json:"disk_type"`
	Bandwidth    int    `json:"bandwidth"`
	Region       string `json:"region"`
	Zone         string `json:"zone"`
	PublicIP     string `json:"public_ip"`
	PrivateIP    string `json:"private_ip"`
	BillingMode  string `json:"billing_mode"`
	// ActorID/ActorName 开通该实例的真实操作人（子账号下单可见，P4-09）
	ActorID   uint64 `json:"actor_user_id"`
	ActorName string `json:"actor_name"`
	// OrderID 来源订单；ProviderInstanceID 为空表示尚未在上游开通（待开通占位行），
	// 前端据此显示「待开通」并只开放「重新开通」动作。
	OrderID            uint64 `json:"order_id"`
	ProviderInstanceID string `json:"provider_instance_id"`
	Pending            bool   `json:"pending"`
	// ProvisionStatus/ProvisionError 待开通实例的履约状态与最近失败原因
	// （pending/running/success/failed/manual；manual=需人工，如上游余额不足）。
	ProvisionStatus string `json:"provision_status,omitempty"`
	ProvisionError  string `json:"provision_error,omitempty"`
	ExpireAt        string `json:"expire_at"`
	// LifecycleStage 生命周期阶段（active/grace/suspended/destroyed）。
	// 与服务状态是两个维度：暂停的机器 status 仍是 stopped，只有阶段能说明
	// 「为什么被停」（到期处置 vs 用户自己关机）。列表/详情的状态徽标据此合成。
	LifecycleStage string `json:"lifecycle_stage"`
	// ExpireState 到期状态（none/expiring/expired/normal），读时计算。
	ExpireState string `json:"expire_state"`
	// DaysLeft 距到期天数（负数表示已过期天数）；未设置到期时间为 0。
	DaysLeft  int    `json:"days_left"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	// Capabilities 当前实例可用的操作（按上游能力推导；待开通实例全为 false）。
	Capabilities Capabilities `json:"capabilities"`
	// NatAccess NAT 机型的访问地址（doc122）。NAT 机型没有独立公网 IP，
	// 客户只能经节点转发地址访问；独立 IP 机型为 nil，前端不渲染该区块。
	NatAccess *NatAccessInfo `json:"nat_access,omitempty"`
}

// NatAccessInfo NAT 机型的访问地址（用户中心下发）。
//
// 对客户而言这是"唯一能连上机器的方式"：NAT 机型没有独立 IP，public_ip 是空的，
// 不把转发地址给出来客户就无从下手。
type NatAccessInfo struct {
	// IsNat 是否 NAT 机型。依据是机型而非"有没有转发规则"：刚开通还没建规则的
	// NAT 机器同样是 NAT 机型，不能让客户以为买到了带独立 IP 的机器。
	IsNat bool `json:"is_nat"`
	// Forwards 端口转发地址（已拼成 "IP:端口"，可直接复制使用）。
	Forwards []string `json:"forwards"`
	// Domains 共享建站域名。
	Domains []string `json:"domains"`
	// Error 读取失败原因。与"没有转发规则"区分：读失败提示可刷新重试，
	// 而不是让客户以为机器没配好。
	Error string `json:"error,omitempty"`
}

// Capabilities 实例可用操作能力位（前端据此置灰按钮，避免"能点但点了报错"）。
type Capabilities struct {
	Power         bool `json:"power"`
	Console       bool `json:"console"`
	Reinstall     bool `json:"reinstall"`
	ResetPassword bool `json:"reset_password"`
	Rescue        bool `json:"rescue"`
	Snapshot      bool `json:"snapshot"`
	Destroy       bool `json:"destroy"`
	// Metrics 是否提供性能监控（CPU/内存/磁盘/网卡图表）。
	Metrics bool `json:"metrics"`
	// NatAccess 是否可读 NAT 访问地址（doc122）：渠道声明了 NatReader 能力时为真。
	NatAccess bool `json:"nat_access"`
}

// ListQuery 我的主机查询。
type ListQuery struct {
	Live bool `form:"live" json:"live"` // 是否刷新上游实时状态
}

// ListSummary 列表汇总（前端卡片统计用；与 items 同一次查询算出，不额外打库）。
type ListSummary struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Stopped   int `json:"stopped"`
	Creating  int `json:"creating"`
	Expiring  int `json:"expiring"`
	Expired   int `json:"expired"`
	Destroyed int `json:"destroyed"`
}

// ListResponse 我的主机列表响应。
type ListResponse struct {
	Items   []InstanceInfo `json:"items"`
	Total   int64          `json:"total"`
	Summary ListSummary    `json:"summary"`
}

// MetricsCatalogResponse 监控图表目录。
//
// Supported=false 表示当前实例所在平台不提供监控（或实例尚未开通）：
// 前端据此显示说明文案，而不是画一张永远没有数据的空图。
type MetricsCatalogResponse struct {
	Supported bool               `json:"supported"`
	Reason    string             `json:"reason,omitempty"`
	Charts    []MetricsChartInfo `json:"charts"`
}

// MetricsChartInfo 单项图表描述。
//
// Type 是平台原生标识：前端必须原样回传给 /metrics/series 的 type 参数
// （财务型上游会把它透传给上游模块，如 flow）。Kind 是归一化语义
// （cpu/memory/disk/network），仅用于前端配色与图标选择。
type MetricsChartInfo struct {
	Type   string               `json:"type"`
	Kind   string               `json:"kind"`
	Title  string               `json:"title"`
	Select []MetricsChartOption `json:"select,omitempty"`
}

// MetricsChartOption 图表子项（如磁盘设备名）。
type MetricsChartOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// MetricsQuery 监控数据查询（毫秒时间戳）。
type MetricsQuery struct {
	Type   string `form:"type" json:"type"`
	Select string `form:"select" json:"select"`
	Start  int64  `form:"start" json:"start"`
	End    int64  `form:"end" json:"end"`
	// Range 预设时间窗（1h/6h/24h/7d），未显式给 start/end 时按它推算。
	Range string `form:"range" json:"range"`
}

// MetricsSeries 监控时间序列（已由适配器归一化单位与曲线）。
type MetricsSeries struct {
	Type      string           `json:"type"`
	Unit      string           `json:"unit"`
	ChartType string           `json:"chart_type"`
	Label     []string         `json:"label"`
	List      [][]MetricsPoint `json:"list"`
}

// MetricsPoint 采样点。
type MetricsPoint struct {
	Time  string  `json:"time"`
	Value float64 `json:"value"`
}

// OperationEntry 实例操作流水项（用户可见的只读审计）。
//
// 不含 Params 与上游原文：前者可能带内部参数，后者是运营排障信息。
type OperationEntry struct {
	ID           uint64 `json:"id"`
	Action       string `json:"action"`
	OperatorType string `json:"operator_type"` // admin/user/system
	OperatorName string `json:"operator_name"`
	Result       string `json:"result"` // success/failed/skipped
	ErrorMessage string `json:"error_message,omitempty"`
	CreatedAt    string `json:"created_at"`
}

// PowerRequest 电源操作请求。
type PowerRequest struct {
	Action string `json:"action" binding:"required,oneof=on off reboot hard_off hard_reboot"`
}

// VNCResult 远程控制台结果。
type VNCResult struct {
	URL      string `json:"url"`
	External bool   `json:"external"`
}

// DestroyRequest 用户侧销毁请求（doc91 §6.4）。
// ConfirmMark 必须与实例标识（instance_id）完全一致才算二次确认，
// 语义与管理端 dto.DestroyRequest 一致。
type DestroyRequest struct {
	ConfirmMark string `json:"confirm_mark" binding:"required"`
	Reason      string `json:"reason"`
}

// ReinstallRequest 用户侧重装系统请求。
type ReinstallRequest struct {
	OS string `json:"os" binding:"required"`
	// Port 重装后的自定义端口（>0 生效）。
	Port int `json:"port"`
	// FormatDataDisk 是否同时格式化数据盘（数据将丢失，默认 false）。
	FormatDataDisk bool `json:"format_data_disk"`
	// SystemDiskSize 目标系统盘大小（>0 生效）。
	SystemDiskSize int    `json:"system_disk_size"`
	Reason         string `json:"reason"`
}

// ReinstallResult 重装结果：平台新签发的初始凭据（一次性展示，不落库）。
type ReinstallResult struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ResetPasswordRequest 用户侧重置登录密码请求。
type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// RescueRequest 用户侧进入救援系统请求。
type RescueRequest struct {
	System       int    `json:"system" binding:"required"`
	TempPassword string `json:"temp_password" binding:"required"`
}

// SnapshotCreateRequest 用户侧创建快照/备份请求。
type SnapshotCreateRequest struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	DiskID string `json:"disk_id"`
}

// SnapshotRestoreRequest 用户侧快照恢复请求（高危，需二次确认）。
type SnapshotRestoreRequest struct {
	SnapshotID  string `json:"snapshot_id" binding:"required"`
	ConfirmMark string `json:"confirm_mark" binding:"required"`
}

// SnapshotInfo 快照/备份条目。
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
