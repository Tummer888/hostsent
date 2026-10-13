package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"hostsent/backend/internal/modules/uc/instance/dto"
	"hostsent/backend/internal/modules/uc/instance/model"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// slowControlProvider 可控延迟的 InstanceControl 桩：
// 记录并发峰值与调用次数，用于验证列表回源是并发执行且受总预算约束。
type slowControlProvider struct {
	delay    time.Duration
	calls    int32
	inFlight int32
	maxSeen  int32
	// statuses 按实例号返回状态，可指定某些实例失败。
	statuses map[string]pkgmodel.StandardInstanceStatus
	failOn   map[string]bool
}

func (p *slowControlProvider) GetType() string                   { return "fake" }
func (p *slowControlProvider) GetName() string                   { return "fake" }
func (p *slowControlProvider) HealthCheck(context.Context) error { return nil }
func (p *slowControlProvider) Capabilities() upstream.CapabilityDescriptor {
	return upstream.CapabilityDescriptor{}
}

func (p *slowControlProvider) GetInstance(ctx context.Context, instanceID string) (*pkgmodel.StandardInstance, error) {
	cur := atomic.AddInt32(&p.inFlight, 1)
	defer atomic.AddInt32(&p.inFlight, -1)
	for {
		old := atomic.LoadInt32(&p.maxSeen)
		if cur <= old || atomic.CompareAndSwapInt32(&p.maxSeen, old, cur) {
			break
		}
	}
	atomic.AddInt32(&p.calls, 1)

	select {
	case <-time.After(p.delay):
	case <-ctx.Done():
		// 预算到期：模拟上游请求被取消，调用方应降级为本地状态。
		return nil, ctx.Err()
	}
	if p.failOn != nil && p.failOn[instanceID] {
		return nil, context.DeadlineExceeded
	}
	st := pkgmodel.InstanceStatusRunning
	if p.statuses != nil {
		if v, ok := p.statuses[instanceID]; ok {
			st = v
		}
	}
	return &pkgmodel.StandardInstance{UpstreamID: instanceID, Status: st}, nil
}

func (p *slowControlProvider) StartInstance(context.Context, string) error      { return nil }
func (p *slowControlProvider) StopInstance(context.Context, string, bool) error { return nil }
func (p *slowControlProvider) RestartInstance(context.Context, string) error    { return nil }
func (p *slowControlProvider) VNC(context.Context, string) (upstream.VNCResult, error) {
	return upstream.VNCResult{}, nil
}

// buildLiveItems 造 n 台"已开通"实例（provider_instance_id 非空才会回源）。
func buildLiveItems(n int) []model.Instance {
	items := make([]model.Instance, 0, n)
	for i := 1; i <= n; i++ {
		items = append(items, model.Instance{
			ID:                 uint64(i),
			InstanceID:         "i-test-" + string(rune('a'+i%26)),
			ProviderID:         7,
			ProviderInstanceID: "up-" + string(rune('a'+i%26)),
			UserID:             2,
			Name:               "vm",
			Status:             "stopped",
		})
	}
	return items
}

// TestListLiveRefreshIsConcurrent 实时刷新必须并发：串行回源是用户实测
// 「列表转很久、最后报错空白」的直接原因（13 台 × 约 2.7s ≈ 30s+ 顶穿网关）。
func TestListLiveRefreshIsConcurrent(t *testing.T) {
	const n = 8
	prov := &slowControlProvider{delay: 50 * time.Millisecond}
	svc := &instanceService{
		repo:    &fakeRepo{items: buildLiveItems(n)},
		resolve: func(context.Context, uint64) (upstream.Provider, error) { return prov, nil },
	}

	start := time.Now()
	resp, err := svc.List(context.Background(), 2, dto.ListQuery{Live: true})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Items) != n {
		t.Fatalf("应返回 %d 台，实际 %d", n, len(resp.Items))
	}
	// 串行需要 n*delay=400ms；并发（度 8）应接近一个 delay。
	if elapsed > 4*prov.delay {
		t.Errorf("实时刷新疑似串行：%d 台耗时 %v（单台 %v）", n, elapsed, prov.delay)
	}
	if got := atomic.LoadInt32(&prov.maxSeen); got < 2 {
		t.Errorf("未观察到并发（峰值 %d），回源仍是串行", got)
	}
	if got := atomic.LoadInt32(&prov.calls); got != n {
		t.Errorf("应回源 %d 次，实际 %d", n, got)
	}
}

// TestListLiveRefreshRespectsBudget 上游 hang 住时要被总预算截断，
// 不能把整个列表请求拖到网关超时（那正是用户看到的"报错五百"）。
func TestListLiveRefreshRespectsBudget(t *testing.T) {
	prov := &slowControlProvider{delay: 30 * time.Second}
	svc := &instanceService{
		repo:    &fakeRepo{items: buildLiveItems(3)},
		resolve: func(context.Context, uint64) (upstream.Provider, error) { return prov, nil },
	}

	ctx, cancel := context.WithTimeout(context.Background(), refreshBudget+5*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := svc.List(ctx, 2, dto.ListQuery{Live: true})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("List 应降级返回而非报错，实际: %v", err)
	}
	if elapsed > refreshBudget+3*time.Second {
		t.Errorf("未受总预算约束：耗时 %v，预算 %v", elapsed, refreshBudget)
	}
	// 全部超时：仍要返回完整列表，状态回落本地值（stopped）。
	if len(resp.Items) != 3 {
		t.Fatalf("超预算时仍应返回全部实例，实际 %d", len(resp.Items))
	}
	for _, it := range resp.Items {
		if it.Status != "stopped" {
			t.Errorf("未取到实时状态时应回落本地状态 stopped，实际 %q", it.Status)
		}
	}
}

// TestListLiveRefreshKeepsOtherRowsOnSingleFailure 单台回源失败不影响其余实例：
// 不能因为一台机器上游报错就让整页数据为空。
func TestListLiveRefreshKeepsOtherRowsOnSingleFailure(t *testing.T) {
	items := buildLiveItems(3)
	prov := &slowControlProvider{
		delay: 10 * time.Millisecond,
		// 让第一台失败。注意 refresh 传给上游的是 InstanceID（不是 ProviderInstanceID）。
		failOn: map[string]bool{items[0].InstanceID: true},
	}
	svc := &instanceService{
		repo:    &fakeRepo{items: items},
		resolve: func(context.Context, uint64) (upstream.Provider, error) { return prov, nil },
	}
	resp, err := svc.List(context.Background(), 2, dto.ListQuery{Live: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Items) != 3 {
		t.Fatalf("单台失败不应丢行，实际 %d 行", len(resp.Items))
	}
	if resp.Items[0].Status != "stopped" {
		t.Errorf("失败那台应回落本地状态，实际 %q", resp.Items[0].Status)
	}
	if resp.Items[1].Status != "running" {
		t.Errorf("成功那台应显示上游状态，实际 %q", resp.Items[1].Status)
	}
}

// TestListWithoutLiveSkipsUpstream 不带 live 时不得回源（首屏路径）：
// 这是用户端进页面时的默认请求，必须只读本地。
func TestListWithoutLiveSkipsUpstream(t *testing.T) {
	prov := &slowControlProvider{delay: time.Second}
	svc := &instanceService{
		repo:    &fakeRepo{items: buildLiveItems(3)},
		resolve: func(context.Context, uint64) (upstream.Provider, error) { return prov, nil },
	}
	start := time.Now()
	resp, err := svc.List(context.Background(), 2, dto.ListQuery{Live: false})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Errorf("不带 live 仍去回源了：耗时 %v", elapsed)
	}
	if got := atomic.LoadInt32(&prov.calls); got != 0 {
		t.Errorf("不带 live 不应调用上游，实际调用 %d 次", got)
	}
	if len(resp.Items) != 3 {
		t.Fatalf("应返回全部实例，实际 %d", len(resp.Items))
	}
}

// TestListSkipsPendingPlaceholderRefresh 待开通占位没有上游实体，不应回源。
func TestListSkipsPendingPlaceholderRefresh(t *testing.T) {
	items := buildLiveItems(2)
	items[0].ProviderInstanceID = "" // 占位行
	prov := &slowControlProvider{delay: 10 * time.Millisecond}
	svc := &instanceService{
		repo:    &fakeRepo{items: items},
		resolve: func(context.Context, uint64) (upstream.Provider, error) { return prov, nil },
	}
	if _, err := svc.List(context.Background(), 2, dto.ListQuery{Live: true}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := atomic.LoadInt32(&prov.calls); got != 1 {
		t.Errorf("应只回源已开通的 1 台，实际 %d 次", got)
	}
}

// TestRefreshStatusesConcurrencyBound 并发度不超过上限（避免对上游形成突发压测）。
func TestRefreshStatusesConcurrencyBound(t *testing.T) {
	const n = 40
	prov := &slowControlProvider{delay: 20 * time.Millisecond}
	svc := &instanceService{resolve: func(context.Context, uint64) (upstream.Provider, error) { return prov, nil }}
	var mu sync.Mutex
	_ = mu

	targets := buildLiveItems(n)
	var wg sync.WaitGroup
	_ = wg
	out := svc.refreshStatuses(context.Background(), targets, 4, 5*time.Second)
	if len(out) != n {
		t.Errorf("应刷新 %d 台，实际 %d", n, len(out))
	}
	if got := atomic.LoadInt32(&prov.maxSeen); got > 4 {
		t.Errorf("并发峰值 %d 超过上限 4", got)
	}
}
