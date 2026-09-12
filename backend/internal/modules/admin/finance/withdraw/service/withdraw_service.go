package service

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	accountservice "hostsent/backend/internal/modules/admin/finance/account/service"
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
	"hostsent/backend/internal/modules/admin/finance/withdraw/dto"
	"hostsent/backend/internal/modules/admin/finance/withdraw/model"
	"hostsent/backend/internal/modules/admin/finance/withdraw/repository"
)

// PayoutPort 打款能力最小接口（由支付中心实现，装配层注入）。
// finance 只依赖这一最小契约，避免反向依赖 payment 模块。
type PayoutPort interface {
	// CreatePayout 为提现单创建打款任务：mode=api 走渠道接口，manual 等待人工登记。
	// mode 为期望模式（可空=按平台默认），actualMode 返回实际生效模式（渠道不可用时回落 manual）。
	CreatePayout(ctx context.Context, withdrawID uint64, withdrawNo string, userID uint64, amount float64, mode string) (payoutID uint64, payoutNo string, actualMode string, err error)
	// MarkPaid 标记打款成功（人工登记）。
	MarkPaid(ctx context.Context, payoutNo string, channelTx, receiptURL, remark string, operatorID uint64) error
	// Fail 标记打款失败并触发退回。
	Fail(ctx context.Context, payoutNo, reason string) error
}

// WithdrawService 提现审核能力：申请 → 冻结 → 审核 → 打款 → 结算。
type WithdrawService interface {
	// Apply 用户提交提现申请（冻结金额）。
	Apply(ctx context.Context, userID uint64, req dto.WithdrawApplyRequest) (*dto.WithdrawInfo, error)
	Approve(ctx context.Context, id uint64, req dto.WithdrawAuditRequest, operatorID uint64, operatorName string) (*dto.WithdrawInfo, error)
	Reject(ctx context.Context, id uint64, req dto.WithdrawAuditRequest, operatorID uint64, operatorName string) (*dto.WithdrawInfo, error)
	List(ctx context.Context, q dto.WithdrawListQuery) (*dto.WithdrawListResponse, error)
	// ListByUser 用户查看自己的提现记录。
	ListByUser(ctx context.Context, userID uint64, q dto.WithdrawListQuery) (*dto.WithdrawListResponse, error)
	// MarkPaid 财务登记打款完成。
	MarkPaid(ctx context.Context, id uint64, req dto.WithdrawPayoutRequest, operatorID uint64) (*dto.WithdrawInfo, error)
	// MarkFailed 财务登记打款失败（解冻退回）。
	MarkFailed(ctx context.Context, id uint64, req dto.WithdrawAuditRequest, operatorID uint64) (*dto.WithdrawInfo, error)
	// SetPayoutPort 注入打款能力（装配层调用）。
	SetPayoutPort(port PayoutPort)
}

type withdrawService struct {
	withdrawRepo repository.WithdrawRepository
	wallet       accountservice.WalletService
	payout       PayoutPort
}

// NewWithdrawService 创建提现服务。
func NewWithdrawService(withdrawRepo repository.WithdrawRepository, wallet accountservice.WalletService) WithdrawService {
	return &withdrawService{withdrawRepo: withdrawRepo, wallet: wallet}
}

func (s *withdrawService) SetPayoutPort(port PayoutPort) { s.payout = port }

// Apply 用户提交提现申请：校验余额 → 写提现单 → 冻结金额。
// 幂等键使用提现单号；冻结失败则删除提现单，避免出现"有单无冻结"。
func (s *withdrawService) Apply(ctx context.Context, userID uint64, req dto.WithdrawApplyRequest) (*dto.WithdrawInfo, error) {
	if req.Amount <= 0 {
		return nil, accountservice.ErrInsufficientBalance
	}
	if req.Channel != "bank" && req.Channel != "alipay" {
		return nil, ErrWithdrawChannelInvalid
	}
	if req.AccountNo == "" || req.AccountName == "" {
		return nil, ErrWithdrawAccountRequired
	}
	w := &model.Withdraw{
		WithdrawNo:  genWithdrawNo(),
		UserID:      userID,
		Amount:      req.Amount,
		Channel:     req.Channel,
		Account:     maskTail(req.AccountNo),
		AccountName: req.AccountName,
		BankName:    req.BankName,
		PayoutMode:  req.PayoutMode,
		Status:      model.WithdrawStatusPending,
		Remark:      req.Remark,
	}
	if w.PayoutMode == "" {
		w.PayoutMode = model.PayoutModeManual
	}
	if err := s.withdrawRepo.Create(ctx, w); err != nil {
		return nil, err
	}
	// 冻结：可用余额 → 冻结余额（提现申请即锁定资金，防止重复支出）
	if _, err := s.wallet.Freeze(ctx, accountservice.FreezeRequest{
		UserID:  userID,
		Amount:  w.Amount,
		RefNo:   w.WithdrawNo,
		BizType: "withdraw",
		Remark:  "提现申请冻结",
	}); err != nil {
		_ = s.withdrawRepo.Delete(ctx, w.ID)
		return nil, err
	}
	return s.withdrawInfo(ctx, w.ID)
}

// Approve 审核通过：pending→approved，创建打款任务（不再直接扣款）。
// 资金已在申请时冻结；打款成功才结算为支出，打款失败则解冻退回。
func (s *withdrawService) Approve(ctx context.Context, id uint64, req dto.WithdrawAuditRequest, operatorID uint64, operatorName string) (*dto.WithdrawInfo, error) {
	w, err := s.withdrawRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapWithdrawErr(err)
	}
	if w.Status != model.WithdrawStatusPending {
		return nil, accountservice.ErrStatusConflict
	}
	w.Status = model.WithdrawStatusApproved
	w.AuditBy = operatorID
	w.AuditByName = operatorName
	now := time.Now()
	w.AuditedAt = &now
	if req.Remark != "" {
		w.Remark = req.Remark
	}
	if err := s.withdrawRepo.Update(ctx, w); err != nil {
		return nil, err
	}
	// 创建打款任务（人工/接口双模）。无打款能力时保持 approved，等待财务人工处理。
	if s.payout != nil {
		payoutID, payoutNo, actualMode, perr := s.payout.CreatePayout(ctx, w.ID, w.WithdrawNo, w.UserID, w.Amount, w.PayoutMode)
		if perr != nil {
			return nil, perr
		}
		w.PayoutNo = payoutNo
		w.PayoutID = payoutID
		w.PayoutMode = actualMode
		// 接口打款已成功时同步推进到已打款并结算冻结资金（人工模式等待财务登记）。
		if actualMode == model.PayoutModeAPI {
			if p, ferr := s.payoutStatus(ctx, payoutNo); ferr == nil {
				switch p {
				case model.WithdrawStatusPaid:
					return s.markPaidSettled(ctx, w, "", "", "渠道接口打款成功", operatorID)
				case model.WithdrawStatusFailed:
					return s.MarkFailed(ctx, w.ID, dto.WithdrawAuditRequest{Remark: "渠道接口打款失败"}, operatorID)
				default:
					w.Status = model.WithdrawStatusPaying
				}
			}
		}
		if err := s.withdrawRepo.Update(ctx, w); err != nil {
			return nil, err
		}
	}
	return s.withdrawInfo(ctx, id)
}

// payoutStatus 读取打款单状态（payout 端口未暴露查询时的保守兜底）。
func (s *withdrawService) payoutStatus(ctx context.Context, payoutNo string) (string, error) {
	if reader, ok := s.payout.(interface {
		StatusByNo(ctx context.Context, payoutNo string) (string, error)
	}); ok {
		return reader.StatusByNo(ctx, payoutNo)
	}
	return "", nil
}

// Reject 审核驳回：pending→rejected，解冻已冻结金额。
func (s *withdrawService) Reject(ctx context.Context, id uint64, req dto.WithdrawAuditRequest, operatorID uint64, operatorName string) (*dto.WithdrawInfo, error) {
	w, err := s.withdrawRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapWithdrawErr(err)
	}
	if w.Status != model.WithdrawStatusPending {
		return nil, accountservice.ErrStatusConflict
	}
	w.Status = model.WithdrawStatusRejected
	w.AuditBy = operatorID
	w.AuditByName = operatorName
	now := time.Now()
	w.AuditedAt = &now
	if req.Remark != "" {
		w.Remark = req.Remark
	}
	if err := s.withdrawRepo.Update(ctx, w); err != nil {
		return nil, err
	}
	if _, err := s.wallet.Unfreeze(ctx, accountservice.FreezeRequest{
		UserID:     w.UserID,
		Amount:     w.Amount,
		RefNo:      w.WithdrawNo + "-reject",
		BizType:    "withdraw",
		Remark:     "提现驳回解冻",
		OperatorID: operatorID,
	}); err != nil {
		return nil, err
	}
	return s.withdrawInfo(ctx, id)
}

// MarkPaid 财务登记打款完成：approved/paying→paid，结算冻结资金（冻结列扣减，计支出）。
func (s *withdrawService) MarkPaid(ctx context.Context, id uint64, req dto.WithdrawPayoutRequest, operatorID uint64) (*dto.WithdrawInfo, error) {
	w, err := s.withdrawRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapWithdrawErr(err)
	}
	if w.Status != model.WithdrawStatusApproved && w.Status != model.WithdrawStatusPaying {
		return nil, accountservice.ErrStatusConflict
	}
	return s.markPaidSettled(ctx, w, req.ChannelTx, req.ReceiptURL, req.Remark, operatorID)
}

// markPaidSettled 置为已打款并结算冻结资金（人工登记与接口打款共用）。
// 幂等：SettleFrozen 以提现单号为 biz 键，重复调用只记一次账。
func (s *withdrawService) markPaidSettled(ctx context.Context, w *model.Withdraw, channelTx, receiptURL, remark string, operatorID uint64) (*dto.WithdrawInfo, error) {
	now := time.Now()
	w.Status = model.WithdrawStatusPaid
	w.PaidAt = &now
	if channelTx != "" {
		w.ChannelTx = channelTx
	}
	if remark != "" {
		w.Remark = remark
	}
	if err := s.withdrawRepo.Update(ctx, w); err != nil {
		return nil, err
	}
	if _, err := s.wallet.SettleFrozen(ctx, accountservice.FreezeRequest{
		UserID:     w.UserID,
		Amount:     w.Amount,
		RefNo:      w.WithdrawNo,
		BizType:    "withdraw-settle",
		TxType:     transmodel.TxTypeSettlement,
		Remark:     "提现打款结算",
		OperatorID: operatorID,
	}); err != nil {
		return nil, err
	}
	if s.payout != nil && w.PayoutNo != "" {
		_ = s.payout.MarkPaid(ctx, w.PayoutNo, channelTx, receiptURL, remark, operatorID)
	}
	return s.withdrawInfo(ctx, w.ID)
}

// MarkFailed 打款失败：解冻退回余额，状态置 failed。
func (s *withdrawService) MarkFailed(ctx context.Context, id uint64, req dto.WithdrawAuditRequest, operatorID uint64) (*dto.WithdrawInfo, error) {
	w, err := s.withdrawRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapWithdrawErr(err)
	}
	if w.Status != model.WithdrawStatusApproved && w.Status != model.WithdrawStatusPaying {
		return nil, accountservice.ErrStatusConflict
	}
	w.Status = model.WithdrawStatusFailed
	if req.Remark != "" {
		w.Remark = req.Remark
	}
	if err := s.withdrawRepo.Update(ctx, w); err != nil {
		return nil, err
	}
	if _, err := s.wallet.Unfreeze(ctx, accountservice.FreezeRequest{
		UserID:     w.UserID,
		Amount:     w.Amount,
		RefNo:      w.WithdrawNo + "-fail",
		BizType:    "withdraw",
		Remark:     "提现打款失败退回",
		OperatorID: operatorID,
	}); err != nil {
		return nil, err
	}
	if s.payout != nil && w.PayoutNo != "" {
		_ = s.payout.Fail(ctx, w.PayoutNo, req.Remark)
	}
	return s.withdrawInfo(ctx, id)
}

func (s *withdrawService) List(ctx context.Context, q dto.WithdrawListQuery) (*dto.WithdrawListResponse, error) {
	return s.list(ctx, q)
}

func (s *withdrawService) ListByUser(ctx context.Context, userID uint64, q dto.WithdrawListQuery) (*dto.WithdrawListResponse, error) {
	q.UserID = userID
	return s.list(ctx, q)
}

func (s *withdrawService) list(ctx context.Context, q dto.WithdrawListQuery) (*dto.WithdrawListResponse, error) {
	items, total, err := s.withdrawRepo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	page := normalizePage(q.Page)
	pageSize := normalizePageSize(q.PageSize)
	resp := make([]dto.WithdrawInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildWithdrawInfo(item))
	}
	return &dto.WithdrawListResponse{Items: resp, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *withdrawService) withdrawInfo(ctx context.Context, id uint64) (*dto.WithdrawInfo, error) {
	w, err := s.withdrawRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapWithdrawErr(err)
	}
	info := buildWithdrawInfo(*w)
	return &info, nil
}

func mapWithdrawErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrWithdrawNotFound
	}
	return err
}
