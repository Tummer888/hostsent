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

// PayoutService 打款能力：人工登记 / 渠道接口双模。
// 实现 finance/withdraw 的 PayoutPort 最小接口（装配层注入）。
type PayoutService interface {
	CreatePayout(ctx context.Context, withdrawID uint64, withdrawNo string, userID uint64, amount float64, mode string) (payoutID uint64, payoutNo string, actualMode string, err error)
	MarkPaid(ctx context.Context, payoutNo string, channelTx, receiptURL, remark string, operatorID uint64) error
	Fail(ctx context.Context, payoutNo, reason string) error
	// StatusByNo 读取打款单状态（提现审批后由 finance 判定 api 打款结果）。
	StatusByNo(ctx context.Context, payoutNo string) (string, error)
	List(ctx context.Context, q dto.PayoutListQuery) (*dto.PayoutListResponse, error)
	// AdminMarkPaid / AdminRetry 管理端操作（同步提现单状态由 finance withdraw 负责）。
	AdminMarkPaid(ctx context.Context, id uint64, req dto.PayoutMarkPaidRequest, operatorID uint64) (*dto.PayoutInfo, error)
	AdminRetry(ctx context.Context, id uint64, operatorID uint64) (*dto.PayoutInfo, error)
	// SetSettler 注入打款结算回调（装配层调用，避免 payment 反向依赖 finance）。
	SetSettler(settler PayoutSettler)
}

// PayoutSettler 打款成功后回调 finance 结算（置提现单为已打款并结算冻结资金）。
// 由装配层注入 withdraw.MarkPaid，使「支付中心打款管理」与「财务提现单」状态一致；
// payment 不直接依赖 finance，耦合点仍只在装配层。
type PayoutSettler func(ctx context.Context, withdrawID uint64, channelTx, receiptURL, remark string, operatorID uint64) error

type payoutService struct {
	payoutRepo repository.PayoutRepository
	resolver   ChannelResolver
	settler    PayoutSettler
}

// NewPayoutService 创建打款服务。
func NewPayoutService(payoutRepo repository.PayoutRepository, resolver ChannelResolver) PayoutService {
	return &payoutService{payoutRepo: payoutRepo, resolver: resolver}
}

// SetSettler 注入打款结算回调（装配层调用）。
func (s *payoutService) SetSettler(settler PayoutSettler) { s.settler = settler }

// CreatePayout 创建打款任务。
// mode=api 时尝试走渠道 Payouter；渠道不支持则回落为 manual（人工登记），不阻断审批。
func (s *payoutService) CreatePayout(ctx context.Context, withdrawID uint64, withdrawNo string, userID uint64, amount float64, mode string) (uint64, string, string, error) {
	if mode != model.PayoutModeAPI {
		mode = model.PayoutModeManual
	}
	if existing, err := s.payoutRepo.FindByWithdrawID(ctx, withdrawID); err == nil && existing != nil {
		return existing.ID, existing.PayoutNo, existing.Mode, nil
	}
	p := &model.PaymentPayout{
		PayoutNo:   genPayoutNo(),
		WithdrawID: withdrawID,
		WithdrawNo: withdrawNo,
		UserID:     userID,
		AmountFen:  yuanToFen(amount),
		Mode:       mode,
		Status:     model.PayoutStatusPending,
	}
	if err := s.payoutRepo.Create(ctx, p); err != nil {
		return 0, "", "", err
	}
	// 人工模式到此为止，等待财务登记打款。
	if mode == model.PayoutModeManual {
		return p.ID, p.PayoutNo, p.Mode, nil
	}
	// 接口模式：尝试渠道打款，失败或渠道不支持时回落人工。
	ch, err := s.resolver.Resolve(ctx, 0, payment.SceneNative, p.AmountFen, "")
	if err != nil {
		p.Mode = model.PayoutModeManual
		p.FailReason = "渠道不可用，转人工打款: " + err.Error()
		_ = s.payoutRepo.Update(ctx, p)
		return p.ID, p.PayoutNo, p.Mode, nil
	}
	cfg, err := s.resolver.BuildConfig(ctx, ch)
	if err != nil {
		p.Mode = model.PayoutModeManual
		p.FailReason = "渠道配置错误，转人工打款: " + err.Error()
		_ = s.payoutRepo.Update(ctx, p)
		return p.ID, p.PayoutNo, p.Mode, nil
	}
	gw, err := payment.Build(ch.Type, cfg)
	if err != nil {
		p.Mode = model.PayoutModeManual
		p.FailReason = "渠道适配器不可用，转人工打款: " + err.Error()
		_ = s.payoutRepo.Update(ctx, p)
		return p.ID, p.PayoutNo, p.Mode, nil
	}
	payouter, ok := gw.(payment.Payouter)
	if !ok {
		p.Mode = model.PayoutModeManual
		p.FailReason = "渠道不支持接口打款，转人工打款"
		_ = s.payoutRepo.Update(ctx, p)
		return p.ID, p.PayoutNo, p.Mode, nil
	}
	p.ChannelID = ch.ID
	p.ChannelCode = ch.ChannelCode
	p.Status = model.PayoutStatusPaying
	p.Attempts++
	res, perr := payouter.Payout(ctx, cfg, &payment.PayoutRequest{
		OutPayoutNo: p.PayoutNo,
		AmountFen:   p.AmountFen,
		Remark:      "用户提现 " + withdrawNo,
	})
	if perr != nil {
		p.Status = model.PayoutStatusFailed
		p.FailReason = perr.Error()
		_ = s.payoutRepo.Update(ctx, p)
		return p.ID, p.PayoutNo, p.Mode, perr
	}
	p.ChannelTx = res.ChannelTx
	p.ReceiptURL = res.Receipt
	switch res.Status {
	case model.PayoutStatusPaid:
		p.Status = model.PayoutStatusPaid
		now := time.Now()
		p.PaidAt = &now
	case model.PayoutStatusFailed:
		p.Status = model.PayoutStatusFailed
		p.FailReason = res.Message
	default:
		p.Status = model.PayoutStatusPaying
	}
	if err := s.payoutRepo.Update(ctx, p); err != nil {
		return p.ID, p.PayoutNo, p.Mode, err
	}
	return p.ID, p.PayoutNo, p.Mode, nil
}

func (s *payoutService) MarkPaid(ctx context.Context, payoutNo string, channelTx, receiptURL, remark string, operatorID uint64) error {
	p, err := s.findByNo(ctx, payoutNo)
	if err != nil {
		return err
	}
	now := time.Now()
	p.Status = model.PayoutStatusPaid
	p.PaidAt = &now
	p.OperatorID = operatorID
	if channelTx != "" {
		p.ChannelTx = channelTx
	}
	if receiptURL != "" {
		p.ReceiptURL = receiptURL
	}
	if remark != "" {
		p.Remark = remark
	}
	return s.payoutRepo.Update(ctx, p)
}

func (s *payoutService) Fail(ctx context.Context, payoutNo, reason string) error {
	p, err := s.findByNo(ctx, payoutNo)
	if err != nil {
		return err
	}
	p.Status = model.PayoutStatusFailed
	p.FailReason = reason
	return s.payoutRepo.Update(ctx, p)
}

// StatusByNo 读取打款单状态。
func (s *payoutService) StatusByNo(ctx context.Context, payoutNo string) (string, error) {
	p, err := s.findByNo(ctx, payoutNo)
	if err != nil {
		return "", err
	}
	return p.Status, nil
}

func (s *payoutService) List(ctx context.Context, q dto.PayoutListQuery) (*dto.PayoutListResponse, error) {
	items, total, err := s.payoutRepo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.PayoutInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildPayoutInfo(item))
	}
	return &dto.PayoutListResponse{
		Items: resp,
		Meta:  dto.ListMeta{Page: normalizePage(q.Page), PageSize: normalizePageSize(q.PageSize), Total: total},
	}, nil
}

func (s *payoutService) AdminMarkPaid(ctx context.Context, id uint64, req dto.PayoutMarkPaidRequest, operatorID uint64) (*dto.PayoutInfo, error) {
	p, err := s.payoutRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPayoutNotFound
		}
		return nil, err
	}
	if p.Status == model.PayoutStatusPaid {
		// 已登记过打款：仍回调一次结算，修复「打款单已 paid 但提现单未结算」的历史数据。
		s.syncWithdraw(ctx, p, operatorID)
		info := buildPayoutInfo(*p)
		return &info, nil
	}
	if p.Status != model.PayoutStatusPending && p.Status != model.PayoutStatusPaying && p.Status != model.PayoutStatusFailed {
		return nil, ErrPayoutStatusConflict
	}
	now := time.Now()
	p.Status = model.PayoutStatusPaid
	p.PaidAt = &now
	p.OperatorID = operatorID
	if req.ChannelTx != "" {
		p.ChannelTx = req.ChannelTx
	}
	if req.ReceiptURL != "" {
		p.ReceiptURL = req.ReceiptURL
	}
	if req.Remark != "" {
		p.Remark = req.Remark
	}
	p.FailReason = ""
	if err := s.payoutRepo.Update(ctx, p); err != nil {
		return nil, err
	}
	s.syncWithdraw(ctx, p, operatorID)
	info := buildPayoutInfo(*p)
	return &info, nil
}

// syncWithdraw 通知 finance 将对应提现单置为已打款并结算冻结资金（幂等）。
// 结算失败不回滚打款单：资金侧以提现单为准，由财务在提现单上重试。
func (s *payoutService) syncWithdraw(ctx context.Context, p *model.PaymentPayout, operatorID uint64) {
	if s.settler == nil || p.WithdrawID == 0 {
		return
	}
	_ = s.settler(ctx, p.WithdrawID, p.ChannelTx, p.ReceiptURL, p.Remark, operatorID)
}

// AdminRetry 重试接口打款（仅 api 模式且非已支付）。
func (s *payoutService) AdminRetry(ctx context.Context, id uint64, operatorID uint64) (*dto.PayoutInfo, error) {
	p, err := s.payoutRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPayoutNotFound
		}
		return nil, err
	}
	if p.Status == model.PayoutStatusPaid {
		return nil, ErrPayoutStatusConflict
	}
	p.Attempts++
	p.Status = model.PayoutStatusPaying
	p.OperatorID = operatorID
	if err := s.payoutRepo.Update(ctx, p); err != nil {
		return nil, err
	}
	info := buildPayoutInfo(*p)
	return &info, nil
}

func (s *payoutService) findByNo(ctx context.Context, payoutNo string) (*model.PaymentPayout, error) {
	p, err := s.payoutRepo.FindByNo(ctx, payoutNo)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPayoutNotFound
		}
		return nil, err
	}
	return p, nil
}
