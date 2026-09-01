package service

import (
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/finance/dto"
	"hostsent/backend/internal/modules/admin/finance/model"
	"hostsent/backend/internal/modules/admin/finance/repository"
	"hostsent/backend/internal/pkg/money"
)

// BillService 账单能力：按账期归集消费与退款。
type BillService interface {
	List(ctx context.Context, q dto.BillListQuery) (*dto.BillListResponse, error)
	Close(ctx context.Context, id uint64) error
	// GenerateForUser 归集某用户在指定账期（如 202608）的消费/退款，生成（upsert）账单。
	GenerateForUser(ctx context.Context, userID uint64, period string) (*model.Bill, error)
}

type billService struct {
	billRepo repository.BillRepository
	txRepo   repository.TransactionRepository
}

// NewBillService 创建账单服务。
func NewBillService(billRepo repository.BillRepository, txRepo repository.TransactionRepository) BillService {
	return &billService{billRepo: billRepo, txRepo: txRepo}
}

func (s *billService) List(ctx context.Context, q dto.BillListQuery) (*dto.BillListResponse, error) {
	items, total, err := s.billRepo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	page := normalizePage(q.Page)
	pageSize := normalizePageSize(q.PageSize)
	resp := make([]dto.BillInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildBillInfo(item))
	}
	return &dto.BillListResponse{Items: resp, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

// Close 关账：仅未结/已结账单可转为已关账。
func (s *billService) Close(ctx context.Context, id uint64) error {
	b, err := s.billRepo.FindByID(ctx, id)
	if err != nil {
		return mapBillErr(err)
	}
	if b.Status == model.BillStatusClosed {
		return nil // 幂等：已关账
	}
	return s.billRepo.Close(ctx, id)
}

func (s *billService) GenerateForUser(ctx context.Context, userID uint64, period string) (*model.Bill, error) {
	start, end, err := periodRange(period)
	if err != nil {
		return nil, err
	}
	// 消费为支出（-），退款为收入（+）。本期应结 = 消费总额 - 退款总额。
	consumeSum, err := s.txRepo.SumByType(ctx, userID, []string{model.TxTypeConsume}, start, end)
	if err != nil {
		return nil, err
	}
	refundSum, err := s.txRepo.SumByType(ctx, userID, []string{model.TxTypeRefund}, start, end)
	if err != nil {
		return nil, err
	}
	consume := money.Round2(-consumeSum) // 消费为正数
	refund := money.Round2(refundSum)    // 退款为正数
	total := money.Round2(consume - refund)
	if total < 0 {
		total = 0
	}

	detail, _ := json.Marshal(map[string]any{"consume": consume, "refund": refund})
	b := &model.Bill{
		BillNo:       genBillNo(),
		UserID:       userID,
		Period:       period,
		TotalAmount:  total,
		RefundAmount: refund,
		Status:       model.BillStatusUnpaid,
		Detail:       string(detail),
	}
	if err := s.billRepo.Upsert(ctx, b); err != nil {
		return nil, err
	}
	return s.billRepo.FindByUserPeriod(ctx, userID, period)
}

func mapBillErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrBillNotFound
	}
	return err
}
