package service

// 确认收款（场外收款人工入账）单元测试。
//
// 覆盖点都是资金安全的关键分支，不是形式覆盖：
//   - 支付方式必填 / 归一 / 拒绝"支付模式"桶名（否则退款去向判定失效）；
//   - 非待支付订单拒绝（防重复收款）；
//   - CAS 落空（并发关单）时不留下"已 paid 但无台账"的坏数据；
//   - 台账登记失败必须回滚订单状态（账实不符是财务事故）；
//   - 余额支付走真实扣款且余额不足整体失败；
//   - 提醒渠道归一与去重、发送失败记为跳过而不是让收款失败。

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/model"
)

// fakeOfflineRecorder 台账登记桩：记录入参并可控地返回错误/空结果。
type fakeOfflineRecorder struct {
	calls    int
	lastRec  OfflinePaymentRecord
	lastOrd  *model.Order
	err      error
	nilResp  bool
	respNo   string
	respNew  bool
	recorded bool
}

func (f *fakeOfflineRecorder) Record(_ context.Context, order *model.Order, req OfflinePaymentRecord) (*OfflinePaymentRecordResult, error) {
	f.calls++
	f.lastRec = req
	f.lastOrd = order
	if f.err != nil {
		return nil, f.err
	}
	if f.nilResp {
		return nil, nil
	}
	f.recorded = true
	no := f.respNo
	if no == "" {
		no = "P20260101000000TEST"
	}
	return &OfflinePaymentRecordResult{PaymentNo: no, Created: !f.respNew}, nil
}

// fakeBalanceDeductor 余额扣款桩。
type fakeBalanceDeductor struct {
	calls  int
	amount float64
	err    error
}

func (f *fakeBalanceDeductor) DeductOrderPayment(_ context.Context, _ *model.Order, amount float64, _ uint64) error {
	f.calls++
	f.amount = amount
	return f.err
}

// fakeNotifier 提醒发送桩。
type fakeNotifier struct {
	calls    int
	channels []string
	notified []string
	skipped  map[string]string
}

func (f *fakeNotifier) NotifyOrderPaid(_ context.Context, _ *model.Order, channels []string) ([]string, map[string]string) {
	f.calls++
	f.channels = channels
	return f.notified, f.skipped
}

func newConfirmTestService(repo *fakeOrderRepo) *orderService {
	return NewOrderService(repo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)
}

// confirmPendingOrder 构造一笔待支付订单（含应付金额），供确认收款用例使用。
// 与 order_service_test.go 的 pendingOrder 区分：这里带 FinalAmount（算价快照优先口径）。
func confirmPendingOrder(id uint64, no string) *model.Order {
	return &model.Order{
		ID: id, OrderNo: no, UserID: 9001, ProductName: "轻量云主机",
		Status: model.OrderStatusPending, TotalAmount: 30, FinalAmount: 30,
	}
}

// TestConfirmPaymentRequiresPayMethod 支付方式必填：留空与"支付模式"桶名都要被拒。
func TestConfirmPaymentRequiresPayMethod(t *testing.T) {
	for _, raw := range []string{"", "   ", "channel", "online", "ALIPAY!!"} {
		repo := newFakeOrderRepo(confirmPendingOrder(1, "A1"))
		svc := newConfirmTestService(repo)
		rec := &fakeOfflineRecorder{}
		svc.SetOfflinePaymentRecorder(rec)

		_, err := svc.ConfirmPayment(context.Background(), 1, dto.ConfirmPaymentRequest{PayMethod: raw}, 7, "运营甲")
		if !errors.Is(err, ErrPayMethodInvalid) {
			t.Fatalf("pay_method=%q 应被拒绝, got %v", raw, err)
		}
		if rec.calls != 0 {
			t.Fatalf("pay_method=%q 被拒时不应登记台账", raw)
		}
		if got := repo.orders[1].Status; got != model.OrderStatusPending {
			t.Fatalf("pay_method=%q 被拒时订单不应改状态, got %s", raw, got)
		}
	}
}

// TestConfirmPaymentNormalizesPayMethod 支付方式归一（微信别名与大小写）。
func TestConfirmPaymentNormalizesPayMethod(t *testing.T) {
	cases := map[string]string{
		"WECHATPAY": "wechat",
		"wxpay":     "wechat",
		"Alipay":    "alipay",
		"manual":    "manual",
		"offline":   "manual",
		"union-pay": "union-pay",
	}
	for raw, want := range cases {
		repo := newFakeOrderRepo(confirmPendingOrder(1, "A1"))
		svc := newConfirmTestService(repo)
		rec := &fakeOfflineRecorder{}
		svc.SetOfflinePaymentRecorder(rec)

		if _, err := svc.ConfirmPayment(context.Background(), 1, dto.ConfirmPaymentRequest{PayMethod: raw}, 7, "运营甲"); err != nil {
			t.Fatalf("pay_method=%q 应当被接受, got %v", raw, err)
		}
		if rec.lastRec.PayMethod != want {
			t.Fatalf("pay_method=%q 应归一为 %q, got %q", raw, want, rec.lastRec.PayMethod)
		}
		if got := repo.orders[1].PayMethod; got != want {
			t.Fatalf("订单 pay_method 应写 %q, got %q", want, got)
		}
	}
}

// TestConfirmPaymentOnlyForPending 仅待支付订单可确认收款（防重复收款）。
func TestConfirmPaymentOnlyForPending(t *testing.T) {
	for _, status := range []string{
		model.OrderStatusPaid, model.OrderStatusActive, model.OrderStatusRefunding,
		model.OrderStatusRefunded, model.OrderStatusCancelled, model.OrderStatusClosed,
	} {
		o := confirmPendingOrder(1, "A1")
		o.Status = status
		repo := newFakeOrderRepo(o)
		svc := newConfirmTestService(repo)
		rec := &fakeOfflineRecorder{}
		svc.SetOfflinePaymentRecorder(rec)

		_, err := svc.ConfirmPayment(context.Background(), 1, dto.ConfirmPaymentRequest{PayMethod: "alipay"}, 7, "运营甲")
		if !errors.Is(err, ErrStatusConflict) {
			t.Fatalf("状态 %s 应返回 ErrStatusConflict, got %v", status, err)
		}
		if rec.calls != 0 {
			t.Fatalf("状态 %s 不应登记台账", status)
		}
	}
}

// TestConfirmPaymentRecordsLedger 正常路径：登记台账 + 写支付方式/实收/到账时间/操作人。
func TestConfirmPaymentRecordsLedger(t *testing.T) {
	repo := newFakeOrderRepo(confirmPendingOrder(1, "A1"))
	svc := newConfirmTestService(repo)
	rec := &fakeOfflineRecorder{respNo: "P123"}
	svc.SetOfflinePaymentRecorder(rec)

	result, err := svc.ConfirmPayment(context.Background(), 1, dto.ConfirmPaymentRequest{
		PayMethod:   "alipay",
		EvidenceNo:  "202601010001",
		Remark:      "对公转账到账",
		ChannelCode: "alipay-main",
	}, 7, "运营甲")
	if err != nil {
		t.Fatalf("确认收款应成功, got %v", err)
	}
	if result.PaymentNo != "P123" {
		t.Fatalf("应回传台账单号, got %q", result.PaymentNo)
	}
	o := repo.orders[1]
	// 收款确认后立即投递履约（ConfirmPayment 内置 EnqueueProvision），终态为开通中。
	if result.ProvisionQueued != true {
		t.Fatalf("应已投递开通, got %+v", result)
	}
	if o.Status != model.OrderStatusProvisioning {
		t.Fatalf("订单应经 paid 推进为开通中, got %s", o.Status)
	}
	if o.PayMethod != "alipay" {
		t.Fatalf("应记录支付方式, got %q", o.PayMethod)
	}
	if o.PaidAmount != 30 {
		t.Fatalf("实收应取订单应付 30, got %v", o.PaidAmount)
	}
	if o.PayTime == nil {
		t.Fatal("应记录到账时间")
	}
	if o.OperatorID != 7 {
		t.Fatalf("应记录操作人, got %d", o.OperatorID)
	}
	if !strings.Contains(o.Remark, "后台确认收款") || !strings.Contains(o.Remark, "202601010001") {
		t.Fatalf("备注应含入账摘要与凭证号, got %q", o.Remark)
	}
	if rec.lastRec.OperatorName != "运营甲" {
		t.Fatalf("台账应带登记人姓名, got %q", rec.lastRec.OperatorName)
	}
	if rec.lastRec.EvidenceNo != "202601010001" {
		t.Fatalf("台账应带凭证号, got %q", rec.lastRec.EvidenceNo)
	}
}

// TestConfirmPaymentCustomAmountAndTime 支持手填实收金额与到账时间（对公转账常有差异）。
func TestConfirmPaymentCustomAmountAndTime(t *testing.T) {
	repo := newFakeOrderRepo(confirmPendingOrder(1, "A1"))
	svc := newConfirmTestService(repo)
	svc.SetOfflinePaymentRecorder(&fakeOfflineRecorder{})

	result, err := svc.ConfirmPayment(context.Background(), 1, dto.ConfirmPaymentRequest{
		PayMethod:  "manual",
		PaidAmount: 28.5,
		PayTime:    "2026-01-02 09:30:00",
	}, 7, "运营甲")
	if err != nil {
		t.Fatalf("确认收款应成功, got %v", err)
	}
	o := repo.orders[1]
	if o.PaidAmount != 28.5 {
		t.Fatalf("实收应取手填值 28.5, got %v", o.PaidAmount)
	}
	want, _ := time.ParseInLocation("2006-01-02 15:04:05", "2026-01-02 09:30:00", time.Local)
	if o.PayTime == nil || !o.PayTime.Equal(want) {
		t.Fatalf("到账时间应取手填值, got %v", o.PayTime)
	}
	if result.Order.PaidAmount != 28.5 {
		t.Fatalf("返回的订单实收应为 28.5, got %v", result.Order.PaidAmount)
	}
}

// TestConfirmPaymentRejectsBadPayTime 到账时间格式非法要报错（不含糊解析）。
func TestConfirmPaymentRejectsBadPayTime(t *testing.T) {
	repo := newFakeOrderRepo(confirmPendingOrder(1, "A1"))
	svc := newConfirmTestService(repo)
	rec := &fakeOfflineRecorder{}
	svc.SetOfflinePaymentRecorder(rec)

	_, err := svc.ConfirmPayment(context.Background(), 1, dto.ConfirmPaymentRequest{
		PayMethod: "alipay", PayTime: "2026/01/02",
	}, 7, "运营甲")
	if !errors.Is(err, ErrPayTimeInvalid) {
		t.Fatalf("非法时间应返回 ErrPayTimeInvalid, got %v", err)
	}
	if rec.calls != 0 {
		t.Fatal("时间非法时不应登记台账")
	}
}

// TestConfirmPaymentRollsBackOnLedgerFailure 台账登记失败必须回滚订单状态。
//
// 这是本功能最关键的不变量：**不能出现"订单说收了钱、账上查不到"**。
func TestConfirmPaymentRollsBackOnLedgerFailure(t *testing.T) {
	repo := newFakeOrderRepo(confirmPendingOrder(1, "A1"))
	svc := newConfirmTestService(repo)
	svc.SetOfflinePaymentRecorder(&fakeOfflineRecorder{err: errors.New("支付中心不可用")})

	_, err := svc.ConfirmPayment(context.Background(), 1, dto.ConfirmPaymentRequest{PayMethod: "alipay"}, 7, "运营甲")
	if err == nil {
		t.Fatal("台账登记失败应报错")
	}
	if got := repo.orders[1].Status; got != model.OrderStatusPending {
		t.Fatalf("台账失败后订单应退回待支付, got %s", got)
	}
}

// TestConfirmPaymentRejectsWhenLedgerUnavailable 未装配台账能力时拒绝而不是静默放过。
func TestConfirmPaymentRejectsWhenLedgerUnavailable(t *testing.T) {
	repo := newFakeOrderRepo(confirmPendingOrder(1, "A1"))
	svc := newConfirmTestService(repo)

	_, err := svc.ConfirmPayment(context.Background(), 1, dto.ConfirmPaymentRequest{PayMethod: "alipay"}, 7, "运营甲")
	if !errors.Is(err, ErrOfflinePaymentUnavailable) {
		t.Fatalf("未装配台账应返回 ErrOfflinePaymentUnavailable, got %v", err)
	}
	if got := repo.orders[1].Status; got != model.OrderStatusPending {
		t.Fatalf("订单应保持待支付, got %s", got)
	}
}

// TestConfirmPaymentBalanceDeducts 余额支付走真实扣款，不登记线下台账。
func TestConfirmPaymentBalanceDeducts(t *testing.T) {
	repo := newFakeOrderRepo(confirmPendingOrder(1, "A1"))
	svc := newConfirmTestService(repo)
	rec := &fakeOfflineRecorder{}
	deductor := &fakeBalanceDeductor{}
	svc.SetOfflinePaymentRecorder(rec)
	svc.SetBalanceDeductor(deductor)

	if _, err := svc.ConfirmPayment(context.Background(), 1, dto.ConfirmPaymentRequest{PayMethod: "balance"}, 7, "运营甲"); err != nil {
		t.Fatalf("余额支付确认收款应成功, got %v", err)
	}
	if deductor.calls != 1 || deductor.amount != 30 {
		t.Fatalf("应按订单应付扣款 30, calls=%d amount=%v", deductor.calls, deductor.amount)
	}
	if rec.calls != 0 {
		t.Fatal("余额支付不应登记线下台账（钱本就在平台内）")
	}
}

// TestConfirmPaymentBalanceInsufficientRollsBack 余额不足：整体失败并回滚状态。
func TestConfirmPaymentBalanceInsufficientRollsBack(t *testing.T) {
	repo := newFakeOrderRepo(confirmPendingOrder(1, "A1"))
	svc := newConfirmTestService(repo)
	svc.SetBalanceDeductor(&fakeBalanceDeductor{err: ErrInsufficientBalance})

	_, err := svc.ConfirmPayment(context.Background(), 1, dto.ConfirmPaymentRequest{PayMethod: "balance"}, 7, "运营甲")
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("应透出余额不足, got %v", err)
	}
	if got := repo.orders[1].Status; got != model.OrderStatusPending {
		t.Fatalf("余额不足后订单应退回待支付, got %s", got)
	}
}

// TestConfirmPaymentNotifiesChosenChannels 提醒按勾选渠道发送，未勾选不发。
func TestConfirmPaymentNotifiesChosenChannels(t *testing.T) {
	repo := newFakeOrderRepo(confirmPendingOrder(1, "A1"))
	svc := newConfirmTestService(repo)
	svc.SetOfflinePaymentRecorder(&fakeOfflineRecorder{})
	notifier := &fakeNotifier{notified: []string{"mail"}}
	svc.SetPaymentNotifier(notifier)

	result, err := svc.ConfirmPayment(context.Background(), 1, dto.ConfirmPaymentRequest{
		PayMethod:      "alipay",
		NotifyChannels: []string{"mail"},
	}, 7, "运营甲")
	if err != nil {
		t.Fatalf("确认收款应成功, got %v", err)
	}
	if notifier.calls != 1 {
		t.Fatalf("应调用提醒发送一次, got %d", notifier.calls)
	}
	if len(result.Notified) != 1 || result.Notified[0] != "mail" {
		t.Fatalf("应回传已发渠道, got %v", result.Notified)
	}

	// 未勾选渠道：不调用提醒（避免默认群发打扰客户）。
	repo2 := newFakeOrderRepo(confirmPendingOrder(1, "A1"))
	svc2 := newConfirmTestService(repo2)
	svc2.SetOfflinePaymentRecorder(&fakeOfflineRecorder{})
	notifier2 := &fakeNotifier{}
	svc2.SetPaymentNotifier(notifier2)
	if _, err := svc2.ConfirmPayment(context.Background(), 1, dto.ConfirmPaymentRequest{PayMethod: "alipay"}, 7, "运营甲"); err != nil {
		t.Fatalf("确认收款应成功, got %v", err)
	}
	if notifier2.calls != 0 {
		t.Fatal("未勾选提醒渠道时不应发送")
	}
}

// TestNormalizePayMethodRejectsReservedBuckets 保留桶名与超长/非法字符必须被拒。
func TestNormalizePayMethodRejectsReservedBuckets(t *testing.T) {
	bad := []string{"", "channel", "online", "  ", "alipay with space", strings.Repeat("a", 21), "支付"}
	for _, raw := range bad {
		if got, ok := NormalizePayMethod(raw); ok {
			t.Fatalf("%q 应被拒绝, got %q", raw, got)
		}
	}
	// 合法值：渠道编码与自定义方式都要能落库（渠道类型可插件式新增，不写死白名单）。
	if got, ok := NormalizePayMethod(" WeChatPay "); !ok || got != "wechat" {
		t.Fatalf("微信别名应归一为 wechat, got %q ok=%v", got, ok)
	}
	if got, ok := NormalizePayMethod("my_channel-2"); !ok || got != "my_channel-2" {
		t.Fatalf("自定义渠道编码应被接受, got %q ok=%v", got, ok)
	}
}

// TestParsePayTimeAcceptsBothLayouts 两种时间格式都要能解析（表单值 / ISO）。
func TestParsePayTimeAcceptsBothLayouts(t *testing.T) {
	if _, err := parsePayTime("2026-01-02 09:30:00"); err != nil {
		t.Fatalf("表单格式应可解析: %v", err)
	}
	if _, err := parsePayTime("2026-01-02T09:30:00+08:00"); err != nil {
		t.Fatalf("RFC3339 应可解析: %v", err)
	}
	if _, err := parsePayTime("2026-01-02"); !errors.Is(err, ErrPayTimeInvalid) {
		t.Fatalf("只给日期应被拒, got %v", err)
	}
}

// TestFormatAmountTrimsZeros 金额展示去掉无意义的末尾零。
func TestFormatAmountTrimsZeros(t *testing.T) {
	cases := map[float64]string{30: "30", 30.5: "30.5", 30.55: "30.55", 0: "0"}
	for in, want := range cases {
		if got := formatAmount(in); got != want {
			t.Fatalf("formatAmount(%v) = %q, want %q", in, got, want)
		}
	}
}
