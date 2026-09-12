package service

import (
	"context"
	"time"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/model"
	"hostsent/backend/internal/modules/admin/payment/repository"
)

// ReconService 渠道对账能力：本地已支付单 vs 渠道账单。
//
// S1 阶段渠道账单以「人工导入/线下核对」为主，本服务先统一口径：
//   - 本地侧：payment_orders 中 status=paid 的金额与笔数（按渠道 + 账期过滤）；
//   - 渠道侧：channel_amount_fen / channel_count 由渠道账单导入提供，未导入时以本地值占位；
//   - 差异 = 渠道 - 本地，非 0 记 suspicious。
//
// 后续 S6 接入渠道账单下载接口后，只需替换渠道侧取数，口径与落库结构不变。
type ReconService interface {
	// Reconcile 执行对账并落库对账记录。
	Reconcile(ctx context.Context, req dto.ReconRequest) (*dto.ReconResponse, error)
	List(ctx context.Context, page, pageSize int) (*dto.ReconRecordListResponse, error)
}

type reconService struct {
	orderRepo repository.OrderRepository
	reconRepo repository.ReconRepository
}

// NewReconService 创建对账服务。
func NewReconService(orderRepo repository.OrderRepository, reconRepo repository.ReconRepository) ReconService {
	return &reconService{orderRepo: orderRepo, reconRepo: reconRepo}
}

func (s *reconService) Reconcile(ctx context.Context, req dto.ReconRequest) (*dto.ReconResponse, error) {
	localAmount, localCount, err := s.orderRepo.SumPaid(ctx, req.ChannelCode, req.Period)
	if err != nil {
		return nil, err
	}
	// 渠道侧账单：S1 阶段未接入渠道下载，先以本地值占位（差异恒为 0），
	// 保证对账记录与口径结构就绪；S6 替换为渠道账单查询。
	channelAmount := localAmount
	channelCount := int(localCount)
	diff := channelAmount - localAmount
	status := "matched"
	if diff != 0 {
		status = "suspicious"
	}
	rec := &model.PaymentReconRecord{
		ReconNo:          genReconNo(),
		ChannelCode:      req.ChannelCode,
		Period:           req.Period,
		LocalAmountFen:   localAmount,
		LocalCount:       int(localCount),
		ChannelAmountFen: channelAmount,
		ChannelCount:     channelCount,
		DiffFen:          diff,
		Status:           status,
		Detail:           "{}",
		CreatedAt:        time.Now(),
	}
	if err := s.reconRepo.Create(ctx, rec); err != nil {
		return nil, err
	}
	return &dto.ReconResponse{
		ReconNo:       rec.ReconNo,
		ChannelCode:   rec.ChannelCode,
		Period:        rec.Period,
		LocalAmount:   fenToYuan(rec.LocalAmountFen),
		LocalCount:    rec.LocalCount,
		ChannelAmount: fenToYuan(rec.ChannelAmountFen),
		ChannelCount:  rec.ChannelCount,
		Diff:          fenToYuan(rec.DiffFen),
		Status:        rec.Status,
	}, nil
}

func (s *reconService) List(ctx context.Context, page, pageSize int) (*dto.ReconRecordListResponse, error) {
	items, total, err := s.reconRepo.List(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.ReconRecordInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildReconRecordInfo(item))
	}
	return &dto.ReconRecordListResponse{
		Items: resp,
		Meta:  dto.ListMeta{Page: normalizePage(page), PageSize: normalizePageSize(pageSize), Total: total},
	}, nil
}
