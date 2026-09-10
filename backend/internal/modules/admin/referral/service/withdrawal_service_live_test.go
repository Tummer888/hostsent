//go:build live

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	accountrepo "hostsent/backend/internal/modules/admin/finance/account/repository"
	finaccountservice "hostsent/backend/internal/modules/admin/finance/account/service"
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
	transrepo "hostsent/backend/internal/modules/admin/finance/transaction/repository"
	referraldto "hostsent/backend/internal/modules/admin/referral/dto"
	"hostsent/backend/internal/modules/admin/referral/model"
	"hostsent/backend/internal/modules/admin/referral/repository"
)

// TestLiveReferralWithdrawal 针对真实库验证返现提现与转出：
// 最低提现校验、冻结/驳回回退、审核通过扣减、转入现金余额的资金一致性、转出失败补偿。
// 仅 -tags live 时运行：go test -tags live ./internal/modules/admin/referral/service/。
func TestLiveReferralWithdrawal(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	ctx := context.Background()
	repo := repository.NewReferralRepository(db)
	referralSvc := NewReferralService(db, repo, zap.NewNop())
	walletSvc := finaccountservice.NewWalletService(db, accountrepo.NewWalletRepository(db), transrepo.NewTransactionRepository(db))
	// 与装配层一致：返现转出桥接到现金钱包，biz_type=referral_transfer，ref_no=转账号（钱包侧幂等）。
	transfer := func(ctx context.Context, userID uint64, amount float64, bizType, refNo, remark string) error {
		_, err := walletSvc.Change(ctx, finaccountservice.ChangeRequest{
			UserID: userID, Type: transmodel.TxTypeReferralTransfer, Direction: transmodel.DirectionIncome,
			Amount: amount, BizType: bizType, RefNo: refNo, Remark: remark,
		})
		return err
	}
	withdrawSvc := NewWithdrawalService(db, repo, transfer, zap.NewNop())

	suffix := time.Now().UnixNano() % 1_000_000
	inviter, invitee := createTestUser(t, db, fmt.Sprintf("wd_inv_%d", suffix)), createTestUser(t, db, fmt.Sprintf("wd_iee_%d", suffix))
	defer cleanupWithdrawalTest(t, db, inviter, invitee)

	seedRates(t, db)
	if err := referralSvc.BindInviter(ctx, invitee, inviter); err != nil {
		t.Fatalf("绑定邀请关系失败: %v", err)
	}
	// 首单 1000 → 返现余额 100.00（首单比率 0.10，最低提现 50）。
	if err := referralSvc.AccrueForOrder(ctx, AccrualInput{OrderID: 9101, OrderNo: fmt.Sprintf("W%d-A", suffix), BuyerUserID: invitee, BaseAmount: 1000}); err != nil {
		t.Fatalf("计提返现失败: %v", err)
	}

	state := func() (balance, frozen, totalOut float64) {
		acc, err := repo.FindAccount(ctx, inviter)
		if err != nil {
			t.Fatalf("查询返现账户失败: %v", err)
		}
		return acc.Balance, acc.Frozen, acc.TotalOut
	}

	// 1) 低于最低提现金额 → 拒绝，账户不动
	if _, err := withdrawSvc.Apply(ctx, inviter, referralWithdrawReq(20)); !errors.Is(err, ErrBelowMinWithdraw) {
		t.Fatalf("低于最低提现期望 ErrBelowMinWithdraw，实际 %v", err)
	}

	// 2) 申请 60 → 可用 40，冻结 60，待审核
	wd, err := withdrawSvc.Apply(ctx, inviter, referralWithdrawReq(60))
	if err != nil {
		t.Fatalf("申请提现失败: %v", err)
	}
	if wd.Status != model.WithdrawStatusPending {
		t.Fatalf("提现单状态期望 pending，实际 %s", wd.Status)
	}
	if bal, frozen, _ := state(); bal != 40.00 || frozen != 60.00 {
		t.Fatalf("申请后期望 可用40/冻结60，实际 %.2f/%.2f", bal, frozen)
	}

	// 3) 余额已被冻结，再申请 50 → 余额不足
	if _, err := withdrawSvc.Apply(ctx, inviter, referralWithdrawReq(50)); !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("余额不足期望 ErrInsufficientBalance，实际 %v", err)
	}

	// 4) 驳回 → 冻结回退为可用
	if _, err := withdrawSvc.Reject(ctx, wd.ID, 1, "tester", "资料不全"); err != nil {
		t.Fatalf("驳回失败: %v", err)
	}
	if bal, frozen, _ := state(); bal != 100.00 || frozen != 0 {
		t.Fatalf("驳回后期望 可用100/冻结0，实际 %.2f/%.2f", bal, frozen)
	}

	// 5) 审核通过 → 冻结扣除并计入累计出账
	wd2, err := withdrawSvc.Apply(ctx, inviter, referralWithdrawReq(60))
	if err != nil {
		t.Fatalf("二次申请失败: %v", err)
	}
	if _, err := withdrawSvc.Approve(ctx, wd2.ID, 1, "tester", "已打款"); err != nil {
		t.Fatalf("审核通过失败: %v", err)
	}
	if bal, frozen, out := state(); bal != 40.00 || frozen != 0 || out != 60.00 {
		t.Fatalf("通过后期望 可用40/冻结0/累计出账60，实际 %.2f/%.2f/%.2f", bal, frozen, out)
	}

	// 6) 转入现金余额 15 → 返现余额 25，钱包余额 15，且落一条钱包流水
	if err := withdrawSvc.TransferToWallet(ctx, inviter, 15); err != nil {
		t.Fatalf("转入现金余额失败: %v", err)
	}
	if bal, _, out := state(); bal != 25.00 || out != 75.00 {
		t.Fatalf("转入后期望 可用25/累计出账75，实际 %.2f/%.2f", bal, out)
	}
	wallet, err := walletSvc.Balance(ctx, inviter)
	if err != nil {
		t.Fatalf("查询钱包失败: %v", err)
	}
	if wallet.Balance != 15.00 {
		t.Fatalf("钱包余额期望 15.00，实际 %.2f", wallet.Balance)
	}
	var txCount int64
	if err := db.Model(&transmodel.WalletTransaction{}).
		Where("user_id = ? AND biz_type = ?", inviter, transmodel.TxTypeReferralTransfer).Count(&txCount).Error; err != nil {
		t.Fatalf("统计钱包流水失败: %v", err)
	}
	if txCount != 1 {
		t.Fatalf("钱包流水期望 1 条，实际 %d", txCount)
	}

	// 7) 钱包入账失败 → 返现余额补偿回滚，账实一致
	failingSvc := NewWithdrawalService(db, repo, func(context.Context, uint64, float64, string, string, string) error {
		return errors.New("upstream wallet error")
	}, zap.NewNop())
	if err := failingSvc.TransferToWallet(ctx, inviter, 10); err == nil {
		t.Fatalf("钱包入账失败时期望返回错误")
	}
	if bal, _, out := state(); bal != 25.00 || out != 75.00 {
		t.Fatalf("补偿后期望 可用25/累计出账75，实际 %.2f/%.2f", bal, out)
	}

	// 8) 台账条数：首单1 + 提现冻结2 + 驳回解冻1 + 转出1 + 失败转出1 + 回滚1 = 7
	// （失败的转入会留下 transfer_out + transfer_return 成对记录，净额为 0，便于审计）
	var ledgerCount int64
	if err := db.Model(&model.ReferralTransaction{}).Where("user_id = ?", inviter).Count(&ledgerCount).Error; err != nil {
		t.Fatalf("统计返现台账失败: %v", err)
	}
	if ledgerCount != 7 {
		var rows []model.ReferralTransaction
		db.Where("user_id = ?", inviter).Order("id").Find(&rows)
		for _, r := range rows {
			t.Logf("台账 %d: type=%s biz=%s ref=%s amount=%.2f", r.ID, r.Type, r.BizType, r.RefNo, r.Amount)
		}
		t.Fatalf("返现台账期望 7 条，实际 %d", ledgerCount)
	}
}

func referralWithdrawReq(amount float64) referraldto.WithdrawRequest {
	return referraldto.WithdrawRequest{Amount: amount, Channel: "alipay", Account: "test@example.com"}
}

func cleanupWithdrawalTest(t *testing.T, db *gorm.DB, inviter, invitee uint64) {
	t.Helper()
	for _, uid := range []uint64{inviter, invitee} {
		db.Exec("DELETE FROM referral_transactions WHERE user_id = ?", uid)
		db.Exec("DELETE FROM referral_accounts WHERE user_id = ?", uid)
		db.Exec("DELETE FROM referral_withdrawals WHERE user_id = ?", uid)
		db.Exec("DELETE FROM wallet_transactions WHERE user_id = ?", uid)
		db.Exec("DELETE FROM wallet_accounts WHERE user_id = ?", uid)
	}
	db.Exec("DELETE FROM users WHERE id IN ?", []uint64{inviter, invitee})
}
