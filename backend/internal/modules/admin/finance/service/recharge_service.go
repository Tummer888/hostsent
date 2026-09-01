package service

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/finance/dto"
	"hostsent/backend/internal/modules/admin/finance/model"
	"hostsent/backend/internal/modules/admin/finance/repository"
	"hostsent/backend/internal/pkg/money"
)

// RechargeService 充值到账能力。
type RechargeService interface {
	Create(ctx context.Context, req dto.RechargeCreateRequest, operatorID uint64) (*dto.RechargeInfo, error)
	Approve(ctx context.Context, id uint64, req dto.RechargeApproveRequest, operatorID uint64) (*dto.RechargeInfo, error)
	ApproveByNo(ctx context.Context, rechargeNo string, req dto.RechargeApproveRequest, operatorID uint64) (*dto.RechargeInfo, error)
	List(ctx context.Context, q dto.RechargeListQuery) (*dto.RechargeListResponse, error)
}

type rechargeService struct {
	rechargeRepo repository.RechargeRepository
	wallet       WalletService
}

// NewRechargeService 创建充值服务。
func NewRechargeService(rechargeRepo repository.RechargeRepository, wallet WalletService) RechargeService {
	return &rechargeService{rechargeRepo: rechargeRepo, wallet: wallet}
}

// Create 线下充值登记（状态 pending，需人工确认到账）。
func (s *rechargeService) Create(ctx context.Context, req dto.RechargeCreateRequest, operatorID uint64) (*dto.RechargeInfo, error) {
	if req.Amount <= 0 {
		return nil, ErrInsufficientBalance
	}
	rc := &model.Recharge{
		RechargeNo: genRechargeNo(),
		UserID:     req.UserID,
		Amount:     money.Round2(req.Amount),
		Method:     req.Method,
		Status:     model.RechargeStatusPending,
		Remark:     req.Remark,
	}
	if err := s.rechargeRepo.Create(ctx, rc); err != nil {
		return nil, err
	}
	return s.rechargeInfo(ctx, rc.ID)
}

// Approve 手工确认到账（幂等）：pending→success 并调用账务核心入账。
// 幂等键 = 充值单号（recharge_no），已到账则直接返回。
func (s *rechargeService) Approve(ctx context.Context, id uint64, req dto.RechargeApproveRequest, operatorID uint64) (*dto.RechargeInfo, error) {
	rc, err := s.rechargeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapRechargeErr(err)
	}
	if err := s.doApprove(ctx, rc, req, operatorID); err != nil {
		return nil, err
	}
	return s.rechargeInfo(ctx, id)
}

// ApproveByNo 依据充值单号确认到账（渠道回调用），复用幂等入账逻辑。
func (s *rechargeService) ApproveByNo(ctx context.Context, rechargeNo string, req dto.RechargeApproveRequest, operatorID uint64) (*dto.RechargeInfo, error) {
	rc, err := s.rechargeRepo.FindByNo(ctx, rechargeNo)
	if err != nil {
		return nil, mapRechargeErr(err)
	}
	if err := s.doApprove(ctx, rc, req, operatorID); err != nil {
		return nil, err
	}
	return s.rechargeInfo(ctx, rc.ID)
}

// doApprove 幂等确认充值单到账：pending→success，并调用账务核心入账。
func (s *rechargeService) doApprove(ctx context.Context, rc *model.Recharge, req dto.RechargeApproveRequest, operatorID uint64) error {
	if rc.Status == model.RechargeStatusSuccess {
		return nil // 幂等：已到账
	}
	if rc.Status != model.RechargeStatusPending {
		return ErrStatusConflict
	}
	rc.Status = model.RechargeStatusSuccess
	rc.ChannelTx = req.ChannelTx
	now := time.Now()
	rc.PaidAt = &now
	if req.Remark != "" {
		rc.Remark = req.Remark
	}
	if err := s.rechargeRepo.Update(ctx, rc); err != nil {
		return err
	}
	// 调用账务核心入账（幂等键 = 充值单号）
	_, err := s.wallet.Change(ctx, ChangeRequest{
		UserID:     rc.UserID,
		Type:       model.TxTypeRecharge,
		Direction:  model.DirectionIncome,
		Amount:     rc.Amount,
		RefNo:      rc.RechargeNo,
		BizType:    "recharge",
		Remark:     rc.Remark,
		OperatorID: operatorID,
	})
	return err
}

func (s *rechargeService) List(ctx context.Context, q dto.RechargeListQuery) (*dto.RechargeListResponse, error) {
	items, total, err := s.rechargeRepo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	page := normalizePage(q.Page)
	pageSize := normalizePageSize(q.PageSize)
	resp := make([]dto.RechargeInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildRechargeInfo(item))
	}
	return &dto.RechargeListResponse{Items: resp, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *rechargeService) rechargeInfo(ctx context.Context, id uint64) (*dto.RechargeInfo, error) {
	rc, err := s.rechargeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapRechargeErr(err)
	}
	info := buildRechargeInfo(*rc)
	return &info, nil
}

func mapRechargeErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrRechargeNotFound
	}
	return err
}
