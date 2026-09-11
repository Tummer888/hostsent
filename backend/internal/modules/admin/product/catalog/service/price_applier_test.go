package service

import (
	"context"
	"math"
	"testing"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
	"hostsent/backend/internal/modules/admin/product/catalog/model"
)

type fakeProductRepo struct {
	items   []model.Product
	updates []model.Product
	history []model.ProductHistory
}

func (f *fakeProductRepo) List(context.Context, dto.ProductListQuery) ([]model.Product, int64, error) {
	return nil, 0, nil
}
func (f *fakeProductRepo) FindByID(context.Context, uint64) (*model.Product, error) { return nil, nil }
func (f *fakeProductRepo) Create(context.Context, *model.Product) error             { return nil }
func (f *fakeProductRepo) Update(_ context.Context, item *model.Product) error {
	f.updates = append(f.updates, *item)
	return nil
}
func (f *fakeProductRepo) Delete(context.Context, uint64) error { return nil }
func (f *fakeProductRepo) ListSpecs(context.Context, uint64) ([]model.ProductSpec, error) {
	return nil, nil
}
func (f *fakeProductRepo) AddHistory(_ context.Context, h *model.ProductHistory) error {
	f.history = append(f.history, *h)
	return nil
}
func (f *fakeProductRepo) ListHistory(context.Context, uint64) ([]model.ProductHistory, error) {
	return nil, nil
}
func (f *fakeProductRepo) ListBySourceProductID(context.Context, uint64) ([]model.Product, error) {
	return f.items, nil
}
func (f *fakeProductRepo) SaveConfigOptions(context.Context, uint64, []interface{}) error { return nil }
func (f *fakeProductRepo) ConfigGroupsByProductID(context.Context, uint64) ([]interface{}, error) {
	return nil, nil
}

// TestApplyConfirmedPriceKeepsSalePriceWithoutMarkup 无加价规则时只更新成本，售价不变。
func TestApplyConfirmedPriceKeepsSalePriceWithoutMarkup(t *testing.T) {
	repo := &fakeProductRepo{items: []model.Product{{ID: 19, Price: 180, CostPrice: 100, SourceProductID: 28975}}}
	svc := &productService{repo: repo}
	n, err := svc.ApplyConfirmedPrice(context.Background(), 28975, 150, "admin", "")
	if err != nil {
		t.Fatalf("ApplyConfirmedPrice: %v", err)
	}
	if n != 1 {
		t.Fatalf("affected = %d, want 1", n)
	}
	if got := repo.updates[0].Price; math.Abs(got-180) > 0.0001 {
		t.Errorf("售价应保持不变 = %v, want 180", got)
	}
	if got := repo.updates[0].CostPrice; math.Abs(got-150) > 0.0001 {
		t.Errorf("成本价 = %v, want 150", got)
	}
	if len(repo.history) != 1 || repo.history[0].ChangeType != model.ChangeTypePrice {
		t.Fatalf("应写入一条调价历史，got %+v", repo.history)
	}
}

// TestApplyConfirmedPriceRecomputesWithMarkup 配置加价规则时按规则重算售价。
func TestApplyConfirmedPriceRecomputesWithMarkup(t *testing.T) {
	repo := &fakeProductRepo{items: []model.Product{
		{ID: 19, Price: 180, CostPrice: 100, UpstreamMarkupType: "percent", UpstreamMarkupValue: 150},
	}}
	svc := &productService{repo: repo}
	if _, err := svc.ApplyConfirmedPrice(context.Background(), 28975, 150, "admin", ""); err != nil {
		t.Fatalf("ApplyConfirmedPrice: %v", err)
	}
	if got := repo.updates[0].Price; math.Abs(got-225) > 0.0001 {
		t.Errorf("percent 150%% 售价 = %v, want 225", got)
	}
}

// TestApplyConfirmedPriceNoBinding 无关联售出商品时空操作，不报错。
func TestApplyConfirmedPriceNoBinding(t *testing.T) {
	repo := &fakeProductRepo{}
	svc := &productService{repo: repo}
	n, err := svc.ApplyConfirmedPrice(context.Background(), 999, 10, "", "")
	if err != nil || n != 0 {
		t.Fatalf("无关联商品应返回 (0,nil)，got (%d,%v)", n, err)
	}
}
