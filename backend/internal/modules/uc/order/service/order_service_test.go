package service

import (
	"context"
	"errors"
	"testing"
	"time"

	orderdto "hostsent/backend/internal/modules/admin/order/dto"
	ordermodel "hostsent/backend/internal/modules/admin/order/model"
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

// fakeOrderRepo 订单仓储最小实现，仅覆盖 Detail 需要的 FindByID。
type fakeOrderRepo struct {
	order *ordermodel.Order
	err   error
}

func (f *fakeOrderRepo) Create(context.Context, *ordermodel.Order) error { return nil }
func (f *fakeOrderRepo) List(context.Context, orderdto.OrderListQuery) ([]ordermodel.Order, int64, error) {
	return nil, 0, nil
}
func (f *fakeOrderRepo) FindByID(context.Context, uint64) (*ordermodel.Order, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.order, nil
}

// TestDetailOwnership 订单详情归属校验：他人订单与不存在订单一律返回 ErrOrderNotFound，
// 避免用订单 ID 探测他人数据。
func TestDetailOwnership(t *testing.T) {
	paidAt := time.Now()
	repo := &fakeOrderRepo{order: &ordermodel.Order{
		ID: 7, OrderNo: "UC1001", UserID: 100, ProductID: 5, ProductName: "云主机",
		SpecCode: "small", Quantity: 2, Cycle: "monthly",
		Status: ordermodel.OrderStatusPaid, PayMethod: ordermodel.PayMethodBalance,
		FinalAmount: 88.8, PayTime: &paidAt,
	}}
	svc := &orderService{orderRepo: repo}

	info, err := svc.Detail(context.Background(), 100, 7)
	if err != nil {
		t.Fatalf("归属账号应可读详情: %v", err)
	}
	if info.SpecCode != "small" || info.Quantity != 2 || info.Cycle != "monthly" {
		t.Fatalf("规格/数量/周期未透出: %+v", info)
	}
	if info.PayTime == "" {
		t.Fatal("支付时间未透出")
	}

	if _, err := svc.Detail(context.Background(), 200, 7); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("非归属账号应返回 ErrOrderNotFound, got %v", err)
	}

	missing := &orderService{orderRepo: &fakeOrderRepo{err: errors.New("record not found")}}
	if _, err := missing.Detail(context.Background(), 100, 7); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("不存在订单应返回 ErrOrderNotFound, got %v", err)
	}
	if _, err := svc.Detail(context.Background(), 100, 0); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("空 ID 应返回 ErrOrderNotFound, got %v", err)
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
