package sessionguard

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/pkg/cache"
)

// fakeStore 可编程的会话数据源，并记录调用次数（用于验证缓存是否真的生效）。
type fakeStore struct {
	active bool
	err    error
	calls  int
}

func (f *fakeStore) IsActive(context.Context, string) (bool, error) {
	f.calls++
	return f.active, f.err
}

// 未装配（store 为 nil）必须一律拒绝：会话校验是安全原语，
// 「校验不了」等于「不通过」，否则忘装配就成了全站绕过。
func TestGuard_NilStoreRejects(t *testing.T) {
	g := New(nil, cache.NewDisabled(zap.NewNop()), zap.NewNop())
	active, err := g.IsActive(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("nil store 不应返回错误: %v", err)
	}
	if active {
		t.Fatal("nil store 必须返回 false（校验不了 = 不通过）")
	}
}

// 空 session_id 直接拒绝，且不应打到底层存储。
func TestGuard_EmptySessionIDRejects(t *testing.T) {
	store := &fakeStore{active: true}
	g := New(store, cache.NewDisabled(zap.NewNop()), zap.NewNop())
	active, err := g.IsActive(context.Background(), "")
	if err != nil || active {
		t.Fatalf("空 session_id 必须返回 (false, nil)，得到 (%v, %v)", active, err)
	}
	if store.calls != 0 {
		t.Fatalf("空 session_id 不应查库，实际查了 %d 次", store.calls)
	}
}

// 第二次查询必须命中缓存：每请求查库会让 user_sessions 成为全站最热的表。
func TestGuard_CachesPositiveResult(t *testing.T) {
	store := &fakeStore{active: true}
	g := New(store, cache.NewDisabled(zap.NewNop()), zap.NewNop())
	for i := 0; i < 5; i++ {
		active, err := g.IsActive(context.Background(), "sess-1")
		if err != nil || !active {
			t.Fatalf("第 %d 次查询应命中有效会话，得到 (%v, %v)", i+1, active, err)
		}
	}
	if store.calls != 1 {
		t.Fatalf("5 次查询应只打库 1 次，实际 %d 次", store.calls)
	}
}

// 撤销后必须立即失效：这正是「强制下线」要修的东西。
// 没有显式失效的话，被踢的令牌还能再用最多 positiveTTL（5 分钟）。
func TestGuard_InvalidateTakesEffectImmediately(t *testing.T) {
	store := &fakeStore{active: true}
	g := New(store, cache.NewDisabled(zap.NewNop()), zap.NewNop())

	if active, _ := g.IsActive(context.Background(), "sess-1"); !active {
		t.Fatal("撤销前应判为有效")
	}
	// 撤销：库里改状态 + 显式失效缓存。
	store.active = false
	g.Invalidate(context.Background(), "sess-1")

	active, err := g.IsActive(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("查询出错: %v", err)
	}
	if active {
		t.Fatal("撤销并失效缓存后必须立即判为无效（强制下线的核心）")
	}
	if store.calls != 2 {
		t.Fatalf("失效后应重新查库，实际查库 %d 次", store.calls)
	}
}

// 查库出错时返回错误（而不是把 false 缓存下来）：
// 库抖动时缓存 "0" 会让刚登录的用户被拒一分钟。
func TestGuard_StoreErrorNotCached(t *testing.T) {
	store := &fakeStore{err: errors.New("db down")}
	g := New(store, cache.NewDisabled(zap.NewNop()), zap.NewNop())

	if _, err := g.IsActive(context.Background(), "sess-1"); err == nil {
		t.Fatal("查库失败必须把错误透出（中间件据此告警）")
	}
	// 库恢复后必须立刻能拿到正确结论。
	store.err = nil
	store.active = true
	active, err := g.IsActive(context.Background(), "sess-1")
	if err != nil || !active {
		t.Fatalf("库恢复后应判为有效，得到 (%v, %v)", active, err)
	}
}

// Invalidate 对空 id / nil 守卫必须安全（撤销路径会顺手调用，不该 panic）。
func TestGuard_InvalidateIsSafe(t *testing.T) {
	g := New(&fakeStore{}, cache.NewDisabled(zap.NewNop()), zap.NewNop())
	g.Invalidate(context.Background(), "", "")
	g.Invalidate(context.Background())

	var nilGuard *Guard
	nilGuard.Invalidate(context.Background(), "sess-1")
	if active, err := nilGuard.IsActive(context.Background(), "sess-1"); active || err != nil {
		t.Fatalf("nil 守卫必须返回 (false, nil)，得到 (%v, %v)", active, err)
	}
}

// 缓存里被写入非预期内容时按未命中处理，不能当成有效会话放行。
func TestGuard_UnknownCacheValueTreatedAsMiss(t *testing.T) {
	c := cache.NewDisabled(zap.NewNop())
	store := &fakeStore{active: false}
	g := New(store, c, zap.NewNop())

	if err := c.Set(context.Background(), keyPrefix+"sess-1", "yes", time.Minute); err != nil {
		t.Fatalf("预置缓存失败: %v", err)
	}
	active, err := g.IsActive(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("查询出错: %v", err)
	}
	if active {
		t.Fatal("无法识别的缓存值必须当未命中，不能放行")
	}
	if store.calls != 1 {
		t.Fatalf("未命中应查库一次，实际 %d 次", store.calls)
	}
}
