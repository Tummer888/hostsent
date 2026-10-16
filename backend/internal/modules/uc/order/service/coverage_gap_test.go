package service

// 批次四 D1c：补齐 uc/order/service 的 List / Quote / 选配标签 / 构造器 / 工具函数覆盖。
// 与 order_service_test.go 互补：那里覆盖 Create/Detail/Pay/Cancel/过期调度，
// 这里只补此前 0% 的读路径与映射函数，不重复已有场景。

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	orderdto "hostsent/backend/internal/modules/admin/order/dto"
	catalogdto "hostsent/backend/internal/modules/admin/product/catalog/dto"
	catalogservice "hostsent/backend/internal/modules/admin/product/catalog/service"
	"hostsent/backend/internal/modules/uc/order/dto"
	"hostsent/backend/internal/pkg/pricing"
)

// fakeActorNames 操作人用户名批量解析。
type fakeActorNames struct {
	names map[uint64]string
	err   error
}

func (f *fakeActorNames) NamesByIDs(context.Context, []uint64) (map[uint64]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.names, nil
}

// fakeProvisionTasks 开通任务批量查询。
type fakeProvisionTasks struct {
	tasks map[uint64]*dto.ProvisionTaskRow
	err   error
}

func (f *fakeProvisionTasks) FindByOrderIDs(context.Context, []uint64) (map[uint64]*dto.ProvisionTaskRow, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tasks, nil
}

// fakeOptionLabels 选配项可读名解析。
type fakeOptionLabels struct {
	groups []catalogservice.CustomerOptionGroup
	err    error
}

func (f *fakeOptionLabels) CustomerOptions(context.Context, uint64) ([]catalogservice.CustomerOptionGroup, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.groups, nil
}

// fakeOptionPricing 选配校验计价。
type fakeOptionPricing struct {
	result *catalogservice.OptionPricing
	err    error
}

func (f *fakeOptionPricing) ResolveOptionPricing(context.Context, uint64, string, map[string]string) (*catalogservice.OptionPricing, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

// fakePriceResolver 算价管线。
type fakePriceResolver struct {
	quote *pricing.Quote
	err   error
}

func (f *fakePriceResolver) Resolve(context.Context, pricing.ResolveInput) (*pricing.Quote, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.quote, nil
}

// listOrderRepo 供 List 使用的仓储：返回固定行集合。
type listOrderRepo struct {
	rows  []dto.OrderRow
	total int64
	err   error
}

func (r *listOrderRepo) Create(context.Context, *dto.OrderRow) error { return nil }
func (r *listOrderRepo) FindByID(context.Context, uint64) (*dto.OrderRow, error) {
	return nil, errors.New("not found")
}
func (r *listOrderRepo) TransitionStatus(context.Context, uint64, string, string) (int64, error) {
	return 1, nil
}
func (r *listOrderRepo) ListExpiredPending(context.Context, time.Time, int) ([]dto.OrderRow, error) {
	return nil, nil
}
func (r *listOrderRepo) List(_ context.Context, q orderdto.OrderListQuery) ([]dto.OrderRow, int64, error) {
	if r.err != nil {
		return nil, 0, r.err
	}
	return r.rows, r.total, nil
}

// —— 构造器与 Setter ——

// fakeOrderActivator 开通执行器（orderActivator 端口）。
type fakeOrderActivator struct{}

func (f *fakeOrderActivator) Activate(context.Context, uint64) error { return nil }

func TestNewOrderServiceWiresAllPorts(t *testing.T) {
	catalog := &fakeCatalogPort{}
	wallet := &fakeWalletPort{}
	repo := &fakeOrderRepo{}
	activator := &fakeOrderActivator{}
	svc := NewOrderService(catalog, wallet, repo, activator, nil, nil, nil, nil, nil, nil, nil, nil)
	if svc == nil {
		t.Fatal("构造器应返回服务实例")
	}
	impl := svc.(*orderService)
	if impl.catalog == nil || impl.wallet == nil || impl.orderRepo == nil || impl.orderSvc == nil {
		t.Fatal("构造器应装配全部必选端口")
	}
}

func TestOrderServiceSetters(t *testing.T) {
	svc := &orderService{}
	svc.SetLogger(zap.NewNop())
	if svc.logger == nil {
		t.Fatal("SetLogger 应注入日志器")
	}
	svc.SetCashierPorts(&fakePrepay{}, &fakeCloser{})
	if svc.prepay == nil || svc.pendingPayments == nil {
		t.Fatal("SetCashierPorts 应注入收银台端口")
	}
	svc.SetOptionPricingPort(&fakeOptionPricing{})
	if svc.optionPricing == nil {
		t.Fatal("SetOptionPricingPort 应注入选配计价")
	}
	svc.SetOptionLabelResolver(&fakeOptionLabels{})
	if svc.optionLabels == nil {
		t.Fatal("SetOptionLabelResolver 应注入标签解析")
	}
	svc.SetSalesOwnerResolver(fakeSalesOwner(3))
	if svc.salesOwner == nil {
		t.Fatal("SetSalesOwnerResolver 应注入销售归属")
	}
	placeholderCalled := 0
	svc.SetInstancePlaceholder(func(context.Context, *dto.OrderRow) error {
		placeholderCalled++
		return nil
	})
	if svc.instancePlaceholder == nil {
		t.Fatal("SetInstancePlaceholder 应注入占位钩子")
	}
	svc.SetPendingInstanceReleaser(func(context.Context, uint64) (int64, error) { return 1, nil })
	if svc.pendingInstanceReleaser == nil {
		t.Fatal("SetPendingInstanceReleaser 应注入占位释放钩子")
	}
	svc.SetPayExpireMinutes(0)
	if got := svc.payExpireMinutesOrDefault(); got != defaultPayExpireMinutes {
		t.Fatalf("非法值应回落默认: %d", got)
	}
	svc.SetPayExpireMinutes(30)
	if got := svc.payExpireMinutesOrDefault(); got != 30 {
		t.Fatalf("显式值应生效: %d", got)
	}
}

type fakeSalesOwner uint64

func (f fakeSalesOwner) SalesAdminForNewOrder(context.Context, uint64) uint64 { return uint64(f) }

// —— List 全链路 ——

func TestListAssemblesOrders(t *testing.T) {
	now := time.Now()
	repo := &listOrderRepo{total: 2, rows: []dto.OrderRow{
		{ID: 1, OrderNo: "UC1", ProductName: "云主机", Status: dto.OrderStatusActive, OperatorID: 11},
		{ID: 2, OrderNo: "UC2", ProductName: "云主机", Status: dto.OrderStatusPending, OperatorID: 11, CreatedAt: now},
	}}
	svc := &orderService{
		orderRepo:  repo,
		actorNames: &fakeActorNames{names: map[uint64]string{11: "运营小明"}},
		provisionTasks: &fakeProvisionTasks{tasks: map[uint64]*dto.ProvisionTaskRow{
			2: {OrderID: 2, Status: "manual", LastError: "余额不足"},
		}},
	}

	resp, err := svc.List(context.Background(), 100, dto.ListQuery{})
	if err != nil {
		t.Fatalf("List 应成功: %v", err)
	}
	if resp.Page != 1 || resp.PageSize != 10 || resp.Total != 2 || len(resp.Items) != 2 {
		t.Fatalf("分页与总数不符: %+v", resp)
	}
	byID := map[uint64]dto.OrderInfo{}
	for _, it := range resp.Items {
		byID[it.ID] = it
	}
	if byID[1].ActorName != "运营小明" {
		t.Fatalf("操作人名应解析: %+v", byID[1])
	}
	if byID[2].ProvisionStatus != "manual" || byID[2].ProvisionError != "余额不足" {
		t.Fatalf("履约状态应挂载: %+v", byID[2])
	}
	if byID[2].PayExpireAt == "" {
		t.Fatal("pending 订单应带支付截止时间")
	}

	// 仓储错误上抛。
	failSvc := &orderService{orderRepo: &listOrderRepo{err: errors.New("db down")}}
	if _, err := failSvc.List(context.Background(), 100, dto.ListQuery{Page: 2, PageSize: 25}); err == nil {
		t.Fatal("仓储错误应上抛")
	}
	// 显式分页透传。
	if resp, err := svc.List(context.Background(), 100, dto.ListQuery{Page: 2, PageSize: 25}); err != nil || resp.Page != 2 || resp.PageSize != 25 {
		t.Fatalf("显式分页应透传: %+v %v", resp, err)
	}
}

func TestResolveProvisionAndActorDegrade(t *testing.T) {
	// 未装配：均返回 nil，不 panic。
	svc := &orderService{}
	if svc.resolveProvisionTasks(context.Background(), []dto.OrderRow{{ID: 1}}) != nil {
		t.Fatal("未装配任务读取器应返回 nil")
	}
	if svc.resolveActorNames(context.Background(), []dto.OrderRow{{ID: 1}}) != nil {
		t.Fatal("未装配名字解析器应返回 nil")
	}
	// 查询失败降级为空映射。
	svc2 := &orderService{provisionTasks: &fakeProvisionTasks{err: errors.New("boom")}, actorNames: &fakeActorNames{err: errors.New("boom")}}
	if svc2.resolveProvisionTasks(context.Background(), []dto.OrderRow{{ID: 1}}) != nil {
		t.Fatal("任务查询失败应降级")
	}
	if svc2.resolveActorNames(context.Background(), []dto.OrderRow{{ID: 1, OperatorID: 5}}) != nil {
		t.Fatal("名字查询失败应降级")
	}
	// 无操作人（OperatorID=0）与去重：只查一次。
	names := &fakeActorNames{names: map[uint64]string{5: "A"}}
	svc3 := &orderService{actorNames: names}
	got := svc3.resolveActorNames(context.Background(), []dto.OrderRow{
		{ID: 1, OperatorID: 0}, {ID: 2, OperatorID: 5}, {ID: 3, OperatorID: 5},
	})
	if got[5] != "A" || len(got) != 1 {
		t.Fatalf("应去重解析操作人: %+v", got)
	}
}

// —— Quote 预结算 ——

func TestQuoteValidationsAndPricing(t *testing.T) {
	catalog := &fakeCatalogPort{product: &catalogdto.ProductInfo{ID: 5, Name: "云主机", Status: 1, Price: 100, PriceModel: "monthly"}}
	base := &orderService{catalog: catalog, sku: &fakeSkuPort{}}
	// 未装配算价管线：回落商品价 × 数量。
	if _, err := base.Quote(context.Background(), 1, dto.QuoteRequest{ProductID: 0}); err == nil {
		t.Fatal("空商品应拒绝")
	}
	offline := &orderService{catalog: &fakeCatalogPort{product: &catalogdto.ProductInfo{ID: 6, Status: 0}}, sku: &fakeSkuPort{}}
	if _, err := offline.Quote(context.Background(), 1, dto.QuoteRequest{ProductID: 6}); !errors.Is(err, ErrProductOffline) {
		t.Fatalf("下架商品应拒绝: %v", err)
	}
	info, err := base.Quote(context.Background(), 1, dto.QuoteRequest{ProductID: 5, Quantity: 2})
	if err != nil {
		t.Fatalf("Quote 应成功: %v", err)
	}
	if info.FinalAmount != 200 || info.OriginalAmount != 200 || len(info.Options) != 0 {
		t.Fatalf("回落算价不符: %+v", info)
	}
	// 装配算价管线 + 选配计价：管线结果优先，选项明细透传。
	policyID := uint64(42)
	priced := &orderService{
		catalog:       catalog,
		sku:           &fakeSkuPort{},
		pricing:       &fakePriceResolver{quote: &pricing.Quote{OriginalAmount: 250, DiscountAmount: 50, FinalAmount: 200, OptionAmount: 20, PolicyID: &policyID, Source: "coupon", Snapshot: []pricing.Rule{{Source: "coupon", Code: "C1"}}}},
		optionPricing: &fakeOptionPricing{result: &catalogservice.OptionPricing{Amount: 20, Items: []catalogservice.OptionPricingItem{{OptionKey: "ip_num", OptionName: "IP 数", Value: "2", ValueName: "2 个", Quantity: 1, UnitPrice: 20, Amount: 20}}}},
	}
	q, err := priced.Quote(context.Background(), 1, dto.QuoteRequest{ProductID: 5, Quantity: 2, ConfigSelections: map[string]string{"ip_num": "2"}})
	if err != nil {
		t.Fatalf("管线 Quote 应成功: %v", err)
	}
	if q.FinalAmount != 200 || q.OptionAmount != 20 || q.PolicyID == nil || *q.PolicyID != 42 {
		t.Fatalf("管线算价不符: %+v", q)
	}
	if len(q.Options) != 1 || q.Options[0].OptionName != "IP 数" || q.Options[0].ValueName != "2 个" {
		t.Fatalf("选项明细应透传: %+v", q.Options)
	}
	// 管线 quote 的 OptionAmount 为 0 时回填选配计价小计。
	priced2 := &orderService{
		catalog:       catalog,
		sku:           &fakeSkuPort{},
		pricing:       &fakePriceResolver{quote: &pricing.Quote{OriginalAmount: 250, FinalAmount: 250, Snapshot: []pricing.Rule{}}},
		optionPricing: &fakeOptionPricing{result: &catalogservice.OptionPricing{Amount: 20, Items: []catalogservice.OptionPricingItem{}}},
	}
	q2, err := priced2.Quote(context.Background(), 1, dto.QuoteRequest{ProductID: 5, Quantity: 2})
	if err != nil || q2.OptionAmount != 20 {
		t.Fatalf("选项加价应回填: %+v %v", q2, err)
	}
	// 选配计价失败上抛。
	bad := &orderService{catalog: catalog, sku: &fakeSkuPort{}, optionPricing: &fakeOptionPricing{err: errors.New("非法选配")}}
	if _, err := bad.Quote(context.Background(), 1, dto.QuoteRequest{ProductID: 5}); err == nil {
		t.Fatal("选配计价失败应上抛")
	}
}

// —— 选配可读名 ——

func TestResolveSelectionLabels(t *testing.T) {
	groups := []catalogservice.CustomerOptionGroup{{Options: []catalogservice.CustomerOption{
		{OptionKey: "network", Name: "网络类型", Values: []catalogservice.CustomerOptionItem{
			{Value: "1", Label: "VPC网络"},
			{Value: "2", Label: "经典网络"},
		}},
		{OptionKey: "hostname", Name: "", Values: nil}, // 文本型：无候选值表
		{OptionKey: "root_password", Name: "登录密码", Values: nil},
	}}}
	svc := &orderService{optionLabels: &fakeOptionLabels{groups: groups}}

	// 未装配 / 空选择：nil。
	if (&orderService{}).resolveSelectionLabels(context.Background(), 1, nil) != nil {
		t.Fatal("未装配应返回 nil")
	}
	empty := &orderService{optionLabels: &fakeOptionLabels{groups: groups}}
	if empty.resolveSelectionLabels(context.Background(), 1, map[string]string{}) != nil {
		t.Fatal("空选择应返回 nil")
	}

	got := svc.resolveSelectionLabels(context.Background(), 1, map[string]string{
		"network":       "1",
		"hostname":      "web-01",
		"root_password": "secret-123",
		"unknown_key":   "x", // 不在选项表里：跳过
	})
	want := []string{
		"hostname：web-01",
		"网络类型：VPC网络",
		"登录密码：已设置",
	}
	if len(got) != len(want) {
		t.Fatalf("标签列表不符: %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("标签[%d] = %q, want %q（排序后应稳定）", i, got[i], want[i])
		}
	}

	// 选项表查询失败：降级 nil，不报错。
	bad := &orderService{optionLabels: &fakeOptionLabels{err: errors.New("db down")}}
	if bad.resolveSelectionLabels(context.Background(), 1, map[string]string{"network": "1"}) != nil {
		t.Fatal("查询失败应降级 nil")
	}
}

// —— 选项计价归一 ——

func TestResolveOptionPricingNormalize(t *testing.T) {
	// 未装配：原样回传选择（含 nil → 空 map）。
	svc := &orderService{}
	r, err := svc.resolveOptionPricing(context.Background(), 1, "monthly", map[string]string{"a": "1"})
	if err != nil || r.Selections["a"] != "1" {
		t.Fatalf("未装配应原样回传: %+v %v", r, err)
	}
	r2, err := svc.resolveOptionPricing(context.Background(), 1, "monthly", nil)
	if err != nil || len(r2.Selections) != 0 {
		t.Fatalf("nil 选择应归一为空 map: %+v %v", r2, err)
	}
	// 装配后：结果 nil / Selections nil 都归一为空 map。
	svc2 := &orderService{optionPricing: &fakeOptionPricing{}}
	r3, err := svc2.resolveOptionPricing(context.Background(), 1, "monthly", nil)
	if err != nil || len(r3.Selections) != 0 {
		t.Fatalf("nil 结果应归一: %+v %v", r3, err)
	}
	r4, err := svc2.resolveOptionPricing(context.Background(), 1, "monthly", nil)
	_ = r4
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
}

// —— 工具函数 ——

func TestSelKeyFirstNonEmptyAndJSON(t *testing.T) {
	if got := selKey("network", "1"); got != "network::1" {
		t.Fatalf("selKey = %q", got)
	}
	if got := firstNonEmptyStr("", "  ", "b"); got != "b" {
		t.Fatalf("应取首个非空白: %q", got)
	}
	if got := firstNonEmptyStr("", " "); got != "" {
		t.Fatalf("全空白应返回空: %q", got)
	}
	if got := selectionsJSON(nil); got != "" {
		t.Fatalf("空选择应落空串: %q", got)
	}
	if got := selectionsJSON(map[string]string{"a": "1"}); got != `{"a":"1"}` {
		t.Fatalf("selectionsJSON = %q", got)
	}
	if got := snapshotJSON(nil); got != "[]" {
		t.Fatalf("nil 快照应落空数组: %q", got)
	}
	if got := snapshotJSON([]pricing.Rule{{Source: "coupon"}}); got == "" || got == "[]" {
		t.Fatalf("非空快照应序列化: %q", got)
	}
	if got := truncRunes("中文备注超长", 3); got != "中文备" {
		t.Fatalf("按字符截断: %q", got)
	}
	if got := truncRunes("abc", 0); got != "abc" {
		t.Fatalf("非正上限不截断: %q", got)
	}
}

func TestFromAdminOrderProjection(t *testing.T) {
	payAt := time.Now()
	expireAt := payAt.Add(24 * time.Hour)
	row := dto.OrderRow{
		ID: 1, OrderNo: "UC1", ProductID: 5, ProductName: "云主机",
		Status: dto.OrderStatusActive, PayMethod: dto.PayModeBalance,
		TotalAmount: 100, PaidAmount: 90, FinalAmount: 90,
		OperatorID: 7, ConfigOptions: `{"network":"1"}`,
		PayTime: &payAt, ExpireTime: &expireAt,
		CustomerRemark: "客户备注",
	}
	info := fromAdminOrder(row, map[uint64]string{7: "小明"})
	if info.ActorName != "小明" || info.CustomerRemark != "客户备注" {
		t.Fatalf("投影不符: %+v", info)
	}
	if info.Remark != "" {
		t.Fatal("运营内部备注绝不能透出给用户")
	}
	if info.ConfigSelections["network"] != "1" {
		t.Fatalf("选配应解析: %+v", info.ConfigSelections)
	}
	if info.PayTime == "" || info.ExpireTime == "" {
		t.Fatal("时间应透出")
	}
	// 无选配 / 无操作人：不 panic。
	info2 := fromAdminOrder(dto.OrderRow{ID: 2, Status: dto.OrderStatusPaid}, nil)
	if info2.ActorName != "" || len(info2.ConfigSelections) != 0 {
		t.Fatalf("空值投影不符: %+v", info2)
	}
}

// —— 过期调度器 Start/runSafely ——

func TestExpireSchedulerStartStops(t *testing.T) {
	svc := &orderService{orderRepo: &fakeOrderRepo{}}
	sched := NewPendingExpireScheduler(svc, func(context.Context) int { return 1440 }, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	sched.Start(ctx)
	time.Sleep(20 * time.Millisecond)
	cancel()
}

func TestExpireSchedulerRunSafely(t *testing.T) {
	repo := &fakeOrderRepo{pending: []dto.OrderRow{{ID: 9, Status: dto.OrderStatusPending, CreatedAt: time.Now().Add(-2 * time.Hour)}}}
	svc := &orderService{orderRepo: repo, logger: zap.NewNop()}
	sched := NewPendingExpireScheduler(svc, func(context.Context) int { return 60 }, zap.NewNop())
	// runOnce 单轮直跑：有超期订单则关单；panic 防护由 runSafely 包住。
	sched.runSafely(context.Background())
	if len(repo.transitions) != 1 {
		t.Fatalf("超期订单应被关单: %+v", repo.transitions)
	}
}

// —— 占位钩子错误日志（logPlaceholderError 分支经 Create 触发） ——

func TestCreateWithPlaceholderFailureStillSucceeds(t *testing.T) {
	catalog := &fakeCatalogPort{product: &catalogdto.ProductInfo{ID: 5, Name: "云主机", Status: 1, Price: 88.8, PriceModel: "monthly"}}
	repo := &fakeOrderRepo{}
	svc := &orderService{
		catalog: catalog, wallet: &fakeWalletPort{}, orderRepo: repo,
		sku:      &fakeSkuPort{},
		orderSvc: &fakeOrderActivator{},
		logger:   zap.NewNop(),
	}
	svc.SetInstancePlaceholder(func(context.Context, *dto.OrderRow) error {
		return errors.New("占位建失败（不阻断下单）")
	})
	if _, err := svc.Create(context.Background(), 1, 1, dto.CreateRequest{ProductID: 5, PayMode: dto.PayModeBalance}); err != nil {
		t.Fatalf("占位失败不应阻断下单: %v", err)
	}
}

// —— payExpireAt 边界（List 之外的直接断言） ——

func TestPayExpireAtEdges(t *testing.T) {
	svc := &orderService{}
	if svc.payExpireAt(nil) != "" {
		t.Fatal("nil 订单应返回空")
	}
	now := time.Now()
	if svc.payExpireAt(&dto.OrderRow{ID: 1, Status: dto.OrderStatusPending, CreatedAt: now}) == "" {
		t.Fatal("pending + CreatedAt 应返回截止时间")
	}
	if svc.payExpireAt(&dto.OrderRow{ID: 1, Status: dto.OrderStatusPaid, CreatedAt: now}) != "" {
		t.Fatal("非 pending 应返回空")
	}
	if svc.payExpireAt(&dto.OrderRow{ID: 1, Status: dto.OrderStatusPending}) != "" {
		t.Fatal("CreatedAt 为零应返回空")
	}
}
