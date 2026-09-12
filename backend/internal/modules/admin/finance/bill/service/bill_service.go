package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

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
	// —— 发票（doc36 §3.3）——
	// RequestInvoice 用户申请开票：写申请单并把账单置为已申请（幂等：同账单唯一待处理申请）。
	RequestInvoice(ctx context.Context, userID uint64, req dto.InvoiceApplyRequest) (*dto.InvoiceInfo, error)
	// IssueInvoice 管理端开票：回填发票号并把账单置为已开票。
	IssueInvoice(ctx context.Context, requestID uint64, req dto.InvoiceIssueRequest, operatorID uint64) (*dto.InvoiceInfo, error)
	// RejectInvoice 管理端驳回开票申请。
	RejectInvoice(ctx context.Context, requestID uint64, reason string, operatorID uint64) (*dto.InvoiceInfo, error)
	// Invoices 管理端发票申请列表。
	Invoices(ctx context.Context, q dto.InvoiceListQuery) (*dto.InvoiceListResponse, error)
	// MyInvoices 用户端我的发票申请。
	MyInvoices(ctx context.Context, userID uint64, q dto.InvoiceListQuery) (*dto.InvoiceListResponse, error)
	// InvoiceFile 取本人发票文件地址（doc36 §3.3 预埋：下载/邮件下发共用取件口）。
	// 未开票或开票时未回填文件地址返回 ErrInvoiceFileNotReady。
	InvoiceFile(ctx context.Context, userID, requestID uint64) (*dto.InvoiceFileInfo, error)
	// EmailInvoice 发票邮件下发（doc36 §3.3 预埋）：本轮校验归属与状态后返回未接入。
	EmailInvoice(ctx context.Context, userID, requestID uint64) error
	// SetPointEarner 注入账单结清后的积分发放钩子（装配层调用，doc36）。
	SetPointEarner(hook BillPointHook)
	// SetChannelRefundReader 注入原路退回扣点读取器（装配层调用，doc36 §3.2）。
	SetChannelRefundReader(reader ChannelRefundReader)
}

// BillPointHook 账单结清后的积分发放钩子（doc36）。只传基础类型，避免反向依赖积分模块。
type BillPointHook func(ctx context.Context, userID uint64, billNo string, amount float64)

// ChannelRefundReader 读取账期内原路退回（channel 模式）的本金与渠道扣点。
// 财务模块不 import 订单模块，由装配层注入订单退款仓储的实现（userID=0 为全平台）。
type ChannelRefundReader func(ctx context.Context, userID uint64, start, end *time.Time) (principal, fee float64, err error)

type billService struct {
	billRepo repository.BillRepository
	txRepo   transrepo.TransactionRepository
	// pointHook 可选：账单结清后发放积分（规则默认停用，启用后生效）。
	pointHook BillPointHook
	// channelRefundReader 可选：原路退回扣点来源；未注入时按余额退回口径处理（不扣点）。
	channelRefundReader ChannelRefundReader
}

// NewBillService 创建账单服务。
func NewBillService(billRepo repository.BillRepository, txRepo transrepo.TransactionRepository) BillService {
	return &billService{billRepo: billRepo, txRepo: txRepo}
}

// SetPointEarner 注入账单结清后的积分发放钩子（装配层调用）。
func (s *billService) SetPointEarner(hook BillPointHook) {
	s.pointHook = hook
}

// SetChannelRefundReader 注入原路退回扣点读取器（装配层调用）。
func (s *billService) SetChannelRefundReader(reader ChannelRefundReader) {
	s.channelRefundReader = reader
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

// GenerateForUser 归集某用户在指定账期的消费/退款与分类拆分，生成（upsert）账单。
//
// 口径（doc36 §3.2/§3.4）：
//
//	consume_amount  = 普通订单消费（正）
//	renewal_amount  = 续费订单消费（正）
//	refund_amount   = 余额退回（正，消费口径不变：钱仍在平台内）
//	channel_refund_amount / refund_fee_amount = 原路退回本金 / 渠道扣点（真金流出平台）
//	total_amount    = consume + renewal - channel_refund（账单金额：本金口径）
//	net_amount      = total_amount - refund_fee_amount（收入统计基数：再扣渠道扣点）
func (s *billService) GenerateForUser(ctx context.Context, userID uint64, period string) (*billmodel.Bill, error) {
	start, end, err := periodRange(period)
	if err != nil {
		return nil, err
	}
	// 消费为支出（-），退款为收入（+）。
	consumeSum, err := s.txRepo.SumByType(ctx, userID, []string{transmodel.TxTypeConsume}, start, end)
	if err != nil {
		return nil, err
	}
	refundSum, err := s.txRepo.SumByType(ctx, userID, []string{transmodel.TxTypeRefund}, start, end)
	if err != nil {
		return nil, err
	}
	// 分类拆分：普通消费 vs 续费消费；原路退款本金与扣点需按退款单拆分。
	purchase, renewalConsume, err := s.txRepo.SumConsumeSplit(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}
	consume := money.Round2(-consumeSum) // 消费为正数
	refund := money.Round2(refundSum)    // 余额退回为正数（原路退回不产生钱包流水）

	// 原路退回部分：从订单退款单拆出本金与扣点（doc36 §3.2）。
	channelRefund, refundFee := s.splitChannelRefund(ctx, userID, start, end)
	// 口径（用户确认）：余额退回不减少消费口径（资金仍在平台内）；
	// 原路退回本金从账单金额中冲减，渠道扣点不从账单金额扣（账单仍是应结金额），
	// 而是在收入统计基数 net_amount 上再扣一次 —— 见 buildBillInfo。
	total := money.Round2(consume - channelRefund)
	if total < 0 {
		total = 0
	}

	detail, _ := json.Marshal(map[string]any{
		"consume": consume, "refund": refund,
		"purchase": purchase, "renewal": renewalConsume,
		"channel_refund": channelRefund, "refund_fee": refundFee,
	})
	b := &billmodel.Bill{
		BillNo:              genBillNo(),
		UserID:              userID,
		Period:              period,
		TotalAmount:         total,
		RefundAmount:        refund,
		Status:              billmodel.BillStatusUnpaid,
		BillType:            classifyBill(purchase, renewalConsume),
		ConsumeAmount:       money.Round2(purchase),
		RenewalAmount:       money.Round2(renewalConsume),
		ChannelRefundAmount: channelRefund,
		RefundFeeAmount:     refundFee,
		Detail:              string(detail),
	}
	if err := s.billRepo.Upsert(ctx, b); err != nil {
		return nil, err
	}
	return s.billRepo.FindByUserPeriod(ctx, userID, period)
}

// splitChannelRefund 读取账期内原路退回（refund_mode='channel'）的本金与渠道扣点。
//
// 来源为订单退款单（装配层注入 reader，doc36 P-03）。未注入读取器时回落 (0,0)：
// 即全部退款按余额退回口径处理，不改变既有账单结果。
func (s *billService) splitChannelRefund(ctx context.Context, userID uint64, start, end *time.Time) (principal, fee float64) {
	if s.channelRefundReader == nil {
		return 0, 0
	}
	principal, fee, err := s.channelRefundReader(ctx, userID, start, end)
	if err != nil {
		// 读取失败不阻断账单生成：按无原路退回口径处理并留待对账核验。
		return 0, 0
	}
	return money.Round2(principal), money.Round2(fee)
}

// classifyBill 依据消费构成判定账单分类。
func classifyBill(purchase, renewal float64) string {
	switch {
	case renewal > 0 && purchase > 0:
		return billmodel.BillTypeMixed
	case renewal > 0:
		return billmodel.BillTypeRenewal
	default:
		return billmodel.BillTypeConsumption
	}
}

// timeNow 便于测试替换（当前仅用于开票时间落库）。
var timeNow = time.Now

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
	if err := s.billRepo.MarkPaid(ctx, id, paidAmountFen, paidMethod, paidChannelID); err != nil {
		return err
	}
	// 账单结清后的积分发放（doc36）：规则默认停用，失败只记日志不影响结清结果。
	if s.pointHook != nil {
		s.pointHook(ctx, b.UserID, b.BillNo, float64(paidAmountFen)/100)
	}
	return nil
}

// —— 发票 ——

// RequestInvoice 用户申请开票：账单须属本人且已结清，同一账单不允许重复待处理申请。
func (s *billService) RequestInvoice(ctx context.Context, userID uint64, req dto.InvoiceApplyRequest) (*dto.InvoiceInfo, error) {
	b, err := s.billRepo.FindByID(ctx, req.BillID)
	if err != nil {
		return nil, mapBillErr(err)
	}
	if b.UserID != userID {
		return nil, ErrBillNotFound // 越权按不存在处理，不泄漏他人账单存在性
	}
	if b.Status != billmodel.BillStatusPaid {
		return nil, ErrBillNotInvoicable
	}
	if b.InvoiceStatus == billmodel.InvoiceStatusIssued {
		return nil, ErrAlreadyInvoiced
	}
	if _, err := s.billRepo.FindPendingInvoiceByBill(ctx, b.ID); err == nil {
		return nil, ErrInvoicePending
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	invoiceType := req.InvoiceType
	if invoiceType == "" {
		invoiceType = billmodel.InvoiceTypeNormal
	}
	// 开票金额取账单应结金额：原路退回本金已在账单金额中冲减，扣点属平台成本不计入票面。
	amount := money.Round2(b.TotalAmount)
	record := &billmodel.InvoiceRequest{
		RequestNo:   genInvoiceNo(),
		BillID:      b.ID,
		BillNo:      b.BillNo,
		UserID:      userID,
		InvoiceType: invoiceType,
		Title:       req.Title,
		TaxNo:       req.TaxNo,
		Amount:      amount,
		Email:       req.Email,
		Status:      billmodel.InvoiceReqPending,
		Channel:     billmodel.InvoiceChannelManual,
	}
	if err := s.billRepo.CreateInvoice(ctx, record); err != nil {
		return nil, err
	}
	if err := s.billRepo.MarkInvoice(ctx, b.ID, billmodel.InvoiceStatusApplied, ""); err != nil {
		return nil, err
	}
	return s.findInvoice(ctx, record.ID)
}

// IssueInvoice 管理端开票：回填发票号并把账单置为已开票。
func (s *billService) IssueInvoice(ctx context.Context, requestID uint64, req dto.InvoiceIssueRequest, operatorID uint64) (*dto.InvoiceInfo, error) {
	row, err := s.billRepo.FindInvoiceByID(ctx, requestID)
	if err != nil {
		return nil, mapInvoiceErr(err)
	}
	if row.Status == billmodel.InvoiceReqIssued {
		return nil, ErrInvoiceStatusConflict // 幂等由前端提示，重复开票直接拒绝以免覆盖发票号
	}
	if row.Status == billmodel.InvoiceReqRejected {
		return nil, ErrInvoiceStatusConflict
	}
	now := timeNow()
	channel := req.Channel
	if channel == "" {
		channel = billmodel.InvoiceChannelManual
	}
	if err := s.billRepo.UpdateInvoice(ctx, requestID, map[string]any{
		"status":      billmodel.InvoiceReqIssued,
		"external_no": req.InvoiceNo,
		"file_url":    req.FileURL,
		"channel":     channel,
		"operator_id": operatorID,
		"issued_at":   &now,
	}); err != nil {
		return nil, err
	}
	if err := s.billRepo.MarkInvoice(ctx, row.BillID, billmodel.InvoiceStatusIssued, req.InvoiceNo); err != nil {
		return nil, err
	}
	return s.findInvoice(ctx, requestID)
}

// RejectInvoice 管理端驳回开票申请：账单回到未开票，用户可重新申请。
func (s *billService) RejectInvoice(ctx context.Context, requestID uint64, reason string, operatorID uint64) (*dto.InvoiceInfo, error) {
	row, err := s.billRepo.FindInvoiceByID(ctx, requestID)
	if err != nil {
		return nil, mapInvoiceErr(err)
	}
	if row.Status != billmodel.InvoiceReqPending {
		return nil, ErrInvoiceStatusConflict
	}
	if err := s.billRepo.UpdateInvoice(ctx, requestID, map[string]any{
		"status":        billmodel.InvoiceReqRejected,
		"reject_reason": reason,
		"operator_id":   operatorID,
	}); err != nil {
		return nil, err
	}
	if err := s.billRepo.MarkInvoice(ctx, row.BillID, billmodel.InvoiceStatusNone, ""); err != nil {
		return nil, err
	}
	return s.findInvoice(ctx, requestID)
}

func (s *billService) Invoices(ctx context.Context, q dto.InvoiceListQuery) (*dto.InvoiceListResponse, error) {
	rows, total, err := s.billRepo.ListInvoices(ctx, q)
	if err != nil {
		return nil, err
	}
	items := make([]dto.InvoiceInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, buildInvoiceInfo(row))
	}
	return &dto.InvoiceListResponse{
		Items: items,
		Meta:  dto.ListMeta{Page: normalizePage(q.Page), PageSize: normalizePageSize(q.PageSize), Total: total},
	}, nil
}

func (s *billService) MyInvoices(ctx context.Context, userID uint64, q dto.InvoiceListQuery) (*dto.InvoiceListResponse, error) {
	q.UserID = userID
	return s.Invoices(ctx, q)
}

// InvoiceFile 取本人发票文件（doc36 §3.3 预埋）。
//
// 越权按「不存在」处理，不泄漏他人申请单存在性；仅已开票且回填了 file_url 才能取件。
func (s *billService) InvoiceFile(ctx context.Context, userID, requestID uint64) (*dto.InvoiceFileInfo, error) {
	row, err := s.billRepo.FindInvoiceByID(ctx, requestID)
	if err != nil {
		return nil, mapInvoiceErr(err)
	}
	if row.UserID != userID {
		return nil, ErrInvoiceNotFound
	}
	if row.Status != billmodel.InvoiceReqIssued || row.FileURL == "" {
		return nil, ErrInvoiceFileNotReady
	}
	return &dto.InvoiceFileInfo{
		RequestNo: row.RequestNo,
		BillNo:    row.BillNo,
		FileURL:   row.FileURL,
		Delivered: true,
	}, nil
}

// EmailInvoice 发票邮件下发（doc36 §3.3 预埋）。
//
// 本轮只做归属与状态校验，真实下发待接税务/邮件网关：校验通过后返回未接入错误，
// 保证接口形状与错误语义已就绪，接入时只需替换尾部实现。
func (s *billService) EmailInvoice(ctx context.Context, userID, requestID uint64) error {
	row, err := s.billRepo.FindInvoiceByID(ctx, requestID)
	if err != nil {
		return mapInvoiceErr(err)
	}
	if row.UserID != userID {
		return ErrInvoiceNotFound
	}
	if row.Status != billmodel.InvoiceReqIssued {
		return ErrInvoiceFileNotReady
	}
	return ErrInvoiceEmailNotReady
}

func (s *billService) findInvoice(ctx context.Context, id uint64) (*dto.InvoiceInfo, error) {
	row, err := s.billRepo.FindInvoiceByID(ctx, id)
	if err != nil {
		return nil, mapInvoiceErr(err)
	}
	info := buildInvoiceInfo(*row)
	return &info, nil
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

func mapInvoiceErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrInvoiceNotFound
	}
	return err
}
