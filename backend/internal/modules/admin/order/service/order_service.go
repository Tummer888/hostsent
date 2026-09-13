package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/model"
	"hostsent/backend/internal/modules/admin/order/repository"
)

// OrderService 定义订单域业务能力（含退款管理）。
type OrderService interface {
	// 订单管理
	List(ctx context.Context, q dto.OrderListQuery) (*dto.OrderListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.OrderDetail, error)
	Cancel(ctx context.Context, id uint64, operatorID uint64, operatorName string) error
	UpdateRemark(ctx context.Context, id uint64, remark string, operatorID uint64) error
	CreateRefund(ctx context.Context, id uint64, req dto.RefundCreateRequest, operatorID uint64, operatorName string) (*dto.RefundInfo, error)
	Activate(ctx context.Context, id uint64) error
	// EnqueueProvision 投递异步开通任务（T5.1）：下单/人工重试统一入口，
	// 未注入任务队列时回落为同步 Activate，保持既有行为。
	EnqueueProvision(ctx context.Context, id uint64) error
	Stats(ctx context.Context) (*dto.OrderStatsResponse, error)
	// SetCashbackHook 注入订单完成后的推广返现计提钩子（装配层调用）。
	SetCashbackHook(hook OrderCashbackHook)
	// SetRefundHook 注入退款审核通过后的返现冲减钩子（装配层调用）。
	SetRefundHook(hook OrderRefundHook)
	// SetProvisionQueue 注入异步开通任务队列（装配层调用，T5.1）。
	SetProvisionQueue(queue ProvisionQueue)
	// SetProductSourceModeResolver 注入商品链路判据解析（装配层调用，仅用于任务留痕）。
	SetProductSourceModeResolver(fn func(ctx context.Context, productID uint64) string)
	// SetSalesOwnerResolver 注入销售归属快照解析（装配层调用，doc86 §3.4，仅用于后台代下单）。
	SetSalesOwnerResolver(r SalesOwnerResolver)
	// MarkPaidByChannel 支付中心回调确认到账：pending→paid 并记录渠道支付方式。
	// 已是 paid/active 时幂等返回；返回是否发生了状态迁移（供装配层决定是否继续开通）。
	MarkPaidByChannel(ctx context.Context, orderNo, payMethod, channelTx string) (bool, error)
	// ActivateByNo 按订单号触发开通（支付成功后调用，复用异步任务队列）。
	ActivateByNo(ctx context.Context, orderNo string) error
	// SetChannelRefundHook 注入渠道退款钩子（装配层调用）：退款审核通过后原路退回，
	// 无渠道支付记录时钩子实现回落为余额回补。
	SetChannelRefundHook(hook OrderChannelRefundHook)
	// 退款管理
	ListRefunds(ctx context.Context, q dto.RefundListQuery) (*dto.RefundListResponse, error)
	FindRefund(ctx context.Context, id uint64) (*dto.RefundInfo, error)
	ApproveRefund(ctx context.Context, id uint64, operatorID uint64, operatorName string, remark string) (*dto.RefundInfo, error)
	RejectRefund(ctx context.Context, id uint64, operatorID uint64, operatorName string, remark string) (*dto.RefundInfo, error)
}

// OrderCashbackHook 订单完成（开通成功）后的推广返现计提钩子。
// 仅传基础类型，避免订单模块反向依赖返现模块。
type OrderCashbackHook func(ctx context.Context, orderID uint64, orderNo string, buyerUserID uint64, baseAmount float64, isRenewal bool) error

// OrderRefundHook 退款审核通过后的返现冲减钩子：按退款额占实付比例冲减邀请人返现。
type OrderRefundHook func(ctx context.Context, orderID uint64, orderNo string, buyerUserID uint64, paidAmount, refundAmount float64, refundNo string) error

// OrderChannelRefundHook 退款审核通过后的资金出口钩子（doc34 F-03 / doc35 §6.4 / doc36 §3.2）：
//   - mode=channel：原路退回支付来源；有渠道支付记录时创建渠道退款单并把单号回传落库，
//     无记录（余额支付）时由钩子实现回落为余额回补；
//   - mode=balance：直接退回余额，不走渠道。
//
// 失败不影响退款审核结果，装配层负责记录日志并留待人工处理。
// 返回 channelRefundNo 供退款单回填（无渠道退款时为空串）。
type OrderChannelRefundHook func(ctx context.Context, orderID uint64, orderNo string, orderRefundNo string, userID uint64, amount, fee float64, mode string) (channelRefundNo string, err error)

// ProvisionQueue 异步开通任务队列（T5.1，由 ProvisionEnqueuer 实现）。
// 订单模块只依赖这一最小接口，避免与 uc/装配层形成反向依赖。
type ProvisionQueue interface {
	EnqueueProvision(ctx context.Context, order *model.Order, sourceMode string) error
}

type orderService struct {
	orderRepo  repository.OrderRepository
	itemRepo   repository.OrderItemRepository
	refundRepo repository.RefundRepository
	provision  ProvisionAdapter
	// provisionQueue 可选：异步开通任务队列，未注入时 Activate/EnqueueProvision 走同步履约。
	provisionQueue ProvisionQueue
	// productSourceMode 可选：按商品 ID 解析链路判据（self/upstream，D6），仅用于任务留痕。
	productSourceMode func(ctx context.Context, productID uint64) string
	// cashbackHook 可选：订单开通成功后计提推广返现，为 nil 时跳过。
	cashbackHook OrderCashbackHook
	// refundHook 可选：退款审核通过后冲减已计提返现，为 nil 时跳过。
	refundHook OrderRefundHook
	// channelRefundHook 可选：退款审核通过后走渠道原路退回（doc35 S1），为 nil 时跳过。
	channelRefundHook OrderChannelRefundHook
	// salesOwner 可选：后台代下单时解析销售归属快照（doc86 §3.4），为 nil 时快照落 0。
	salesOwner SalesOwnerResolver
}

// SalesOwnerResolver 销售归属解析（装配层注入，避免订单模块依赖销售模块）。
type SalesOwnerResolver interface {
	SalesAdminForNewOrder(ctx context.Context, userID uint64) uint64
}

// SetSalesOwnerResolver 注入销售归属解析（装配层调用）。
func (s *orderService) SetSalesOwnerResolver(r SalesOwnerResolver) {
	s.salesOwner = r
}

// NewOrderService 创建订单业务服务。
func NewOrderService(orderRepo repository.OrderRepository, itemRepo repository.OrderItemRepository, refundRepo repository.RefundRepository, provision ProvisionAdapter) OrderService {
	return &orderService{orderRepo: orderRepo, itemRepo: itemRepo, refundRepo: refundRepo, provision: provision}
}

// SetCashbackHook 注入推广返现计提钩子（装配层调用，避免改构造签名影响既有装配点）。
func (s *orderService) SetCashbackHook(hook OrderCashbackHook) {
	s.cashbackHook = hook
}

// SetRefundHook 注入返现冲减钩子（装配层调用）。
func (s *orderService) SetRefundHook(hook OrderRefundHook) {
	s.refundHook = hook
}

// SetProvisionQueue 注入异步开通任务队列（装配层调用，T5.1）。
func (s *orderService) SetProvisionQueue(queue ProvisionQueue) {
	s.provisionQueue = queue
}

// SetProductSourceModeResolver 注入商品链路判据解析（装配层调用）。
func (s *orderService) SetProductSourceModeResolver(fn func(ctx context.Context, productID uint64) string) {
	s.productSourceMode = fn
}

// SetChannelRefundHook 注入渠道退款钩子（装配层调用）。
func (s *orderService) SetChannelRefundHook(hook OrderChannelRefundHook) {
	s.channelRefundHook = hook
}

// MarkPaidByChannel 支付中心确认到账：pending→paid，记录渠道支付方式与渠道交易号。
// 已 paid/active/provisioning 等已收款状态幂等返回 false；返回 true 表示本次完成了迁移。
func (s *orderService) MarkPaidByChannel(ctx context.Context, orderNo, payMethod, channelTx string) (bool, error) {
	item, err := s.orderRepo.FindByNo(ctx, orderNo)
	if err != nil {
		return false, mapOrderErr(err)
	}
	if item.Status != model.OrderStatusPending {
		// 已收款/已开通：幂等，不重复迁移与开通。
		return false, nil
	}
	if err := EnsureStatus(item.Status, model.OrderStatusPaid); err != nil {
		return false, err
	}
	now := time.Now()
	item.Status = model.OrderStatusPaid
	if payMethod != "" {
		item.PayMethod = payMethod
	}
	item.PayTime = &now
	item.Remark = remarkWithChannelTx(item.Remark, channelTx)
	if err := s.orderRepo.Update(ctx, item); err != nil {
		return false, err
	}
	return true, nil
}

// ActivateByNo 按订单号触发开通（支付成功回调路径复用异步任务队列）。
func (s *orderService) ActivateByNo(ctx context.Context, orderNo string) error {
	item, err := s.orderRepo.FindByNo(ctx, orderNo)
	if err != nil {
		return mapOrderErr(err)
	}
	return s.EnqueueProvision(ctx, item.ID)
}

// remarkWithChannelTx 把渠道交易号追加到订单备注（保留人工备注，幂等去重）。
func remarkWithChannelTx(remark, channelTx string) string {
	channelTx = strings.TrimSpace(channelTx)
	if channelTx == "" || strings.Contains(remark, channelTx) {
		return remark
	}
	tag := "渠道流水号:" + channelTx
	if remark == "" {
		return tag
	}
	return remark + " | " + tag
}

// EnqueueProvision 投递异步开通任务（T5.1）。
//
// 未注入队列时回落为同步 Activate：人工重试接口（/:id/activate）无论哪种装配都可用。
// 队列路径下订单状态由工作池推进（paid→provisioning→active），本方法只负责投递。
func (s *orderService) EnqueueProvision(ctx context.Context, id uint64) error {
	item, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return mapOrderErr(err)
	}
	if s.provisionQueue == nil {
		return s.Activate(ctx, id)
	}
	// source_mode 判据（D6）：按订单绑定的商品取链路；读取失败不影响投递（仅留痕）。
	sourceMode := ""
	if s.productSourceMode != nil {
		sourceMode = s.productSourceMode(ctx, item.ProductID)
	}
	if err := s.provisionQueue.EnqueueProvision(ctx, item, sourceMode); err != nil {
		return err
	}
	return nil
}

func (s *orderService) List(ctx context.Context, q dto.OrderListQuery) (*dto.OrderListResponse, error) {
	items, total, err := s.orderRepo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	respItems := make([]dto.OrderInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, buildOrderInfo(item))
	}
	return &dto.OrderListResponse{Items: respItems, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *orderService) FindByID(ctx context.Context, id uint64) (*dto.OrderDetail, error) {
	item, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	orderItems, err := s.itemRepo.ListByOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	refunds, err := s.refundRepo.FindByOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	itemInfos := make([]dto.OrderItemInfo, 0, len(orderItems))
	for _, it := range orderItems {
		itemInfos = append(itemInfos, buildOrderItemInfo(it))
	}
	refundInfos := make([]dto.RefundInfo, 0, len(refunds))
	for _, rf := range refunds {
		refundInfos = append(refundInfos, buildRefundInfo(rf, item.OrderNo))
	}
	return &dto.OrderDetail{OrderInfo: buildOrderInfo(*item), Items: itemInfos, Refunds: refundInfos}, nil
}

// Cancel 取消订单：仅待支付可取消，校验状态机并留痕。
func (s *orderService) Cancel(ctx context.Context, id uint64, operatorID uint64, operatorName string) error {
	item, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return mapOrderErr(err)
	}
	if err := EnsureStatus(item.Status, model.OrderStatusCancelled); err != nil {
		return err
	}
	item.Status = model.OrderStatusCancelled
	item.OperatorID = operatorID
	return s.orderRepo.Update(ctx, item)
}

func (s *orderService) UpdateRemark(ctx context.Context, id uint64, remark string, operatorID uint64) error {
	item, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return mapOrderErr(err)
	}
	item.Remark = remark
	item.OperatorID = operatorID
	return s.orderRepo.Update(ctx, item)
}

// CreateRefund 发起退款：校验可退金额，创建待审核退款单，服务中订单转入退款中。
func (s *orderService) CreateRefund(ctx context.Context, id uint64, req dto.RefundCreateRequest, operatorID uint64, operatorName string) (*dto.RefundInfo, error) {
	item, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	switch item.Status {
	case model.OrderStatusPaid, model.OrderStatusProvisioning, model.OrderStatusActive, model.OrderStatusRefunding:
	default:
		return nil, ErrStatusConflict
	}
	if req.Amount <= 0 {
		return nil, ErrRefundExceeded
	}
	effective, err := s.refundRepo.SumEffective(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Amount+effective > item.PaidAmount {
		return nil, ErrRefundExceeded
	}
	// 退款去向（doc36 §3.2）：默认退回余额；原路退回时扣点为渠道不退还的手续费。
	refundMode := strings.TrimSpace(req.RefundMode)
	if refundMode == "" {
		refundMode = model.RefundModeBalance
	}
	if refundMode != model.RefundModeBalance && refundMode != model.RefundModeChannel {
		return nil, ErrRefundModeInvalid
	}
	fee := 0.0
	if refundMode == model.RefundModeChannel {
		if req.FeeAmount < 0 || req.FeeAmount > req.Amount {
			return nil, ErrRefundFeeInvalid
		}
		fee = round2(req.FeeAmount)
	}

	refund := &model.OrderRefund{
		RefundNo:   genRefundNo(),
		OrderID:    item.ID,
		UserID:     item.UserID,
		Amount:     req.Amount,
		Reason:     req.Reason,
		Status:     model.RefundStatusPending,
		RefundMode: refundMode,
		FeeAmount:  fee,
		NetAmount:  round2(req.Amount + fee),
	}
	if err := s.refundRepo.Create(ctx, refund); err != nil {
		return nil, err
	}

	// 服务中订单转入退款中（状态机校验：active → refunding）
	if item.Status == model.OrderStatusActive {
		if err := EnsureStatus(item.Status, model.OrderStatusRefunding); err != nil {
			return nil, err
		}
		item.Status = model.OrderStatusRefunding
		item.OperatorID = operatorID
		if err := s.orderRepo.Update(ctx, item); err != nil {
			return nil, err
		}
	}
	return s.FindRefund(ctx, refund.ID)
}

// Activate 重新触发开通实例（幂等）。
func (s *orderService) Activate(ctx context.Context, id uint64) error {
	item, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return mapOrderErr(err)
	}
	if err := s.provision.Activate(ctx, item); err != nil {
		return err
	}
	if err := s.orderRepo.Update(ctx, item); err != nil {
		return err
	}
	// 订单开通成功后的推广返现计提：失败不回滚订单，钩子实现负责记录错误。
	if s.cashbackHook != nil {
		_ = s.cashbackHook(ctx, item.ID, item.OrderNo, item.UserID, item.PaidAmount, item.RenewalID != 0)
	}
	return nil
}

func (s *orderService) Stats(ctx context.Context) (*dto.OrderStatsResponse, error) {
	todayOrders, err := s.orderRepo.StatsCountToday(ctx)
	if err != nil {
		return nil, err
	}
	todaySales, err := s.orderRepo.StatsSalesToday(ctx)
	if err != nil {
		return nil, err
	}
	total, sales, err := s.orderRepo.StatsTotal(ctx)
	if err != nil {
		return nil, err
	}
	refundAmount, err := s.refundRepo.SumRefundedAmount(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := s.orderRepo.CountByStatus(ctx)
	if err != nil {
		return nil, err
	}
	trendPoints, err := s.orderRepo.StatsTrend(ctx, 7)
	if err != nil {
		return nil, err
	}

	avgOrderValue := 0.0
	if total > 0 {
		avgOrderValue = round2(sales / float64(total))
	}
	refundRate := 0.0
	if sales > 0 {
		refundRate = round2(refundAmount / sales * 100)
	}
	distribution := make([]dto.OrderStatusStat, 0, len(counts))
	for _, c := range counts {
		distribution = append(distribution, dto.OrderStatusStat{Status: c.Status, Count: c.Count})
	}

	return &dto.OrderStatsResponse{
		TodaySales:         round2(todaySales),
		TodayOrders:        todayOrders,
		AvgOrderValue:      avgOrderValue,
		RefundRate:         refundRate,
		StatusDistribution: distribution,
		Trend:              fillTrend(trendPoints, 7),
	}, nil
}

func (s *orderService) ListRefunds(ctx context.Context, q dto.RefundListQuery) (*dto.RefundListResponse, error) {
	items, total, err := s.refundRepo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	return s.refundListResponse(ctx, items, total, page, pageSize)
}

func (s *orderService) FindRefund(ctx context.Context, id uint64) (*dto.RefundInfo, error) {
	refund, err := s.refundRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapRefundErr(err)
	}
	orderNo, err := s.orderNoByID(ctx, refund.OrderID)
	if err != nil {
		return nil, err
	}
	info := buildRefundInfo(*refund, orderNo)
	return &info, nil
}

// ApproveRefund 审核通过：通过后若覆盖全部实付金额则回写订单为已退款。
func (s *orderService) ApproveRefund(ctx context.Context, id uint64, operatorID uint64, operatorName string, remark string) (*dto.RefundInfo, error) {
	refund, err := s.refundRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapRefundErr(err)
	}
	if refund.Status != model.RefundStatusPending {
		return nil, ErrStatusConflict
	}
	refund.Status = model.RefundStatusApproved
	refund.AuditBy = operatorID
	refund.AuditByName = operatorName
	now := time.Now()
	refund.AuditedAt = &now
	if err := s.refundRepo.Update(ctx, refund); err != nil {
		return nil, err
	}

	if order, err := s.orderRepo.FindByID(ctx, refund.OrderID); err == nil {
		effective, perr := s.refundRepo.SumEffective(ctx, order.ID)
		if perr == nil && effective >= order.PaidAmount {
			// 全额退款：paid / provisioning / refunding → refunded
			switch order.Status {
			case model.OrderStatusPaid, model.OrderStatusProvisioning, model.OrderStatusRefunding:
				if err := EnsureStatus(order.Status, model.OrderStatusRefunded); err == nil {
					order.Status = model.OrderStatusRefunded
					order.OperatorID = operatorID
					_ = s.orderRepo.Update(ctx, order)
				}
			}
		} else if perr == nil && order.Status == model.OrderStatusRefunding {
			// 部分退款：服务中订单回退为服务中
			order.Status = model.OrderStatusActive
			order.OperatorID = operatorID
			_ = s.orderRepo.Update(ctx, order)
		}
	}
	// 退款审核通过 → 按退款额占比冲减邀请人已计提返现（失败不回滚退款，钩子实现负责记录错误）。
	if s.refundHook != nil {
		if order, err := s.orderRepo.FindByID(ctx, refund.OrderID); err == nil {
			_ = s.refundHook(ctx, order.ID, order.OrderNo, order.UserID, order.PaidAmount, refund.Amount, refund.RefundNo)
		}
	}
	// 退款审核通过 → 按去向执行资金出口（doc36 §3.2）。
	// balance：直接退回余额，消费口径不变；channel：原路退回，收入口径扣减本金与扣点。
	// 失败不回滚审核结果：支付中心留存退款单与失败原因，由财务人工重试或线下处理。
	if s.channelRefundHook != nil {
		if order, err := s.orderRepo.FindByID(ctx, refund.OrderID); err == nil {
			channelNo, herr := s.channelRefundHook(ctx, order.ID, order.OrderNo, refund.RefundNo,
				order.UserID, refund.Amount, refund.FeeAmount, refund.RefundMode)
			refund.ChannelRefundNo = channelNo
			switch {
			case herr == nil && channelNo != "":
				// 渠道退款单已建：资金将按渠道异步流出，账单侧据此扣减口径。
				refund.ChannelRefundStatus = model.ChannelRefundStatusPending
			case refund.RefundMode == model.RefundModeChannel:
				// 意图原路退回但未能建渠道退款单（无渠道支付记录 / 渠道不支持 / 渠道缺失）：
				// 钩子实现已回落余额回补，资金并未流出平台，标记 failed。
				// 账单口径只统计 pending/success，故不会误扣本金与扣点。
				refund.ChannelRefundStatus = model.ChannelRefundStatusFailed
			}
			_ = s.refundRepo.Update(ctx, refund)
		}
	}
	return s.FindRefund(ctx, refund.ID)
}

// RejectRefund 驳回退款：若订单无其他待审核退款则回退为服务中。
func (s *orderService) RejectRefund(ctx context.Context, id uint64, operatorID uint64, operatorName string, remark string) (*dto.RefundInfo, error) {
	refund, err := s.refundRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapRefundErr(err)
	}
	if refund.Status != model.RefundStatusPending {
		return nil, ErrStatusConflict
	}
	refund.Status = model.RefundStatusRejected
	refund.AuditBy = operatorID
	refund.AuditByName = operatorName
	now := time.Now()
	refund.AuditedAt = &now
	if err := s.refundRepo.Update(ctx, refund); err != nil {
		return nil, err
	}

	if order, err := s.orderRepo.FindByID(ctx, refund.OrderID); err == nil && order.Status == model.OrderStatusRefunding {
		remains, rerr := s.refundRepo.FindByOrder(ctx, order.ID)
		hasInFlight := false
		for _, r := range remains {
			if r.Status == model.RefundStatusPending || r.Status == model.RefundStatusApproved {
				hasInFlight = true
				break
			}
		}
		if rerr == nil && !hasInFlight {
			order.Status = model.OrderStatusActive
			order.OperatorID = operatorID
			_ = s.orderRepo.Update(ctx, order)
		}
	}
	return s.FindRefund(ctx, refund.ID)
}

func (s *orderService) refundListResponse(ctx context.Context, items []model.OrderRefund, total int64, page, pageSize int) (*dto.RefundListResponse, error) {
	respItems := make([]dto.RefundInfo, 0, len(items))
	orderNos := make(map[uint64]string, len(items))
	var missing []uint64
	for _, rf := range items {
		if _, ok := orderNos[rf.OrderID]; !ok {
			missing = append(missing, rf.OrderID)
		}
	}
	if len(missing) > 0 {
		if orders, err := s.orderRepo.FindByIDs(ctx, missing); err == nil {
			for _, o := range orders {
				orderNos[o.ID] = o.OrderNo
			}
		}
	}
	for _, rf := range items {
		respItems = append(respItems, buildRefundInfo(rf, orderNos[rf.OrderID]))
	}
	return &dto.RefundListResponse{Items: respItems, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *orderService) orderNoByID(ctx context.Context, orderID uint64) (string, error) {
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return "", mapOrderErr(err)
	}
	return order.OrderNo, nil
}

// —— 构建器 ——

func buildOrderInfo(item model.Order) dto.OrderInfo {
	return dto.OrderInfo{
		ID:          item.ID,
		OrderNo:     item.OrderNo,
		UserID:      item.UserID,
		ProductID:   item.ProductID,
		ProductName: item.ProductName,
		Specs:       item.Specs,
		Quantity:    item.Quantity,
		PriceModel:  item.PriceModel,
		TotalAmount: item.TotalAmount,
		PaidAmount:  item.PaidAmount,
		Status:      item.Status,
		PayMethod:   item.PayMethod,
		PayTime:     formatTime(item.PayTime),
		ExpireTime:  formatTime(item.ExpireTime),
		Remark:      item.Remark,
		// 计费周期与续费标记（doc36 §3.4）：续费订单以 renewal_id != 0 判定。
		Cycle:     item.Cycle,
		RenewalID: item.RenewalID,
		// 算价快照（P5-01/P5-04）：原价 / 优惠 / 实付与命中来源。
		OriginalAmount: item.OriginalAmount,
		DiscountAmount: item.DiscountAmount,
		FinalAmount:    item.FinalAmount,
		DiscountSource: item.DiscountSource,
		// 支付单号与渠道流水号（doc36 §3.1）：由仓储从支付中心反查填充，空即未走渠道。
		PaymentNo: item.PaymentNo,
		ChannelTx: item.ChannelTx,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
		UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
	}
}

func buildOrderItemInfo(item model.OrderItem) dto.OrderItemInfo {
	return dto.OrderItemInfo{
		ID:          item.ID,
		OrderID:     item.OrderID,
		ProductID:   item.ProductID,
		ProductName: item.ProductName,
		SpecCode:    item.SpecCode,
		Specs:       item.Specs,
		Price:       item.Price,
		Quantity:    item.Quantity,
		Amount:      item.Amount,
		CreatedAt:   item.CreatedAt.Format(time.RFC3339),
	}
}

func buildRefundInfo(rf model.OrderRefund, orderNo string) dto.RefundInfo {
	return dto.RefundInfo{
		ID:          rf.ID,
		RefundNo:    rf.RefundNo,
		OrderID:     rf.OrderID,
		OrderNo:     orderNo,
		UserID:      rf.UserID,
		Amount:      rf.Amount,
		Reason:      rf.Reason,
		Status:      rf.Status,
		AuditBy:     rf.AuditBy,
		AuditByName: rf.AuditByName,
		AuditedAt:   formatTime(rf.AuditedAt),
		// 退款去向与扣点（doc36 §3.2）：空值视为余额退回（兼容迁移前存量数据）。
		RefundMode:          defaultRefundMode(rf.RefundMode),
		FeeAmount:           rf.FeeAmount,
		NetAmount:           rf.NetAmount,
		ChannelRefundNo:     rf.ChannelRefundNo,
		ChannelRefundStatus: rf.ChannelRefundStatus,
		CreatedAt:           rf.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           rf.UpdatedAt.Format(time.RFC3339),
	}
}

// defaultRefundMode 兼容存量退款单：迁移前无 refund_mode 列，一律按余额退回口径。
func defaultRefundMode(mode string) string {
	if strings.TrimSpace(mode) == "" {
		return model.RefundModeBalance
	}
	return mode
}

// —— 工具 ——

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func genRefundNo() string {
	return fmt.Sprintf("RF%s%03d", time.Now().Format("20060102150405"), rand.Intn(1000))
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// fillTrend 保证返回连续 N 日趋势点（缺失日期补零）。
func fillTrend(points []model.OrderTrendPoint, days int) []dto.OrderTrendPoint {
	byDate := make(map[string]model.OrderTrendPoint, len(points))
	for _, p := range points {
		byDate[p.Date] = p
	}
	result := make([]dto.OrderTrendPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		p, ok := byDate[d]
		if !ok {
			p = model.OrderTrendPoint{Date: d}
		}
		result = append(result, dto.OrderTrendPoint{Date: d, Sales: round2(p.Sales), Orders: p.Orders})
	}
	return result
}

func mapOrderErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrOrderNotFound
	}
	return err
}

func mapRefundErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrRefundNotFound
	}
	return err
}
