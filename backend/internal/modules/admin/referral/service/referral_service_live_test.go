//go:build live

package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/referral/model"
	"hostsent/backend/internal/modules/admin/referral/repository"
)

// TestLiveReferralCashback 针对真实库验证返现核心账务：
// 首单/后续/续费三档比率、订单幂等、按比例退款冲减、冲减幂等。
// 仅 -tags live 时运行：go test -tags live ./internal/modules/admin/referral/service/。
func TestLiveReferralCashback(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	ctx := context.Background()
	svc := NewReferralService(db, repository.NewReferralRepository(db), zap.NewNop())

	suffix := time.Now().UnixNano() % 1_000_000
	inviter, invitee := createTestUser(t, db, fmt.Sprintf("ref_inv_%d", suffix)), createTestUser(t, db, fmt.Sprintf("ref_iee_%d", suffix))
	defer cleanupReferralTest(t, db, inviter, invitee)

	// 固定三档比率，避免依赖 system_configs 现有值。
	seedRates(t, db)
	if err := svc.BindInviter(ctx, invitee, inviter); err != nil {
		t.Fatalf("绑定邀请关系失败: %v", err)
	}

	accOf := func() float64 {
		acc, err := svc.repo.FindAccount(ctx, inviter)
		if err != nil {
			return 0
		}
		return acc.Balance
	}

	// 1) 首单 100 → 10.00（首单比率 0.10）
	if err := svc.AccrueForOrder(ctx, AccrualInput{OrderID: 9001, OrderNo: fmt.Sprintf("T%d-A", suffix), BuyerUserID: invitee, BaseAmount: 100}); err != nil {
		t.Fatalf("首单计提失败: %v", err)
	}
	if got := accOf(); got != 10.00 {
		t.Fatalf("首单返现期望 10.00，实际 %.2f", got)
	}

	// 2) 同一订单重复开通 → 幂等，不变
	if err := svc.AccrueForOrder(ctx, AccrualInput{OrderID: 9001, OrderNo: fmt.Sprintf("T%d-A", suffix), BuyerUserID: invitee, BaseAmount: 100}); err != nil {
		t.Fatalf("重复计提报错: %v", err)
	}
	if got := accOf(); got != 10.00 {
		t.Fatalf("幂等期望仍为 10.00，实际 %.2f", got)
	}

	// 3) 后续下单 200 → +10.00（后续比率 0.05）
	if err := svc.AccrueForOrder(ctx, AccrualInput{OrderID: 9002, OrderNo: fmt.Sprintf("T%d-B", suffix), BuyerUserID: invitee, BaseAmount: 200}); err != nil {
		t.Fatalf("后续单计提失败: %v", err)
	}
	if got := accOf(); got != 20.00 {
		t.Fatalf("后续返现期望 20.00（10+10），实际 %.2f", got)
	}

	// 4) 续费 200 → +6.00（续费比率 0.03）
	if err := svc.AccrueForOrder(ctx, AccrualInput{OrderID: 9003, OrderNo: fmt.Sprintf("T%d-C", suffix), BuyerUserID: invitee, BaseAmount: 200, IsRenewal: true}); err != nil {
		t.Fatalf("续费计提失败: %v", err)
	}
	if got := accOf(); got != 26.00 {
		t.Fatalf("续费返现期望 26.00（20+6），实际 %.2f", got)
	}

	// 5) 订单 B 部分退款 100/200 → 冲减 10 × 100/200 = 5.00
	if err := svc.ClawbackForRefund(ctx, ClawbackInput{
		OrderID: 9002, OrderNo: fmt.Sprintf("T%d-B", suffix), BuyerUserID: invitee,
		PaidAmount: 200, RefundAmount: 100, RefundNo: fmt.Sprintf("R%d-B1", suffix),
	}); err != nil {
		t.Fatalf("退款冲减失败: %v", err)
	}
	if got := accOf(); got != 21.00 {
		t.Fatalf("冲减后期望 21.00，实际 %.2f", got)
	}

	// 6) 同一退款单重复审核 → 幂等
	if err := svc.ClawbackForRefund(ctx, ClawbackInput{
		OrderID: 9002, OrderNo: fmt.Sprintf("T%d-B", suffix), BuyerUserID: invitee,
		PaidAmount: 200, RefundAmount: 100, RefundNo: fmt.Sprintf("R%d-B1", suffix),
	}); err != nil {
		t.Fatalf("重复冲减报错: %v", err)
	}
	if got := accOf(); got != 21.00 {
		t.Fatalf("冲减幂等期望仍为 21.00，实际 %.2f", got)
	}

	// 7) 剩余 100 全额退完 → 再冲 5.00，冲减累计不超过已计提
	if err := svc.ClawbackForRefund(ctx, ClawbackInput{
		OrderID: 9002, OrderNo: fmt.Sprintf("T%d-B", suffix), BuyerUserID: invitee,
		PaidAmount: 200, RefundAmount: 100, RefundNo: fmt.Sprintf("R%d-B2", suffix),
	}); err != nil {
		t.Fatalf("二次冲减失败: %v", err)
	}
	if got := accOf(); got != 16.00 {
		t.Fatalf("二次冲减后期望 16.00，实际 %.2f", got)
	}

	// 8) 台账条数：首单+后续+续费 3 入账 + 2 冲减 = 5（幂等未新增）
	var txCount int64
	if err := db.Model(&model.ReferralTransaction{}).Where("user_id = ?", inviter).Count(&txCount).Error; err != nil {
		t.Fatalf("统计台账失败: %v", err)
	}
	if txCount != 5 {
		t.Fatalf("台账期望 5 条，实际 %d", txCount)
	}

	// 9) 无邀请人的用户下单 → 不产生返现（也不报错）
	orphan := createTestUser(t, db, fmt.Sprintf("ref_orph_%d", suffix))
	defer func() { db.Exec("DELETE FROM users WHERE id = ?", orphan) }()
	if err := svc.AccrueForOrder(ctx, AccrualInput{OrderID: 9004, OrderNo: fmt.Sprintf("T%d-D", suffix), BuyerUserID: orphan, BaseAmount: 500}); err != nil {
		t.Fatalf("无邀请人计提不应报错: %v", err)
	}
}

func createTestUser(t *testing.T, db *gorm.DB, username string) uint64 {
	t.Helper()
	var id uint64
	err := db.Raw(`INSERT INTO users (username, email, password_hash, status, tier, balance, total_consume_amount, is_sub_account, created_at, updated_at)
		VALUES (?, ?, 'x', 'active', 'free', 0, 0, false, now(), now()) RETURNING id`,
		username, username+"@test.local").Scan(&id).Error
	if err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}
	if id == 0 {
		t.Fatalf("创建测试用户未返回 ID")
	}
	return id
}

func seedRates(t *testing.T, db *gorm.DB) {
	t.Helper()
	rows := map[string]string{
		model.ConfigKeyEnabled:     "true",
		model.ConfigKeyFirstOrder:  "0.10",
		model.ConfigKeySubsequent:  "0.05",
		model.ConfigKeyRenewal:     "0.03",
		model.ConfigKeyMinWithdraw: "50.00",
	}
	for k, v := range rows {
		if err := db.Exec(`INSERT INTO system_configs (config_key, config_value, value_type, config_group, status, sort_order, created_at, updated_at)
			VALUES (?, ?, 'string', ?, 'active', 0, now(), now())
			ON CONFLICT (config_key) DO UPDATE SET config_value = EXCLUDED.config_value, config_group = EXCLUDED.config_group, status = 'active'`,
			k, v, model.ConfigGroup).Error; err != nil {
			t.Fatalf("写入返现配置失败: %v", err)
		}
	}
}

func cleanupReferralTest(t *testing.T, db *gorm.DB, inviter, invitee uint64) {
	t.Helper()
	for _, uid := range []uint64{inviter, invitee} {
		db.Exec("DELETE FROM referral_transactions WHERE user_id = ?", uid)
		db.Exec("DELETE FROM referral_accounts WHERE user_id = ?", uid)
		db.Exec("DELETE FROM referral_withdrawals WHERE user_id = ?", uid)
	}
	db.Exec("DELETE FROM users WHERE id IN ?", []uint64{inviter, invitee})
}
