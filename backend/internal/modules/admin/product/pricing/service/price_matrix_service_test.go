package service

import (
	"context"
	"testing"

	"hostsent/backend/internal/modules/admin/product/pricing/dto"
	"hostsent/backend/internal/modules/admin/product/pricing/model"
	"hostsent/backend/internal/modules/admin/product/pricing/repository"
)

// fakeRepo 内存版周期价格仓储，覆盖矩阵服务的业务规则测试。
type fakeRepo struct {
	rows []model.ProductPrice
}

func (f *fakeRepo) List(_ context.Context, productID, specID uint64, currency string) ([]model.ProductPrice, error) {
	out := make([]model.ProductPrice, 0)
	for _, r := range f.rows {
		if r.ProductID != productID || r.ProductSpecID != specID {
			continue
		}
		if currency != "" && r.Currency != currency {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeRepo) FindActive(_ context.Context, productID, specID uint64, cycle, currency string) (*model.ProductPrice, error) {
	for i, r := range f.rows {
		if r.ProductID == productID && r.ProductSpecID == specID &&
			r.Cycle == cycle && r.Currency == currency && r.Status == repository.PriceStatusEnabled {
			row := f.rows[i]
			return &row, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) ListByProductIDs(context.Context, []uint64, string) ([]model.ProductPrice, error) {
	return f.rows, nil
}

func (f *fakeRepo) Upsert(_ context.Context, item *model.ProductPrice) error {
	for i, r := range f.rows {
		if r.ProductID == item.ProductID && r.ProductSpecID == item.ProductSpecID &&
			r.Cycle == item.Cycle && r.Currency == item.Currency {
			f.rows[i] = *item
			return nil
		}
	}
	f.rows = append(f.rows, *item)
	return nil
}

func (f *fakeRepo) DeleteMissing(_ context.Context, productID, specID uint64, currency string, keep []string) (int64, error) {
	keepSet := map[string]struct{}{}
	for _, c := range keep {
		keepSet[c] = struct{}{}
	}
	kept := make([]model.ProductPrice, 0, len(f.rows))
	var removed int64
	for _, r := range f.rows {
		if r.ProductID == productID && r.ProductSpecID == specID && r.Currency == currency {
			if _, ok := keepSet[r.Cycle]; !ok {
				removed++
				continue
			}
		}
		kept = append(kept, r)
	}
	f.rows = kept
	return removed, nil
}

func (f *fakeRepo) DeleteByProduct(_ context.Context, productID uint64) error {
	kept := make([]model.ProductPrice, 0, len(f.rows))
	for _, r := range f.rows {
		if r.ProductID != productID {
			kept = append(kept, r)
		}
	}
	f.rows = kept
	return nil
}

type fixedMeta struct{ meta *MatrixProductMeta }

func (f fixedMeta) ProductMeta(context.Context, uint64) (*MatrixProductMeta, error) {
	return f.meta, nil
}

type fixedChannels struct{ cycles []string }

func (f fixedChannels) BillingCycles(context.Context, uint64) ([]string, error) {
	return f.cycles, nil
}

// TestCycleBasePricePrefersSpecRow SKU 自有周期行优先于商品级行。
func TestCycleBasePricePrefersSpecRow(t *testing.T) {
	repo := &fakeRepo{rows: []model.ProductPrice{
		{ProductID: 7, ProductSpecID: 0, Cycle: "monthly", Currency: "CNY", Price: 10, Status: 1},
		{ProductID: 7, ProductSpecID: 3, Cycle: "monthly", Currency: "CNY", Price: 12, Status: 1},
	}}
	svc := NewPriceMatrixService(repo, nil, nil)

	price, hasMatrix, err := svc.CycleBasePrice(context.Background(), 7, 3, "monthly")
	if err != nil || !hasMatrix || price != 12 {
		t.Fatalf("SKU 自有行应优先，got price=%v hasMatrix=%v err=%v", price, hasMatrix, err)
	}
	// 无自有行的 SKU 回落商品级行。
	price, hasMatrix, err = svc.CycleBasePrice(context.Background(), 7, 4, "monthly")
	if err != nil || !hasMatrix || price != 10 {
		t.Fatalf("无自有行应回落商品级，got price=%v hasMatrix=%v err=%v", price, hasMatrix, err)
	}
}

// TestCycleBasePriceNoMatrix 无任何矩阵行时 hasMatrix=false，调用方回落旧单价。
func TestCycleBasePriceNoMatrix(t *testing.T) {
	svc := NewPriceMatrixService(&fakeRepo{}, nil, nil)
	_, hasMatrix, err := svc.CycleBasePrice(context.Background(), 7, 0, "monthly")
	if err != nil {
		t.Fatalf("无矩阵不应报错: %v", err)
	}
	if hasMatrix {
		t.Fatal("无矩阵行时 hasMatrix 应为 false")
	}
}

// TestCycleBasePriceUnavailableCycle 矩阵存在但档位停用/缺失时显式报错，不静默降级。
func TestCycleBasePriceUnavailableCycle(t *testing.T) {
	repo := &fakeRepo{rows: []model.ProductPrice{
		{ProductID: 7, Cycle: "monthly", Currency: "CNY", Price: 10, Status: 1},
		{ProductID: 7, Cycle: "quarterly", Currency: "CNY", Price: 30, Status: 0},
	}}
	svc := NewPriceMatrixService(repo, nil, nil)
	if _, _, err := svc.CycleBasePrice(context.Background(), 7, 0, "quarterly"); err == nil {
		t.Fatal("停用档位应报错")
	}
	if _, _, err := svc.CycleBasePrice(context.Background(), 7, 0, "annually"); err == nil {
		t.Fatal("缺失档位应报错")
	}
}

// TestMatrixListsUpstreamCycles 上游商品矩阵行来自渠道能力声明的周期并归一。
func TestMatrixListsUpstreamCycles(t *testing.T) {
	meta := &MatrixProductMeta{ID: 7, Name: "上游机", SourceMode: "upstream", SourceProviderID: 2}
	svc := NewPriceMatrixService(&fakeRepo{}, fixedMeta{meta}, fixedChannels{cycles: []string{"month", "year", "quarterly"}})

	resp, err := svc.Matrix(context.Background(), dto.PriceMatrixQuery{ProductID: 7})
	if err != nil {
		t.Fatalf("Matrix 失败: %v", err)
	}
	want := []string{"monthly", "quarterly", "annually"}
	if len(resp.Items) != len(want) {
		t.Fatalf("档位数量 = %d, want %d", len(resp.Items), len(want))
	}
	for i, cycle := range want {
		if resp.Items[i].Cycle != cycle {
			t.Fatalf("第 %d 档 = %s, want %s", i, resp.Items[i].Cycle, cycle)
		}
	}
	if resp.Items[0].CycleName != "月付" {
		t.Fatalf("档位中文名 = %q, want 月付", resp.Items[0].CycleName)
	}
}

// TestSaveMatrixRejectsNonUpstreamCycle 上游商品不允许保存渠道未声明的周期。
func TestSaveMatrixRejectsNonUpstreamCycle(t *testing.T) {
	meta := &MatrixProductMeta{ID: 7, SourceMode: "upstream", SourceProviderID: 2}
	svc := NewPriceMatrixService(&fakeRepo{}, fixedMeta{meta}, fixedChannels{cycles: []string{"monthly", "annually"}})

	_, err := svc.SaveMatrix(context.Background(), dto.PriceMatrixSaveRequest{
		ProductID: 7,
		Items:     []dto.PriceMatrixItemInput{{Cycle: "quarterly", Price: 30, Status: 1}},
	})
	if err == nil {
		t.Fatal("渠道不提供的周期应被拒绝")
	}
}

// TestSaveMatrixKeepsUpstreamDerivedPrice 上游派生行只接受启停，价格保持同步结果不被人工改写。
func TestSaveMatrixKeepsUpstreamDerivedPrice(t *testing.T) {
	repo := &fakeRepo{rows: []model.ProductPrice{
		{ProductID: 7, Cycle: "monthly", Currency: "CNY", Price: 20, CostPrice: 20, Source: model.PriceSourceMarkup, Status: 1},
	}}
	meta := &MatrixProductMeta{ID: 7, SourceMode: "upstream", SourceProviderID: 2}
	svc := NewPriceMatrixService(repo, fixedMeta{meta}, fixedChannels{cycles: []string{"monthly"}})

	if _, err := svc.SaveMatrix(context.Background(), dto.PriceMatrixSaveRequest{
		ProductID: 7,
		Items:     []dto.PriceMatrixItemInput{{Cycle: "monthly", Price: 999, Status: 0}},
	}); err != nil {
		t.Fatalf("SaveMatrix 失败: %v", err)
	}
	if repo.rows[0].Price != 20 {
		t.Fatalf("上游派生行价格不应被人工改写，got %v", repo.rows[0].Price)
	}
	if repo.rows[0].Status != 0 {
		t.Fatalf("启停应被接受，got status=%d", repo.rows[0].Status)
	}
	if repo.rows[0].Source != model.PriceSourceMarkup {
		t.Fatalf("来源应保持 markup，got %q", repo.rows[0].Source)
	}
}

// TestSaveMatrixSelfProductFullEdit 自营商品可自由改价并删除未提交档位。
func TestSaveMatrixSelfProductFullEdit(t *testing.T) {
	repo := &fakeRepo{rows: []model.ProductPrice{
		{ProductID: 9, Cycle: "monthly", Currency: "CNY", Price: 50, Source: model.PriceSourceManual, Status: 1},
		{ProductID: 9, Cycle: "onetime", Currency: "CNY", Price: 500, Source: model.PriceSourceManual, Status: 1},
	}}
	meta := &MatrixProductMeta{ID: 9, SourceMode: "self"}
	svc := NewPriceMatrixService(repo, fixedMeta{meta}, nil)

	resp, err := svc.SaveMatrix(context.Background(), dto.PriceMatrixSaveRequest{
		ProductID: 9,
		Items:     []dto.PriceMatrixItemInput{{Cycle: "monthly", Price: 66, Status: 1}},
	})
	if err != nil {
		t.Fatalf("SaveMatrix 失败: %v", err)
	}
	if len(repo.rows) != 1 || repo.rows[0].Cycle != "monthly" || repo.rows[0].Price != 66 {
		t.Fatalf("自营商品应可改价并删除未提交档位，got %+v", repo.rows)
	}
	if len(resp.Items) == 0 {
		t.Fatal("保存后应返回完整矩阵")
	}
}

// TestApplyUpstreamPricesPreservesManualRows 上游同步不覆盖人工行。
func TestApplyUpstreamPricesPreservesManualRows(t *testing.T) {
	repo := &fakeRepo{rows: []model.ProductPrice{
		{ProductID: 7, Cycle: "monthly", Currency: "CNY", Price: 88, Source: model.PriceSourceManual, Status: 1},
	}}
	svc := NewPriceMatrixService(repo, nil, nil)

	err := svc.ApplyUpstreamPrices(context.Background(), 7, []dto.CyclePriceRow{
		{Cycle: "monthly", Price: 20, CostPrice: 20, Source: model.PriceSourceUpstream},
		{Cycle: "annually", Price: 200, CostPrice: 200, Source: model.PriceSourceUpstream},
	})
	if err != nil {
		t.Fatalf("ApplyUpstreamPrices 失败: %v", err)
	}
	byCycle := map[string]model.ProductPrice{}
	for _, r := range repo.rows {
		byCycle[r.Cycle] = r
	}
	if byCycle["monthly"].Price != 88 {
		t.Fatalf("人工行不应被覆盖，got %v", byCycle["monthly"].Price)
	}
	if byCycle["annually"].Price != 200 {
		t.Fatalf("缺失档位应新增，got %v", byCycle["annually"].Price)
	}
}
