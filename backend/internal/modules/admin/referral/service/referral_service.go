package service

import (
	"context"
	crand "crypto/rand"
	"errors"
	"fmt"
	"math/big"
	mrand "math/rand"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/referral/dto"
	"hostsent/backend/internal/modules/admin/referral/model"
	"hostsent/backend/internal/modules/admin/referral/repository"
	"hostsent/backend/internal/pkg/money"
)

// AccrualInput 订单完成后的返现计提入参。
type AccrualInput struct {
	OrderID     uint64
	OrderNo     string
	BuyerUserID uint64
	BaseAmount  float64 // 计提基数 = 订单实付金额
	IsRenewal   bool
}

// ClawbackInput 退款审核通过后的返现冲减入参。
type ClawbackInput struct {
	OrderID      uint64
	OrderNo      string
	BuyerUserID  uint64
	PaidAmount   float64 // 订单实付金额（冲减比例分母）
	RefundAmount float64
	RefundNo     string
}

// CashbackService 返现计提与冲减。
type CashbackService interface {
	// AccrueForOrder 订单完成（服务中）时按被邀请人首单/后续/续费三档比率计提返现给邀请人。
	AccrueForOrder(ctx context.Context, in AccrualInput) error
	// ClawbackForRefund 退款审核通过时按退款额占实付比例冲减该订单已计提的返现。
	ClawbackForRefund(ctx context.Context, in ClawbackInput) error
}

// InviteService 邀请关系与返现查询（用户端）。
type InviteService interface {
	// ResolveInviter 按邀请码解析邀请人；无效码返回 0（不报错，注册不因邀请码失败）。
	ResolveInviter(ctx context.Context, code string) (uint64, error)
	// EnsureInviteCode 返回用户邀请码，缺失时生成并落库。
	EnsureInviteCode(ctx context.Context, userID uint64) (string, error)
	// BindInviter 绑定单级邀请关系（一次性，已有邀请人不覆盖）。
	BindInviter(ctx context.Context, inviteeID, inviterID uint64) error
	// Profile 返现概览。
	Profile(ctx context.Context, userID uint64) (*dto.Profile, error)
	// Invitees 我的邀请列表。
	Invitees(ctx context.Context, userID uint64, page, pageSize int) (*dto.InviteeListResponse, error)
	// Cashbacks 返现台账（userID>0 时只看该用户）。
	Cashbacks(ctx context.Context, userID uint64, typeFilter string, page, pageSize int) (*dto.CashbackListResponse, error)
	// Rates 当前生效的返现比率配置。
	Rates(ctx context.Context) (model.Rates, error)
}

type referralService struct {
	db     *gorm.DB
	repo   repository.ReferralRepository
	logger *zap.Logger
}

// NewReferralService 创建推广返现服务。
func NewReferralService(db *gorm.DB, repo repository.ReferralRepository, logger *zap.Logger) *referralService {
	return &referralService{db: db, repo: repo, logger: logger}
}

// —— 计提及冲减 ——

// AccrueForOrder 计提返现。
// 幂等键 (邀请人, cashback|renewal_cashback, 订单号)：同一订单重复开通只记一次账。
func (s *referralService) AccrueForOrder(ctx context.Context, in AccrualInput) error {
	if in.OrderNo == "" || in.BaseAmount <= 0 || in.BuyerUserID == 0 {
		return nil
	}
	rates, err := s.repo.Rates(ctx)
	if err != nil {
		return err
	}
	if !rates.Enabled {
		return nil
	}
	inviterID, err := s.repo.FindInviterID(ctx, in.BuyerUserID)
	if err != nil {
		return err
	}
	if inviterID == 0 || inviterID == in.BuyerUserID {
		return nil // 无邀请人或自邀（注册已拒绝，此处双保险）
	}

	bizType := model.TxTypeCashback
	rate := rates.Subsequent
	if in.IsRenewal {
		bizType = model.TxTypeRenewalCashback
		rate = rates.Renewal
	} else {
		existed, err := s.repo.ExistsIncomeForInvitee(ctx, in.BuyerUserID)
		if err != nil {
			return err
		}
		if !existed {
			rate = rates.FirstOrder // 被邀请人第一笔成功订单
		}
	}
	amount := money.Round2(in.BaseAmount * rate)
	if amount <= 0 {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		acc, err := s.repo.EnsureAccount(tx, inviterID)
		if err != nil {
			return err
		}
		if _, err := s.repo.FindTxByBiz(tx, inviterID, bizType, in.OrderNo); err == nil {
			return nil // 已计提
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		before := acc.Balance
		after := money.Round2(before + amount)
		acc.Balance = after
		acc.TotalIncome = money.Round2(acc.TotalIncome + amount)
		acc.Version++
		if err := s.repo.UpdateAccount(tx, acc); err != nil {
			return err
		}
		remark := fmt.Sprintf("被邀请人 %d 下单返现（%s）", in.BuyerUserID, rateLabel(in.IsRenewal))
		return s.repo.CreateTx(tx, &model.ReferralTransaction{
			TxNo:          genTxNo(),
			UserID:        inviterID,
			Type:          bizType,
			Direction:     model.DirectionIncome,
			Amount:        amount,
			BalanceBefore: before,
			BalanceAfter:  after,
			BizType:       bizType,
			RefNo:         in.OrderNo,
			OrderID:       in.OrderID,
			OrderNo:       in.OrderNo,
			InviterUserID: inviterID,
			InviteeUserID: in.BuyerUserID,
			Remark:        remark,
		})
	})
}

// ClawbackForRefund 按比例冲减返现。
// 冲减额 = 该订单已计提返现 × 退款额 / 实付额，且不超过「已计提 - 已冲减」；
// 允许余额为负（已提现/已转出后仍发生退款时形成欠款），幂等键 (邀请人, refund_clawback, 退款号)。
func (s *referralService) ClawbackForRefund(ctx context.Context, in ClawbackInput) error {
	if in.RefundNo == "" || in.RefundAmount <= 0 || in.PaidAmount <= 0 || in.BuyerUserID == 0 {
		return nil
	}
	inviterID, err := s.repo.FindInviterID(ctx, in.BuyerUserID)
	if err != nil {
		return err
	}
	if inviterID == 0 {
		return nil
	}
	accrued, err := s.repo.SumTxAmount(ctx, inviterID, in.OrderID, model.TxTypeCashback, model.TxTypeRenewalCashback)
	if err != nil {
		return err
	}
	if accrued <= 0 {
		return nil // 该订单未计提过返现
	}
	clawed, err := s.repo.SumTxAmount(ctx, inviterID, in.OrderID, model.TxTypeRefundClawback)
	if err != nil {
		return err
	}
	remaining := money.Round2(accrued - clawed)
	if remaining <= 0 {
		return nil
	}
	amount := money.Round2(accrued * in.RefundAmount / in.PaidAmount)
	if amount > remaining {
		amount = remaining
	}
	if amount <= 0 {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		acc, err := s.repo.EnsureAccount(tx, inviterID)
		if err != nil {
			return err
		}
		if _, err := s.repo.FindTxByBiz(tx, inviterID, model.TxTypeRefundClawback, in.RefundNo); err == nil {
			return nil // 该退款单已冲减
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		before := acc.Balance
		after := money.Round2(before - amount)
		acc.Balance = after
		acc.TotalOut = money.Round2(acc.TotalOut + amount)
		acc.Version++
		if err := s.repo.UpdateAccount(tx, acc); err != nil {
			return err
		}
		return s.repo.CreateTx(tx, &model.ReferralTransaction{
			TxNo:          genTxNo(),
			UserID:        inviterID,
			Type:          model.TxTypeRefundClawback,
			Direction:     model.DirectionExpense,
			Amount:        amount,
			BalanceBefore: before,
			BalanceAfter:  after,
			BizType:       model.TxTypeRefundClawback,
			RefNo:         in.RefundNo,
			OrderID:       in.OrderID,
			OrderNo:       in.OrderNo,
			InviterUserID: inviterID,
			InviteeUserID: in.BuyerUserID,
			Remark:        fmt.Sprintf("订单 %s 退款 %.2f 冲减返现", in.OrderNo, in.RefundAmount),
		})
	})
}

// —— 邀请关系 ——

func (s *referralService) ResolveInviter(ctx context.Context, code string) (uint64, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return 0, nil
	}
	id, err := s.repo.FindIDByInviteCode(ctx, code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return id, nil
}

func (s *referralService) EnsureInviteCode(ctx context.Context, userID uint64) (string, error) {
	if code, err := s.repo.InviteCodeOf(ctx, userID); err != nil {
		return "", err
	} else if code != "" {
		return code, nil
	}
	var lastErr error
	for i := 0; i < 5; i++ {
		candidate := genInviteCode()
		final, err := s.repo.EnsureInviteCode(ctx, userID, candidate)
		if err == nil {
			return final, nil
		}
		lastErr = err
		// 并发下可能已被别的写入落定，或候选码撞库；已存在则直接返回。
		if cur, cerr := s.repo.InviteCodeOf(ctx, userID); cerr == nil && cur != "" {
			return cur, nil
		}
	}
	return "", lastErr
}

func (s *referralService) BindInviter(ctx context.Context, inviteeID, inviterID uint64) error {
	if inviteeID == 0 || inviterID == 0 || inviteeID == inviterID {
		return nil // 自邀或参数缺失直接忽略
	}
	return s.repo.BindInviter(ctx, inviteeID, inviterID)
}

// —— 查询 ——

func (s *referralService) Profile(ctx context.Context, userID uint64) (*dto.Profile, error) {
	code, err := s.EnsureInviteCode(ctx, userID)
	if err != nil {
		return nil, err
	}
	rates, err := s.repo.Rates(ctx)
	if err != nil {
		return nil, err
	}
	profile := &dto.Profile{
		InviteCode:        code,
		Enabled:           rates.Enabled,
		MinWithdrawAmount: rates.MinWithdraw,
		FirstOrderRate:    rates.FirstOrder,
		SubsequentRate:    rates.Subsequent,
		RenewalRate:       rates.Renewal,
	}
	acc, err := s.repo.FindAccount(ctx, userID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if acc != nil {
		profile.Balance = acc.Balance
		profile.Frozen = acc.Frozen
		profile.TotalIncome = acc.TotalIncome
		profile.TotalOut = acc.TotalOut
	}
	if profile.InviteeCount, err = s.repo.CountInvitees(ctx, userID); err != nil {
		return nil, err
	}
	if profile.PendingWithdraw, err = s.repo.SumPendingWithdrawals(ctx, userID); err != nil {
		return nil, err
	}
	return profile, nil
}

func (s *referralService) Invitees(ctx context.Context, userID uint64, page, pageSize int) (*dto.InviteeListResponse, error) {
	rows, total, err := s.repo.ListInvitees(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]dto.InviteeInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.InviteeInfo{
			UserID:        row.UserID,
			Username:      row.Username,
			Email:         row.Email,
			InvitedAt:     row.InvitedAt,
			CashbackTotal: row.CashbackTotal,
			OrderCount:    row.OrderCount,
		})
	}
	p, ps := normalizePage(page, pageSize)
	return &dto.InviteeListResponse{Items: items, Meta: dto.ListMeta{Page: p, PageSize: ps, Total: total}}, nil
}

func (s *referralService) Cashbacks(ctx context.Context, userID uint64, typeFilter string, page, pageSize int) (*dto.CashbackListResponse, error) {
	rows, total, err := s.repo.ListTxRows(ctx, repository.TxFilter{UserID: userID, Type: typeFilter, Page: page, PageSize: pageSize})
	if err != nil {
		return nil, err
	}
	items := make([]dto.CashbackInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.CashbackInfo{
			ID:              row.ID,
			TxNo:            row.TxNo,
			Type:            row.Type,
			Direction:       row.Direction,
			Amount:          row.Amount,
			BalanceAfter:    row.BalanceAfter,
			OrderID:         row.OrderID,
			OrderNo:         row.OrderNo,
			InviteeUserID:   row.InviteeUserID,
			InviteeUsername: row.InviteeUsername,
			InviterUserID:   row.InviterUserID,
			InviterUsername: row.InviterUsername,
			RefNo:           row.RefNo,
			Remark:          row.Remark,
			CreatedAt:       row.CreatedAt.Format(time.RFC3339),
		})
	}
	p, ps := normalizePage(page, pageSize)
	return &dto.CashbackListResponse{Items: items, Meta: dto.ListMeta{Page: p, PageSize: ps, Total: total}}, nil
}

func (s *referralService) Rates(ctx context.Context) (model.Rates, error) {
	return s.repo.Rates(ctx)
}

// —— 内部工具 ——

const inviteCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // 去掉易混字符 I/O/0/1

// genTxNo 生成返现台账流水号，如 RF2026091012000012345。
func genTxNo() string {
	return fmt.Sprintf("RF%s%05d", time.Now().Format("20060102150405"), mrand.Intn(100000))
}

// genInviteCode 生成 8 位随机邀请码（crypto/rand，无歧义字符集）。
func genInviteCode() string {
	buf := make([]byte, 8)
	for i := range buf {
		n, err := crand.Int(crand.Reader, big.NewInt(int64(len(inviteCodeAlphabet))))
		if err != nil {
			// 随机源不可用时退化为时间戳派生，保证注册不被阻断。
			buf[i] = inviteCodeAlphabet[(time.Now().UnixNano()+int64(i))%int64(len(inviteCodeAlphabet))]
			continue
		}
		buf[i] = inviteCodeAlphabet[n.Int64()]
	}
	return string(buf)
}

func rateLabel(isRenewal bool) string {
	if isRenewal {
		return "续费"
	}
	return "首单/复购"
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
