package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/model"
)

// TestVerifyMarksOrder 核验：CAS 写入核验人/时间/备注；重复核验与未开通订单拒绝。
func TestVerifyMarksOrder(t *testing.T) {
	active := &model.Order{ID: 1, OrderNo: "UC1", Status: model.OrderStatusActive, PaidAmount: 100}
	repo := newFakeOrderRepo(active)
	svc := NewOrderService(repo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)

	if err := svc.Verify(context.Background(), 1, 7, "张三", "配置与订单一致"); err != nil {
		t.Fatalf("服务中订单应可核验: %v", err)
	}
	o, _ := repo.FindByID(context.Background(), 1)
	if o.VerifiedAt == nil || o.VerifiedBy != 7 || o.VerifiedByName != "张三" || o.VerifyRemark != "配置与订单一致" {
		t.Fatalf("核验字段未写入: %+v", o)
	}
	// 重复核验：状态冲突（CAS 语义，幂等拒绝）。
	if err := svc.Verify(context.Background(), 1, 8, "李四", ""); !errors.Is(err, ErrStatusConflict) {
		t.Fatalf("重复核验应返回状态冲突, got %v", err)
	}
	// 未开通订单不可核验。
	repo.orders[2] = &model.Order{ID: 2, OrderNo: "UC2", Status: model.OrderStatusProvisioning}
	if err := svc.Verify(context.Background(), 2, 7, "张三", ""); !errors.Is(err, ErrVerifyNotAllowed) {
		t.Fatalf("开通中订单核验应返回 ErrVerifyNotAllowed, got %v", err)
	}
	// 不存在订单。
	if err := svc.Verify(context.Background(), 999, 7, "张三", ""); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("未知订单应返回 ErrOrderNotFound, got %v", err)
	}
}

// TestApplyVerifyState 待核验推导：开关关闭不提示；开启且服务中未核验 → pending；已核验 → verified。
func TestApplyVerifyState(t *testing.T) {
	base := dto.OrderInfo{Status: model.OrderStatusActive}

	// 开关关闭：不推导，字段保持空。
	off := base
	applyVerifyState(&off, false)
	if off.VerifyState != "" || off.VerifyRequired {
		t.Fatalf("开关关闭不应推导待核验: %+v", off)
	}

	// 开关开启 + 服务中 + 未核验 → pending。
	on := base
	applyVerifyState(&on, true)
	if on.VerifyState != dto.VerifyStatePending || !on.VerifyRequired {
		t.Fatalf("开启且未核验应推导为待核验: %+v", on)
	}

	// 开关开启 + 已核验 → verified（不再提示）。
	verified := dto.OrderInfo{Status: model.OrderStatusActive, VerifiedAt: "2026-10-14T10:00:00Z"}
	applyVerifyState(&verified, true)
	if verified.VerifyState != dto.VerifyStateVerified {
		t.Fatalf("已核验应推导为 verified: %+v", verified)
	}

	// 开关开启但未开通（非 active）→ 不提示。
	notActive := dto.OrderInfo{Status: model.OrderStatusPaid}
	applyVerifyState(&notActive, true)
	if notActive.VerifyState != "" {
		t.Fatalf("未开通订单不应推导待核验: %+v", notActive)
	}
}

// TestApplyInstanceRef 关联实例回填：有实例写字段；无实例不写入。
func TestApplyInstanceRef(t *testing.T) {
	info := dto.OrderInfo{}
	applyInstanceRef(&info, OrderInstanceRef{ID: 88, No: "i-mfy-88", Pending: true})
	if info.InstanceID != 88 || info.InstanceNo != "i-mfy-88" || !info.InstancePending {
		t.Fatalf("关联实例未回填: %+v", info)
	}

	empty := dto.OrderInfo{}
	applyInstanceRef(&empty, OrderInstanceRef{})
	if empty.InstanceID != 0 || empty.InstanceNo != "" {
		t.Fatalf("无实例不应回填: %+v", empty)
	}
}

// TestNormalizeBatchIDs 批量 ID 归一：去重保序、上限与空集校验。
func TestNormalizeBatchIDs(t *testing.T) {
	out, err := normalizeBatchIDs([]uint64{3, 1, 3, 0, 2})
	if err != nil || len(out) != 3 || out[0] != 3 || out[1] != 1 || out[2] != 2 {
		t.Fatalf("去重保序不符: out=%v err=%v", out, err)
	}
	if _, err := normalizeBatchIDs(nil); !errors.Is(err, ErrBatchEmpty) {
		t.Fatalf("空集合应返回 ErrBatchEmpty, got %v", err)
	}
	if _, err := normalizeBatchIDs([]uint64{0}); !errors.Is(err, ErrBatchEmpty) {
		t.Fatalf("全零应返回 ErrBatchEmpty, got %v", err)
	}
	tooMany := make([]uint64, maxBatchIDs+1)
	for i := range tooMany {
		tooMany[i] = uint64(i + 1)
	}
	if _, err := normalizeBatchIDs(tooMany); !errors.Is(err, ErrBatchTooLarge) {
		t.Fatalf("超上限应返回 ErrBatchTooLarge, got %v", err)
	}
}

// TestBatchCancelMixed 批量取消：待支付成功、已支付跳过，逐条结果齐全。
func TestBatchCancelMixed(t *testing.T) {
	repo := newFakeOrderRepo(
		pendingOrder(1, "UC1"),
		&model.Order{ID: 2, OrderNo: "UC2", Status: model.OrderStatusPaid},
		pendingOrder(3, "UC3"),
	)
	svc := NewOrderService(repo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)

	resp, err := svc.BatchCancel(context.Background(), []uint64{1, 2, 3}, 7, "op")
	if err != nil {
		t.Fatalf("批量取消应返回结果: %v", err)
	}
	if resp.Succeeded != 2 || resp.Skipped != 1 || resp.Failed != 0 {
		t.Fatalf("分类计数不符: %+v", resp)
	}
	if len(resp.Items) != 3 {
		t.Fatalf("应逐条返回结果: %+v", resp.Items)
	}
	// 已支付订单应被标记 skipped 且带原因。
	if resp.Items[1].Status != dto.BatchStatusSkipped || resp.Items[1].Message == "" {
		t.Fatalf("已支付订单应跳过并带原因: %+v", resp.Items[1])
	}
	o1, _ := repo.FindByID(context.Background(), 1)
	o3, _ := repo.FindByID(context.Background(), 3)
	if o1.Status != model.OrderStatusCancelled || o3.Status != model.OrderStatusCancelled {
		t.Fatalf("待支付订单应已取消: %s / %s", o1.Status, o3.Status)
	}
	o2, _ := repo.FindByID(context.Background(), 2)
	if o2.Status != model.OrderStatusPaid {
		t.Fatalf("已支付订单不应被改动: %s", o2.Status)
	}
	// 空集合拒绝。
	if _, err := svc.BatchCancel(context.Background(), nil, 7, "op"); !errors.Is(err, ErrBatchEmpty) {
		t.Fatalf("空集合应拒绝, got %v", err)
	}
}

// TestBatchActivateReusesQueue 批量重新开通：逐条投递开通任务（复用幂等队列）。
func TestBatchActivateReusesQueue(t *testing.T) {
	repo := newFakeOrderRepo(
		&model.Order{ID: 1, OrderNo: "UC1", Status: model.OrderStatusPaid},
		&model.Order{ID: 2, OrderNo: "UC2", Status: model.OrderStatusProvisioning},
		&model.Order{ID: 3, OrderNo: "UC3", Status: model.OrderStatusPending},
	)
	prov := &fakeProvision{}
	svc := NewOrderService(repo, newFakeItemRepo(), newFakeRefundRepo(), prov).(*orderService)

	resp, err := svc.BatchActivate(context.Background(), []uint64{1, 2, 3})
	if err != nil {
		t.Fatalf("批量开通应返回结果: %v", err)
	}
	// 1 与 2 是 paid/provisioning → 同步 Activate 成功；3 是 pending → 状态冲突跳过。
	if resp.Succeeded != 2 || resp.Skipped != 1 {
		t.Fatalf("分类计数不符: %+v", resp)
	}
}

// TestBatchVerifyMixed 批量核验：已开通成功、未开通跳过、已核验跳过。
func TestBatchVerifyMixed(t *testing.T) {
	verified := &model.Order{ID: 9, OrderNo: "UC9", Status: model.OrderStatusActive}
	fixed := time.Now()
	verified.VerifiedAt = &fixed
	repo := newFakeOrderRepo(
		&model.Order{ID: 1, OrderNo: "UC1", Status: model.OrderStatusActive},
		&model.Order{ID: 2, OrderNo: "UC2", Status: model.OrderStatusPaid},
		verified,
	)
	svc := NewOrderService(repo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)

	resp, err := svc.BatchVerify(context.Background(), []uint64{1, 2, 9}, 7, "op", "批量核对")
	if err != nil {
		t.Fatalf("批量核验应返回结果: %v", err)
	}
	if resp.Succeeded != 1 || resp.Skipped != 2 {
		t.Fatalf("分类计数不符: %+v", resp)
	}
	o, _ := repo.FindByID(context.Background(), 1)
	if o.VerifiedAt == nil || o.VerifyRemark != "批量核对" {
		t.Fatalf("核验未写入: %+v", o)
	}
}

// TestBatchDeleteGuards 批量删除守卫：仅已取消/已关闭且无退款记录可删；活单与有退款单均拒绝。
func TestBatchDeleteGuards(t *testing.T) {
	repo := newFakeOrderRepo(
		&model.Order{ID: 1, OrderNo: "UC1", Status: model.OrderStatusCancelled},
		&model.Order{ID: 2, OrderNo: "UC2", Status: model.OrderStatusActive},
		&model.Order{ID: 3, OrderNo: "UC3", Status: model.OrderStatusClosed},
	)
	refunds := newFakeRefundRepo()
	// 订单 3 走过退款流程（哪怕被驳回也不可删）。
	refunds.refunds[1] = &model.OrderRefund{ID: 1, OrderID: 3, RefundNo: "RF1", Amount: 10, Status: model.RefundStatusRejected}
	svc := NewOrderService(repo, newFakeItemRepo(), refunds, &fakeProvision{}).(*orderService)

	resp, err := svc.BatchDelete(context.Background(), []uint64{1, 2, 3})
	if err != nil {
		t.Fatalf("批量删除应返回结果: %v", err)
	}
	if resp.Succeeded != 1 || resp.Skipped != 2 || resp.Failed != 0 {
		t.Fatalf("分类计数不符: %+v", resp)
	}
	if resp.Items[1].Message == "" || resp.Items[2].Message == "" {
		t.Fatalf("跳过项应带原因: %+v", resp.Items)
	}
	// 订单 1 已软删除；2 与 3 未动。
	o1, _ := repo.FindByID(context.Background(), 1)
	o2, _ := repo.FindByID(context.Background(), 2)
	o3, _ := repo.FindByID(context.Background(), 3)
	if o1.DeletedAt == nil {
		t.Fatalf("已取消订单应被软删除: %+v", o1)
	}
	if o2.DeletedAt != nil || o3.DeletedAt != nil {
		t.Fatalf("活单与有退款单不应被删除: %+v / %+v", o2, o3)
	}
}

// TestBatchRefundAudit 批量退款审核：按 pending 状态逐条通过/驳回。
func TestBatchRefundAudit(t *testing.T) {
	order := &model.Order{ID: 1, OrderNo: "UC1", Status: model.OrderStatusRefunding, PaidAmount: 100}
	repo := newFakeOrderRepo(order)
	refunds := newFakeRefundRepo()
	refunds.refunds[1] = &model.OrderRefund{ID: 1, OrderID: 1, RefundNo: "RF1", Amount: 30, Status: model.RefundStatusPending}
	refunds.refunds[2] = &model.OrderRefund{ID: 2, OrderID: 1, RefundNo: "RF2", Amount: 20, Status: model.RefundStatusPending}
	refunds.refunds[3] = &model.OrderRefund{ID: 3, OrderID: 1, RefundNo: "RF3", Amount: 10, Status: model.RefundStatusRejected}
	svc := NewOrderService(repo, newFakeItemRepo(), refunds, &fakeProvision{}).(*orderService)

	// 通过 1、2（pending），3 已驳回 → 跳过。
	resp, err := svc.BatchApproveRefunds(context.Background(), []uint64{1, 2, 3}, 7, "op", "批量通过")
	if err != nil {
		t.Fatalf("批量通过应返回结果: %v", err)
	}
	if resp.Succeeded != 2 || resp.Skipped != 1 {
		t.Fatalf("分类计数不符: %+v", resp)
	}
	if refunds.refunds[1].Status != model.RefundStatusApproved || refunds.refunds[1].AuditByName != "op" {
		t.Fatalf("退款单 1 应已通过并留痕: %+v", refunds.refunds[1])
	}

	// 驳回场景：新建一笔 pending 后批量驳回。
	refunds.refunds[4] = &model.OrderRefund{ID: 4, OrderID: 1, RefundNo: "RF4", Amount: 5, Status: model.RefundStatusPending}
	resp, err = svc.BatchRejectRefunds(context.Background(), []uint64{4}, 7, "op", "批量驳回")
	if err != nil {
		t.Fatalf("批量驳回应返回结果: %v", err)
	}
	if resp.Succeeded != 1 {
		t.Fatalf("驳回应成功: %+v", resp)
	}
	if refunds.refunds[4].Status != model.RefundStatusRejected {
		t.Fatalf("退款单 4 应已驳回: %+v", refunds.refunds[4])
	}
}

// TestInstanceRefAndVerifyReaders 读路径注入：列表/详情回填关联实例与核验状态。
func TestInstanceRefAndVerifyReaders(t *testing.T) {
	repo := newFakeOrderRepo(
		&model.Order{ID: 1, OrderNo: "UC1", Status: model.OrderStatusActive, PaidAmount: 100},
		&model.Order{ID: 2, OrderNo: "UC2", Status: model.OrderStatusPending},
	)
	svc := NewOrderService(repo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)
	svc.SetInstanceRefReader(func(_ context.Context, orderIDs []uint64) (map[uint64]OrderInstanceRef, error) {
		return map[uint64]OrderInstanceRef{
			1: {ID: 77, No: "i-mfy-77"},
		}, nil
	})
	svc.SetVerifyRequiredReader(func(context.Context) bool { return true })

	list, err := svc.List(context.Background(), dto.OrderListQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("列表应成功: %v", err)
	}
	byID := map[uint64]dto.OrderInfo{}
	for _, it := range list.Items {
		byID[it.ID] = it
	}
	if byID[1].InstanceID != 77 || byID[1].InstanceNo != "i-mfy-77" {
		t.Fatalf("订单 1 应回填关联实例: %+v", byID[1])
	}
	if byID[1].VerifyState != dto.VerifyStatePending {
		t.Fatalf("服务中未核验订单应为待核验: %+v", byID[1])
	}
	if byID[2].InstanceID != 0 || byID[2].VerifyState != "" {
		t.Fatalf("待支付订单不应有实例/核验状态: %+v", byID[2])
	}

	detail, err := svc.FindByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("详情应成功: %v", err)
	}
	if detail.InstanceID != 77 || detail.VerifyState != dto.VerifyStatePending {
		t.Fatalf("详情应回填实例与核验状态: %+v", detail.OrderInfo)
	}
}

// TestInstanceRefReaderFailureTolerated 读路径容错：读取器报错不影响订单列表。
func TestInstanceRefReaderFailureTolerated(t *testing.T) {
	repo := newFakeOrderRepo(&model.Order{ID: 1, OrderNo: "UC1", Status: model.OrderStatusActive})
	svc := NewOrderService(repo, newFakeItemRepo(), newFakeRefundRepo(), &fakeProvision{}).(*orderService)
	svc.SetInstanceRefReader(func(context.Context, []uint64) (map[uint64]OrderInstanceRef, error) {
		return nil, errors.New("db down")
	})
	svc.SetVerifyRequiredReader(func(context.Context) bool { return false })

	list, err := svc.List(context.Background(), dto.OrderListQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("读取器失败不应让列表失败: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].InstanceID != 0 {
		t.Fatalf("应返回订单且不带实例: %+v", list.Items)
	}
}
