package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/model"
)

// fakeOrderRepo 内存订单仓储：覆盖订单服务核心链路用到的方法。
type fakeOrderRepo struct {
	orders    map[uint64]*model.Order
	byNo      map[string]*model.Order
	nextID    uint64
	transitOK bool // TransitionStatus 是否成功迁移（false 模拟并发 CAS 失败）

	// —— Stats 桩数据（批次四 D1b 读路径补测用） ——
	statsCountToday int64
	statsSalesToday float64
	statsTotalN     int64
	statsTotalSales float64
	statsCounts     []model.OrderStatusCount
	statsTrend      []model.OrderTrendPoint
	statsErr        error
}

func newFakeOrderRepo(items ...*model.Order) *fakeOrderRepo {
	r := &fakeOrderRepo{orders: map[uint64]*model.Order{}, byNo: map[string]*model.Order{}, transitOK: true}
	for _, o := range items {
		r.orders[o.ID] = o
		r.byNo[o.OrderNo] = o
	}
	return r
}

func (r *fakeOrderRepo) List(_ context.Context, q dto.OrderListQuery) ([]model.Order, int64, error) {
	out := make([]model.Order, 0, len(r.orders))
	for _, o := range r.orders {
		out = append(out, *o)
	}
	return out, int64(len(out)), nil
}
func (r *fakeOrderRepo) FindByID(_ context.Context, id uint64) (*model.Order, error) {
	o, ok := r.orders[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return o, nil
}
func (r *fakeOrderRepo) FindByNo(_ context.Context, no string) (*model.Order, error) {
	o, ok := r.byNo[no]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return o, nil
}
func (r *fakeOrderRepo) FindByIDs(_ context.Context, ids []uint64) ([]model.Order, error) {
	out := make([]model.Order, 0, len(ids))
	for _, id := range ids {
		if o, ok := r.orders[id]; ok {
			out = append(out, *o)
		}
	}
	return out, nil
}
func (r *fakeOrderRepo) Create(_ context.Context, item *model.Order) error {
	if item.ID == 0 {
		r.nextID++
		item.ID = r.nextID
	}
	cp := *item
	r.orders[item.ID] = &cp
	r.byNo[item.OrderNo] = &cp
	return nil
}
func (r *fakeOrderRepo) Update(_ context.Context, item *model.Order) error {
	if _, ok := r.orders[item.ID]; !ok {
		return gorm.ErrRecordNotFound
	}
	cp := *item
	r.orders[item.ID] = &cp
	r.byNo[item.OrderNo] = &cp
	return nil
}
func (r *fakeOrderRepo) SetUpstreamInvoiceID(context.Context, uint64, uint64) error { return nil }
func (r *fakeOrderRepo) SetDownstreamToken(context.Context, uint64, string) error   { return nil }
func (r *fakeOrderRepo) SetCredentialPassword(context.Context, uint64, string) error {
	return nil
}
func (r *fakeOrderRepo) SetCredentialPasswordIfEmpty(context.Context, uint64, string) error {
	return nil
}
func (r *fakeOrderRepo) ListProvisioningStale(context.Context, time.Time, int) ([]model.Order, error) {
	return nil, nil
}
func (r *fakeOrderRepo) ListExpiredPending(context.Context, time.Time, int) ([]model.Order, error) {
	return nil, nil
}
func (r *fakeOrderRepo) TransitionStatus(_ context.Context, id uint64, from, to string) (int64, error) {
	if !r.transitOK {
		return 0, nil
	}
	if o, ok := r.orders[id]; ok && o.Status == from {
		o.Status = to
		return 1, nil
	}
	return 0, nil
}
func (r *fakeOrderRepo) StatsCountToday(context.Context) (int64, error) {
	return r.statsCountToday, r.statsErr
}
func (r *fakeOrderRepo) StatsSalesToday(context.Context) (float64, error) {
	return r.statsSalesToday, r.statsErr
}
func (r *fakeOrderRepo) StatsTotal(context.Context) (int64, float64, error) {
	return r.statsTotalN, r.statsTotalSales, r.statsErr
}
func (r *fakeOrderRepo) CountByStatus(context.Context) ([]model.OrderStatusCount, error) {
	return r.statsCounts, r.statsErr
}
func (r *fakeOrderRepo) StatsTrend(context.Context, int) ([]model.OrderTrendPoint, error) {
	return r.statsTrend, r.statsErr
}

// fakeItemRepo 内存订单项仓储。
type fakeItemRepo struct{ items map[uint64][]model.OrderItem }

func newFakeItemRepo() *fakeItemRepo { return &fakeItemRepo{items: map[uint64][]model.OrderItem{}} }
func (r *fakeItemRepo) ListByOrder(_ context.Context, orderID uint64) ([]model.OrderItem, error) {
	return r.items[orderID], nil
}
func (r *fakeItemRepo) ListByOrders(_ context.Context, ids []uint64) ([]model.OrderItem, error) {
	var out []model.OrderItem
	for _, id := range ids {
		out = append(out, r.items[id]...)
	}
	return out, nil
}
func (r *fakeItemRepo) Create(_ context.Context, item *model.OrderItem) error {
	r.items[item.OrderID] = append(r.items[item.OrderID], *item)
	return nil
}

// fakeRefundRepo 内存退款单仓储。
type fakeRefundRepo struct {
	refunds map[uint64]*model.OrderRefund
	nextID  uint64
	// sumEffective 按订单返回已生效退款合计；未配置的订单返回 0。
	sumEffective map[uint64]float64
	// sumRefunded 全局已退款合计（Stats 的退款率口径，批次四 D1b）。
	sumRefunded float64
}

func newFakeRefundRepo() *fakeRefundRepo {
	return &fakeRefundRepo{refunds: map[uint64]*model.OrderRefund{}, sumEffective: map[uint64]float64{}}
}
func (r *fakeRefundRepo) List(_ context.Context, q dto.RefundListQuery) ([]model.OrderRefund, int64, error) {
	out := make([]model.OrderRefund, 0, len(r.refunds))
	for _, rf := range r.refunds {
		out = append(out, *rf)
	}
	return out, int64(len(out)), nil
}
func (r *fakeRefundRepo) FindByID(_ context.Context, id uint64) (*model.OrderRefund, error) {
	rf, ok := r.refunds[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return rf, nil
}
func (r *fakeRefundRepo) FindByOrder(_ context.Context, orderID uint64) ([]model.OrderRefund, error) {
	out := []model.OrderRefund{}
	for _, rf := range r.refunds {
		if rf.OrderID == orderID {
			out = append(out, *rf)
		}
	}
	return out, nil
}
func (r *fakeRefundRepo) Create(_ context.Context, item *model.OrderRefund) error {
	if item.ID == 0 {
		r.nextID++
		item.ID = r.nextID
	}
	cp := *item
	r.refunds[item.ID] = &cp
	return nil
}
func (r *fakeRefundRepo) Update(_ context.Context, item *model.OrderRefund) error {
	if _, ok := r.refunds[item.ID]; !ok {
		return gorm.ErrRecordNotFound
	}
	cp := *item
	r.refunds[item.ID] = &cp
	return nil
}
func (r *fakeRefundRepo) SumEffective(_ context.Context, orderID uint64) (float64, error) {
	// 优先用显式配置；否则按内存里 approved/refunded 状态求和。
	if v, ok := r.sumEffective[orderID]; ok {
		return v, nil
	}
	var sum float64
	for _, rf := range r.refunds {
		if rf.OrderID == orderID && (rf.Status == model.RefundStatusApproved || rf.Status == model.RefundStatusDone) {
			sum += rf.Amount
		}
	}
	return sum, nil
}
func (r *fakeRefundRepo) SumRefundedAmount(context.Context) (float64, error) {
	return r.sumRefunded, nil
}
func (r *fakeRefundRepo) SumChannelRefund(context.Context, uint64, *time.Time, *time.Time) (float64, float64, error) {
	return 0, 0, nil
}

// fakeProvision 记录 Activate 调用的履约适配器。
type fakeProvision struct {
	activated []uint64
	err       error
}

func (f *fakeProvision) Activate(_ context.Context, order *model.Order) error {
	f.activated = append(f.activated, order.ID)
	if f.err != nil {
		return f.err
	}
	switch order.Status {
	case model.OrderStatusPaid:
		order.Status = model.OrderStatusProvisioning
	case model.OrderStatusProvisioning:
		order.Status = model.OrderStatusActive
	case model.OrderStatusActive:
	default:
		return errors.New("invalid status for activate")
	}
	return nil
}

func pendingOrder(id uint64, no string) *model.Order {
	return &model.Order{ID: id, OrderNo: no, Status: model.OrderStatusPending, TotalAmount: 100, PaidAmount: 0}
}

// TestMarkPaidByChannel 支付回调：pending→paid、幂等、应付回填与渠道流水号追加。
func TestMarkPaidByChannel(t *testing.T) {
	repo := newFakeOrderRepo(pendingOrder(1, "UC1001"))
	svc := NewOrderService(repo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)

	moved, err := svc.MarkPaidByChannel(context.Background(), "UC1001", "wechat", "txn-1")
	if err != nil || !moved {
		t.Fatalf("首次回调应迁移成功: moved=%v err=%v", moved, err)
	}
	o, _ := repo.FindByID(context.Background(), 1)
	if o.Status != model.OrderStatusPaid || o.PayMethod != "wechat" || o.PaidAmount != 100 {
		t.Fatalf("迁移后状态/支付方式/实付不符: %+v", o)
	}
	if !strings.Contains(o.Remark, "txn-1") {
		t.Fatalf("渠道流水号应追加进备注: %q", o.Remark)
	}
	// 幂等：已 paid 再回调不迁移、不重复追加流水号。
	moved, err = svc.MarkPaidByChannel(context.Background(), "UC1001", "wechat", "txn-1")
	if err != nil || moved {
		t.Fatalf("重复回调应幂等返回 false: moved=%v err=%v", moved, err)
	}
	// 未知订单 → ErrOrderNotFound。
	if _, err := svc.MarkPaidByChannel(context.Background(), "NOPE", "wechat", ""); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("未知订单应返回 ErrOrderNotFound, got %v", err)
	}
}

// TestCancelReleasesHoldings 后台取消：状态机守卫、占位释放与库存回补都触发。
func TestCancelReleasesHoldings(t *testing.T) {
	repo := newFakeOrderRepo(pendingOrder(9, "UC9009"))
	svc := NewOrderService(repo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)

	var releasedInstance, releasedStock int
	svc.SetPendingInstanceReleaser(func(context.Context, uint64) (int64, error) {
		releasedInstance++
		return 1, nil
	})
	svc.SetStockReleaser(func(context.Context, *model.Order) error {
		releasedStock++
		return nil
	})
	if err := svc.Cancel(context.Background(), 9, 7, "op"); err != nil {
		t.Fatalf("待支付订单应可取消: %v", err)
	}
	o, _ := repo.FindByID(context.Background(), 9)
	if o.Status != model.OrderStatusCancelled || o.OperatorID != 7 {
		t.Fatalf("取消后状态/操作人不符: %+v", o)
	}
	if releasedInstance != 1 || releasedStock != 1 {
		t.Fatalf("取消应释放占位实例与库存: instance=%d stock=%d", releasedInstance, releasedStock)
	}
	// 已支付订单不可取消。
	paid := &model.Order{ID: 10, OrderNo: "UC9010", Status: model.OrderStatusPaid}
	repo.orders[10] = paid
	if err := svc.Cancel(context.Background(), 10, 7, "op"); !errors.Is(err, ErrStatusConflict) {
		t.Fatalf("已支付订单取消应返回状态冲突, got %v", err)
	}
}

// TestCreateRefundValidations 发起退款：金额、去向模式与状态机校验。
func TestCreateRefundValidations(t *testing.T) {
	repo := newFakeOrderRepo(&model.Order{ID: 1, OrderNo: "UC1", Status: model.OrderStatusActive, PaidAmount: 100})
	refunds := newFakeRefundRepo()
	svc := NewOrderService(repo, newFakeItemRepo(), refunds, &fakeProvision{}).(*orderService)

	// 超退拒单。
	if _, err := svc.CreateRefund(context.Background(), 1, dto.RefundCreateRequest{Amount: 150}, 7, "op"); !errors.Is(err, ErrRefundExceeded) {
		t.Fatalf("超退应返回 ErrRefundExceeded, got %v", err)
	}
	// 非法去向拒单。
	if _, err := svc.CreateRefund(context.Background(), 1, dto.RefundCreateRequest{Amount: 10, RefundMode: "nope"}, 7, "op"); !errors.Is(err, ErrRefundModeInvalid) {
		t.Fatalf("非法退款去向应返回 ErrRefundModeInvalid, got %v", err)
	}
	// channel 模式扣点非法拒单。
	if _, err := svc.CreateRefund(context.Background(), 1, dto.RefundCreateRequest{Amount: 10, RefundMode: "channel", FeeAmount: 20}, 7, "op"); !errors.Is(err, ErrRefundFeeInvalid) {
		t.Fatalf("扣点超本金应返回 ErrRefundFeeInvalid, got %v", err)
	}
	// 合法退款：active → refunding，退款单 pending。
	info, err := svc.CreateRefund(context.Background(), 1, dto.RefundCreateRequest{Amount: 30, Reason: "测试"}, 7, "op")
	if err != nil {
		t.Fatalf("合法退款应成功: %v", err)
	}
	if info.Status != model.RefundStatusPending || info.Amount != 30 {
		t.Fatalf("退款单状态/金额不符: %+v", info)
	}
	o, _ := repo.FindByID(context.Background(), 1)
	if o.Status != model.OrderStatusRefunding {
		t.Fatalf("服务中订单应转入退款中, got %s", o.Status)
	}
	// 待支付订单不可发起退款。
	pending := &model.Order{ID: 2, OrderNo: "UC2", Status: model.OrderStatusPending, PaidAmount: 0}
	repo.orders[2] = pending
	if _, err := svc.CreateRefund(context.Background(), 2, dto.RefundCreateRequest{Amount: 10}, 7, "op"); !errors.Is(err, ErrStatusConflict) {
		t.Fatalf("待支付订单退款应返回状态冲突, got %v", err)
	}
}

// TestApproveRefundFullAndPartial 审核通过：全额 → refunded；部分 → 回 active；钩子调用。
func TestApproveRefundFullAndPartial(t *testing.T) {
	order := &model.Order{ID: 1, OrderNo: "UC1", Status: model.OrderStatusRefunding, PaidAmount: 100}
	repo := newFakeOrderRepo(order)
	refunds := newFakeRefundRepo()
	svc := NewOrderService(repo, newFakeItemRepo(), refunds, &fakeProvision{}).(*orderService)

	// 预置一笔待审核退款（全额）。
	full := &model.OrderRefund{ID: 1, OrderID: 1, RefundNo: "RF1", Amount: 100, Status: model.RefundStatusPending}
	refunds.refunds[1] = full

	var hookRefundNo string
	svc.SetRefundHook(func(_ context.Context, _ uint64, _ string, _ uint64, _, _ float64, refundNo string) error {
		hookRefundNo = refundNo
		return nil
	})
	info, err := svc.ApproveRefund(context.Background(), 1, 7, "op", "")
	if err != nil {
		t.Fatalf("审核通过应成功: %v", err)
	}
	if info.Status != model.RefundStatusApproved {
		t.Fatalf("退款单应已通过: %+v", info)
	}
	o, _ := repo.FindByID(context.Background(), 1)
	if o.Status != model.OrderStatusRefunded {
		t.Fatalf("全额退款后订单应已退款, got %s", o.Status)
	}
	if hookRefundNo != "RF1" {
		t.Fatalf("退款冲减钩子应被调用且带退款单号, got %q", hookRefundNo)
	}

	// 部分退款场景：refunding 订单回 active。
	order2 := &model.Order{ID: 2, OrderNo: "UC2", Status: model.OrderStatusRefunding, PaidAmount: 100}
	repo.orders[2] = order2
	partial := &model.OrderRefund{ID: 2, OrderID: 2, RefundNo: "RF2", Amount: 30, Status: model.RefundStatusPending}
	refunds.refunds[2] = partial
	if _, err := svc.ApproveRefund(context.Background(), 2, 7, "op", ""); err != nil {
		t.Fatalf("部分退款审核应成功: %v", err)
	}
	o2, _ := repo.FindByID(context.Background(), 2)
	if o2.Status != model.OrderStatusActive {
		t.Fatalf("部分退款后订单应回服务中, got %s", o2.Status)
	}

	// 重复审核 → 状态冲突。
	if _, err := svc.ApproveRefund(context.Background(), 1, 7, "op", ""); !errors.Is(err, ErrStatusConflict) {
		t.Fatalf("重复审核应返回状态冲突, got %v", err)
	}
}

// TestRejectRefundRestoresActive 驳回：无在途退款回 active；有在途退款保持 refunding。
func TestRejectRefundRestoresActive(t *testing.T) {
	order := &model.Order{ID: 1, OrderNo: "UC1", Status: model.OrderStatusRefunding, PaidAmount: 100}
	repo := newFakeOrderRepo(order)
	refunds := newFakeRefundRepo()
	refunds.refunds[1] = &model.OrderRefund{ID: 1, OrderID: 1, RefundNo: "RF1", Amount: 30, Status: model.RefundStatusPending}
	refunds.refunds[2] = &model.OrderRefund{ID: 2, OrderID: 1, RefundNo: "RF2", Amount: 20, Status: model.RefundStatusApproved}
	svc := NewOrderService(repo, newFakeItemRepo(), refunds, &fakeProvision{}).(*orderService)

	// 在途的 approved 还在 → 驳回 pending 后订单保持 refunding。
	if _, err := svc.RejectRefund(context.Background(), 1, 7, "op", ""); err != nil {
		t.Fatalf("驳回应成功: %v", err)
	}
	o, _ := repo.FindByID(context.Background(), 1)
	if o.Status != model.OrderStatusRefunding {
		t.Fatalf("仍有在途退款时订单应保持退款中, got %s", o.Status)
	}

	// 清掉在途退款后驳回 → 回 active。
	refunds.refunds[2].Status = model.RefundStatusRejected
	refunds.refunds[3] = &model.OrderRefund{ID: 3, OrderID: 1, RefundNo: "RF3", Amount: 10, Status: model.RefundStatusPending}
	o.Status = model.OrderStatusRefunding
	if _, err := svc.RejectRefund(context.Background(), 3, 7, "op", ""); err != nil {
		t.Fatalf("驳回应成功: %v", err)
	}
	o, _ = repo.FindByID(context.Background(), 1)
	if o.Status != model.OrderStatusActive {
		t.Fatalf("无在途退款时驳回应回服务中, got %s", o.Status)
	}
}

// TestEnqueueProvisionPaths 开通投递：续费订单跳过；无队列回落同步 Activate。
func TestEnqueueProvisionPaths(t *testing.T) {
	// 续费订单：直接跳过。
	renewal := &model.Order{ID: 1, OrderNo: "UC1", Status: model.OrderStatusPaid, RenewalID: 55}
	repo := newFakeOrderRepo(renewal)
	prov := &fakeProvision{}
	svc := NewOrderService(repo, newFakeItemRepo(), newFakeRefundRepo(), prov).(*orderService)
	if err := svc.EnqueueProvision(context.Background(), 1); err != nil {
		t.Fatalf("续费订单应跳过开通: %v", err)
	}
	if len(prov.activated) != 0 {
		t.Fatalf("续费订单不应触发 Activate, got %v", prov.activated)
	}

	// 无队列：回落同步 Activate（paid → provisioning）。
	normal := &model.Order{ID: 2, OrderNo: "UC2", Status: model.OrderStatusPaid}
	repo.orders[2] = normal
	if err := svc.EnqueueProvision(context.Background(), 2); err != nil {
		t.Fatalf("同步开通应成功: %v", err)
	}
	if len(prov.activated) != 1 || prov.activated[0] != 2 {
		t.Fatalf("应同步触发订单 2 的 Activate, got %v", prov.activated)
	}

	// 有队列：走投递，不触发同步 Activate。
	enqueued := 0
	svc.SetProvisionQueue(&fakeQueue{fn: func(_ context.Context, o *model.Order, mode string) error {
		enqueued++
		if o.ID != 3 {
			t.Fatalf("投递订单不符: %v", o.ID)
		}
		return nil
	}})
	normal3 := &model.Order{ID: 3, OrderNo: "UC3", Status: model.OrderStatusPaid}
	repo.orders[3] = normal3
	if err := svc.EnqueueProvision(context.Background(), 3); err != nil {
		t.Fatalf("队列投递应成功: %v", err)
	}
	if enqueued != 1 {
		t.Fatalf("应投递一次任务, got %d", enqueued)
	}
}

// fakeQueue 开通任务队列最小实现。
type fakeQueue struct {
	fn func(ctx context.Context, order *model.Order, sourceMode string) error
}

func (q *fakeQueue) EnqueueProvision(ctx context.Context, order *model.Order, sourceMode string) error {
	return q.fn(ctx, order, sourceMode)
}

// TestFindByIDTranslatesNotFound 详情：查无订单翻译为 ErrOrderNotFound 且聚合明细。
func TestFindByIDTranslatesNotFound(t *testing.T) {
	repo := newFakeOrderRepo(&model.Order{ID: 1, OrderNo: "UC1", Status: model.OrderStatusActive})
	items := newFakeItemRepo()
	items.items[1] = []model.OrderItem{{ID: 1, OrderID: 1, ProductName: "云主机", Amount: 88}}
	refunds := newFakeRefundRepo()
	refunds.refunds[5] = &model.OrderRefund{ID: 5, OrderID: 1, RefundNo: "RF5", Amount: 10, Status: model.RefundStatusApproved}
	svc := NewOrderService(repo, items, refunds, &fakeProvision{}).(*orderService)

	detail, err := svc.FindByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("详情应成功: %v", err)
	}
	if len(detail.Items) != 1 || len(detail.Refunds) != 1 {
		t.Fatalf("明细聚合不符: items=%d refunds=%d", len(detail.Items), len(detail.Refunds))
	}
	if _, err := svc.FindByID(context.Background(), 999); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("未知订单应返回 ErrOrderNotFound, got %v", err)
	}
}
