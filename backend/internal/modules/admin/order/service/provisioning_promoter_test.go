package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/order/model"
)

// TestProvisioningPromoterPromotesOnlyRunning 锁定推进器的核心口径：
//
// 财务型上游（zjmf）创建主机是异步的 —— 支付成功时 host/header 还是 Pending，
// 而该上游没有实例同步 scope，没有任何后台任务会再回来看一眼。没有这个推进器，
// 订单与实例会永远停在 provisioning/creating（用户实测）。
//
// 判据（本次钉死）：**只有回源确认主机 running 才推进**；仍在创建、
// 刷新失败、无实例（空状态）一律保持原状 —— 推进器绝不能制造额外上游副作用，
// 也不能因为一次网络抖动就把未就绪的订单标成服务中。
func TestProvisioningPromoterPromotesOnlyRunning(t *testing.T) {
	cases := []struct {
		name          string
		refreshStatus string
		refreshErr    error
		wantActivated bool
	}{
		{"主机已 running → 推进", "running", nil, true},
		{"主机仍在 creating → 保持", "creating", nil, false},
		{"主机已停止 → 保持（不擅自激活已停机实例）", "stopped", nil, false},
		{"无实例（非云主机订单）→ 跳过", "", nil, false},
		{"回源失败（上游抖动）→ 跳过，不推进", "", errors.New("upstream unreachable"), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var activated []uint64
			p := NewProvisioningPromoter(
				stubStaleStore{orders: []model.Order{{ID: 843, Status: model.OrderStatusProvisioning}}},
				stubRefresher{status: c.refreshStatus, err: c.refreshErr},
				stubActivator{record: &activated},
				zap.NewNop(),
				ProvisioningPromoterOptions{StaleAfter: time.Millisecond, Batch: 10},
			)
			p.runOnce(context.Background())

			if c.wantActivated {
				if len(activated) != 1 || activated[0] != 843 {
					t.Fatalf("应推进订单 843 到服务中，实际 activated=%v", activated)
				}
			} else if len(activated) != 0 {
				t.Fatalf("不得推进（状态=%q err=%v），实际 activated=%v", c.refreshStatus, c.refreshErr, activated)
			}
		})
	}
}

// TestProvisioningPromoterIsolatesFailures 单笔失败不影响其余订单推进。
func TestProvisioningPromoterIsolatesFailures(t *testing.T) {
	var activated []uint64
	p := NewProvisioningPromoter(
		stubStaleStore{orders: []model.Order{{ID: 1}, {ID: 2}, {ID: 3}}},
		failingThenOKRefresher{failFor: 2},
		stubActivator{record: &activated},
		zap.NewNop(),
		ProvisioningPromoterOptions{StaleAfter: time.Millisecond, Batch: 10},
	)
	p.runOnce(context.Background())
	if len(activated) != 2 {
		t.Fatalf("应推进 2 单（中间一单失败不影响其余），实际 activated=%v", activated)
	}
}

// TestProvisioningPromoterBacksOffStuckOrders 无进展的订单必须退避，不每轮重查上游。
//
// 实测：某实例卡在 creating 30 分钟，推进器每分钟回源一次，在实例流水里堆出
// 32 条重复失败 + 35 条重复成功记录，同时持续打上游接口。退避把重复查询压到
// 指数级（5m → 10m → 20m … 上限 1h），但仍会周期性复查（不能永久放弃）。
func TestProvisioningPromoterBacksOffStuckOrders(t *testing.T) {
	refresher := &countingRefresher{status: "creating"}
	p := NewProvisioningPromoter(
		stubStaleStore{orders: []model.Order{{ID: 42, Status: model.OrderStatusProvisioning}}},
		refresher,
		stubActivator{record: &[]uint64{}},
		zap.NewNop(),
		ProvisioningPromoterOptions{
			StaleAfter: time.Millisecond, Batch: 10,
			BackoffBase: 50 * time.Millisecond, BackoffMax: 200 * time.Millisecond,
		},
	)
	// 第一次：正常回源一次。
	p.runOnce(context.Background())
	if refresher.calls != 1 {
		t.Fatalf("首轮应回源 1 次，实际 %d", refresher.calls)
	}
	// 紧接着第二轮：处于退避窗口内，不得再查上游。
	p.runOnce(context.Background())
	if refresher.calls != 1 {
		t.Fatalf("退避窗口内不应再回源，实际 %d 次", refresher.calls)
	}
	// 等过退避窗口：应恢复复查（卡死订单不能被永久放弃）。
	time.Sleep(80 * time.Millisecond)
	p.runOnce(context.Background())
	if refresher.calls != 2 {
		t.Fatalf("退避到期后应恢复回源，实际 %d 次", refresher.calls)
	}
	// 退避上限生效：连读到上限时长。
	var last time.Duration
	for i := 0; i < 12; i++ {
		last = p.noteNoProgress(42)
	}
	if last > 200*time.Millisecond {
		t.Fatalf("退避应封顶在 BackoffMax，实际 %v", last)
	}
}

// TestProvisioningPromoterClearsBackoffOnPromote 推进成功后必须清除退避状态，
// 否则同一订单的历史退避会拖慢后续（如续费后再次滞留）的复查。
func TestProvisioningPromoterClearsBackoffOnPromote(t *testing.T) {
	refresher := &flipRefresher{status: "creating"}
	p := NewProvisioningPromoter(
		stubStaleStore{orders: []model.Order{{ID: 7, Status: model.OrderStatusProvisioning}}},
		refresher,
		stubActivator{record: &[]uint64{}},
		zap.NewNop(),
		ProvisioningPromoterOptions{
			StaleAfter: time.Millisecond, Batch: 10,
			BackoffBase: 10 * time.Second, // 长退避：若没被清除，第二轮必被跳过
		},
	)
	p.runOnce(context.Background()) // creating → 进入退避
	refresher.status = "running"
	// 手动清除退避窗口（模拟时间流逝），只验证"推进后状态被清除"。
	p.mu.Lock()
	p.backoff[7].nextAt = time.Now().Add(-time.Second)
	p.mu.Unlock()
	p.runOnce(context.Background()) // running → 推进并 forget
	refresher.status = "creating"
	p.runOnce(context.Background()) // 若 forget 生效，本轮应立刻回源并重新退避
	if refresher.calls != 3 {
		t.Fatalf("推进后应清除退避状态（应回源 3 次），实际 %d 次", refresher.calls)
	}
}

// ---------- 测试替身 ----------

type stubStaleStore struct{ orders []model.Order }

func (s stubStaleStore) ListProvisioningStale(context.Context, time.Time, int) ([]model.Order, error) {
	return s.orders, nil
}

type stubRefresher struct {
	status string
	err    error
}

func (s stubRefresher) RefreshByOrder(context.Context, uint64) (string, error) {
	return s.status, s.err
}

type failingThenOKRefresher struct{ failFor uint64 }

func (f failingThenOKRefresher) RefreshByOrder(_ context.Context, orderID uint64) (string, error) {
	if orderID == f.failFor {
		return "", errors.New("boom")
	}
	return "running", nil
}

type stubActivator struct{ record *[]uint64 }

func (s stubActivator) Activate(_ context.Context, id uint64) error {
	*s.record = append(*s.record, id)
	return nil
}

type countingRefresher struct {
	status string
	calls  int
}

func (c *countingRefresher) RefreshByOrder(context.Context, uint64) (string, error) {
	c.calls++
	return c.status, nil
}

type flipRefresher struct {
	status string
	calls  int
}

func (f *flipRefresher) RefreshByOrder(context.Context, uint64) (string, error) {
	f.calls++
	return f.status, nil
}
