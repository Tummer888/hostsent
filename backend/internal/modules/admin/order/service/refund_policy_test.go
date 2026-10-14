package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/model"
)

// fakePolicyRepo 内存退款策略仓储。
type fakePolicyRepo struct {
	items  []model.RefundPolicy
	nextID uint64
}

func newFakePolicyRepo(items ...model.RefundPolicy) *fakePolicyRepo {
	r := &fakePolicyRepo{}
	for _, it := range items {
		r.nextID++
		it.ID = r.nextID
		r.items = append(r.items, it)
	}
	return r
}

func (r *fakePolicyRepo) List(context.Context) ([]model.RefundPolicy, error) {
	return r.items, nil
}
func (r *fakePolicyRepo) FindByID(_ context.Context, id uint64) (*model.RefundPolicy, error) {
	for i := range r.items {
		if r.items[i].ID == id {
			return &r.items[i], nil
		}
	}
	return nil, errRecordNotFound
}
func (r *fakePolicyRepo) Create(_ context.Context, item *model.RefundPolicy) error {
	r.nextID++
	item.ID = r.nextID
	r.items = append(r.items, *item)
	return nil
}
func (r *fakePolicyRepo) Update(_ context.Context, item *model.RefundPolicy) error {
	for i := range r.items {
		if r.items[i].ID == item.ID {
			r.items[i] = *item
			return nil
		}
	}
	return errRecordNotFound
}
func (r *fakePolicyRepo) Delete(_ context.Context, id uint64) error {
	out := r.items[:0]
	for _, it := range r.items {
		if it.ID != id {
			out = append(out, it)
		}
	}
	r.items = out
	return nil
}

var errRecordNotFound = errors.New("record not found")

// fakeProductInfo 商品来源/分类读取桩。
type fakeProductInfo struct {
	sourceMode string
	categoryID uint64
}

func (f fakeProductInfo) ProductPolicyInfo(context.Context, uint64) (string, uint64, error) {
	return f.sourceMode, f.categoryID, nil
}

// fakeCategoryChain 分类祖先链桩：模拟「商品分类 → 父分类 → 根」。
type fakeCategoryChain struct{ chain []uint64 }

func (f fakeCategoryChain) CategoryChain(context.Context, uint64) ([]uint64, error) {
	return f.chain, nil
}

func newPolicySvc(repo *fakePolicyRepo, source string, categoryID uint64, chain []uint64) RefundPolicyService {
	return NewRefundPolicyService(repo, fakeProductInfo{sourceMode: source, categoryID: categoryID}, fakeCategoryChain{chain: chain})
}

// TestRefundPolicyResolveDeepestFirst 分类越具体优先级越高（商品分类优先于其父分类）。
func TestRefundPolicyResolveDeepestFirst(t *testing.T) {
	repo := newFakePolicyRepo(
		// 父分类（云主机）：7 天
		model.RefundPolicy{SourceMode: PolicySourceSelf, CategoryID: 10, Refundable: true, WindowHours: 168, Status: model.RefundPolicyStatusActive},
		// 子分类（云主机/香港）：24 小时
		model.RefundPolicy{SourceMode: PolicySourceSelf, CategoryID: 20, Refundable: true, WindowHours: 24, AutoApprove: true, Status: model.RefundPolicyStatusActive},
	)
	// 商品落在子分类 20，链路为 [20, 10, 1]
	svc := newPolicySvc(repo, PolicySourceSelf, 20, []uint64{20, 10, 1})

	decision, err := svc.Resolve(context.Background(), 100)
	if err != nil {
		t.Fatalf("解析策略失败: %v", err)
	}
	if decision == nil {
		t.Fatal("应命中策略")
	}
	if decision.WindowHours != 24 || !decision.AutoApprove {
		t.Fatalf("应命中更具体的子分类策略（24h/免审）: %+v", decision)
	}
}

// TestRefundPolicyResolveFallbackToSource 链上无命中时回落「同来源 + 全部商品」。
func TestRefundPolicyResolveFallbackToSource(t *testing.T) {
	repo := newFakePolicyRepo(
		model.RefundPolicy{SourceMode: PolicySourceSelf, CategoryID: 0, Refundable: true, WindowHours: 48, Status: model.RefundPolicyStatusActive},
		// 另一来源的策略不应命中
		model.RefundPolicy{SourceMode: PolicySourceUpstream, CategoryID: 0, Refundable: false, Status: model.RefundPolicyStatusActive},
	)
	svc := newPolicySvc(repo, PolicySourceSelf, 20, []uint64{20, 10})

	decision, err := svc.Resolve(context.Background(), 100)
	if err != nil || decision == nil {
		t.Fatalf("应回落命中断来源兜底策略: decision=%+v err=%v", decision, err)
	}
	if decision.WindowHours != 48 || !decision.Refundable {
		t.Fatalf("兜底策略口径不符: %+v", decision)
	}
}

// TestRefundPolicyResolveNoMatch 无任何命中时返回 nil（调用方按不可自助退款处理）。
func TestRefundPolicyResolveNoMatch(t *testing.T) {
	repo := newFakePolicyRepo(
		model.RefundPolicy{SourceMode: PolicySourceUpstream, CategoryID: 0, Refundable: true, Status: model.RefundPolicyStatusActive},
	)
	svc := newPolicySvc(repo, PolicySourceSelf, 20, []uint64{20})

	decision, err := svc.Resolve(context.Background(), 100)
	if err != nil {
		t.Fatalf("解析不应报错: %v", err)
	}
	if decision != nil {
		t.Fatalf("无命中应返回 nil，实际: %+v", decision)
	}
}

// TestRefundPolicyIgnoresDisabled 停用策略不参与匹配。
func TestRefundPolicyIgnoresDisabled(t *testing.T) {
	repo := newFakePolicyRepo(
		model.RefundPolicy{SourceMode: PolicySourceSelf, CategoryID: 0, Refundable: true, WindowHours: 24, Status: model.RefundPolicyStatusDisabled},
	)
	svc := newPolicySvc(repo, PolicySourceSelf, 20, []uint64{20})

	decision, err := svc.Resolve(context.Background(), 100)
	if err != nil || decision != nil {
		t.Fatalf("停用策略不应命中: decision=%+v err=%v", decision, err)
	}
}

// TestRefundPolicyDuplicateRejected 同一「来源 + 分类」不允许重复配置。
func TestRefundPolicyDuplicateRejected(t *testing.T) {
	repo := newFakePolicyRepo(
		model.RefundPolicy{SourceMode: PolicySourceSelf, CategoryID: 10, Refundable: true, Status: model.RefundPolicyStatusActive},
	)
	svc := newPolicySvc(repo, PolicySourceSelf, 10, []uint64{10})

	_, err := svc.Create(context.Background(), dto.RefundPolicyRequest{
		SourceMode: PolicySourceSelf, CategoryID: 10, Refundable: true, WindowHours: 24,
	})
	if !errors.Is(err, ErrRefundPolicyDuplicate) {
		t.Fatalf("重复配置应被拒绝, got %v", err)
	}
	// 同来源不同分类可以建。
	if _, err := svc.Create(context.Background(), dto.RefundPolicyRequest{
		SourceMode: PolicySourceSelf, CategoryID: 11, Refundable: true, WindowHours: 24,
	}); err != nil {
		t.Fatalf("不同分类应可创建: %v", err)
	}
}

// TestRefundPolicyRejectsInvalidSource 非法来源取值被拒绝。
func TestRefundPolicyRejectsInvalidSource(t *testing.T) {
	svc := newPolicySvc(newFakePolicyRepo(), PolicySourceSelf, 10, []uint64{10})
	_, err := svc.Create(context.Background(), dto.RefundPolicyRequest{SourceMode: "bogus", Refundable: true})
	if !errors.Is(err, ErrRefundModeInvalid) {
		t.Fatalf("非法来源应返回 ErrRefundModeInvalid, got %v", err)
	}
}

// —— 可退性提示 ——

func activeOrderWithActivation(hoursAgo float64) *model.Order {
	at := time.Now().Add(-time.Duration(hoursAgo * float64(time.Hour)))
	return &model.Order{
		ID: 1, OrderNo: "UC1", UserID: 7, ProductID: 100,
		Status: model.OrderStatusActive, PaidAmount: 100, ActivatedAt: &at,
	}
}

// TestRefundPolicyHintWithinWindow 在时限内可退，且下发截止时间。
func TestRefundPolicyHintWithinWindow(t *testing.T) {
	repo := newFakePolicyRepo(
		model.RefundPolicy{SourceMode: PolicySourceSelf, CategoryID: 0, Refundable: true, WindowHours: 24, AutoApprove: true, Status: model.RefundPolicyStatusActive},
	)
	orderRepo := newFakeOrderRepo(activeOrderWithActivation(2))
	svc := NewOrderService(orderRepo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)
	svc.SetRefundPolicyMatcher(newPolicySvc(repo, PolicySourceSelf, 10, []uint64{10}))

	order, _ := orderRepo.FindByID(context.Background(), 1)
	hint := svc.RefundPolicyHint(context.Background(), order)
	if hint == nil || !hint.Refundable {
		t.Fatalf("开通 2 小时应在 24 小时窗口内可退: %+v", hint)
	}
	if hint.Deadline == "" {
		t.Fatal("应下发可退截止时间")
	}
	if !hint.AutoApprove {
		t.Fatal("应下发免审核标记")
	}
	if hint.MaxAmount != 100 {
		t.Fatalf("可退上限应为实付 100: %v", hint.MaxAmount)
	}
}

// TestRefundPolicyHintExpired 超出时限不可退，且给出原因。
func TestRefundPolicyHintExpired(t *testing.T) {
	repo := newFakePolicyRepo(
		model.RefundPolicy{SourceMode: PolicySourceSelf, CategoryID: 0, Refundable: true, WindowHours: 24, Status: model.RefundPolicyStatusActive},
	)
	orderRepo := newFakeOrderRepo(activeOrderWithActivation(30))
	svc := NewOrderService(orderRepo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)
	svc.SetRefundPolicyMatcher(newPolicySvc(repo, PolicySourceSelf, 10, []uint64{10}))

	order, _ := orderRepo.FindByID(context.Background(), 1)
	hint := svc.RefundPolicyHint(context.Background(), order)
	if hint == nil || hint.Refundable {
		t.Fatalf("开通 30 小时超出 24 小时窗口应不可退: %+v", hint)
	}
	if hint.Reason == "" {
		t.Fatal("不可退应给出原因")
	}
}

// TestRefundPolicyHintNoPolicyDenies 未命中策略 = 不可自助退款（保守默认）。
func TestRefundPolicyHintNoPolicyDenies(t *testing.T) {
	orderRepo := newFakeOrderRepo(activeOrderWithActivation(1))
	svc := NewOrderService(orderRepo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)
	// 不注入策略匹配器：等同未配置任何策略
	order, _ := orderRepo.FindByID(context.Background(), 1)
	hint := svc.RefundPolicyHint(context.Background(), order)
	if hint == nil || hint.Refundable {
		t.Fatalf("无策略时应不可自助退款: %+v", hint)
	}
}

// TestRefundPolicyHintNotActivatedIgnoresWindow 未开通订单不受时限约束（尚未交付可随时申请）。
func TestRefundPolicyHintNotActivatedIgnoresWindow(t *testing.T) {
	repo := newFakePolicyRepo(
		model.RefundPolicy{SourceMode: PolicySourceSelf, CategoryID: 0, Refundable: true, WindowHours: 1, Status: model.RefundPolicyStatusActive},
	)
	orderRepo := newFakeOrderRepo(&model.Order{
		ID: 2, OrderNo: "UC2", UserID: 7, ProductID: 100,
		Status: model.OrderStatusPaid, PaidAmount: 100,
		// 已支付但还没开通成功：ActivatedAt 为 nil
	})
	svc := NewOrderService(orderRepo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)
	svc.SetRefundPolicyMatcher(newPolicySvc(repo, PolicySourceSelf, 10, []uint64{10}))

	order, _ := orderRepo.FindByID(context.Background(), 2)
	hint := svc.RefundPolicyHint(context.Background(), order)
	if hint == nil || !hint.Refundable {
		t.Fatalf("未开通订单不应受过期限制: %+v", hint)
	}
	if hint.Deadline != "" {
		t.Fatalf("未开通订单不应有截止时间: %+v", hint)
	}
}

// TestUserCreateRefundRejectsNoPolicy 无策略时客户申请被拒。
func TestUserCreateRefundRejectsNoPolicy(t *testing.T) {
	orderRepo := newFakeOrderRepo(activeOrderWithActivation(1))
	svc := NewOrderService(orderRepo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)

	_, err := svc.UserCreateRefund(context.Background(), 7, 1, dto.UserRefundRequest{Reason: "不想要了"})
	if !errors.Is(err, ErrRefundNotAllowed) {
		t.Fatalf("无策略应拒绝并返回 ErrRefundNotAllowed, got %v", err)
	}
}

// TestUserCreateRefundOwnershipGuard 他人订单按不存在处理。
func TestUserCreateRefundOwnershipGuard(t *testing.T) {
	orderRepo := newFakeOrderRepo(activeOrderWithActivation(1))
	svc := NewOrderService(orderRepo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)

	_, err := svc.UserCreateRefund(context.Background(), 999, 1, dto.UserRefundRequest{Reason: "x"})
	if !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("他人订单应返回 ErrOrderNotFound, got %v", err)
	}
}

// TestUserCreateRefundChannelUnavailable 余额支付订单不允许选原路退回。
func TestUserCreateRefundChannelUnavailable(t *testing.T) {
	repo := newFakePolicyRepo(
		model.RefundPolicy{SourceMode: PolicySourceSelf, CategoryID: 0, Refundable: true, WindowHours: 24, Status: model.RefundPolicyStatusActive},
	)
	orderRepo := newFakeOrderRepo(activeOrderWithActivation(1))
	svc := NewOrderService(orderRepo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)
	svc.SetRefundPolicyMatcher(newPolicySvc(repo, PolicySourceSelf, 10, []uint64{10}))
	// 不注入渠道支付判据：等同余额支付订单

	_, err := svc.UserCreateRefund(context.Background(), 7, 1, dto.UserRefundRequest{
		Reason: "不想要了", RefundMode: model.RefundModeChannel,
	})
	if !errors.Is(err, ErrRefundChannelUnavailable) {
		t.Fatalf("余额支付订单选原路退回应被拒, got %v", err)
	}
}

// TestUserCreateRefundAutoApprove 命中免审（全额+余额+策略允许+已开通在时限内）→ 自动退款并置待复核。
func TestUserCreateRefundAutoApprove(t *testing.T) {
	repo := newFakePolicyRepo(
		model.RefundPolicy{SourceMode: PolicySourceSelf, CategoryID: 0, Refundable: true, WindowHours: 24, AutoApprove: true, Status: model.RefundPolicyStatusActive},
	)
	orderRepo := newFakeOrderRepo(activeOrderWithActivation(2))
	refunds := newFakeRefundRepo()
	svc := NewOrderService(orderRepo, newFakeItemRepo(), refunds, &fakeProvision{}).(*orderService)
	svc.SetRefundPolicyMatcher(newPolicySvc(repo, PolicySourceSelf, 10, []uint64{10}))

	res, err := svc.UserCreateRefund(context.Background(), 7, 1, dto.UserRefundRequest{Reason: "24小时内无理由退款"})
	if err != nil {
		t.Fatalf("应自动退款成功: %v", err)
	}
	if !res.AutoApproved {
		t.Fatalf("应标记为自动退款: %+v", res)
	}
	if res.Status != model.RefundStatusApproved {
		t.Fatalf("退款单应已通过: %+v", res)
	}
	// 订单应已退款（全额）+ 退款单进入待复核。
	order, _ := orderRepo.FindByID(context.Background(), 1)
	if order.Status != model.OrderStatusRefunded {
		t.Fatalf("全额退款后订单应为已退款, got %s", order.Status)
	}
	found := false
	for _, rf := range refunds.refunds {
		if rf.OrderID == 1 {
			found = true
			if rf.ReviewState != model.RefundReviewPending {
				t.Fatalf("自动退款单应置待复核: %+v", rf)
			}
			if rf.Origin != model.RefundOriginUser {
				t.Fatalf("来源应为客户申请: %+v", rf)
			}
		}
	}
	if !found {
		t.Fatal("应创建退款单")
	}
}

// TestUserCreateRefundManualWhenAutoDisabled 策略未开免审 → 落待审核单。
func TestUserCreateRefundManualWhenAutoDisabled(t *testing.T) {
	repo := newFakePolicyRepo(
		model.RefundPolicy{SourceMode: PolicySourceSelf, CategoryID: 0, Refundable: true, WindowHours: 24, AutoApprove: false, Status: model.RefundPolicyStatusActive},
	)
	orderRepo := newFakeOrderRepo(activeOrderWithActivation(2))
	svc := NewOrderService(orderRepo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)
	svc.SetRefundPolicyMatcher(newPolicySvc(repo, PolicySourceSelf, 10, []uint64{10}))

	res, err := svc.UserCreateRefund(context.Background(), 7, 1, dto.UserRefundRequest{Reason: "不想要了"})
	if err != nil {
		t.Fatalf("应落待审核单: %v", err)
	}
	if res.AutoApproved || res.Status != model.RefundStatusPending {
		t.Fatalf("未开免审应待人工审核: %+v", res)
	}
}

// TestUserCreateRefundNotActivatedAlwaysManual 未开通订单即使策略开免审也走人工。
func TestUserCreateRefundNotActivatedAlwaysManual(t *testing.T) {
	repo := newFakePolicyRepo(
		model.RefundPolicy{SourceMode: PolicySourceSelf, CategoryID: 0, Refundable: true, WindowHours: 24, AutoApprove: true, Status: model.RefundPolicyStatusActive},
	)
	orderRepo := newFakeOrderRepo(&model.Order{
		ID: 3, OrderNo: "UC3", UserID: 7, ProductID: 100,
		Status: model.OrderStatusProvisioning, PaidAmount: 100,
	})
	svc := NewOrderService(orderRepo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)
	svc.SetRefundPolicyMatcher(newPolicySvc(repo, PolicySourceSelf, 10, []uint64{10}))

	res, err := svc.UserCreateRefund(context.Background(), 7, 3, dto.UserRefundRequest{Reason: "开通太慢"})
	if err != nil {
		t.Fatalf("未开通订单应可申请: %v", err)
	}
	if res.AutoApproved {
		t.Fatalf("未开通订单不应自动退款（可能涉及上游退单）: %+v", res)
	}
}

// TestReviewRefund 事后复核：待复核可复核，重复复核状态冲突。
func TestReviewRefund(t *testing.T) {
	orderRepo := newFakeOrderRepo(activeOrderWithActivation(2))
	refunds := newFakeRefundRepo()
	refunds.refunds[1] = &model.OrderRefund{
		ID: 1, OrderID: 1, RefundNo: "RF1", Amount: 100,
		Status: model.RefundStatusApproved, ReviewState: model.RefundReviewPending,
	}
	svc := NewOrderService(orderRepo, newFakeItemRepo(), refunds, &fakeProvision{}).(*orderService)

	info, err := svc.ReviewRefund(context.Background(), 1, 9, "财务-王五", "已核对余额入账")
	if err != nil {
		t.Fatalf("复核应成功: %v", err)
	}
	if info.ReviewState != model.RefundReviewDone || info.ReviewedByName != "财务-王五" {
		t.Fatalf("复核字段未落库: %+v", info)
	}
	if _, err := svc.ReviewRefund(context.Background(), 1, 9, "财务-王五", "重复"); !errors.Is(err, ErrStatusConflict) {
		t.Fatalf("重复复核应返回状态冲突, got %v", err)
	}
	// 未进入复核流程的单不可复核。
	refunds.refunds[2] = &model.OrderRefund{ID: 2, OrderID: 1, RefundNo: "RF2", Status: model.RefundStatusApproved}
	if _, err := svc.ReviewRefund(context.Background(), 2, 9, "x", ""); !errors.Is(err, ErrStatusConflict) {
		t.Fatalf("非待复核单应拒绝, got %v", err)
	}
}

// TestApproveRefundPersistsAuditRemark 审核备注与凭据单号落库（此前 remark 从未落库）。
func TestApproveRefundPersistsAuditRemark(t *testing.T) {
	orderRepo := newFakeOrderRepo(&model.Order{ID: 1, OrderNo: "UC1", Status: model.OrderStatusRefunding, PaidAmount: 100})
	refunds := newFakeRefundRepo()
	refunds.refunds[1] = &model.OrderRefund{ID: 1, OrderID: 1, RefundNo: "RF1", Amount: 100, Status: model.RefundStatusPending}
	svc := NewOrderService(orderRepo, newFakeItemRepo(), refunds, &fakeProvision{}).(*orderService)

	info, err := svc.ApproveRefund(context.Background(), 1, 7, "op", dto.RefundApproveRequest{
		Remark: "已与客户协商一致", EvidenceNo: "TXN20261014001",
	})
	if err != nil {
		t.Fatalf("审核应成功: %v", err)
	}
	if info.AuditRemark != "已与客户协商一致" {
		t.Fatalf("审核备注应落库: %+v", info)
	}
	if info.EvidenceNo != "TXN20261014001" {
		t.Fatalf("凭据单号应落库: %+v", info)
	}
}
