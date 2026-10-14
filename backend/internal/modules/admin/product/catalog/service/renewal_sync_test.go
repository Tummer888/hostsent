package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"hostsent/backend/internal/modules/admin/product/catalog/model"
)

// fakeRenewalSyncer 记录调用参数，供同步续费价的服务层用例断言。
type fakeRenewalSyncer struct {
	syncCalls  int
	clearCalls int
	lastDryRun bool
	affected   int
	err        error
}

func (f *fakeRenewalSyncer) SyncBoundRenewalPrices(_ context.Context, _ uint64, dryRun bool) (int, error) {
	f.syncCalls++
	f.lastDryRun = dryRun
	return f.affected, f.err
}

func (f *fakeRenewalSyncer) ClearBoundRenewalPrices(_ context.Context, _ uint64, dryRun bool) (int, error) {
	f.clearCalls++
	f.lastDryRun = dryRun
	return f.affected, f.err
}

// TestSyncRenewalPricesRoutesByClear 「重设绑定」与「清除绑定」必须走各自分支。
func TestSyncRenewalPricesRoutesByClear(t *testing.T) {
	t.Run("重设绑定", func(t *testing.T) {
		repo := &fakeProductRepo{items: []model.Product{upstreamProduct()}}
		syncer := &fakeRenewalSyncer{affected: 3}
		svc := &productService{repo: repo, renewalSyncer: syncer}

		res, err := svc.SyncRenewalPrices(context.Background(), 176, false, false, 1, "admin")
		if err != nil {
			t.Fatalf("同步失败: %v", err)
		}
		if syncer.syncCalls != 1 || syncer.clearCalls != 0 {
			t.Fatalf("应走重设分支，got sync=%d clear=%d", syncer.syncCalls, syncer.clearCalls)
		}
		if res.Affected != 3 || res.Cleared {
			t.Fatalf("结果应反映 3 台且非清除，got %+v", res)
		}
		if !strings.Contains(res.Message, "3") {
			t.Fatalf("说明应含台数，got %q", res.Message)
		}
		if len(repo.history) != 1 {
			t.Fatalf("应写一条变更历史，got %d", len(repo.history))
		}
	})

	t.Run("清除绑定", func(t *testing.T) {
		repo := &fakeProductRepo{items: []model.Product{upstreamProduct()}}
		syncer := &fakeRenewalSyncer{affected: 2}
		svc := &productService{repo: repo, renewalSyncer: syncer}

		res, err := svc.SyncRenewalPrices(context.Background(), 176, true, false, 1, "admin")
		if err != nil {
			t.Fatalf("清除失败: %v", err)
		}
		if syncer.clearCalls != 1 || syncer.syncCalls != 0 {
			t.Fatalf("应走清除分支，got sync=%d clear=%d", syncer.syncCalls, syncer.clearCalls)
		}
		if !res.Cleared {
			t.Fatalf("结果应标记为清除，got %+v", res)
		}
	})
}

// TestSyncRenewalPricesDryRunWritesNoHistory 试算不得留痕。
//
// dry_run 的语义是"先看会影响几台"，如果它也写商品变更历史，
// 运营试算几次后历史里就会出现一串"已改价"，无法分辨哪次是真的执行了。
func TestSyncRenewalPricesDryRunWritesNoHistory(t *testing.T) {
	repo := &fakeProductRepo{items: []model.Product{upstreamProduct()}}
	syncer := &fakeRenewalSyncer{affected: 4}
	svc := &productService{repo: repo, renewalSyncer: syncer}

	res, err := svc.SyncRenewalPrices(context.Background(), 176, false, true, 1, "admin")
	if err != nil {
		t.Fatalf("试算失败: %v", err)
	}
	if !res.DryRun || res.Affected != 4 {
		t.Fatalf("试算结果应标记 dry_run 且带台数，got %+v", res)
	}
	if !syncer.lastDryRun {
		t.Fatal("应把 dry_run 透传给实现")
	}
	if len(repo.history) != 0 {
		t.Fatalf("试算不应写历史，got %d 条", len(repo.history))
	}
	if !strings.Contains(res.Message, "将") {
		t.Fatalf("试算文案应为将来时，got %q", res.Message)
	}
}

// TestSyncRenewalPricesWithoutSyncerFailsLoudly 端口未装配必须显式报错。
//
// 静默成功会让运营以为老机器已经按新价续费了 —— 比报错更危险。
func TestSyncRenewalPricesWithoutSyncerFailsLoudly(t *testing.T) {
	repo := &fakeProductRepo{items: []model.Product{upstreamProduct()}}
	svc := &productService{repo: repo}

	if _, err := svc.SyncRenewalPrices(context.Background(), 176, false, false, 1, "admin"); err == nil {
		t.Fatal("未装配同步能力时应报错")
	}
}

// TestSyncRenewalPricesPropagatesError 实现层报错要向上抛，不能吞掉当成功。
func TestSyncRenewalPricesPropagatesError(t *testing.T) {
	repo := &fakeProductRepo{items: []model.Product{upstreamProduct()}}
	syncer := &fakeRenewalSyncer{err: errors.New("db down")}
	svc := &productService{repo: repo, renewalSyncer: syncer}

	if _, err := svc.SyncRenewalPrices(context.Background(), 176, false, false, 1, "admin"); err == nil {
		t.Fatal("实现层错误应向上抛")
	}
	if len(repo.history) != 0 {
		t.Fatal("失败时不应写历史")
	}
}
