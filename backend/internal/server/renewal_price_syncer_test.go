package server

import (
	"context"
	"testing"

	instancerepo "hostsent/backend/internal/modules/admin/instance/repository"
	pricingdto "hostsent/backend/internal/modules/admin/product/pricing/dto"
)

// fakeInstanceRepoForRenewal 只实现续费价同步用到的方法，其余走嵌入接口的零值实现。
type fakeInstanceRepoForRenewal struct {
	instancerepo.InstanceRepository // 嵌入接口：只覆写本用例需要的方法
	bound                           []instancerepo.BoundRenewalInstance
	setCalls                        []renewalSetCall
	bulkClearCalls                  int
}

type renewalSetCall struct {
	id    uint64
	price float64
}

func (f *fakeInstanceRepoForRenewal) ListBoundRenewalByProduct(context.Context, uint64) ([]instancerepo.BoundRenewalInstance, error) {
	return f.bound, nil
}

func (f *fakeInstanceRepoForRenewal) SetRenewalPrice(_ context.Context, id uint64, price *float64) error {
	call := renewalSetCall{id: id}
	if price != nil {
		call.price = *price
	}
	f.setCalls = append(f.setCalls, call)
	return nil
}

func (f *fakeInstanceRepoForRenewal) SetRenewalPriceByProduct(context.Context, uint64, *float64, bool) (int64, error) {
	f.bulkClearCalls++
	return int64(len(f.bound)), nil
}

// fakeMatrixForRenewal 只提供 Matrix（取周期价用）。
type fakeMatrixForRenewal struct {
	items []pricingdto.PriceMatrixItem
}

func (f *fakeMatrixForRenewal) Matrix(context.Context, pricingdto.PriceMatrixQuery) (*pricingdto.PriceMatrixResponse, error) {
	return &pricingdto.PriceMatrixResponse{Items: f.items}, nil
}
func (f *fakeMatrixForRenewal) SaveMatrix(context.Context, pricingdto.PriceMatrixSaveRequest) (*pricingdto.PriceMatrixResponse, error) {
	return nil, nil
}
func (f *fakeMatrixForRenewal) CycleBasePrice(context.Context, uint64, uint64, string) (float64, bool, error) {
	return 0, false, nil
}
func (f *fakeMatrixForRenewal) ApplyUpstreamPrices(context.Context, uint64, []pricingdto.CyclePriceRow) error {
	return nil
}
func (f *fakeMatrixForRenewal) EnabledCycles(context.Context, uint64) ([]string, error) {
	return nil, nil
}
func (f *fakeMatrixForRenewal) SellableCycles(context.Context, uint64) ([]string, map[uint64][]string, error) {
	return nil, nil, nil
}

func fp(v float64) *float64 { return &v }

// TestSyncBoundRenewalPricesPicksPriceByInstanceCycle 按各实例自己的付款周期取价。
//
// 关键：绑定价是"每周期单价"，实例的周期各不相同（月付/年付混在一起）。
// 若给所有实例写同一个价，年付实例会按"月价"续费，平台按年付收一次却只收到月价。
func TestSyncBoundRenewalPricesPicksPriceByInstanceCycle(t *testing.T) {
	repo := &fakeInstanceRepoForRenewal{bound: []instancerepo.BoundRenewalInstance{
		{ID: 1, BillingMode: "monthly", RenewalPrice: fp(30)},
		{ID: 2, BillingMode: "annually", RenewalPrice: fp(300)},
		{ID: 3, BillingMode: "quarterly", RenewalPrice: fp(90)},
	}}
	matrix := &fakeMatrixForRenewal{items: []pricingdto.PriceMatrixItem{
		{Cycle: "monthly", Price: 35, Status: 1},
		{Cycle: "quarterly", Price: 100, Status: 1},
		{Cycle: "annually", Price: 360, Status: 1},
	}}
	adapter := &renewalPriceSyncerAdapter{instances: repo, matrix: matrix}

	n, err := adapter.SyncBoundRenewalPrices(context.Background(), 176, false)
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}
	if n != 3 {
		t.Fatalf("应更新 3 台，got %d", n)
	}
	want := map[uint64]float64{1: 35, 2: 360, 3: 100}
	for _, c := range repo.setCalls {
		if want[c.id] != c.price {
			t.Fatalf("实例 %d 应取本周期价 %.2f，got %.2f", c.id, want[c.id], c.price)
		}
	}
}

// TestSyncBoundRenewalPricesSkipsUnchanged 已是目标价的不写库（也计入"无需改动"）。
func TestSyncBoundRenewalPricesSkipsUnchanged(t *testing.T) {
	repo := &fakeInstanceRepoForRenewal{bound: []instancerepo.BoundRenewalInstance{
		{ID: 1, BillingMode: "monthly", RenewalPrice: fp(35)}, // 已是目标价
		{ID: 2, BillingMode: "monthly", RenewalPrice: fp(30)}, // 需更新
	}}
	matrix := &fakeMatrixForRenewal{items: []pricingdto.PriceMatrixItem{
		{Cycle: "monthly", Price: 35, Status: 1},
	}}
	adapter := &renewalPriceSyncerAdapter{instances: repo, matrix: matrix}

	n, err := adapter.SyncBoundRenewalPrices(context.Background(), 176, false)
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("只应更新真正变化的 1 台，got %d", n)
	}
	if len(repo.setCalls) != 1 || repo.setCalls[0].id != 2 {
		t.Fatalf("只应写实例 2，got %+v", repo.setCalls)
	}
}

// TestSyncBoundRenewalPricesSkipsCycleWithoutPrice 该周期无可售价时保留原绑定值。
//
// 清零会让续费直接算不出钱（续费守卫拒绝 0 元单），比"没同步"更糟。
func TestSyncBoundRenewalPricesSkipsCycleWithoutPrice(t *testing.T) {
	repo := &fakeInstanceRepoForRenewal{bound: []instancerepo.BoundRenewalInstance{
		{ID: 1, BillingMode: "annually", RenewalPrice: fp(300)},
	}}
	matrix := &fakeMatrixForRenewal{items: []pricingdto.PriceMatrixItem{
		{Cycle: "monthly", Price: 35, Status: 1}, // 没有 annually 档
	}}
	adapter := &renewalPriceSyncerAdapter{instances: repo, matrix: matrix}

	n, err := adapter.SyncBoundRenewalPrices(context.Background(), 176, false)
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}
	if n != 0 || len(repo.setCalls) != 0 {
		t.Fatalf("取不到该档价时不应改任何实例，got n=%d calls=%+v", n, repo.setCalls)
	}
}

// TestSyncBoundRenewalPricesIgnoresDisabledCycle 停用档位不算可售（不能拿它当续费价）。
func TestSyncBoundRenewalPricesIgnoresDisabledCycle(t *testing.T) {
	repo := &fakeInstanceRepoForRenewal{bound: []instancerepo.BoundRenewalInstance{
		{ID: 1, BillingMode: "monthly", RenewalPrice: fp(30)},
	}}
	matrix := &fakeMatrixForRenewal{items: []pricingdto.PriceMatrixItem{
		{Cycle: "monthly", Price: 35, Status: 0}, // 停用
	}}
	adapter := &renewalPriceSyncerAdapter{instances: repo, matrix: matrix}

	n, err := adapter.SyncBoundRenewalPrices(context.Background(), 176, false)
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("停用档位不应参与同步，got %d", n)
	}
}

// TestSyncBoundRenewalPricesDryRunCountsWithoutWrite 试算只数不写。
func TestSyncBoundRenewalPricesDryRunCountsWithoutWrite(t *testing.T) {
	repo := &fakeInstanceRepoForRenewal{bound: []instancerepo.BoundRenewalInstance{
		{ID: 1, BillingMode: "monthly", RenewalPrice: fp(30)},
		{ID: 2, BillingMode: "monthly", RenewalPrice: fp(30)},
	}}
	matrix := &fakeMatrixForRenewal{items: []pricingdto.PriceMatrixItem{
		{Cycle: "monthly", Price: 35, Status: 1},
	}}
	adapter := &renewalPriceSyncerAdapter{instances: repo, matrix: matrix}

	n, err := adapter.SyncBoundRenewalPrices(context.Background(), 176, true)
	if err != nil {
		t.Fatalf("试算失败: %v", err)
	}
	if n != 2 {
		t.Fatalf("试算应数到 2 台，got %d", n)
	}
	if len(repo.setCalls) != 0 {
		t.Fatalf("试算不应写库，got %+v", repo.setCalls)
	}
}

// TestClearBoundRenewalPrices 清除绑定走批量清空，试算不写。
func TestClearBoundRenewalPrices(t *testing.T) {
	repo := &fakeInstanceRepoForRenewal{bound: []instancerepo.BoundRenewalInstance{
		{ID: 1, BillingMode: "monthly", RenewalPrice: fp(30)},
		{ID: 2, BillingMode: "monthly", RenewalPrice: fp(30)},
	}}
	adapter := &renewalPriceSyncerAdapter{instances: repo, matrix: &fakeMatrixForRenewal{}}

	if n, err := adapter.ClearBoundRenewalPrices(context.Background(), 176, true); err != nil || n != 2 {
		t.Fatalf("试算应返回 2 台，got n=%d err=%v", n, err)
	}
	if repo.bulkClearCalls != 0 {
		t.Fatal("试算不应执行清空")
	}
	if n, err := adapter.ClearBoundRenewalPrices(context.Background(), 176, false); err != nil || n != 2 {
		t.Fatalf("执行应返回 2 台，got n=%d err=%v", n, err)
	}
	if repo.bulkClearCalls != 1 {
		t.Fatalf("应执行一次清空，got %d", repo.bulkClearCalls)
	}
}
