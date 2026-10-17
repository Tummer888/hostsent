package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	accountdto "hostsent/backend/internal/modules/admin/finance/account/dto"
	transdto "hostsent/backend/internal/modules/admin/finance/transaction/dto"
	orderdto "hostsent/backend/internal/modules/admin/order/dto"
	catalogdto "hostsent/backend/internal/modules/admin/product/catalog/dto"
	paydto "hostsent/backend/internal/modules/admin/system/payment/dto"
	"hostsent/backend/internal/modules/uc/order/dto"

	"go.uber.org/zap"
)

type fakeSkuPort struct {
	specs []catalogdto.ProductSpecInfo
	// incremented 记录回补库存的 specID（取消/关单时应命中一次）。
	incremented []uint64
	// productIncremented 记录回补商品级库存的商品 ID（无 SKU 商品关单时应命中一次）。
	productIncremented []uint64
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
func (f *fakeSkuPort) DecrementProductStock(context.Context, uint64, int) (int64, error) {
	return 1, nil
}
func (f *fakeSkuPort) IncrementProductStock(_ context.Context, productID uint64, _ int) (int64, error) {
	f.productIncremented = append(f.productIncremented, productID)
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
	order *dto.OrderRow
	err   error

	// pending 为待扫描的超期订单（ListExpiredPending 返回）。
	pending []dto.OrderRow
	// transitions 记录 CAS 调用的 (id, from, to)。
	transitions []string
	// casMiss 为 true 时模拟 CAS 未生效（并发下订单已被支付/关单）。
	casMiss bool
}

func (f *fakeOrderRepo) Create(_ context.Context, order *dto.OrderRow) error {
	// 对齐真实仓储：GORM 落库时自动填充 CreatedAt（payExpireAt 依赖它推导截止时间）。
	order.CreatedAt = time.Now()
	return nil
}
func (f *fakeOrderRepo) List(context.Context, orderdto.OrderListQuery) ([]dto.OrderRow, int64, error) {
	return nil, 0, nil
}
func (f *fakeOrderRepo) FindByID(context.Context, uint64) (*dto.OrderRow, error) {
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
func (f *fakeOrderRepo) ListExpiredPending(context.Context, time.Time, int) ([]dto.OrderRow, error) {
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
	repo := &fakeOrderRepo{order: &dto.OrderRow{
		ID: 7, OrderNo: "UC1001", UserID: 100, ProductID: 5, ProductName: "云主机",
		SpecCode: "small", Quantity: 2, Cycle: "monthly",
		Status: dto.OrderStatusPaid, PayMethod: dto.PayModeBalance,
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
	order := &dto.OrderRow{
		ID: 9, OrderNo: "UC9001", UserID: 2, ProductName: "云主机",
		Status: dto.OrderStatusPending, TotalAmount: 200, FinalAmount: 88.8,
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
	if prepay.calls[0].BizType != paydto.BizTypeOrder || prepay.calls[0].BizNo != "UC9001" {
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
		orderRepo: &fakeOrderRepo{order: &dto.OrderRow{
			ID: 9, UserID: 2, Status: dto.OrderStatusPaid, FinalAmount: 88.8,
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
		orderRepo: &fakeOrderRepo{order: &dto.OrderRow{
			ID: 9, UserID: 100, Status: dto.OrderStatusPending, FinalAmount: 88.8,
		}},
		prepay: &fakePrepay{},
	}
	if _, err := svc.Pay(context.Background(), 2, 9, dto.PayRequest{}); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("他人订单应返回 ErrOrderNotFound, got %v", err)
	}
}

// TestCancelReleasesHoldings 取消待支付订单：CAS 关单 → 关支付单 → 回补库存。
func TestCancelReleasesHoldings(t *testing.T) {
	order := &dto.OrderRow{
		ID: 9, OrderNo: "UC9001", UserID: 2, ProductID: 5, ProductName: "云主机",
		Status: dto.OrderStatusPending, SpecCode: "small", Quantity: 2, FinalAmount: 88.8,
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
	if len(closer.calls) != 1 || closer.calls[0] != paydto.BizTypeOrder+":9" {
		t.Fatalf("应关闭订单的未支付支付单，got %v", closer.calls)
	}
	if len(sku.incremented) != 1 || sku.incremented[0] != 55 {
		t.Fatalf("应回补所选 SKU 库存，got %v", sku.incremented)
	}
	if info.Status != dto.OrderStatusCancelled {
		t.Fatalf("取消后状态应为 cancelled，got %s", info.Status)
	}
}

// TestCancelReleasesProductStockWithoutSku 无 SKU 商品（上游转售）取消时回补商品级库存。
//
// 背景：上游限量商品的库存挂在商品行上（没有 SKU），此前释放逻辑只处理 SKU，
// 取消一单就永久吃掉一份库存（越卖越少）。
func TestCancelReleasesProductStockWithoutSku(t *testing.T) {
	order := &dto.OrderRow{
		ID: 12, OrderNo: "UC9012", UserID: 2, ProductID: 186, ProductName: "西安电信|一区·标准",
		Status: dto.OrderStatusPending, SpecCode: "", Quantity: 1, FinalAmount: 68,
	}
	repo := &fakeOrderRepo{order: order}
	closer := &fakeCloser{}
	sku := &fakeSkuPort{}
	svc := &orderService{orderRepo: repo, pendingPayments: closer, sku: sku}

	if _, err := svc.Cancel(context.Background(), 2, 12); err != nil {
		t.Fatalf("待支付订单应可取消: %v", err)
	}
	if len(sku.incremented) != 0 {
		t.Fatalf("无 SKU 商品不应回补 SKU 库存，got %v", sku.incremented)
	}
	if len(sku.productIncremented) != 1 || sku.productIncremented[0] != 186 {
		t.Fatalf("应回补商品级库存，got %v", sku.productIncremented)
	}
}

// TestCancelConflictKeepsHoldings 并发下订单已被支付/关单（CAS 未生效）时，
// 不得关支付单也不得回补库存，否则会造成「已付款却退库存」的错账。
func TestCancelConflictKeepsHoldings(t *testing.T) {
	repo := &fakeOrderRepo{
		order: &dto.OrderRow{
			ID: 9, UserID: 2, ProductID: 5, Status: dto.OrderStatusPending,
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
	repo := &fakeOrderRepo{pending: []dto.OrderRow{{
		ID: 9, OrderNo: "UC9001", UserID: 2, ProductID: 5,
		Status: dto.OrderStatusPending, SpecCode: "small", Quantity: 2,
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
		pending: []dto.OrderRow{{
			ID: 9, UserID: 2, ProductID: 5, Status: dto.OrderStatusPending,
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

	pending := &dto.OrderRow{Status: dto.OrderStatusPending, CreatedAt: created}
	if got, want := svc.payExpireAt(pending), created.Add(30*time.Minute).Format(time.RFC3339); got != want {
		t.Fatalf("待支付订单截止时间应为 %s，got %s", want, got)
	}
	if got := svc.payExpireAt(&dto.OrderRow{Status: dto.OrderStatusPaid, CreatedAt: created}); got != "" {
		t.Fatalf("已支付订单不应有支付截止时间，got %s", got)
	}
	// 默认值兜底：未注入配置时按 30 分钟。
	if got := (&orderService{}).payExpireMinutesOrDefault(); got != defaultPayExpireMinutes {
		t.Fatalf("未注入配置应回落 %d，got %d", defaultPayExpireMinutes, got)
	}
}

// TestPayableAmountFallback 应付金额优先算价快照，兼容未写快照的存量订单。
func TestPayableAmountFallback(t *testing.T) {
	if got := payableAmount(&dto.OrderRow{FinalAmount: 88.8, TotalAmount: 200}); got != 88.8 {
		t.Fatalf("应优先取算价快照 88.8，got %v", got)
	}
	if got := payableAmount(&dto.OrderRow{TotalAmount: 200}); got != 200 {
		t.Fatalf("无快照应回落应付总额 200，got %v", got)
	}
}

// —— 补测：下单链路（Create）与过期调度器 ——

// fakeCatalogPort 商品读取最小实现。
type fakeCatalogPort struct {
	product *catalogdto.ProductInfo
	err     error
}

func (f *fakeCatalogPort) FindByID(context.Context, uint64) (*catalogdto.ProductInfo, error) {
	return f.product, f.err
}

// fakeWalletPort 账务调整最小实现。
type fakeWalletPort struct {
	adjusted []accountdto.AdjustRequest
	err      error
}

func (f *fakeWalletPort) Adjust(_ context.Context, req accountdto.AdjustRequest, _ uint64) (*transdto.TransactionInfo, error) {
	f.adjusted = append(f.adjusted, req)
	return &transdto.TransactionInfo{}, f.err
}

// fakeProvisionQueue 开通投递最小实现。
type fakeProvisionQueue struct {
	enqueued int
}

func (f *fakeProvisionQueue) EnqueueProvision(context.Context, *dto.OrderRow, string) error {
	f.enqueued++
	return nil
}

// TestCreateRejectsEmptyProduct 下单：空商品 ID 直接拒绝，不碰任何端口。
func TestCreateRejectsEmptyProduct(t *testing.T) {
	svc := &orderService{}
	if _, err := svc.Create(context.Background(), 1, 1, dto.CreateRequest{}); err == nil {
		t.Fatal("空商品应报错")
	}
}

// TestCreateRejectsInvalidPayMode 下单：非法支付方式拒绝。
func TestCreateRejectsInvalidPayMode(t *testing.T) {
	svc := &orderService{}
	if _, err := svc.Create(context.Background(), 1, 1, dto.CreateRequest{ProductID: 5, PayMode: "bitcoin"}); !errors.Is(err, ErrPayModeInvalid) {
		t.Fatalf("非法支付方式应返回 ErrPayModeInvalid, got %v", err)
	}
}

// TestCreateBalanceFlow 余额下单主链路：扣款 → 落单 → 投递开通 → 返回 paid。
func TestCreateBalanceFlow(t *testing.T) {
	catalog := &fakeCatalogPort{product: &catalogdto.ProductInfo{ID: 5, Name: "云主机", Status: 1, Price: 88.8, PriceModel: "monthly"}}
	wallet := &fakeWalletPort{}
	repo := &fakeOrderRepo{}
	queue := &fakeProvisionQueue{}
	svc := &orderService{
		catalog: catalog, wallet: wallet, orderRepo: repo,
		provisionQueue: queue,
		sku:            &fakeSkuPort{}, // 无 SKU：走商品级库存
		isBalanceErr:   func(error) bool { return false },
	}
	info, err := svc.Create(context.Background(), 1, 1, dto.CreateRequest{ProductID: 5})
	if err != nil {
		t.Fatalf("余额下单应成功: %v", err)
	}
	if len(wallet.adjusted) != 1 || wallet.adjusted[0].Amount != 88.8 {
		t.Fatalf("应扣款一次且金额为算价结果: %+v", wallet.adjusted)
	}
	if queue.enqueued != 1 {
		t.Fatalf("应投递一次开通任务, got %d", queue.enqueued)
	}
	if info.Status != dto.OrderStatusPaid {
		t.Fatalf("余额链路订单应落 paid, got %s", info.Status)
	}
}

// TestCreateChannelFlow 渠道下单：不扣款、落 pending、不投递开通。
func TestCreateChannelFlow(t *testing.T) {
	catalog := &fakeCatalogPort{product: &catalogdto.ProductInfo{ID: 5, Name: "云主机", Status: 1, Price: 88.8, PriceModel: "monthly"}}
	wallet := &fakeWalletPort{}
	repo := &fakeOrderRepo{}
	queue := &fakeProvisionQueue{}
	svc := &orderService{
		catalog: catalog, wallet: wallet, orderRepo: repo,
		provisionQueue: queue,
		sku:            &fakeSkuPort{},
	}
	info, err := svc.Create(context.Background(), 1, 1, dto.CreateRequest{ProductID: 5, PayMode: dto.PayModeChannel})
	if err != nil {
		t.Fatalf("渠道下单应成功: %v", err)
	}
	if len(wallet.adjusted) != 0 {
		t.Fatalf("渠道链路不应扣款: %+v", wallet.adjusted)
	}
	if info.Status != dto.OrderStatusPending {
		t.Fatalf("渠道链路订单应落 pending, got %s", info.Status)
	}
	if queue.enqueued != 0 {
		t.Fatalf("未收款不应投递开通, got %d", queue.enqueued)
	}
	if info.PayExpireAt == "" {
		t.Fatal("待支付订单应带支付截止时间")
	}
}

// TestCreateOfflineProduct 下架商品拒单。
func TestCreateOfflineProduct(t *testing.T) {
	catalog := &fakeCatalogPort{product: &catalogdto.ProductInfo{ID: 5, Name: "云主机", Status: 0, Price: 88.8}}
	svc := &orderService{catalog: catalog}
	if _, err := svc.Create(context.Background(), 1, 1, dto.CreateRequest{ProductID: 5}); !errors.Is(err, ErrProductOffline) {
		t.Fatalf("下架商品应返回 ErrProductOffline, got %v", err)
	}
}

// TestCreateBalanceInsufficient 余额不足翻译为用户可读错误。
func TestCreateBalanceInsufficient(t *testing.T) {
	catalog := &fakeCatalogPort{product: &catalogdto.ProductInfo{ID: 5, Name: "云主机", Status: 1, Price: 88.8, PriceModel: "monthly"}}
	wallet := &fakeWalletPort{err: errors.New("insufficient balance")}
	svc := &orderService{
		catalog: catalog, wallet: wallet, orderRepo: &fakeOrderRepo{},
		sku:          &fakeSkuPort{},
		isBalanceErr: func(error) bool { return true },
	}
	if _, err := svc.Create(context.Background(), 1, 1, dto.CreateRequest{ProductID: 5}); !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("余额不足应返回 ErrInsufficientBalance, got %v", err)
	}
}

// fakeExpireOrderService 过期调度器的最小服务实现。
type fakeExpireOrderService struct {
	minutes   int
	closed    int
	expireErr error
}

func (f *fakeExpireOrderService) SetPayExpireMinutes(m int) { f.minutes = m }
func (f *fakeExpireOrderService) ExpirePending(context.Context, int) (int, error) {
	return f.closed, f.expireErr
}
func (f *fakeExpireOrderService) Create(context.Context, uint64, uint64, dto.CreateRequest) (*dto.OrderInfo, error) {
	return nil, nil
}
func (f *fakeExpireOrderService) List(context.Context, uint64, dto.ListQuery) (*dto.ListResponse, error) {
	return nil, nil
}
func (f *fakeExpireOrderService) Detail(context.Context, uint64, uint64) (*dto.OrderInfo, error) {
	return nil, nil
}
func (f *fakeExpireOrderService) Quote(context.Context, uint64, dto.QuoteRequest) (*dto.QuoteInfo, error) {
	return nil, nil
}
func (f *fakeExpireOrderService) Pay(context.Context, uint64, uint64, dto.PayRequest) (*dto.PayInfo, error) {
	return nil, nil
}
func (f *fakeExpireOrderService) Cancel(context.Context, uint64, uint64) (*dto.OrderInfo, error) {
	return nil, nil
}
func (f *fakeExpireOrderService) SetCashierPorts(prepayPort, pendingPaymentCloser) {}
func (f *fakeExpireOrderService) SetOptionPricingPort(optionPricingPort)           {}
func (f *fakeExpireOrderService) SetSalesOwnerResolver(salesOwnerResolver)         {}
func (f *fakeExpireOrderService) SetInstancePlaceholder(func(context.Context, *dto.OrderRow) error) {
}
func (f *fakeExpireOrderService) SetPendingInstanceReleaser(func(context.Context, uint64) (int64, error)) {
}
func (f *fakeExpireOrderService) SetLogger(*zap.Logger)                  {}
func (f *fakeExpireOrderService) SetOptionLabelResolver(optionLabelPort) {}
func (f *fakeExpireOrderService) SetRefundPort(refundPort)               {}
func (f *fakeExpireOrderService) SetPayMethodResolver(payMethodResolver) {}
func (f *fakeExpireOrderService) RequestRefund(context.Context, uint64, uint64, dto.UserRefundRequest) (*dto.UserRefundResult, error) {
	return nil, nil
}

// TestExpireSchedulerRunOnce 调度器单轮：读配置 → 刷新有效期 → 关单。
func TestExpireSchedulerRunOnce(t *testing.T) {
	svc := &fakeExpireOrderService{closed: 3}
	s := NewPendingExpireScheduler(svc, func(context.Context) int { return 30 }, zap.NewNop())
	s.runOnce(context.Background())
	if svc.minutes != 30 {
		t.Fatalf("调度器应把配置有效期灌进服务, got %d", svc.minutes)
	}
}

// TestExpireSchedulerDefaultMinutes 配置缺失/非法回落默认有效期。
func TestExpireSchedulerDefaultMinutes(t *testing.T) {
	svc := &fakeExpireOrderService{}
	s := NewPendingExpireScheduler(svc, nil, zap.NewNop())
	s.runOnce(context.Background())
	if svc.minutes != defaultPayExpireMinutes {
		t.Fatalf("无配置应回落默认 %d 分钟, got %d", defaultPayExpireMinutes, svc.minutes)
	}
}
