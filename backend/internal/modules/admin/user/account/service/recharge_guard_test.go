package service

import (
	"context"
	"errors"
	"testing"

	"hostsent/backend/internal/modules/admin/user/account/model"
)

// TestRechargeRejectsSubAccount 子账号不能人工充值。
//
// 依据设计 §4.6：子账号不独立结算，余额只有一个入口（主账号）。而所有消费路径
// （下单/续费）都按 EffectiveUserID 取主账号，子账号 wallet 里的钱既花不掉也
// 提不出来——放行只会造出一笔财务无法归集的「孤儿钱」。
func TestRechargeRejectsSubAccount(t *testing.T) {
	owner := uint64(100)
	repo := &fakeUserRepo{user: &model.User{ID: 200, IsSubAccount: true, OwnerUserID: &owner}}
	called := false
	svc := &userService{
		repo: repo,
		recharge: func(context.Context, uint64, float64, string, uint64) (*RechargeResult, error) {
			called = true
			return &RechargeResult{RechargeNo: "RC-should-not-happen"}, nil
		},
	}

	for _, amount := range []float64{10, -10} {
		_, err := svc.Recharge(context.Background(), 200, amount, "验收", 1)
		if !errors.Is(err, ErrRechargeSubAccountDenied) {
			t.Fatalf("amount=%v: err = %v, want ErrRechargeSubAccountDenied", amount, err)
		}
	}
	if called {
		t.Fatal("子账号充值被拒绝时不应触达 Recharger（否则已产生充值单与入账）")
	}
}

// TestRechargeAllowsOwner 主账号充值照常放行，确认上面的拦截没有误伤正常路径。
func TestRechargeAllowsOwner(t *testing.T) {
	repo := &fakeUserRepo{user: &model.User{ID: 100}}
	svc := &userService{
		repo: repo,
		recharge: func(_ context.Context, userID uint64, amount float64, _ string, _ uint64) (*RechargeResult, error) {
			if userID != 100 || amount != 50 {
				t.Fatalf("Recharger 入参 = (%d, %v), want (100, 50)", userID, amount)
			}
			return &RechargeResult{RechargeNo: "RC-ok"}, nil
		},
	}

	got, err := svc.Recharge(context.Background(), 100, 50, "人工充值", 1)
	if err != nil {
		t.Fatalf("主账号充值失败: %v", err)
	}
	if got == nil || got.RechargeNo != "RC-ok" {
		t.Fatalf("result = %+v, want RechargeNo=RC-ok", got)
	}
}

// TestRechargeRejectsZeroAmount 金额为 0 依旧按原语义拒绝（走的是同一条前置校验）。
func TestRechargeRejectsZeroAmount(t *testing.T) {
	repo := &fakeUserRepo{user: &model.User{ID: 100}}
	svc := &userService{repo: repo}

	if _, err := svc.Recharge(context.Background(), 100, 0, "", 1); !errors.Is(err, ErrInvalidAdjustAmount) {
		t.Fatalf("err = %v, want ErrInvalidAdjustAmount", err)
	}
}
