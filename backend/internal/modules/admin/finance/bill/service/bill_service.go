package service

import (
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/finance/bill/dto"
	billmodel "hostsent/backend/internal/modules/admin/finance/bill/model"
	"hostsent/backend/internal/modules/admin/finance/bill/repository"
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
	transrepo "hostsent/backend/internal/modules/admin/finance/transaction/repository"
	"hostsent/backend/internal/pkg/money"
)

// BillService 账单能力：按账期归集消费与退款。
type BillService interface {
	List(ctx context.Context, q dto.BillListQuery) (*dto.BillListResponse, error)
	Close(ctx context.Context, id uint64) error
	// GenerateForUser 归集某用户在指定账期（如 202608）的消费/退款，生成（upsert）账单。
	GenerateForUser(ctx context.Context, userID uint64, period string) (*billmodel.Bill, error)
	// FindByID 读取账单（支付前校验应结金额）。
	FindByID(ctx context.Context, id uint64) (*billmodel.Bill, error)
	// MarkPaid 账单结清并登记支付方式（由支付单 paid 事件触发，幂等）。
	MarkPaid(ctx context.Context, id uint64, paidAmountFen int64, paidMethod string, paidChannelID uint64) error
}

type billService struct {
	billRepo repository.BillRepository
	txRepo   transrepo.TransactionRepository
}

// NewBillService 创建账单服务。
func NewBillService(billRepo repository.BillRepository, txRepo transrepo.TransactionRepository) BillService {
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
	if b.Status == billmodel.BillStatusClosed {
		return nil // 幂等：已关账
	}
	return s.billRepo.Close(ctx, id)
}

func (s *billService) GenerateForUser(ctx context.Context, userID uint64, period string) (*billmodel.Bill, error) {
	start, end, err := periodRange(period)
	if err != nil {
		return nil, err
	}
	// 消费为支出（-），退款为收入（+）。本期应结 = 消费总额 - 退款总额。
	consumeSum, err := s.txRepo.SumByType(ctx, userID, []string{transmodel.TxTypeConsume}, start, end)
	if err != nil {
		return nil, err
	}
	refundSum, err := s.txRepo.SumByType(ctx, userID, []string{transmodel.TxTypeRefund}, start, end)
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
	b := &billmodel.Bill{
		BillNo:       genBillNo(),
		UserID:       userID,
		Period:       period,
		TotalAmount:  total,
		RefundAmount: refund,
		Status:       billmodel.BillStatusUnpaid,
		Detail:       string(detail),
	}
	if err := s.billRepo.Upsert(ctx, b); err != nil {
		return nil, err
	}
	return s.billRepo.FindByUserPeriod(ctx, userID, period)
}

// FindByID 读取账单（支付前校验应结金额）。
func (s *billService) FindByID(ctx context.Context, id uint64) (*billmodel.Bill, error) {
	b, err := s.billRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapBillErr(err)
	}
	return b, nil
}

// MarkPaid 账单结清并登记支付方式；已结清时直接返回（幂等）。
func (s *billService) MarkPaid(ctx context.Context, id uint64, paidAmountFen int64, paidMethod string, paidChannelID uint64) error {
	b, err := s.billRepo.FindByID(ctx, id)
	if err != nil {
		return mapBillErr(err)
	}
	if b.Status == billmodel.BillStatusPaid {
		return nil
	}
	if b.Status == billmodel.BillStatusClosed {
		return ErrStatusConflict
	}
	return s.billRepo.MarkPaid(ctx, id, paidAmountFen, paidMethod, paidChannelID)
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
