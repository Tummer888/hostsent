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
