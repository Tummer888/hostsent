package service

import (
	"context"
	"errors"
	"fmt"

	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	"hostsent/backend/internal/pkg/upstream"
)

// ============================================================================
// T3.1 scope 注册表：以「渠道 × scope」替代硬编码的三类同步
//
// 现状（改造前）：SyncAll 固定循环 product/pool/instance 三类，syncProducts/
// syncPools/syncInstances 用类型断言判断支持性。结果是"描述符声明不支持"的
// 渠道仍会生成注定失败的任务（如魔方云被判 product 同步失败）。
//
// 改造后：调度按 CapabilityDescriptor.SyncScopes ∩ 已注册 handler 生成任务，
// 新增一类同步只需在此登记一个 handler，调度器/任务派发不改。
// ============================================================================

// ScopeHandler 一类同步的实现单元。
type ScopeHandler struct {
	// Scope 唯一标识（与 syncmodel.Scope* 同值）。
	Scope string
	// DisplayName 后台展示名。
	DisplayName string
	// DefaultIntervalSeconds 新建调度行时的默认节奏（目录小时级、实例分钟级）。
	DefaultIntervalSeconds int
	// DefaultFullSyncIntervalSeconds 默认全量对账周期。
	DefaultFullSyncIntervalSeconds int
	// Supported 判定该适配器能否执行本 scope：以能力描述符为准，未声明时
	// 回退类型断言（兼容尚未登记描述符的旧适配器）。
	Supported func(d upstream.CapabilityDescriptor, p upstream.Provider) bool
	// Run 执行一次同步。
	Run func(ctx context.Context, e *SyncEngine, task *syncmodel.SyncTask, p upstream.Provider) error
}

// scopeHandlers 已实现的 scope 注册表（进程内静态注册）。
// catalog/price/pool/region/instance 有实现；image/stock 尚未接入（描述符可声明，
// 调度时会因无 handler 被显式跳过，而不是生成失败任务）。
var scopeHandlers = buildScopeHandlers()

func buildScopeHandlers() map[string]ScopeHandler {
	items := []ScopeHandler{
		{
			Scope:       syncmodel.ScopeCatalog,
			DisplayName: "商品目录",
			// 目录与价格变化慢，1 小时一次；全量对账每天一次。
			DefaultIntervalSeconds:         3600,
			DefaultFullSyncIntervalSeconds: 86400,
			Supported: func(d upstream.CapabilityDescriptor, p upstream.Provider) bool {
				return d.HasScope(syncmodel.ScopeCatalog) && hasProductCatalog(p)
			},
			Run: func(ctx context.Context, e *SyncEngine, task *syncmodel.SyncTask, p upstream.Provider) error {
				return e.syncCatalog(ctx, task, p)
			},
		},
		{
			Scope:                          syncmodel.ScopePrice,
			DisplayName:                    "价格",
			DefaultIntervalSeconds:         3600,
			DefaultFullSyncIntervalSeconds: 86400,
			Supported: func(d upstream.CapabilityDescriptor, p upstream.Provider) bool {
				return d.HasScope(syncmodel.ScopePrice) && hasProductCatalog(p)
			},
			Run: func(ctx context.Context, e *SyncEngine, task *syncmodel.SyncTask, p upstream.Provider) error {
				return e.syncPrices(ctx, task, p)
			},
		},
		{
			Scope:                          syncmodel.ScopePool,
			DisplayName:                    "资源池",
			DefaultIntervalSeconds:         3600,
			DefaultFullSyncIntervalSeconds: 604800,
			Supported: func(d upstream.CapabilityDescriptor, p upstream.Provider) bool {
				return d.HasScope(syncmodel.ScopePool) && hasPoolReader(p)
			},
			Run: func(ctx context.Context, e *SyncEngine, task *syncmodel.SyncTask, p upstream.Provider) error {
				return e.syncPools(ctx, task, p)
			},
		},
		{
			// region 与 pool 同源（魔方云 /areas 即区域），独立 scope 便于差异化节奏。
			Scope:                          syncmodel.ScopeRegion,
			DisplayName:                    "区域",
			DefaultIntervalSeconds:         21600,
			DefaultFullSyncIntervalSeconds: 604800,
			Supported: func(d upstream.CapabilityDescriptor, p upstream.Provider) bool {
				return d.HasScope(syncmodel.ScopeRegion) && hasPoolReader(p)
			},
			Run: func(ctx context.Context, e *SyncEngine, task *syncmodel.SyncTask, p upstream.Provider) error {
				return e.syncRegions(ctx, task, p)
			},
		},
		{
			Scope:                          syncmodel.ScopeInstance,
			DisplayName:                    "实例",
			DefaultIntervalSeconds:         300,
			DefaultFullSyncIntervalSeconds: 86400,
			Supported: func(d upstream.CapabilityDescriptor, p upstream.Provider) bool {
				return d.HasScope(syncmodel.ScopeInstance) && hasInstanceAdmin(p)
			},
			Run: func(ctx context.Context, e *SyncEngine, task *syncmodel.SyncTask, p upstream.Provider) error {
				return e.syncInstances(ctx, task, p)
			},
		},
	}
	registry := make(map[string]ScopeHandler, len(items))
	for _, item := range items {
		registry[item.Scope] = item
	}
	return registry
}

// GetScopeHandler 取某 scope 的处理器。
func GetScopeHandler(scope string) (ScopeHandler, bool) {
	h, ok := scopeHandlers[syncmodel.NormalizeScope(scope)]
	return h, ok
}

// RegisteredScopeHandlers 返回全部已注册处理器（稳定顺序，供后台展示）。
func RegisteredScopeHandlers() []ScopeHandler {
	out := make([]ScopeHandler, 0, len(scopeHandlers))
	for _, scope := range syncmodel.AllScopes() {
		if h, ok := scopeHandlers[scope]; ok {
			out = append(out, h)
		}
	}
	return out
}

// ScopeDisplayName 返回 scope 的中文名（未注册时回落为原值）。
func ScopeDisplayName(scope string) string {
	if h, ok := GetScopeHandler(scope); ok {
		return h.DisplayName
	}
	return scope
}

// PlanScopes 计算某适配器实际可调度的 scope 列表（描述符 ∩ handler ∩ 能力）。
// provider 为 nil 时只按描述符 + handler 判定。
func PlanScopes(d upstream.CapabilityDescriptor, p upstream.Provider) []string {
	out := make([]string, 0, len(d.SyncScopes))
	for _, scope := range d.SyncScopes {
		scope = syncmodel.NormalizeScope(scope)
		h, ok := scopeHandlers[scope]
		if !ok {
			continue
		}
		if p != nil && !h.Supported(d, p) {
			continue
		}
		out = append(out, scope)
	}
	return out
}

// UnsupportedScopes 返回描述符声明了、但当前无法执行的 scope 及原因（供后台提示）。
func UnsupportedScopes(d upstream.CapabilityDescriptor, p upstream.Provider) map[string]string {
	out := map[string]string{}
	for _, scope := range d.SyncScopes {
		scope = syncmodel.NormalizeScope(scope)
		h, ok := scopeHandlers[scope]
		if !ok {
			out[scope] = "尚未提供同步处理器"
			continue
		}
		if p != nil && !h.Supported(d, p) {
			out[scope] = "适配器未实现该能力"
		}
	}
	return out
}

// errScopeNoHandler 描述符声明了 scope 但无处理器/适配器缺能力——属"跳过"而非失败。
var errScopeNoHandler = errors.New("该同步类型暂无可用处理器")

// hasProductCatalog / hasPoolReader / hasInstanceAdmin 为能力判定的类型断言兜底，
// 供尚未登记描述符的适配器使用（新代码应优先由描述符判定）。
func hasProductCatalog(p upstream.Provider) bool {
	_, ok := p.(upstream.ProductCatalog)
	return ok
}

func hasPoolReader(p upstream.Provider) bool {
	_, ok := p.(upstream.PoolReader)
	return ok
}

func hasInstanceAdmin(p upstream.Provider) bool {
	_, ok := p.(upstream.InstanceAdministration)
	return ok
}

// scopeSkipReason 生成跳过原因文案（含 scope 名，便于日志与后台展示）。
func scopeSkipReason(scope string) string {
	return fmt.Sprintf("%s：%s", ScopeDisplayName(scope), errScopeNoHandler.Error())
}
