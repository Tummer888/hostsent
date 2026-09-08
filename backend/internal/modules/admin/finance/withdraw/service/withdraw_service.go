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

// WithdrawService 提现审核能力。
type WithdrawService interface {
	Approve(ctx context.Context, id uint64, req dto.WithdrawAuditRequest, operatorID uint64, operatorName string) (*dto.WithdrawInfo, error)
	Reject(ctx context.Context, id uint64, req dto.WithdrawAuditRequest, operatorID uint64, operatorName string) (*dto.WithdrawInfo, error)
	List(ctx context.Context, q dto.WithdrawListQuery) (*dto.WithdrawListResponse, error)
}

type withdrawService struct {
	withdrawRepo repository.WithdrawRepository
	wallet       accountservice.WalletService
}

// NewWithdrawService 创建提现服务。
func NewWithdrawService(withdrawRepo repository.WithdrawRepository, wallet accountservice.WalletService) WithdrawService {
	return &withdrawService{withdrawRepo: withdrawRepo, wallet: wallet}
}

// Approve 提现审核通过：pending→approved，并调用账务核心出账（支出流水），幂等键 = 提现单号。
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
	// 出账：写支出流水（结算方向）
	_, err = s.wallet.Change(ctx, accountservice.ChangeRequest{
		UserID:     w.UserID,
		Type:       transmodel.TxTypeSettlement,
		Direction:  transmodel.DirectionExpense,
		Amount:     w.Amount,
		RefNo:      w.WithdrawNo,
		BizType:    "withdraw",
		Remark:     w.Remark,
		OperatorID: operatorID,
	})
	if err != nil {
		return nil, err
	}
	return s.withdrawInfo(ctx, id)
}

// Reject 提现驳回：pending→rejected，不改动余额。
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
	return s.withdrawInfo(ctx, id)
}

func (s *withdrawService) List(ctx context.Context, q dto.WithdrawListQuery) (*dto.WithdrawListResponse, error) {
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
