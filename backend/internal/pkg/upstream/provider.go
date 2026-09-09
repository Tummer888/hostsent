// Package upstream 定义统一的云资源上游提供商抽象。
//
// 本包在第一阶段仅落地接口与结构定义（骨架），各上游适配器的具体实现
// （魔方云/阿里云/腾讯云等）在后续阶段补齐。
package upstream

import (
	"context"
	"fmt"

	"hostsent/backend/internal/pkg/model"
)

// Provider 上游提供商统一接口 —— 仅保留全适配器共有的最小能力。
type Provider interface {
	// 基础信息
	GetType() string
	GetName() string
	HealthCheck(ctx context.Context) error
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
	MaxRetries  int    `json:"max_retries"`

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
