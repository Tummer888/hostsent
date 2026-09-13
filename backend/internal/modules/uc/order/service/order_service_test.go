package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	orderdto "hostsent/backend/internal/modules/admin/order/dto"
	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	paydto "hostsent/backend/internal/modules/admin/payment/dto"
	paymodel "hostsent/backend/internal/modules/admin/payment/model"
	catalogdto "hostsent/backend/internal/modules/admin/product/catalog/dto"
	"hostsent/backend/internal/modules/uc/order/dto"
)

type fakeSkuPort struct {
	specs []catalogdto.ProductSpecInfo
	// incremented 记录回补库存的 specID（取消/关单时应命中一次）。
	incremented []uint64
}

func (f *fakeSkuPort) ListSpecs(context.Context, uint64) ([]catalogdto.ProductSpecInfo, error) {
	return f.specs, nil
}
func (f *fakeSkuPort) DecrementSpecStock(context.Context, uint64, int) (int64, error) {
	return 1, nil
}
func (f *fakeSkuPort) IncrementSpecStock(_ context.Context, specID uint64, _ int) (int64, error) {
	f.incremented = append(f.incremented, specID)
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

// fakeOrderRepo 订单仓储最小实现，覆盖 Detail/Pay/Cancel/ExpirePending 需要的方法。
type fakeOrderRepo struct {
	order *ordermodel.Order
	err   error

	// pending 为待扫描的超期订单（ListExpiredPending 返回）。
	pending []ordermodel.Order
	// transitions 记录 CAS 调用的 (id, from, to)。
	transitions []string
	// casMiss 为 true 时模拟 CAS 未生效（并发下订单已被支付/关单）。
	casMiss bool
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
func (f *fakeOrderRepo) TransitionStatus(_ context.Context, id uint64, from, to string) (int64, error) {
	f.transitions = append(f.transitions, fmt.Sprintf("%d:%s->%s", id, from, to))
	if f.err != nil {
		return 0, f.err
	}
	if f.casMiss {
		return 0, nil
	}
	return 1, nil
}
func (f *fakeOrderRepo) ListExpiredPending(context.Context, time.Time, int) ([]ordermodel.Order, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.pending, nil
}

// fakePrepay 收银台发起支付的最小实现。
type fakePrepay struct {
	calls   []paydto.PrepayRequest
	info    *paydto.OrderInfo
	err     error
	orderID uint64
	userID  uint64
}

func (f *fakePrepay) Prepay(_ context.Context, userID uint64, req paydto.PrepayRequest) (*paydto.OrderInfo, error) {
	f.calls = append(f.calls, req)
	f.userID = userID
	f.orderID = req.BizID
	if f.err != nil {
		return nil, f.err
	}
	return f.info, nil
}

// fakeCloser 关闭未支付支付单的最小实现。
type fakeCloser struct {
	calls []string
	err   error
}

func (f *fakeCloser) CloseByBiz(_ context.Context, bizType string, bizID uint64) (int, error) {
	f.calls = append(f.calls, fmt.Sprintf("%s:%d", bizType, bizID))
	return 1, f.err
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

// TestPayUsesOrderAmount 收银台发起支付：金额必须取订单算价快照，前端无法改价。
func TestPayUsesOrderAmount(t *testing.T) {
	order := &ordermodel.Order{
		ID: 9, OrderNo: "UC9001", UserID: 2, ProductName: "云主机",
		Status: ordermodel.OrderStatusPending, TotalAmount: 200, FinalAmount: 88.8,
	}
	repo := &fakeOrderRepo{order: order}
	prepay := &fakePrepay{info: &paydto.OrderInfo{
		ID: 77, PaymentNo: "PAY1", Amount: 88.8, ChannelCode: "manual_main",
		ChannelName: "人工转账", Status: "paying",
	}}
	svc := &orderService{orderRepo: repo, prepay: prepay}

	info, err := svc.Pay(context.Background(), 2, 9, dto.PayRequest{ChannelCode: "manual_main"})
	if err != nil {
		t.Fatalf("待支付订单应可发起支付: %v", err)
	}
	if len(prepay.calls) != 1 {
		t.Fatalf("应调用一次 Prepay，got %d", len(prepay.calls))
	}
	// 金额取 final_amount（算价快照）而非 total_amount（原价），更不取前端值。
	if got := prepay.calls[0].Amount; got != 88.8 {
		t.Fatalf("支付金额应取算价快照 88.8，got %v", got)
	}
	if prepay.calls[0].BizType != paymodel.BizTypeOrder || prepay.calls[0].BizNo != "UC9001" {
		t.Fatalf("支付单应绑定订单业务：%+v", prepay.calls[0])
	}
	if info.PaymentNo != "PAY1" || info.ChannelName != "人工转账" {
		t.Fatalf("收银台参数未透出: %+v", info)
	}
}

// TestPayRejectsNonPending 已支付/已取消的订单不可再次发起支付（避免重复收款）。
func TestPayRejectsNonPending(t *testing.T) {
	prepay := &fakePrepay{}
	svc := &orderService{
		orderRepo: &fakeOrderRepo{order: &ordermodel.Order{
			ID: 9, UserID: 2, Status: ordermodel.OrderStatusPaid, FinalAmount: 88.8,
		}},
		prepay: prepay,
	}
	if _, err := svc.Pay(context.Background(), 2, 9, dto.PayRequest{}); !errors.Is(err, ErrOrderNotPayable) {
		t.Fatalf("已支付订单应返回 ErrOrderNotPayable, got %v", err)
	}
	if len(prepay.calls) != 0 {
		t.Fatal("不可支付的订单不应调用 Prepay")
	}
}

// TestPayRejectsForeignOrder 他人订单按不存在处理，不得探测金额与状态。
func TestPayRejectsForeignOrder(t *testing.T) {
	svc := &orderService{
		orderRepo: &fakeOrderRepo{order: &ordermodel.Order{
			ID: 9, UserID: 100, Status: ordermodel.OrderStatusPending, FinalAmount: 88.8,
		}},
		prepay: &fakePrepay{},
	}
	if _, err := svc.Pay(context.Background(), 2, 9, dto.PayRequest{}); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("他人订单应返回 ErrOrderNotFound, got %v", err)
	}
}

// TestCancelReleasesHoldings 取消待支付订单：CAS 关单 → 关支付单 → 回补库存。
func TestCancelReleasesHoldings(t *testing.T) {
	order := &ordermodel.Order{
		ID: 9, OrderNo: "UC9001", UserID: 2, ProductID: 5, ProductName: "云主机",
		Status: ordermodel.OrderStatusPending, SpecCode: "small", Quantity: 2, FinalAmount: 88.8,
	}
	repo := &fakeOrderRepo{order: order}
	closer := &fakeCloser{}
	sku := &fakeSkuPort{specs: []catalogdto.ProductSpecInfo{{ID: 55, SpecCode: "small", Status: 1}}}
	svc := &orderService{orderRepo: repo, pendingPayments: closer, sku: sku}

	info, err := svc.Cancel(context.Background(), 2, 9)
	if err != nil {
		t.Fatalf("待支付订单应可取消: %v", err)
	}
	if len(repo.transitions) != 1 || repo.transitions[0] != "9:pending->cancelled" {
		t.Fatalf("应先 CAS 关单，got %v", repo.transitions)
	}
	if len(closer.calls) != 1 || closer.calls[0] != paymodel.BizTypeOrder+":9" {
		t.Fatalf("应关闭订单的未支付支付单，got %v", closer.calls)
	}
	if len(sku.incremented) != 1 || sku.incremented[0] != 55 {
		t.Fatalf("应回补所选 SKU 库存，got %v", sku.incremented)
	}
	if info.Status != ordermodel.OrderStatusCancelled {
		t.Fatalf("取消后状态应为 cancelled，got %s", info.Status)
	}
}

// TestCancelConflictKeepsHoldings 并发下订单已被支付/关单（CAS 未生效）时，
// 不得关支付单也不得回补库存，否则会造成「已付款却退库存」的错账。
func TestCancelConflictKeepsHoldings(t *testing.T) {
	repo := &fakeOrderRepo{
		order: &ordermodel.Order{
			ID: 9, UserID: 2, ProductID: 5, Status: ordermodel.OrderStatusPending,
			SpecCode: "small", Quantity: 2,
		},
		// CAS 未生效：模拟订单刚被支付成功回调改成 paid。
		casMiss: true,
	}
	closer := &fakeCloser{}
	sku := &fakeSkuPort{specs: []catalogdto.ProductSpecInfo{{ID: 55, SpecCode: "small", Status: 1}}}
	svc := &orderService{orderRepo: repo, pendingPayments: closer, sku: sku}

	if _, err := svc.Cancel(context.Background(), 2, 9); !errors.Is(err, ErrOrderNotCancellable) {
		t.Fatalf("CAS 未生效应返回 ErrOrderNotCancellable, got %v", err)
	}
	if len(closer.calls) != 0 || len(sku.incremented) != 0 {
		t.Fatalf("CAS 未生效时不得动支付单与库存，got closer=%v sku=%v", closer.calls, sku.incremented)
	}
}

// TestExpirePendingClosesAndReleases 超期关单：置 closed（与 cancelled 区分）并释放占用。
func TestExpirePendingClosesAndReleases(t *testing.T) {
	repo := &fakeOrderRepo{pending: []ordermodel.Order{{
		ID: 9, OrderNo: "UC9001", UserID: 2, ProductID: 5,
		Status: ordermodel.OrderStatusPending, SpecCode: "small", Quantity: 2,
	}}}
	closer := &fakeCloser{}
	sku := &fakeSkuPort{specs: []catalogdto.ProductSpecInfo{{ID: 55, SpecCode: "small", Status: 1}}}
	svc := &orderService{orderRepo: repo, pendingPayments: closer, sku: sku}

	closed, err := svc.ExpirePending(context.Background(), 30)
	if err != nil {
		t.Fatalf("超期扫描不应报错: %v", err)
	}
	if closed != 1 {
		t.Fatalf("应关掉 1 笔超期订单，got %d", closed)
	}
	if len(repo.transitions) != 1 || repo.transitions[0] != "9:pending->closed" {
		t.Fatalf("应 CAS 置 closed，got %v", repo.transitions)
	}
	if len(closer.calls) != 1 || len(sku.incremented) != 1 {
		t.Fatalf("超期关单应释放支付单与库存，got closer=%v sku=%v", closer.calls, sku.incremented)
	}
}

// TestExpirePendingSkipsPaidRace 扫描与支付成功并发：CAS 未生效的订单不释放占用。
func TestExpirePendingSkipsPaidRace(t *testing.T) {
	repo := &fakeOrderRepo{
		pending: []ordermodel.Order{{
			ID: 9, UserID: 2, ProductID: 5, Status: ordermodel.OrderStatusPending,
			SpecCode: "small", Quantity: 2,
		}},
		casMiss: true,
	}
	closer := &fakeCloser{}
	sku := &fakeSkuPort{specs: []catalogdto.ProductSpecInfo{{ID: 55, SpecCode: "small", Status: 1}}}
	svc := &orderService{orderRepo: repo, pendingPayments: closer, sku: sku}

	closed, err := svc.ExpirePending(context.Background(), 30)
	if err != nil {
		t.Fatalf("超期扫描不应报错: %v", err)
	}
	if closed != 0 {
		t.Fatalf("已被支付的订单不应计入关单数，got %d", closed)
	}
	if len(closer.calls) != 0 || len(sku.incremented) != 0 {
		t.Fatalf("已被支付的订单不得释放占用，got closer=%v sku=%v", closer.calls, sku.incremented)
	}
}

// TestExpireDefaultMinutes 有效期非法（<=0）时回落默认 30 分钟，不会把刚下的单立刻关掉。
func TestExpireDefaultMinutes(t *testing.T) {
	repo := &fakeOrderRepo{}
	svc := &orderService{orderRepo: repo}
	if _, err := svc.ExpirePending(context.Background(), 0); err != nil {
		t.Fatalf("非法有效期应按默认值处理: %v", err)
	}
}

// TestPayExpireAtOnlyForPending 支付截止时间只对待支付订单透出（其余状态为空串）。
func TestPayExpireAtOnlyForPending(t *testing.T) {
	svc := &orderService{}
	svc.SetPayExpireMinutes(30)
	created := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)

	pending := &ordermodel.Order{Status: ordermodel.OrderStatusPending, CreatedAt: created}
	if got, want := svc.payExpireAt(pending), created.Add(30*time.Minute).Format(time.RFC3339); got != want {
		t.Fatalf("待支付订单截止时间应为 %s，got %s", want, got)
	}
	if got := svc.payExpireAt(&ordermodel.Order{Status: ordermodel.OrderStatusPaid, CreatedAt: created}); got != "" {
		t.Fatalf("已支付订单不应有支付截止时间，got %s", got)
	}
	// 默认值兜底：未注入配置时按 30 分钟。
	if got := (&orderService{}).payExpireMinutesOrDefault(); got != defaultPayExpireMinutes {
		t.Fatalf("未注入配置应回落 %d，got %d", defaultPayExpireMinutes, got)
	}
}

// TestPayableAmountFallback 应付金额优先算价快照，兼容未写快照的存量订单。
func TestPayableAmountFallback(t *testing.T) {
	if got := payableAmount(&ordermodel.Order{FinalAmount: 88.8, TotalAmount: 200}); got != 88.8 {
		t.Fatalf("应优先取算价快照 88.8，got %v", got)
	}
	if got := payableAmount(&ordermodel.Order{TotalAmount: 200}); got != 200 {
		t.Fatalf("无快照应回落应付总额 200，got %v", got)
	}
}
