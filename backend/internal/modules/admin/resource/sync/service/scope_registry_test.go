package service

import (
	"context"
	"testing"

	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	"hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// normalizeProvider 测试用最小 Provider：实现 ProductCatalog（目录能力）但不实现
// PoolReader/InstanceAdministration，用于验证"描述符 ∩ handler ∩ 能力"的判定。
type normalizeProvider struct {
	typ string
}

func (p *normalizeProvider) GetType() string                   { return p.typ }
func (p *normalizeProvider) GetName() string                   { return p.typ }
func (p *normalizeProvider) HealthCheck(context.Context) error { return nil }
func (p *normalizeProvider) Capabilities() upstream.CapabilityDescriptor {
	return upstream.CapabilitiesOf(p)
}
func (p *normalizeProvider) ListProducts(context.Context) ([]*model.StandardProduct, error) {
	return nil, nil
}
func (p *normalizeProvider) GetProduct(context.Context, string) (*model.StandardProduct, error) {
	return nil, nil
}

// TestNormalizeScope 兼容旧三类任务类型，避免历史任务/旧前端调用失效。
func TestNormalizeScope(t *testing.T) {
	cases := map[string]string{
		"product":  syncmodel.ScopeCatalog,
		"pool":     syncmodel.ScopePool,
		"instance": syncmodel.ScopeInstance,
		"catalog":  syncmodel.ScopeCatalog,
		"price":    syncmodel.ScopePrice,
		"region":   syncmodel.ScopeRegion,
		"whatever": "whatever", // 未知值原样返回，由调用方判定无 handler
	}
	for in, want := range cases {
		if got := syncmodel.NormalizeScope(in); got != want {
			t.Errorf("NormalizeScope(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestAllScopesRegistered 每个受支持 scope 都应有处理器，否则调度会误判为跳过。
func TestAllScopesRegistered(t *testing.T) {
	for _, scope := range syncmodel.AllScopes() {
		if _, ok := GetScopeHandler(scope); !ok {
			// image/stock 尚未接入实现，属预期缺口（描述符声明时会被显式跳过）。
			if scope == syncmodel.ScopeImage || scope == syncmodel.ScopeStock {
				continue
			}
			t.Errorf("scope %q 无注册处理器", scope)
		}
	}
}

// TestPlanScopesDrivenByDescriptor 验证「能力描述符 ∩ 已注册 handler」决定可调度范围。
func TestPlanScopesDrivenByDescriptor(t *testing.T) {
	// 财务渠道：只声明 catalog/price/instance，且无 instance 能力。
	finance := &normalizeProvider{typ: "mofangfinance"}
	desc := upstream.CapabilityDescriptor{
		SyncScopes: []string{upstream.ScopeCatalog, upstream.ScopePrice, upstream.ScopeInstance},
	}
	got := PlanScopes(desc, finance)
	want := []string{syncmodel.ScopeCatalog, syncmodel.ScopePrice}
	if len(got) != len(want) {
		t.Fatalf("PlanScopes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("PlanScopes = %v, want %v", got, want)
		}
	}

	// 未注册实现的 scope（stock）即便描述符声明也不应进入可调度集合。
	desc2 := upstream.CapabilityDescriptor{SyncScopes: []string{upstream.ScopeStock}}
	if got := PlanScopes(desc2, finance); len(got) != 0 {
		t.Fatalf("PlanScopes(stock) = %v, want empty", got)
	}
}

// TestPlanScopesFallsBackToTypeAssertion 描述符缺失 scope 声明时，按类型断言兜底判定。
func TestPlanScopesFallsBackToTypeAssertion(t *testing.T) {
	// normalizeProvider 不实现任何能力接口，描述符为空 → 无可用 scope。
	if got := PlanScopes(upstream.CapabilityDescriptor{}, &normalizeProvider{typ: "x"}); len(got) != 0 {
		t.Fatalf("PlanScopes(no caps) = %v, want empty", got)
	}
}

// TestUnsupportedScopes 声明了但无法执行的 scope 应带原因返回，供后台提示。
func TestUnsupportedScopes(t *testing.T) {
	desc := upstream.CapabilityDescriptor{
		SyncScopes: []string{upstream.ScopeCatalog, upstream.ScopeStock, upstream.ScopeInstance},
	}
	got := UnsupportedScopes(desc, &normalizeProvider{typ: "mofangfinance"})
	if _, ok := got[syncmodel.ScopeStock]; !ok {
		t.Errorf("stock 应报告为不可执行（无处理器），got %v", got)
	}
	if _, ok := got[syncmodel.ScopeCatalog]; ok {
		t.Errorf("catalog 不应出现在不可执行集合，got %v", got)
	}
	if _, ok := got[syncmodel.ScopeInstance]; !ok {
		t.Errorf("instance 应报告为不可执行（适配器缺能力），got %v", got)
	}
}
