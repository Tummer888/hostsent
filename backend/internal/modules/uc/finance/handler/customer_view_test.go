package handler

import (
	"encoding/json"
	"strings"
	"testing"

	accountdto "hostsent/backend/internal/modules/admin/finance/account/dto"
	rechdto "hostsent/backend/internal/modules/admin/finance/account/recharge/dto"
	transdto "hostsent/backend/internal/modules/admin/finance/transaction/dto"
)

// TestCustomerTransactionDropsInternalFields 用户侧资金流水不得下发经办人 ID 与用户 ID：
// 「谁给这笔账做的操作」属于平台内部账务口径。
func TestCustomerTransactionDropsInternalFields(t *testing.T) {
	in := &transdto.TransactionListResponse{
		Items: []transdto.TransactionInfo{{
			ID:            1,
			TxNo:          "W20261013001",
			UserID:        2,
			Username:      "tester01",
			Type:          "recharge",
			BizType:       "recharge",
			Direction:     1,
			Amount:        100,
			BalanceBefore: 50,
			BalanceAfter:  150,
			OrderNo:       "",
			RefNo:         "RC20261013001",
			Remark:        "在线充值",
			OperatorID:    1,
			CreatedAt:     "2026-10-13T14:00:00Z",
		}},
		Meta: transdto.ListMeta{Page: 1, PageSize: 20, Total: 1},
		Summary: transdto.TransactionSummary{
			IncomeTotal: 100, ExpenseTotal: 0, NetTotal: 100, TxCount: 1, InternalCount: 3,
		},
	}

	raw, err := json.Marshal(customerTransactionListFrom(in))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)

	for _, field := range []string{"user_id", "username", "operator_id", "biz_type", "internal_count", "order_id"} {
		if strings.Contains(body, `"`+field+`"`) {
			t.Errorf("用户侧资金流水不应包含内部字段 %q，实际: %s", field, body)
		}
	}
	for _, keep := range []string{"tx_no", "amount", "balance_after", "remark", "income_total", "net_total"} {
		if !strings.Contains(body, `"`+keep+`"`) {
			t.Errorf("用户侧资金流水缺少必需字段 %q，实际: %s", keep, body)
		}
	}
}

// TestCustomerRechargeDropsInternalFields 用户侧充值单不得下发渠道流水号与支付单主键。
func TestCustomerRechargeDropsInternalFields(t *testing.T) {
	in := &rechdto.RechargeListResponse{
		Items: []rechdto.RechargeInfo{{
			ID:             9,
			RechargeNo:     "RC20261013001",
			UserID:         2,
			Amount:         100,
			Method:         "manual",
			Status:         "success",
			ChannelTx:      "UI-CHAIN-001",
			ChannelCode:    "manual_backup",
			PaymentOrderID: 76,
			PaidAt:         "2026-10-13T14:10:00Z",
			Remark:         "在线充值",
			CreatedAt:      "2026-10-13T14:00:00Z",
			UpdatedAt:      "2026-10-13T14:10:00Z",
		}},
		Meta: rechdto.ListMeta{Page: 1, PageSize: 20, Total: 1},
	}

	raw, err := json.Marshal(customerRechargeListFrom(in))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)

	for _, field := range []string{"user_id", "channel_tx", "payment_order_id", "remark", "updated_at"} {
		if strings.Contains(body, `"`+field+`"`) {
			t.Errorf("用户侧充值单不应包含内部字段 %q，实际: %s", field, body)
		}
	}
	if strings.Contains(body, "UI-CHAIN-001") {
		t.Errorf("用户侧充值单泄露渠道流水号: %s", body)
	}
	for _, keep := range []string{"recharge_no", "amount", "method", "status", "channel_code", "paid_at"} {
		if !strings.Contains(body, `"`+keep+`"`) {
			t.Errorf("用户侧充值单缺少必需字段 %q，实际: %s", keep, body)
		}
	}

	// 单条视图与列表同一口径。
	single, err := json.Marshal(customerRechargeFrom(&in.Items[0]))
	if err != nil {
		t.Fatalf("marshal single: %v", err)
	}
	if strings.Contains(string(single), "UI-CHAIN-001") || strings.Contains(string(single), "payment_order_id") {
		t.Errorf("单条充值单未脱敏: %s", single)
	}
}

// TestCustomerWalletDropsInternalFields 我的钱包不下发 user_id 与乐观锁版本号：
// 客户端只需要金额口径，版本号是服务端并发控制字段。
func TestCustomerWalletDropsInternalFields(t *testing.T) {
	in := &accountdto.WalletInfo{
		UserID:       2,
		Balance:      186.8,
		Frozen:       1,
		TotalIncome:  908,
		TotalExpense: 720.2,
		Version:      29,
	}
	raw, err := json.Marshal(customerWalletFrom(in))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	for _, field := range []string{"user_id", "version"} {
		if strings.Contains(body, `"`+field+`"`) {
			t.Errorf("我的钱包不应包含内部字段 %q，实际: %s", field, body)
		}
	}
	for _, keep := range []string{"balance", "frozen", "total_income", "total_expense"} {
		if !strings.Contains(body, `"`+keep+`"`) {
			t.Errorf("我的钱包缺少必需字段 %q，实际: %s", keep, body)
		}
	}
}
