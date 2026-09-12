package service

import (
	"context"
	"errors"
	"testing"

	catalogdto "hostsent/backend/internal/modules/admin/product/catalog/dto"
)

type fakeSkuPort struct {
	specs []catalogdto.ProductSpecInfo
}

func (f *fakeSkuPort) ListSpecs(context.Context, uint64) ([]catalogdto.ProductSpecInfo, error) {
	return f.specs, nil
}
func (f *fakeSkuPort) DecrementSpecStock(context.Context, uint64, int) (int64, error) {
	return 1, nil
}
func (f *fakeSkuPort) IncrementSpecStock(context.Context, uint64, int) (int64, error) {
	return 1, nil
}

// 商品未挂规格：返回 nil，照旧按商品级价格下单（存量商品兼容）。
func TestResolveSkuNoSpecs(t *testing.T) {
	svc := &orderService{sku: &fakeSkuPort{}}
	spec, err := svc.resolveSku(context.Background(), 1, "")
	if err != nil || spec != nil {
		t.Fatalf("无 SKU 商品应返回 (nil,nil)，got (%+v,%v)", spec, err)
	}
}

// 挂了规格但未选编码：拒绝下单，避免误用商品级价格。
func TestResolveSkuRequiresSelection(t *testing.T) {
	svc := &orderService{sku: &fakeSkuPort{specs: []catalogdto.ProductSpecInfo{
		{ID: 11, SpecCode: "small", Status: 1},
		{ID: 12, SpecCode: "big", Status: 1},
	}}}
	if _, err := svc.resolveSku(context.Background(), 1, ""); err == nil {
		t.Fatal("未选规格应报错")
	}
}

// 选中的规格已停用：视为不存在（下架规格不可售）。
func TestResolveSkuDisabledSpecRejected(t *testing.T) {
	svc := &orderService{sku: &fakeSkuPort{specs: []catalogdto.ProductSpecInfo{
		{ID: 11, SpecCode: "small", Status: 0},
		{ID: 12, SpecCode: "big", Status: 1},
	}}}
	if _, err := svc.resolveSku(context.Background(), 1, "small"); err == nil {
		t.Fatal("停用规格应报错")
	}
	spec, err := svc.resolveSku(context.Background(), 1, "big")
	if err != nil || spec == nil || spec.ID != 12 {
		t.Fatalf("启用规格应命中，got (%+v,%v)", spec, err)
	}
}

// 规格全部停用：明确报"已下架"，而不是回落商品级下单。
func TestResolveSkuAllDisabled(t *testing.T) {
	svc := &orderService{sku: &fakeSkuPort{specs: []catalogdto.ProductSpecInfo{
		{ID: 11, SpecCode: "small", Status: 0},
	}}}
	_, err := svc.resolveSku(context.Background(), 1, "small")
	if err == nil {
		t.Fatal("规格全下架应报错")
	}
	if errors.Is(err, ErrProductOffline) {
		t.Fatalf("不应误用商品下架错误: %v", err)
	}
}

// TestResolveCycle 下单周期归一（doc25）：显式周期接受规范值/上游别名，
// 非法值直接拒绝（不静默落到默认档），留空回落商品 price_model。
func TestResolveCycle(t *testing.T) {
	svc := &orderService{}
	cases := []struct {
		priceModel string
		raw        string
		want       string
		wantErr    bool
	}{
		{"monthly", "", "monthly", false},
		{"fixed", "", "onetime", false},
		{"hourly", "", "hourly", false},
		{"monthly", "annually", "annually", false},
		{"monthly", "year", "annually", false},
		{"monthly", "quarter", "quarterly", false},
		{"monthly", "semiannually", "semiannually", false},
		{"monthly", "biennially", "biennially", false},
		{"monthly", "nonsense", "", true},
	}
	for _, tc := range cases {
		got, err := svc.resolveCycle(&catalogdto.ProductInfo{PriceModel: tc.priceModel}, tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("cycle=%q 应报错", tc.raw)
			}
			continue
		}
		if err != nil {
			t.Fatalf("cycle=%q 不应报错: %v", tc.raw, err)
		}
		if got != tc.want {
			t.Fatalf("cycle=%q price_model=%q → %q, want %q", tc.raw, tc.priceModel, got, tc.want)
		}
	}
}
