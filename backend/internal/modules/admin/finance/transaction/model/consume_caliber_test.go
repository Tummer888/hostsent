package model

import "testing"

// 回归 R8：消费口径只认一处，避免各链路再各写一份白名单。
//
// 这条断言看起来像是「测常量」，但它挡的是一个真实缺陷：后台代客下单扣款用的是
// order 类型，而消费口径曾只认 consume，于是那笔钱扣了却不算消费 ——
// 实测用户 2730 买了 18 元代客单，总消费额停在 1 元。
// 若将来有人删掉 TxTypeAdminOrder，或只改 ConsumeTypes 而忘了 IsConsumeType 的
// 一致性，这里立刻失败。
func TestConsumeTypesCoversAdminOrder(t *testing.T) {
	if !IsConsumeType(TxTypeConsume) {
		t.Fatal("用户端下单扣款(consume)必须计入消费")
	}
	if !IsConsumeType(TxTypeAdminOrder) {
		t.Fatal("后台代客下单扣款(order)必须计入消费：它是真实收款的购买行为")
	}
}

// 不该算消费的类型：充值/退款/调账是资金进出但非购买，
// settlement 是提现打款出账（钱离开平台但不是买货），freeze/unfreeze 是内部划转。
func TestNonConsumeTypesExcluded(t *testing.T) {
	for _, txType := range []string{
		TxTypeRecharge, TxTypeRefund, TxTypeAdjust, TxTypeSettlement,
		TxTypeFreeze, TxTypeUnfreeze, TxTypeCommission, TxTypeReferralTransfer,
		"unknown_type", "",
	} {
		if IsConsumeType(txType) {
			t.Errorf("%q 不是消费，不应计入消费口径", txType)
		}
	}
}

// 内部划转集合必须恰好是冻结/解冻：统计与对账都靠它剔除「可用↔冻结」的搬运，
// 多一个会漏算真实收支，少一个会把内部搬运算成收付。
func TestInternalTransferTypes(t *testing.T) {
	if len(InternalTransferTypes) != 2 {
		t.Fatalf("内部划转类型应为 2 个（freeze/unfreeze），实际 %d", len(InternalTransferTypes))
	}
	for _, want := range []string{TxTypeFreeze, TxTypeUnfreeze} {
		found := false
		for _, got := range InternalTransferTypes {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("内部划转类型缺少 %q", want)
		}
	}
	if IsConsumeType(TxTypeSettlement) {
		t.Error("settlement（提现结算出账）不是消费")
	}
}
