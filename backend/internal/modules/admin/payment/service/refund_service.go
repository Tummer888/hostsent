package service

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/model"
	"hostsent/backend/internal/modules/admin/payment/repository"
	"hostsent/backend/internal/pkg/payment"
)

// RefundService 渠道退款能力：订单退款审批通过后原路退回。
type RefundService interface {
	// CreateForOrder 依据订单退款单发起渠道退款（幂等：同订单退款单号只退一次）。
	CreateForOrder(ctx context.Context, orderRefundNo string, orderNo string, userID uint64, amount float64) (*dto.RefundInfo, error)
	List(ctx context.Context, q dto.RefundListQuery) (*dto.RefundListResponse, error)
}

type refundService struct {
	refundRepo repository.RefundRepository
	orderRepo  repository.OrderRepository
	resolver   ChannelResolver
}

// NewRefundService 创建退款服务。
func NewRefundService(refundRepo repository.RefundRepository, orderRepo repository.OrderRepository, resolver ChannelResolver) RefundService {
	return &refundService{refundRepo: refundRepo, orderRepo: orderRepo, resolver: resolver}
}

// CreateForOrder 依据订单退款单发起渠道退款。
// 找不到对应支付单（如余额支付）时返回 nil，由调用方走内部余额回补。
func (s *refundService) CreateForOrder(ctx context.Context, orderRefundNo string, orderNo string, userID uint64, amount float64) (*dto.RefundInfo, error) {
	if amount <= 0 {
		return nil, ErrRefundAmountExceeded
	}
	// 幂等：同订单退款单号已有退款记录直接返回。
	if orderRefundNo != "" {
		if existing, _, lerr := s.refundRepo.List(ctx, dto.RefundListQuery{Page: 1, PageSize: 100}); lerr == nil {
			for _, r := range existing {
				if r.OrderRefundNo == orderRefundNo {
					info := buildRefundInfo(r)
					return &info, nil
				}
			}
		}
	}
	// 定位该订单的已支付支付单（按业务单号 + 已支付状态）。
	target, terr := s.orderRepo.FindPaidByBiz(ctx, model.BizTypeOrder, orderNo)
	if terr != nil {
		if errors.Is(terr, gorm.ErrRecordNotFound) {
			return nil, nil // 无渠道支付记录：余额支付，由 finance 内部回补
		}
		return nil, terr
	}
	amountFen := yuanToFen(amount)
	refunded, err := s.refundRepo.SumSuccessByPayment(ctx, target.PaymentNo)
	if err != nil {
		return nil, err
	}
	if refunded+amountFen > target.AmountFen {
		return nil, ErrRefundAmountExceeded
	}

	ch, err := s.channelByID(ctx, target.ChannelID)
	if err != nil {
		return nil, err
	}
	refund := &model.PaymentRefund{
		RefundNo:      genRefundNo(),
		PaymentNo:     target.PaymentNo,
		OrderRefundNo: orderRefundNo,
		UserID:        userID,
		AmountFen:     amountFen,
		ChannelID:     ch.ID,
		Status:        model.RefundStatusPending,
		Reason:        "订单退款",
	}
	if err := s.refundRepo.Create(ctx, refund); err != nil {
		return nil, err
	}
	cfg, err := s.resolver.BuildConfig(ctx, ch)
	if err != nil {
		return nil, err
	}
	gw, err := payment.Build(ch.Type, cfg)
	if err != nil {
		return nil, err
	}
	refunder, ok := gw.(payment.Refunder)
	if !ok {
		refund.Status = model.RefundStatusFailed
		refund.FailReason = "渠道不支持退款，请财务人工处理"
		_ = s.refundRepo.Update(ctx, refund)
		return nil, ErrRefundNotSupported
	}
	res, err := refunder.Refund(ctx, cfg, &payment.RefundRequest{
		OutTradeNo:  target.PaymentNo,
		OutRefundNo: refund.RefundNo,
		AmountFen:   amountFen,
		TotalFen:    target.AmountFen,
		Reason:      refund.Reason,
	})
	if err != nil {
		refund.Status = model.RefundStatusFailed
		refund.FailReason = err.Error()
		_ = s.refundRepo.Update(ctx, refund)
		return nil, err
	}
	refund.ChannelRefundID = res.ChannelRefundID
	if res.Status == model.RefundStatusSuccess {
		refund.Status = model.RefundStatusSuccess
		now := time.Now()
		refund.RefundedAt = &now
	} else {
		refund.Status = model.RefundStatusPending
	}
	if err := s.refundRepo.Update(ctx, refund); err != nil {
		return nil, err
	}
	info := buildRefundInfo(*refund)
	return &info, nil
}

func (s *refundService) List(ctx context.Context, q dto.RefundListQuery) (*dto.RefundListResponse, error) {
	items, total, err := s.refundRepo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.RefundInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildRefundInfo(item))
	}
	return &dto.RefundListResponse{
		Items: resp,
		Meta:  dto.ListMeta{Page: normalizePage(q.Page), PageSize: normalizePageSize(q.PageSize), Total: total},
	}, nil
}

func (s *refundService) channelByID(ctx context.Context, id uint64) (*model.PaymentChannel, error) {
	cr, ok := s.resolver.(*channelService)
	if !ok {
		return nil, ErrChannelNotFound
	}
	ch, err := cr.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}
	return ch, nil
}
