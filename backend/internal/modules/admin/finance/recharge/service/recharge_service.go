package service

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	accountservice "hostsent/backend/internal/modules/admin/finance/account/service"
	"hostsent/backend/internal/modules/admin/finance/recharge/dto"
	"hostsent/backend/internal/modules/admin/finance/recharge/model"
	"hostsent/backend/internal/modules/admin/finance/recharge/repository"
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
	"hostsent/backend/internal/pkg/money"
)

// RechargeService 充值到账能力。
type RechargeService interface {
	Create(ctx context.Context, req dto.RechargeCreateRequest, operatorID uint64) (*dto.RechargeInfo, error)
	// CreateAndApprove 登记并立即确认到账（管理端「用户列表 → 充值」用）。
	// 与 Create + Approve 两次调用等价，但只暴露一个入口：调用方拿不到中间态，
	// 也就不会出现「登记成功、确认失败」后只剩一张 pending 单而无人察觉。
	CreateAndApprove(ctx context.Context, req dto.RechargeCreateRequest, operatorID uint64) (*dto.RechargeInfo, error)
	Approve(ctx context.Context, id uint64, req dto.RechargeApproveRequest, operatorID uint64) (*dto.RechargeInfo, error)
	ApproveByNo(ctx context.Context, rechargeNo string, req dto.RechargeApproveRequest, operatorID uint64) (*dto.RechargeInfo, error)
	// BindPaymentOrder 绑定支付单（在线充值走支付中心时登记，不改变状态）。
	BindPaymentOrder(ctx context.Context, rechargeNo string, paymentOrderID uint64, channelCode string) error
	// FindByNo 按充值单号读取（支付成功事件回查用）。
	FindByNo(ctx context.Context, rechargeNo string) (*dto.RechargeInfo, error)
	List(ctx context.Context, q dto.RechargeListQuery) (*dto.RechargeListResponse, error)
	// SetBillGenerator 注入账单归集能力（装配层调用，见 BillGenerator）。
	SetBillGenerator(gen BillGenerator)
	// SetLogger 注入日志器（可选）；未注入时账单归集失败只静默忽略。
	SetLogger(logger *zap.Logger)
}

// BillGenerator 充值到账后开单笔充值账单（由装配层注入账单服务实现）。
//
// 为什么抽成端口而不是直接 import 账单服务：充值只该知道「这笔钱到账了，
// 去开一张属于这笔充值的账单」，不该知道账单怎么算。装配层知道两边，
// 模块之间不必知道 —— 与 bill.ChannelRefundReader 同一形态。
type BillGenerator func(ctx context.Context, in RechargeBill) error

// RechargeBill 开一张充值账单所需的全部事实。只传基础类型：
// 账单模块不必知道充值单的结构，也就不会反向依赖充值模块。
type RechargeBill struct {
	UserID     uint64
	RechargeNo string
	Amount     float64
	Method     string
	PaidAt     time.Time
}

type rechargeService struct {
	rechargeRepo repository.RechargeRepository
	wallet       accountservice.WalletService
	billGen      BillGenerator
	logger       *zap.Logger
}

// NewRechargeService 创建充值服务。
func NewRechargeService(rechargeRepo repository.RechargeRepository, wallet accountservice.WalletService) RechargeService {
	return &rechargeService{rechargeRepo: rechargeRepo, wallet: wallet}
}

// SetBillGenerator 注入账单归集能力（装配层调用）。
func (s *rechargeService) SetBillGenerator(gen BillGenerator) {
	s.billGen = gen
}

// SetLogger 注入日志器（可选，装配层调用）。
func (s *rechargeService) SetLogger(logger *zap.Logger) {
	s.logger = logger
}

// Create 线下充值登记（状态 pending，需人工确认到账）。
func (s *rechargeService) Create(ctx context.Context, req dto.RechargeCreateRequest, operatorID uint64) (*dto.RechargeInfo, error) {
	if req.Amount <= 0 {
		return nil, accountservice.ErrInsufficientBalance
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

// CreateAndApprove 登记并立即确认到账：管理端人工代充值的入口。
//
// 方法固定 manual：管理端当场收到的就是线下款（现金/转账），method 记成
// alipay/wechat 会让「充值管理」按渠道统计时把线下款算进线上渠道。
func (s *rechargeService) CreateAndApprove(ctx context.Context, req dto.RechargeCreateRequest, operatorID uint64) (*dto.RechargeInfo, error) {
	if req.Method == "" {
		req.Method = model.RechargeMethodManual
	}
	created, err := s.Create(ctx, req, operatorID)
	if err != nil {
		return nil, err
	}
	return s.Approve(ctx, created.ID, dto.RechargeApproveRequest{
		Remark: req.Remark,
	}, operatorID)
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
//
// 入账成功后开一张本笔充值的独立账单：充值额此前只躺在资金流水里，账单侧完全看不到，
// 运营在「账单」页对不上「充值管理」页的数。开单放在入账之后、且失败只记日志
// 不回滚 —— 钱已经进了用户余额，把到账改回失败会造成账实不符；
// 账单是可重算的派生数据，漏了补一次即可（管理端有「生成账单」按钮）。
func (s *rechargeService) doApprove(ctx context.Context, rc *model.Recharge, req dto.RechargeApproveRequest, operatorID uint64) error {
	if rc.Status == model.RechargeStatusSuccess {
		return nil // 幂等：已到账
	}
	if rc.Status != model.RechargeStatusPending {
		return accountservice.ErrStatusConflict
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
	if _, err := s.wallet.Change(ctx, accountservice.ChangeRequest{
		UserID:     rc.UserID,
		Type:       transmodel.TxTypeRecharge,
		Direction:  transmodel.DirectionIncome,
		Amount:     rc.Amount,
		RefNo:      rc.RechargeNo,
		BizType:    "recharge",
		Remark:     rc.Remark,
		OperatorID: operatorID,
	}); err != nil {
		return err
	}
	s.generateBill(ctx, rc)
	return nil
}

// generateBill 为本笔充值开一张独立账单。
//
// 账期按 PaidAt 取（到账时间），而不是 CreatedAt：跨月补确认的充值单
// （上月登记、本月到账）若按登记时间归档，账单会落在已经关账的账期里。
//
// 一笔充值一张账单，幂等键是充值单号（账单模块侧），所以这里不需要先查是否已开过：
// 渠道重复回调到 ApproveByNo 时，doApprove 在状态判断处就已幂等返回。
func (s *rechargeService) generateBill(ctx context.Context, rc *model.Recharge) {
	if s.billGen == nil || rc.PaidAt == nil {
		return
	}
	err := s.billGen(ctx, RechargeBill{
		UserID:     rc.UserID,
		RechargeNo: rc.RechargeNo,
		Amount:     rc.Amount,
		Method:     rc.Method,
		PaidAt:     *rc.PaidAt,
	})
	if err != nil && s.logger != nil {
		s.logger.Warn("recharge: generate bill after approve failed",
			zap.Uint64("user_id", rc.UserID),
			zap.String("recharge_no", rc.RechargeNo),
			zap.String("period", rc.PaidAt.Format("200601")),
			zap.Error(err))
	}
}

// BindPaymentOrder 绑定支付单：在线充值下单后登记支付单 ID 与渠道编码，状态仍为 pending，
// 到账由支付中心 paid 事件经 ApproveByNo 幂等触发。
func (s *rechargeService) BindPaymentOrder(ctx context.Context, rechargeNo string, paymentOrderID uint64, channelCode string) error {
	rc, err := s.rechargeRepo.FindByNo(ctx, rechargeNo)
	if err != nil {
		return mapRechargeErr(err)
	}
	rc.PaymentOrderID = paymentOrderID
	rc.ChannelCode = channelCode
	return s.rechargeRepo.Update(ctx, rc)
}

// FindByNo 按充值单号读取。
func (s *rechargeService) FindByNo(ctx context.Context, rechargeNo string) (*dto.RechargeInfo, error) {
	rc, err := s.rechargeRepo.FindByNo(ctx, rechargeNo)
	if err != nil {
		return nil, mapRechargeErr(err)
	}
	info := buildRechargeInfo(*rc)
	return &info, nil
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
