// Package service 提供用户中心支付收银台编排：把「用户点支付」翻译成
// 「建业务单 → 发起支付 → 回绑关联」的确定链路。
//
// 依赖方向（doc35 §3）：本模块只通过装配层注入的最小接口访问财务与支付能力，
// 不 import admin 的 service 层，也不让 payment 反向依赖 finance。
package service

import (
	"context"
	"errors"
	"math"
	"strings"

	billdto "hostsent/backend/internal/modules/admin/finance/bill/dto"
	rechdto "hostsent/backend/internal/modules/admin/finance/recharge/dto"
	withdrawdto "hostsent/backend/internal/modules/admin/finance/withdraw/dto"
	paydto "hostsent/backend/internal/modules/admin/payment/dto"
	paymodel "hostsent/backend/internal/modules/admin/payment/model"
)

// ErrBillNotPayable 账单不可支付（不属于本人 / 已结清 / 已关账）。
var ErrBillNotPayable = errors.New("账单不可支付")

// 装配层注入的最小能力接口。
type (
	// rechargePort 充值单登记与支付单回绑。
	rechargePort interface {
		Create(ctx context.Context, req rechdto.RechargeCreateRequest, operatorID uint64) (*rechdto.RechargeInfo, error)
		BindPaymentOrder(ctx context.Context, rechargeNo string, paymentOrderID uint64, channelCode string) error
	}
	// prepayPort 支付单发起。
	prepayPort interface {
		Prepay(ctx context.Context, userID uint64, req paydto.PrepayRequest) (*paydto.OrderInfo, error)
	}
	// payoutAccountPort 取用户提现收款账户（解密后）。
	payoutAccountPort interface {
		AccountForPayout(ctx context.Context, userID, id uint64) (*paymodel.UserPayoutAccount, error)
	}
	// withdrawPort 提现申请（冻结资金）与记录查询。
	withdrawPort interface {
		Apply(ctx context.Context, userID uint64, req withdrawdto.WithdrawApplyRequest) (*withdrawdto.WithdrawInfo, error)
		ListByUser(ctx context.Context, userID uint64, q withdrawdto.WithdrawListQuery) (*withdrawdto.WithdrawListResponse, error)
	}
	// billPort 账单读取（校验归属与应结金额，防止代付他人账单或篡改金额）。
	billPort interface {
		List(ctx context.Context, query billdto.BillListQuery) (*billdto.BillListResponse, error)
	}
)

// CashierService 收银台编排能力。
type CashierService interface {
	// StartRecharge 在线充值：创建充值单（pending）→ 发起支付 → 回绑支付单。
	// 支付失败时充值单保留 pending，用户可重新发起（避免"有单无支付"错账）。
	StartRecharge(ctx context.Context, userID uint64, amount float64, channelCode, scene string) (*paydto.OrderInfo, error)
	// StartBillPay 账单支付：为指定账单发起渠道支付（金额以账单应结为准）。
	StartBillPay(ctx context.Context, userID uint64, billID uint64, billNo string, amount float64, channelCode, scene string) (*paydto.OrderInfo, error)
	// StartWithdraw 提现申请：按收款账户 ID 解析目标账户 → 提交提现（冻结余额）。
	StartWithdraw(ctx context.Context, userID uint64, req WithdrawInput) (*withdrawdto.WithdrawInfo, error)
	// ListWithdrawals 我的提现记录（含所用收款账户与打款方式）。
	ListWithdrawals(ctx context.Context, userID uint64, page, pageSize int) (*withdrawdto.WithdrawListResponse, error)
}

// WithdrawInput 用户提现入参（对外只暴露收款账户 ID，账号信息由服务端解析）。
type WithdrawInput struct {
	Amount        float64
	AccountID     uint64
	PayoutChannel string // api/manual，空按平台默认
	Remark        string
}

type cashierService struct {
	recharge  rechargePort
	prepay    prepayPort
	accounts  payoutAccountPort
	withdraws withdrawPort
	bills     billPort
}

// NewCashierService 创建收银台编排服务。
func NewCashierService(recharge rechargePort, prepay prepayPort, accounts payoutAccountPort, withdraws withdrawPort, bills billPort) CashierService {
	return &cashierService{recharge: recharge, prepay: prepay, accounts: accounts, withdraws: withdraws, bills: bills}
}

func (s *cashierService) StartRecharge(ctx context.Context, userID uint64, amount float64, channelCode, scene string) (*paydto.OrderInfo, error) {
	rc, err := s.recharge.Create(ctx, rechdto.RechargeCreateRequest{
		UserID: userID,
		Amount: amount,
		Method: rechargeMethod(channelCode),
		Remark: "在线充值",
	}, userID)
	if err != nil {
		return nil, err
	}
	info, err := s.prepay.Prepay(ctx, userID, paydto.PrepayRequest{
		BizType:     paymodel.BizTypeRecharge,
		BizID:       rc.ID,
		BizNo:       rc.RechargeNo,
		Amount:      amount,
		Subject:     "账户余额充值",
		Scene:       scene,
		ChannelCode: channelCode,
	})
	if err != nil {
		return nil, err
	}
	// 回绑支付单：到账由支付单 paid 事件经 ApproveByNo 幂等触发。
	if err := s.recharge.BindPaymentOrder(ctx, rc.RechargeNo, info.ID, info.ChannelCode); err != nil {
		return nil, err
	}
	return info, nil
}

// StartBillPay 账单支付：先校验账单归属与应结金额（防止代付他人账单或篡改金额），
// 再按账单应结发起渠道支付；已结清/已关账账单直接拒绝。
func (s *cashierService) StartBillPay(ctx context.Context, userID uint64, billID uint64, billNo string, amount float64, channelCode, scene string) (*paydto.OrderInfo, error) {
	bill, err := s.findOwnedBill(ctx, userID, billID)
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		amount = bill.TotalAmount - bill.PaidAmount
	}
	// 金额以账单应结为准：请求金额与服务端计算不一致时拒绝，避免少付/多付。
	due := round2(bill.TotalAmount - bill.PaidAmount)
	if due <= 0 {
		return nil, ErrBillNotPayable
	}
	if amount <= 0 || math.Abs(round2(amount)-due) > 0.001 {
		return nil, ErrBillNotPayable
	}
	if billNo == "" {
		billNo = bill.BillNo
	}
	return s.prepay.Prepay(ctx, userID, paydto.PrepayRequest{
		BizType:     paymodel.BizTypeBill,
		BizID:       bill.ID,
		BizNo:       billNo,
		Amount:      due,
		Subject:     "账单支付 " + bill.Period,
		Scene:       scene,
		ChannelCode: channelCode,
	})
}

// findOwnedBill 在用户自己的账单中定位目标账单（越权与不存在一律按不可支付处理）。
func (s *cashierService) findOwnedBill(ctx context.Context, userID, billID uint64) (*billdto.BillInfo, error) {
	if s.bills == nil {
		return nil, ErrBillNotPayable
	}
	resp, err := s.bills.List(ctx, billdto.BillListQuery{UserID: userID, Page: 1, PageSize: 100})
	if err != nil {
		return nil, err
	}
	for i := range resp.Items {
		if resp.Items[i].ID != billID {
			continue
		}
		if resp.Items[i].Status != "unpaid" {
			return nil, ErrBillNotPayable
		}
		return &resp.Items[i], nil
	}
	return nil, ErrBillNotPayable
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func (s *cashierService) StartWithdraw(ctx context.Context, userID uint64, req WithdrawInput) (*withdrawdto.WithdrawInfo, error) {
	account, err := s.accounts.AccountForPayout(ctx, userID, req.AccountID)
	if err != nil {
		return nil, err
	}
	return s.withdraws.Apply(ctx, userID, withdrawdto.WithdrawApplyRequest{
		Amount:      req.Amount,
		Channel:     account.Channel,
		AccountNo:   account.AccountNo,
		AccountName: account.AccountName,
		BankName:    account.BankName,
		PayoutMode:  req.PayoutChannel,
		Remark:      req.Remark,
	})
}

// ListWithdrawals 我的提现记录（只透出本人数据，账号信息由服务层脱敏后返回）。
func (s *cashierService) ListWithdrawals(ctx context.Context, userID uint64, page, pageSize int) (*withdrawdto.WithdrawListResponse, error) {
	return s.withdraws.ListByUser(ctx, userID, withdrawdto.WithdrawListQuery{Page: page, PageSize: pageSize})
}

// rechargeMethod 把渠道编码归一为充值单 method（用于列表筛选）：
// 渠道编码形如 alipay_main/wechatpay_prod，取类型前缀；未知一律 manual。
func rechargeMethod(channelCode string) string {
	code := strings.ToLower(strings.TrimSpace(channelCode))
	switch {
	case code == "":
		return "manual"
	case strings.HasPrefix(code, "alipay"):
		return "alipay"
	case strings.HasPrefix(code, "wechat") || strings.HasPrefix(code, "wx"):
		return "wechat"
	case strings.HasPrefix(code, "manual"):
		return "manual"
	default:
		return "manual"
	}
}
