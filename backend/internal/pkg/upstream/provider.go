// Package upstream 定义统一的云资源上游提供商抽象。
//
// 本包在第一阶段仅落地接口与结构定义（骨架），各上游适配器的具体实现
// （魔方云/阿里云/腾讯云等）在后续阶段补齐。
package upstream

import (
	"context"
	"fmt"
	"time"

	"hostsent/backend/internal/pkg/model"
)

// Provider 上游提供商统一接口 —— 仅保留全适配器共有的最小能力。
// Capabilities 返回字段级能力描述（契约②，见 capability.go）；实现方通常直接
// 返回本包注册的描述符，旧实现可依赖 CapabilitiesOf 的类型断言兜底。
type Provider interface {
	// 基础信息
	GetType() string
	GetName() string
	HealthCheck(ctx context.Context) error
	// Capabilities 字段级能力描述（T2.1）：驱动后台能力矩阵/动态表单、开通前校验、同步任务生成。
	Capabilities() CapabilityDescriptor
}

// ProductCatalog 商品目录读取能力。
type ProductCatalog interface {
	ListProducts(ctx context.Context) ([]*model.StandardProduct, error)
	GetProduct(ctx context.Context, upstreamID string) (*model.StandardProduct, error)
}

// InstanceProvisioning 实例开通能力（下单/创建实例）。
type InstanceProvisioning interface {
	CreateInstance(ctx context.Context, req *model.CreateInstanceRequest) (*model.StandardInstance, error)
}

// InstanceControl 实例电源与查询能力（详情/开机/关机/重启/控制台）。
type InstanceControl interface {
	GetInstance(ctx context.Context, instanceID string) (*model.StandardInstance, error)
	StartInstance(ctx context.Context, instanceID string) error
	StopInstance(ctx context.Context, instanceID string, force bool) error
	RestartInstance(ctx context.Context, instanceID string) error
	VNC(ctx context.Context, instanceID string) (VNCResult, error)
}

// InstanceAdministration 实例管理与维护能力（列表/删除/升降配）。
type InstanceAdministration interface {
	ListInstances(ctx context.Context, filters map[string]string) ([]*model.StandardInstance, error)
	DeleteInstance(ctx context.Context, instanceID string) error
	ResizeInstance(ctx context.Context, instanceID string, specs *model.StandardProductSpec) error
}

// InstanceRenewal 实例续费能力（T5.2）：把实例账期在上游/平台侧顺延。
//
// 链路 A（上游）由上游返回权威的新到期时间；链路 B（自营）多数平台无续费接口，
// 由本地账期顺延 + 平台侧动作承接（描述符 RenewMode 为 RenewModeNone）。
// 实现方返回上游回执单号便于对账（RenewResult.UpstreamOrderRef）。
type InstanceRenewal interface {
	RenewInstance(ctx context.Context, req *RenewRequest) (*RenewResult, error)
}

// RenewRequest 续费请求。
type RenewRequest struct {
	// ProviderInstanceID 上游/平台侧实例号。
	ProviderInstanceID string
	// Period 续费周期数（1 个月 / 3 个月 / 1 年等，含义随 Cycle）。
	Period int
	// Cycle 计费周期模式：month/monthly、year/yearly、quarter/monthly、day/daily。
	Cycle string
	// Extra 上游特有参数（原样透传）。
	Extra map[string]interface{}
}

// RenewResult 续费结果。
type RenewResult struct {
	// NewExpireAt 续费后的到期时间；零值表示上游未返回（链路 B 由调用方本地顺延）。
	NewExpireAt time.Time
	// UpstreamOrderRef 上游回执（账单号/订单号等），用于对账与幂等。
	UpstreamOrderRef string
	// Raw 上游原始响应（排障用）。
	Raw map[string]interface{}
}

// InstanceSuspension 实例暂停/恢复能力（T5.5 生命周期推进器使用）。
//
// 与 InstanceControl.Stop 的区别：Stop 是"关机"（客户可自行开机），
// Suspend 是"因欠费/违规暂停"（平台侧置为暂停态并通常禁止自助恢复）。
// 上游未实现本接口时，分派层退化为 StopInstance/StartInstance，
// 两者都不支持则返回 ErrCapabilityMissing，绝不静默假装完成。
type InstanceSuspension interface {
	SuspendInstance(ctx context.Context, instanceID, reason string) error
	UnsuspendInstance(ctx context.Context, instanceID string) error
}

// InstanceTermination 实例销毁能力（T5.5）。语义比 InstanceAdministration.DeleteInstance
// 更窄：只做"终止"。上游以取消/退订实现终止时实现本接口，无需实现整表管理能力。
type InstanceTermination interface {
	TerminateInstance(ctx context.Context, instanceID, reason string) error
}

// InstancePowerHard 实例硬电源能力（诚恳版：硬关/硬重启是独立指令，不是"软关再开"）。
//
// 与 InstanceControl.Stop(force=true) 的区别：force 只是"尽量硬一点"，本接口明确表达
// "这条指令就是硬关"。平台有独立接口时实现它，分派层优先走这里；
// 没实现时由服务层退化为硬关+开机（语义等价但中间态可见）。
type InstancePowerHard interface {
	HardStopInstance(ctx context.Context, instanceID string) error
	HardRestartInstance(ctx context.Context, instanceID string) error
}

// InstanceReinstall 实例重装系统能力。
//
// 平台侧重装是一个长任务；返回的 ReinstallResult.Password/Username 是平台为新系统
// 生成的初始凭据（部分平台返回），调用方应回写本地实例记录，否则客户改完系统就登不上。
type InstanceReinstall interface {
	ReinstallInstance(ctx context.Context, req *ReinstallRequest) (*ReinstallResult, error)
}

// ReinstallRequest 重装请求。
type ReinstallRequest struct {
	// ProviderInstanceID 平台侧实例号。
	ProviderInstanceID string
	// OS 目标镜像 ID（平台口径，与开通时的 os 同源）。
	OS string
	// Port 重装后的自定义端口（>0 才下发；SSH/RDP 端口）。
	Port int
	// FormatDataDisk 是否同时格式化数据盘（危险：数据盘数据将丢失）。
	FormatDataDisk bool
	// SystemDiskSize 目标系统盘大小（>0 才下发）。
	SystemDiskSize int
	// Extra 平台特有参数原样透传。
	Extra map[string]interface{}
}

// ReinstallResult 重装结果。
type ReinstallResult struct {
	// Password / Username 平台重装后生成的新初始凭据；为空表示平台未返回（保持原凭据）。
	Password string
	Username string
	// Raw 平台原始响应（排障用）。
	Raw map[string]interface{}
}

// InstancePasswordReset 实例登录密码重置能力（改 root/administrator 密码，不重装）。
type InstancePasswordReset interface {
	ResetInstancePassword(ctx context.Context, instanceID, newPassword string) error
}

// InstanceRescue 救援系统能力：用救援镜像临时启动，用于系统损坏时救数据/修配置。
//
// 与 Stop/Start 的语义差别：救援态下客户拿到的是**临时系统**，必须显式退出救援
// 才会回到原系统，所以 ExitRescueInstance 与 RescueInstance 成对出现。
type InstanceRescue interface {
	// RescueInstance 进入救援系统。system 为救援系统类型（平台口径，魔方云 1/2）。
	RescueInstance(ctx context.Context, instanceID string, system int, tempPassword string) error
	// ExitRescueInstance 退出救援系统。
	ExitRescueInstance(ctx context.Context, instanceID string) error
}

// InstanceSnapshot 磁盘快照与备份能力。
//
// 快照（snapshot）与备份（backup）在平台侧是同一套接口的两种 type，因此合并到一个能力里；
// 恢复快照会覆盖当前系统盘，属高危动作，由服务层单独授权。
type InstanceSnapshot interface {
	// ListSnapshots 列出实例的磁盘快照/备份（type 为空表示两类都返回）。
	ListSnapshots(ctx context.Context, instanceID, snapshotType string) ([]SnapshotInfo, error)
	// CreateSnapshot 创建快照/备份。snapshotType 取 SnapshotTypeSnap / SnapshotTypeBackup。
	CreateSnapshot(ctx context.Context, req *CreateSnapshotRequest) error
	// DeleteSnapshot 删除快照/备份（按快照 ID，非实例 ID）。
	DeleteSnapshot(ctx context.Context, snapshotID string) error
	// RestoreSnapshot 用快照/备份恢复实例（覆盖当前系统盘）。
	RestoreSnapshot(ctx context.Context, instanceID, snapshotID string) error
}

// 快照类型（平台口径 type=snap|backup）。
const (
	SnapshotTypeSnap   = "snap"
	SnapshotTypeBackup = "backup"
)

// CreateSnapshotRequest 创建快照/备份请求。
type CreateSnapshotRequest struct {
	// ProviderInstanceID 平台侧实例号（用于取系统盘）。
	ProviderInstanceID string
	// DiskID 目标磁盘 ID（平台侧；快照挂在磁盘上）。
	DiskID string
	// Type SnapshotTypeSnap / SnapshotTypeBackup。
	Type string
	// Name 快照名称。
	Name string
}

// SnapshotInfo 快照/备份条目。
type SnapshotInfo struct {
	ID         string
	Name       string
	Type       string // snap / backup
	Size       string
	Status     int
	DiskID     string
	DiskName   string
	CreateTime string
	Remarks    string
}

// InstanceHardware 实例硬件直读/直改能力（带宽/扩展 IP/IPv6/数据盘）。
//
// 这些不是"电源类"动作，而是与平台资源直接交互的硬件变更；平台各自接口差异大，
// 因此单独成接口，分派层按能力提供，不做跨平台语义归一。
type InstanceHardware interface {
	// UpdateBandwidth 修改上下行带宽（Mbps）。单方向为 0 表示不改该方向。
	UpdateBandwidth(ctx context.Context, instanceID string, inBw, outBw int) error
	// AddIPs 增加 IP：num 个，ipGroup 可选（平台 IP 分组 ID，空则由平台决定）。
	AddIPs(ctx context.Context, instanceID string, num int, ipGroup string) error
	// AddIPv6 增加 IPv6 地址数量。
	AddIPv6(ctx context.Context, instanceID string, num int) error
	// AttachDataDisk 挂载一块数据盘（GB）；store 为空由平台决定。
	AttachDataDisk(ctx context.Context, instanceID string, sizeGB int, store string) error
}

// InstanceLifecycle 实例全生命周期能力：开通 + 电源控制 + 管理维护的组合。
type InstanceLifecycle interface {
	InstanceProvisioning
	InstanceControl
	InstanceAdministration
}

// PoolReader 资源池读取能力。
type PoolReader interface {
	ListPools(ctx context.Context) ([]*StandardPool, error)
}

// AccountReader 账户信息读取能力。
type AccountReader interface {
	GetAccountInfo(ctx context.Context) (*AccountInfo, error)
}

// FinanceLedgerReader 上游财务账本读取能力（成本管理 doc111 §5.2）。
//
// 与 AccountReader（只给当前余额）的区别：账本给的是**可归集的流水**——
// 每一笔扣款/充值的时间、金额、类型与上游账单号，因此月度成本不必再靠余额差推算，
// 逐笔可查、可回填历史、可与余额快照互校。
type FinanceLedgerReader interface {
	// ListConsumptionRecords 消费流水（余额支付的开通/续费），按时间倒序分页。
	ListConsumptionRecords(ctx context.Context, page, limit int) ([]LedgerEntry, int, error)
	// ListTopupRecords 充值/入账流水，按时间倒序分页。
	ListTopupRecords(ctx context.Context, page, limit int) ([]LedgerEntry, int, error)
	// ListDueHosts 上游主机清单中与「待付」相关的信息（续费金额 + 到期日），用于余额水位告警。
	ListDueHosts(ctx context.Context) ([]DueHost, error)
}

// 账本条目方向。
const (
	// LedgerKindConsume 消费（钱从上游账户花出去）。
	LedgerKindConsume = "consume"
	// LedgerKindTopup 充值/入账（钱进上游账户）。
	LedgerKindTopup = "topup"
)

// LedgerEntry 上游账本条目（消费或充值）。
type LedgerEntry struct {
	// ExternalID 上游记录 ID（字符串化；与 ProviderID+Kind 组成唯一键，重复同步即幂等）。
	ExternalID string
	// Kind LedgerKindConsume / LedgerKindTopup。
	Kind string
	// OccurredAt 发生时间（上游给的是秒级时间戳，已转 UTC）。
	OccurredAt time.Time
	// Amount 金额（正数）。
	Amount float64
	// RefundAmount 该笔对应的退款金额（正数=已冲回；消费净额 = Amount − RefundAmount）。
	RefundAmount float64
	// Category 上游给的类型（订购产品 / 续费 / 用户充值 / 人工入账…），仅作展示与分组。
	Category string
	// RefNo 上游账单号 / 交易号（对账用）。
	RefNo string
	// Description 上游描述原文。
	Description string
}

// DueHost 上游主机的续费信息（余额水位告警用）。
type DueHost struct {
	UpstreamID  string
	ProductName string
	Domain      string
	// Amount 下一期续费金额（上游口径）。
	Amount float64
	// NextDueAt 到期时间。
	NextDueAt time.Time
	Status    string
}

// PlatformResourceReader 平台资源目录读取能力（自营规格配置用）。
// 自营商品的规格必须映射到"平台确实存在的取值"（区域/节点/存储/镜像）；
// 没有这份目录，运营只能靠记忆手填 ID，规格映射就形同虚设。
type PlatformResourceReader interface {
	ListPlatformResources(ctx context.Context) (*PlatformResources, error)
}

// PlatformResources 平台可售资源的取值目录。
type PlatformResources struct {
	Areas  []PlatformResourceItem `json:"areas"`
	Nodes  []PlatformResourceItem `json:"nodes"`
	Stores []PlatformResourceItem `json:"stores"`
	Images []PlatformResourceItem `json:"images"`
}

// PlatformResourceItem 单个平台资源项。ParentID 表达层级（节点属于区域、存储属于区域、
// 镜像属于节点）；Value 是下发到平台的原始取值（魔方云的 area/node/os/store 均为数字 ID）。
// Group 是资源自身的分组名（镜像家族 Ubuntu/Windows/CentOS），取值入库时作为 group_label。
type PlatformResourceItem struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	ParentID string `json:"parent_id,omitempty"`
	Group    string `json:"group,omitempty"`
	Status   string `json:"status,omitempty"`
}

// ProviderConfig 提供商配置
type ProviderConfig struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	APIEndpoint string `json:"api_endpoint"`
	APIKey      string `json:"api_key"`
	APISecret   string `json:"api_secret"`
	Region      string `json:"region"`
	Timeout     int    `json:"timeout"` // 秒

	// 魔方云 / 魔方财务扩展字段
	UpstreamType string `json:"upstream_type,omitempty"` // 接口类型：zjmf_api/resource 等
	Secure       bool   `json:"secure,omitempty"`        // 是否 https
	Port         string `json:"port,omitempty"`          // 接口端口，如 8443
	UserPrefix   string `json:"user_prefix,omitempty"`   // 财务标识（拼在云主机用户名前）
	AccountType  string `json:"account_type,omitempty"`  // 魔方云账号类型：admin/agent
}

// StandardPool 统一资源池
type StandardPool struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // region/zone/cluster
	TotalCPU    int    `json:"total_cpu"`
	TotalMemory int    `json:"total_memory"` // MB
	TotalDisk   int    `json:"total_disk"`   // GB
	UsedCPU     int    `json:"used_cpu"`
	UsedMemory  int    `json:"used_memory"`
	UsedDisk    int    `json:"used_disk"`
	Status      string `json:"status"`
	// Region/Zone 池所属地域与可用区（S2 位置检测）；上游未提供时留空，
	// 由同步层回退到渠道 region。
	Region string `json:"region"`
	Zone   string `json:"zone"`
}

// AccountInfo 账户信息
type AccountInfo struct {
	TotalCPU    int     `json:"total_cpu"`
	TotalMemory int     `json:"total_memory"`
	TotalDisk   int     `json:"total_disk"`
	UsedCPU     int     `json:"used_cpu"`
	UsedMemory  int     `json:"used_memory"`
	UsedDisk    int     `json:"used_disk"`
	Balance     float64 `json:"balance"`
	Currency    string  `json:"currency"`
}

// VNCResult VNC 远程控制台结果。
type VNCResult struct {
	// URL 可直接打开的远程控制台地址（可能为 http(s) iframe 地址或 websocket 地址）。
	URL string `json:"url"`
	// Password 控制台密码（URL 未内嵌时单独返回，可用于 noVNC 客户端）。
	Password string `json:"password,omitempty"`
	// External 是否外部独立控制台地址（true 时前端可直接 iframe/新窗口打开）。
	External bool `json:"external"`
}

// ErrNotImplemented 用于占位适配器：标识能力尚未实现
type ErrNotImplemented struct{}

func (e ErrNotImplemented) Error() string {
	return "upstream provider: not implemented"
}

// ProviderError 适配器统一错误类型，携带操作、上游业务码/信息与底层错误，便于运维排查。
type ProviderError struct {
	Op         string // 操作描述，如 "POST /index.php?m=api&a=xxx"
	StatusCode int    // HTTP 状态码（若为网络层错误则为 0）
	Code       int    // 上游业务码（非 0 即业务失败）
	Msg        string // 上游返回的 message
	Err        error  // 底层包装错误
}

func (e *ProviderError) Error() string {
	msg := e.Msg
	if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}
	if e.StatusCode != 0 {
		return fmt.Sprintf("upstream %s: http=%d code=%d msg=%s", e.Op, e.StatusCode, e.Code, msg)
	}
	return fmt.Sprintf("upstream %s: code=%d msg=%s", e.Op, e.Code, msg)
}

// Unwrap 支持 errors.Is/As 链式判定
func (e *ProviderError) Unwrap() error { return e.Err }
