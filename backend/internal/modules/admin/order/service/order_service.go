package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
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
	Stats(ctx context.Context) (*dto.OrderStatsResponse, error)
	// SetCashbackHook 注入订单完成后的推广返现计提钩子（装配层调用）。
	SetCashbackHook(hook OrderCashbackHook)
	// SetRefundHook 注入退款审核通过后的返现冲减钩子（装配层调用）。
	SetRefundHook(hook OrderRefundHook)
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

type orderService struct {
	orderRepo  repository.OrderRepository
	itemRepo   repository.OrderItemRepository
	refundRepo repository.RefundRepository
	provision  ProvisionAdapter
	// cashbackHook 可选：订单开通成功后计提推广返现，为 nil 时跳过。
	cashbackHook OrderCashbackHook
	// refundHook 可选：退款审核通过后冲减已计提返现，为 nil 时跳过。
	refundHook OrderRefundHook
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

	refund := &model.OrderRefund{
		RefundNo: genRefundNo(),
		OrderID:  item.ID,
		UserID:   item.UserID,
		Amount:   req.Amount,
		Reason:   req.Reason,
		Status:   model.RefundStatusPending,
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
		CreatedAt:   item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   item.UpdatedAt.Format(time.RFC3339),
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
		CreatedAt:   rf.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   rf.UpdatedAt.Format(time.RFC3339),
	}
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
