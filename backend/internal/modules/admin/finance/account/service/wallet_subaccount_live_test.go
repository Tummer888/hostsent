//go:build live

package service

import (
	"context"
	"errors"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestWalletRejectsSubAccount 子账号不能有独立钱包。
//
// 这条守卫挡的是「懒创建」：Change/moveFrozen 查不到钱包就建一个，
// 于是一条指向子账号的记账调用会顺手给它建出钱包，而这笔钱既花不掉
// （消费走 EffectiveUserID = 主账号）也提不出来，成为财务无法归集的孤儿钱。
//
// 需要真实库：users 表跨模块只读，没有可替换的 fake。
// 运行：LIVE_DB_DSN=... go test -tags live ./internal/modules/admin/finance/account/service/ -run SubAccount
func TestWalletRejectsSubAccount(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		t.Skip("LIVE_DB_DSN 未设置，跳过 live 用例")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连库失败: %v", err)
	}

	// 取一个真实子账号；库里没有就跳过（不造数据，避免污染）。
	var sub struct {
		ID uint64
	}
	if err := db.Raw(`SELECT id FROM users WHERE is_sub_account = true AND deleted_at IS NULL LIMIT 1`).
		Scan(&sub).Error; err != nil {
		t.Fatalf("查子账号失败: %v", err)
	}
	if sub.ID == 0 {
		t.Skip("库中没有子账号，跳过")
	}
	// 取一个真实主账号。
	var owner struct {
		ID uint64
	}
	if err := db.Raw(`SELECT id FROM users WHERE is_sub_account = false AND deleted_at IS NULL LIMIT 1`).
		Scan(&owner).Error; err != nil {
		t.Fatalf("查主账号失败: %v", err)
	}
	if owner.ID == 0 {
		t.Skip("库中没有主账号，跳过")
	}

	svc := &walletService{db: db}

	if err := svc.rejectSubAccount(context.Background(), sub.ID); !errors.Is(err, ErrSubAccountWalletDenied) {
		t.Fatalf("子账号(%d)应被拒，得到 %v", sub.ID, err)
	}
	if err := svc.rejectSubAccount(context.Background(), owner.ID); err != nil {
		t.Fatalf("主账号(%d)应放行，得到 %v", owner.ID, err)
	}
	// 不存在的用户放行（交给下游既有校验报错），不能在这里变成「账号不存在」。
	if err := svc.rejectSubAccount(context.Background(), 0); err != nil {
		t.Fatalf("userID=0 应放行，得到 %v", err)
	}
}
