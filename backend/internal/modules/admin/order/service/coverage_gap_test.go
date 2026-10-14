package service

// 批次四 D1b：补齐 admin/order/service 读路径与履约工作池的单元覆盖。
// 与 order_service_test.go / upstream_provision_recover_test.go 互补：
// 这里只覆盖此前 0% 的 List/Stats/ListRefunds/工作池/辅佐函数，不重复已有场景。

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/model"
	"hostsent/backend/internal/pkg/credentials"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// dtoOrderListQuery 构造空分页参数的订单列表查询（覆盖分页兜底）。
func dtoOrderListQuery(page, pageSize int) dto.OrderListQuery {
	return dto.OrderListQuery{Page: page, PageSize: pageSize}
}

// dtoRefundListQuery 构造空分页参数的退款列表查询。
func dtoRefundListQuery() dto.RefundListQuery {
	return dto.RefundListQuery{}
}

// gormErrNotFound 直接引用 gorm 哨兵（map 函数把它翻译成业务哨兵）。
var gormErrNotFound = gorm.ErrRecordNotFound

// reflectValueOf 反射取值（toUint 只吃 reflect.Value，便于表驱动）。
func reflectValueOf(v interface{}) reflect.Value { return reflect.ValueOf(v) }

// —— 列表 / 统计 / 退款列表（读路径） ——

func TestListAttachesProvisionTasks(t *testing.T) {
	repo := newFakeOrderRepo(
		&model.Order{ID: 1, OrderNo: "A1", Status: model.OrderStatusActive},
		&model.Order{ID: 2, OrderNo: "A2", Status: model.OrderStatusProvisioning},
	)
	svc := NewOrderService(repo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)

	// 注入任务读取器：列表必须把任务状态/错误挂到订单上。
	svc.SetProvisionTaskReader(func(_ context.Context, ids []uint64) (map[uint64]*model.ProvisionTask, error) {
		return map[uint64]*model.ProvisionTask{
			2: {ID: 20, OrderID: 2, Status: model.ProvisionTaskManual, LastError: "上游余额不足"},
		}, nil
	})
	resp, err := svc.List(context.Background(), dtoOrderListQuery(0, 0))
	if err != nil {
		t.Fatalf("List 应成功: %v", err)
	}
	if resp.Meta.Page != 1 || resp.Meta.PageSize != 10 || resp.Meta.Total != 2 {
		t.Fatalf("分页兜底不符: %+v", resp.Meta)
	}
	byID := map[uint64]string{}
	for _, it := range resp.Items {
		byID[it.ID] = it.ProvisionStatus + "|" + it.ProvisionError
	}
	if byID[1] != "|" {
		t.Fatalf("无任务订单应不带履约状态, got %q", byID[1])
	}
	if byID[2] != "manual|上游余额不足" {
		t.Fatalf("任务状态未挂载: %q", byID[2])
	}

	// 未注入读取器：列表仍可用，只是不带履约状态。
	plain := NewOrderService(repo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)
	resp2, err := plain.List(context.Background(), dtoOrderListQuery(0, 0))
	if err != nil || len(resp2.Items) != 2 {
		t.Fatalf("无读取器时列表应正常: %v %d", err, len(resp2.Items))
	}
}

func TestStatsAggregatesAndGuardsZero(t *testing.T) {
	now := time.Now()
	repo := newFakeOrderRepo()
	repo.statsCountToday = 3
	repo.statsSalesToday = 450
	repo.statsTotalN = 10
	repo.statsTotalSales = 1200
	repo.statsCounts = []model.OrderStatusCount{{Status: model.OrderStatusActive, Count: 8}}
	repo.statsTrend = []model.OrderTrendPoint{{Date: now.Format("2006-01-02"), Sales: 100, Orders: 2}}
	refunds := newFakeRefundRepo()
	refunds.sumRefunded = 60
	svc := NewOrderService(repo, newFakeItemRepo(), refunds, &fakeProvision{}).(*orderService)

	resp, err := svc.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats 应成功: %v", err)
	}
	if resp.TodayOrders != 3 || resp.TodaySales != 450 {
		t.Fatalf("今日口径不符: %+v", resp)
	}
	if resp.AvgOrderValue != 120 {
		t.Fatalf("客单价 = %v, want 120", resp.AvgOrderValue)
	}
	if resp.RefundRate != 5 {
		t.Fatalf("退款率 = %v, want 5%%", resp.RefundRate)
	}
	if len(resp.StatusDistribution) != 1 || resp.StatusDistribution[0].Status != model.OrderStatusActive {
		t.Fatalf("状态分布不符: %+v", resp.StatusDistribution)
	}
	if len(resp.Trend) != 7 {
		t.Fatalf("趋势应补齐 7 天, got %d", len(resp.Trend))
	}
	last := resp.Trend[len(resp.Trend)-1]
	if last.Date != now.Format("2006-01-02") || last.Sales != 100 || last.Orders != 2 {
		t.Fatalf("今日趋势点不符: %+v", last)
	}

	// 零订单/零销售：不除零，均回落 0。
	zero := NewOrderService(newFakeOrderRepo(), newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)
	zresp, err := zero.Stats(context.Background())
	if err != nil || zresp.AvgOrderValue != 0 || zresp.RefundRate != 0 {
		t.Fatalf("零值口径应安全: %+v %v", zresp, err)
	}
}

func TestStatsPropagatesRepoError(t *testing.T) {
	repo := newFakeOrderRepo()
	repo.statsErr = errors.New("db down")
	svc := NewOrderService(repo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)
	if _, err := svc.Stats(context.Background()); err == nil {
		t.Fatal("仓储错误应上抛")
	}
}

func TestListRefundsBuildsOrderNos(t *testing.T) {
	repo := newFakeOrderRepo(&model.Order{ID: 7, OrderNo: "O7"})
	refunds := newFakeRefundRepo()
	now := time.Now()
	refunds.refunds[1] = &model.OrderRefund{
		ID: 1, RefundNo: "RF1", OrderID: 7, Amount: 10, Status: model.RefundStatusDone,
		CreatedAt: now, UpdatedAt: now,
	}
	svc := NewOrderService(repo, newFakeItemRepo(), refunds, &fakeProvision{}).(*orderService)

	resp, err := svc.ListRefunds(context.Background(), dtoRefundListQuery())
	if err != nil {
		t.Fatalf("ListRefunds 应成功: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].OrderNo != "O7" {
		t.Fatalf("退款单应回填订单号: %+v", resp.Items)
	}
	if resp.Items[0].RefundMode != model.RefundModeBalance {
		t.Fatalf("空 refund_mode 应回落余额退回: %q", resp.Items[0].RefundMode)
	}
	if resp.Meta.Page != 1 || resp.Meta.PageSize != 10 || resp.Meta.Total != 1 {
		t.Fatalf("退款分页兜底不符: %+v", resp.Meta)
	}

	// 订单号查不到（订单被清理）：不报错，仅订单号留空。
	orphan := newFakeRefundRepo()
	orphan.refunds[2] = &model.OrderRefund{ID: 2, RefundNo: "RF2", OrderID: 999, CreatedAt: now, UpdatedAt: now}
	svc2 := NewOrderService(newFakeOrderRepo(), newFakeItemRepo(), orphan, &fakeProvision{}).(*orderService)
	resp2, err := svc2.ListRefunds(context.Background(), dtoRefundListQuery())
	if err != nil || resp2.Items[0].OrderNo != "" {
		t.Fatalf("缺失订单号应留空不报错: %+v %v", resp2.Items, err)
	}
}

// —— 错误映射 / 工具 ——

func TestMapOrderRefundErrs(t *testing.T) {
	if err := mapOrderErr(nil); err != nil {
		t.Fatalf("nil 应透传: %v", err)
	}
	if err := mapOrderErr(gormErrNotFound); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("gorm 未找到应映射 ErrOrderNotFound: %v", err)
	}
	other := errors.New("sql timeout")
	if err := mapOrderErr(other); !errors.Is(err, other) {
		t.Fatalf("其他错误应透传: %v", err)
	}
	if err := mapRefundErr(gormErrNotFound); !errors.Is(err, ErrRefundNotFound) {
		t.Fatalf("gorm 未找到应映射 ErrRefundNotFound: %v", err)
	}
}

func TestPayableAmountAndFormatTime(t *testing.T) {
	payTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	// 算价快照优先，兼容未写快照的存量订单回落总额。
	o := &model.Order{FinalAmount: 80, TotalAmount: 88}
	if v := payableAmount(o); v != 80 {
		t.Fatalf("算价快照应优先: %v", v)
	}
	o.FinalAmount = 0
	if v := payableAmount(o); v != 88 {
		t.Fatalf("无快照应回落应付总额: %v", v)
	}
	if formatTime(nil) != "" {
		t.Fatalf("空时间应输出空串")
	}
	if got := formatTime(&payTime); got != "2026-01-02T03:04:05Z" {
		t.Fatalf("formatTime = %q", got)
	}
}

// —— 默认履约适配器 ——

func TestDefaultProvisionAdapterStates(t *testing.T) {
	a := NewDefaultProvisionAdapter()
	paid := &model.Order{Status: model.OrderStatusPaid}
	if err := a.Activate(context.Background(), paid); err != nil || paid.Status != model.OrderStatusProvisioning {
		t.Fatalf("paid 应推进到 provisioning: %v %s", err, paid.Status)
	}
	if err := a.Activate(context.Background(), paid); err != nil || paid.Status != model.OrderStatusActive {
		t.Fatalf("provisioning 应推进到 active: %v %s", err, paid.Status)
	}
	if err := a.Activate(context.Background(), paid); err != nil {
		t.Fatalf("active 应幂等返回: %v", err)
	}
	if err := a.Activate(context.Background(), &model.Order{Status: model.OrderStatusPending}); !errors.Is(err, ErrStatusConflict) {
		t.Fatalf("pending 应拒绝: %v", err)
	}
}

func TestAdvanceLegacyStates(t *testing.T) {
	a := &UpstreamProvisionAdapter{}
	o := &model.Order{Status: model.OrderStatusPaid}
	if err := a.advanceLegacy(o); err != nil || o.Status != model.OrderStatusProvisioning {
		t.Fatalf("legacy paid 应推进: %v %s", err, o.Status)
	}
	if err := a.advanceLegacy(o); err != nil || o.Status != model.OrderStatusActive {
		t.Fatalf("legacy provisioning 应推进: %v %s", err, o.Status)
	}
	if err := a.advanceLegacy(&model.Order{Status: model.OrderStatusCancelled}); !errors.Is(err, ErrStatusConflict) {
		t.Fatalf("legacy 非法状态应拒绝: %v", err)
	}
}

// —— 上游履约辅佐函数（纯函数） ——

func TestDownstreamTokenFlow(t *testing.T) {
	a := &UpstreamProvisionAdapter{}
	// 已有 32 位令牌：直接复用，不落库。
	kept := &model.Order{ID: 1, DownstreamToken: "0123456789abcdef0123456789abcdef"}
	tok, err := a.downstreamToken(context.Background(), kept)
	if err != nil || tok != kept.DownstreamToken || kept.DownstreamToken != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("既有令牌应复用: %q %v", tok, err)
	}
	// 无令牌：生成并落库。
	recorded := ""
	a2 := &UpstreamProvisionAdapter{deps: ProvisionDeps{RecordDownstreamToken: func(_ context.Context, id uint64, t string) error {
		recorded = t
		return nil
	}}}
	o := &model.Order{ID: 2}
	tok, err = a2.downstreamToken(context.Background(), o)
	if err != nil || len(tok) != 32 || recorded != tok || o.DownstreamToken != tok {
		t.Fatalf("应生成 32 位令牌并固化: %q %v", tok, err)
	}
	// 落库失败：仍返回令牌（不阻断履约），但不上抛？——语义是上抛供调用方告警。
	a3 := &UpstreamProvisionAdapter{deps: ProvisionDeps{RecordDownstreamToken: func(context.Context, uint64, string) error {
		return errors.New("disk full")
	}}}
	if tok, err := a3.downstreamToken(context.Background(), &model.Order{ID: 3}); err == nil || len(tok) != 32 {
		t.Fatalf("落库失败应上抛且令牌仍可用: %q %v", tok, err)
	}
	// 未装配落库钩子：只返回令牌。
	a4 := &UpstreamProvisionAdapter{}
	o4 := &model.Order{ID: 4}
	if tok, err := a4.downstreamToken(context.Background(), o4); err != nil || len(tok) != 32 {
		t.Fatalf("无落库钩子也应生成令牌: %q %v", tok, err)
	}
}

func TestDownstreamURLFallbacks(t *testing.T) {
	a := &UpstreamProvisionAdapter{deps: ProvisionDeps{PublicURL: func(context.Context) string { return "  https://shop.example.com " }}}
	if got := a.downstreamURL(context.Background()); got != "https://shop.example.com" {
		t.Fatalf("合法 URL 应裁剪后使用: %q", got)
	}
	a2 := &UpstreamProvisionAdapter{deps: ProvisionDeps{PublicURL: func(context.Context) string { return "shop.example.com" }}}
	if got := a2.downstreamURL(context.Background()); got != defaultDownstreamURL {
		t.Fatalf("缺协议应回落占位: %q", got)
	}
	if got := (&UpstreamProvisionAdapter{}).downstreamURL(context.Background()); got != defaultDownstreamURL {
		t.Fatalf("未装配应回落占位: %q", got)
	}
}

func TestOrderBillingCycleAndSelections(t *testing.T) {
	if got := orderBillingCycle(&model.Order{Cycle: "monthly", PriceModel: "fixed"}); got != "monthly" {
		t.Fatalf("cycle 权威: %q", got)
	}
	if got := orderBillingCycle(&model.Order{PriceModel: "yearly"}); got != "yearly" {
		t.Fatalf("cycle 空应回落 price_model: %q", got)
	}
	// 选配项解析：空 / 非法 / 合法 / 空 map。
	if orderConfigSelections(&model.Order{}) != nil {
		t.Fatal("空 JSON 应返回 nil")
	}
	if orderConfigSelections(&model.Order{ConfigOptions: "{broken"}) != nil {
		t.Fatal("非法 JSON 应返回 nil")
	}
	sel := orderConfigSelections(&model.Order{ConfigOptions: `{"network":"vpc"}`})
	if sel["network"] != "vpc" {
		t.Fatalf("合法选配应解析: %+v", sel)
	}
	if orderConfigSelections(&model.Order{ConfigOptions: `{}`}) != nil {
		t.Fatal("空对象应视为无选配")
	}
}

func TestPreferredPasswordAndOptions(t *testing.T) {
	// 生产链路里 raw 是 BuildProvisionRequest 返回的请求结构体（反射取 ConfigOptions 字段）。
	raw := testProvisionRequest{ConfigOptions: map[string]interface{}{"root_password": "  pass@123 ", "rootpass": nil}}
	if got := preferredPassword(raw); got != "pass@123" {
		t.Fatalf("应取第一个非空键: %q", got)
	}
	if got := firstNonEmptyOption(raw, "rootpass", "root_password"); got != "pass@123" {
		t.Fatalf("nil 值应跳过: %q", got)
	}
	if got := firstNonEmptyOption(raw, "missing"); got != "" {
		t.Fatalf("缺键应返回空: %q", got)
	}
	if got := firstNonEmptyOption(nil); got != "" {
		t.Fatalf("nil 入参应返回空: %q", got)
	}
}

func TestProviderIDAndExtract(t *testing.T) {
	if providerID(nil) != 0 {
		t.Fatal("nil 应返回 0")
	}
	var p *testProvisionRequest
	if providerID(p) != 0 {
		t.Fatal("nil 指针应返回 0")
	}
	if got := providerID(provisionRequestForTest()); got != 7 {
		t.Fatalf("结构体应取 ProviderID: %d", got)
	}
	if got := providerID(map[string]string{}); got != 0 {
		t.Fatalf("无字段类型应返回 0: %d", got)
	}
	opts := extractConfigOptions(provisionRequestForTest())
	if opts["upstream_pid"] != "529" {
		t.Fatalf("应提取 ConfigOptions: %+v", opts)
	}
	if opts := extractConfigOptions(nil); opts == nil || len(opts) != 0 {
		t.Fatalf("nil 应返回空 map: %+v", opts)
	}
}

func TestToUintKinds(t *testing.T) {
	cases := []struct {
		v    interface{}
		want uint64
	}{
		{int(3), 3}, {int8(4), 4}, {uint(5), 5}, {uint64(6), 6},
		{float64(7.9), 7}, {"8", 0},
	}
	for _, c := range cases {
		if got := toUint(reflectValueOf(c.v)); got != c.want {
			t.Fatalf("toUint(%v) = %d, want %d", c.v, got, c.want)
		}
	}
}

// —— 开通工作池 ——

type fakeTaskStore struct {
	claimQ     []*model.ProvisionTask
	claimErr   error
	success    []uint64
	retries    []string // "id:msg"
	manuals    []string
	released   int64
	releaseErr error
}

func (s *fakeTaskStore) Claim(context.Context, time.Duration) (*model.ProvisionTask, error) {
	if s.claimErr != nil {
		return nil, s.claimErr
	}
	if len(s.claimQ) == 0 {
		return nil, nil
	}
	t := s.claimQ[0]
	s.claimQ = s.claimQ[1:]
	return t, nil
}
func (s *fakeTaskStore) MarkSuccess(_ context.Context, id uint64) error {
	s.success = append(s.success, id)
	return nil
}
func (s *fakeTaskStore) MarkRetry(_ context.Context, id uint64, msg string, _ time.Time) error {
	s.retries = append(s.retries, fmt.Sprintf("%d:%s", id, msg))
	return nil
}
func (s *fakeTaskStore) MarkManual(_ context.Context, id uint64, msg string) error {
	s.manuals = append(s.manuals, fmt.Sprintf("%d:%s", id, msg))
	return nil
}
func (s *fakeTaskStore) ReleaseStale(context.Context, time.Time) (int64, error) {
	return s.released, s.releaseErr
}

type fakeActivator struct {
	err error
}

func (f *fakeActivator) Activate(context.Context, uint64) error { return f.err }

func newTestWorker(store *fakeTaskStore, activator ProvisionActivator) *ProvisionWorker {
	return NewProvisionWorker(store, activator, nil, zap.NewNop(), ProvisionWorkerOptions{})
}

func TestWorkerProcessSuccessAndConflict(t *testing.T) {
	// 成功：MarkSuccess。
	s1 := &fakeTaskStore{}
	newTestWorker(s1, &fakeActivator{}).process(context.Background(), &model.ProvisionTask{ID: 1, OrderID: 10})
	if len(s1.success) != 1 || s1.success[0] != 1 {
		t.Fatalf("成功应标记: %+v", s1.success)
	}
	// 订单终态冲突：判成功收尾，不再重试。
	s2 := &fakeTaskStore{}
	newTestWorker(s2, &fakeActivator{err: ErrStatusConflict}).process(context.Background(), &model.ProvisionTask{ID: 2, OrderID: 11})
	if len(s2.success) != 1 || len(s2.retries) != 0 || len(s2.manuals) != 0 {
		t.Fatalf("状态冲突应判成功收尾: %+v %+v %+v", s2.success, s2.retries, s2.manuals)
	}
}

func TestWorkerProcessTerminalErrorGoesManual(t *testing.T) {
	notified := 0
	s := &fakeTaskStore{}
	w := NewProvisionWorker(s, &fakeActivator{err: &upstream.ProviderError{StatusCode: 400, Code: 1001, Msg: "余额不足"}},
		func(context.Context, *model.ProvisionTask) { notified++ }, zap.NewNop(), ProvisionWorkerOptions{})
	w.process(context.Background(), &model.ProvisionTask{ID: 3, OrderID: 12, Attempts: 1})
	if len(s.manuals) != 1 || notified != 1 || len(s.retries) != 0 {
		t.Fatalf("终端错误应首次即转人工并告警: %+v notified=%d", s.manuals, notified)
	}
	// 未注入通知器：只落人工队列不 panic。
	s2 := &fakeTaskStore{}
	newTestWorker(s2, &fakeActivator{err: &upstream.ProviderError{StatusCode: 422}}).process(
		context.Background(), &model.ProvisionTask{ID: 4, OrderID: 13})
	if len(s2.manuals) != 1 {
		t.Fatalf("4xx 无业务码也应转人工: %+v", s2.manuals)
	}
}

func TestWorkerProcessRetryAndMaxAttempts(t *testing.T) {
	// 网络类错误：按退避重试。
	s1 := &fakeTaskStore{}
	newTestWorker(s1, &fakeActivator{err: errors.New("connection reset")}).process(
		context.Background(), &model.ProvisionTask{ID: 5, OrderID: 14, Attempts: 1, MaxAttempts: 5})
	if len(s1.retries) != 1 || len(s1.manuals) != 0 {
		t.Fatalf("瞬时错误应重试: %+v %+v", s1.retries, s1.manuals)
	}
	// 达上限：转人工。
	s2 := &fakeTaskStore{}
	newTestWorker(s2, &fakeActivator{err: errors.New("still down")}).process(
		context.Background(), &model.ProvisionTask{ID: 6, OrderID: 15, Attempts: 5, MaxAttempts: 5})
	if len(s2.manuals) != 1 || len(s2.retries) != 0 {
		t.Fatalf("达上限应转人工: %+v %+v", s2.manuals, s2.retries)
	}
	// MaxAttempts=0 视为不限：持续重试。
	s3 := &fakeTaskStore{}
	newTestWorker(s3, &fakeActivator{err: errors.New("down")}).process(
		context.Background(), &model.ProvisionTask{ID: 7, OrderID: 16, Attempts: 99})
	if len(s3.retries) != 1 || len(s3.manuals) != 0 {
		t.Fatalf("不限次数应持续重试: %+v %+v", s3.retries, s3.manuals)
	}
}

func TestIsTerminalProvisionErrorWrapped(t *testing.T) {
	// 判据本体已在 provision_terminal_test.go 表驱动锁定，这里只补两条
	// 边界：带业务码的 4xx 与零值结构体（worker.process 依赖该分支）。
	if !isTerminalProvisionError(&upstream.ProviderError{StatusCode: 422}) {
		t.Fatal("4xx 应判定终端")
	}
	if isTerminalProvisionError(&upstream.ProviderError{}) {
		t.Fatal("零值 ProviderError 应可重试")
	}
}

func TestWorkerRetryDelayCaps(t *testing.T) {
	w := NewProvisionWorker(nil, nil, nil, zap.NewNop(), ProvisionWorkerOptions{})
	if d := w.retryDelay(0); d != w.backoffMin {
		t.Fatalf("attempts<1 应按 1 退避: %v", d)
	}
	if d := w.retryDelay(2); d != 2*w.backoffMin {
		t.Fatalf("线性退避: %v", d)
	}
	if d := w.retryDelay(100); d != 30*time.Minute {
		t.Fatalf("退避应封顶 30 分钟: %v", d)
	}
}

func TestWorkerLoopStopsOnCancelAndClaimErr(t *testing.T) {
	// 领取到错误（非取消）：记日志继续轮询，直到 ctx 取消退出。
	s := &fakeTaskStore{claimErr: errors.New("db hiccup")}
	ctx, cancel := context.WithCancel(context.Background())
	w := newTestWorker(s, &fakeActivator{})
	done := make(chan struct{})
	go func() { w.loop(ctx); close(done) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("loop 未随 ctx 取消退出")
	}
}

func TestWorkerStartStopIdempotent(t *testing.T) {
	s := &fakeTaskStore{}
	w := newTestWorker(s, &fakeActivator{})
	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	w.Start(ctx) // 幂等：once 保护，不重复起协程
	cancel()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	w.Stop(stopCtx) // 等所有协程收尾
}

func TestWorkerReapStale(t *testing.T) {
	s := &fakeTaskStore{released: 2}
	newTestWorker(s, &fakeActivator{}).reapStale(context.Background())
	if s.released != 2 {
		t.Fatalf("回收数应透传: %d", s.released)
	}
	s2 := &fakeTaskStore{releaseErr: errors.New("locked")}
	newTestWorker(s2, &fakeActivator{}).reapStale(context.Background()) // 错误只记日志不 panic
}

// —— 任务投递器 ——

type fakeProvisionTaskRepo struct {
	enqueued *model.ProvisionTask
	err      error
}

// 其余方法仅为满足 ProvisionTaskRepository 接口的桩（enqueuer 只用 Enqueue）。
func (r *fakeProvisionTaskRepo) Claim(context.Context, time.Duration) (*model.ProvisionTask, error) {
	return nil, nil
}
func (r *fakeProvisionTaskRepo) MarkSuccess(context.Context, uint64) error { return nil }
func (r *fakeProvisionTaskRepo) MarkRetry(context.Context, uint64, string, time.Time) error {
	return nil
}
func (r *fakeProvisionTaskRepo) MarkManual(context.Context, uint64, string) error { return nil }
func (r *fakeProvisionTaskRepo) ReleaseStale(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (r *fakeProvisionTaskRepo) FindByOrderID(context.Context, uint64) (*model.ProvisionTask, error) {
	return nil, nil
}
func (r *fakeProvisionTaskRepo) FindByOrderIDs(context.Context, []uint64) (map[uint64]*model.ProvisionTask, error) {
	return nil, nil
}
func (r *fakeProvisionTaskRepo) ListManual(context.Context, int, int) ([]model.ProvisionTask, int64, error) {
	return nil, 0, nil
}

func (r *fakeProvisionTaskRepo) Enqueue(_ context.Context, task *model.ProvisionTask) error {
	if r.err != nil {
		return r.err
	}
	r.enqueued = task
	return nil
}

func TestEnqueuerPaths(t *testing.T) {
	repo := &fakeProvisionTaskRepo{}
	e := NewProvisionEnqueuer(repo, zap.NewNop())
	if err := e.EnqueueProvision(context.Background(), nil, ""); err != nil {
		t.Fatalf("nil 订单应无操作: %v", err)
	}
	o := &model.Order{ID: 9, UserID: 3, ProductID: 4}
	if err := e.EnqueueProvision(context.Background(), o, "upstream"); err != nil {
		t.Fatalf("投递应成功: %v", err)
	}
	if repo.enqueued == nil || repo.enqueued.OrderID != 9 || repo.enqueued.Status != model.ProvisionTaskPending ||
		repo.enqueued.MaxAttempts != model.DefaultProvisionMaxAttempts || repo.enqueued.SourceMode != "upstream" {
		t.Fatalf("任务字段不符: %+v", repo.enqueued)
	}
	fail := &fakeProvisionTaskRepo{err: errors.New("uk conflict")}
	if err := NewProvisionEnqueuer(fail, zap.NewNop()).EnqueueProvision(context.Background(), o, ""); err == nil {
		t.Fatal("仓储错误应上抛")
	}
}

// —— 上游履约：密码三件套（provision 内的加密/解密分支） ——

func TestProvisionPasswordFlow(t *testing.T) {
	key := "test-secret-key-32-bytes-long-xxxx"
	// got 由 CreateInstance 闭包写入：必须传指针的指针，闭包执行时才能回填到调用方。
	mkAdapter := func(order *model.Order, ensure func(context.Context, uint64, string) (string, error), got **pkgmodel.CreateInstanceRequest) *UpstreamProvisionAdapter {
		return NewUpstreamProvisionAdapter(ProvisionDeps{
			BuildProvisionRequest: func(context.Context, uint64, string, string, string, map[string]string) (interface{}, error) {
				return provisionRequestForTest(), nil
			},
			BuildProviderConfig: func(context.Context, uint64) (*upstream.ProviderConfig, error) {
				return &upstream.ProviderConfig{ID: 7, Type: "zjmf"}, nil
			},
			CreateInstance: func(_ context.Context, _ *upstream.ProviderConfig, req *pkgmodel.CreateInstanceRequest) (*pkgmodel.StandardInstance, error) {
				*got = req
				return &pkgmodel.StandardInstance{Status: pkgmodel.InstanceStatusCreating}, nil
			},
			RecordInstance: func(context.Context, *pkgmodel.StandardInstance, *model.Order) error {
				return nil
			},
			EnsureCredentialPassword: ensure,
			EncryptKey:               key,
			Logger:                   zap.NewNop(),
		})
	}

	t.Run("首次生成密文后解密下发", func(t *testing.T) {
		order := &model.Order{ID: 1, ProductID: 5, Status: model.OrderStatusPaid}
		enc, _ := credentials.EncryptSecret("op-password", key)
		var got *pkgmodel.CreateInstanceRequest
		a := mkAdapter(order, func(context.Context, uint64, string) (string, error) { return enc, nil }, &got)
		if err := a.Activate(context.Background(), order); err != nil {
			t.Fatalf("Activate 应成功: %v", err)
		}
		if got == nil || got.Password != "op-password" {
			t.Fatalf("应解密下发运营密码: %+v", got)
		}
		if order.CredentialPassword != enc {
			t.Fatal("密文应固化在订单上（重试复用）")
		}
	})
	t.Run("固化失败只告警不阻断", func(t *testing.T) {
		order := &model.Order{ID: 2, ProductID: 5, Status: model.OrderStatusPaid}
		var got *pkgmodel.CreateInstanceRequest
		a := mkAdapter(order, func(context.Context, uint64, string) (string, error) {
			return "", errors.New("crypto unavailable")
		}, &got)
		if err := a.Activate(context.Background(), order); err != nil {
			t.Fatalf("密码失败不应阻断开通: %v", err)
		}
		if got == nil || got.Password != "" {
			t.Fatalf("失败时不应带密码: %+v", got)
		}
	})
	t.Run("解密失败不带密文下发", func(t *testing.T) {
		order := &model.Order{ID: 3, ProductID: 5, Status: model.OrderStatusPaid, CredentialPassword: "enc:v1:corrupted"}
		var got *pkgmodel.CreateInstanceRequest
		a := mkAdapter(order, nil, &got)
		if err := a.Activate(context.Background(), order); err != nil {
			t.Fatalf("Activate 应成功: %v", err)
		}
		if got == nil || got.Password != "" {
			t.Fatalf("解密失败绝不能把密文当明文: %+v", got)
		}
	})
}
