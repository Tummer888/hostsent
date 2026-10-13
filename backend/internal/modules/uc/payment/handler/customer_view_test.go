package handler

import (
	"encoding/json"
	"strings"
	"testing"

	withdrawdto "hostsent/backend/internal/modules/admin/finance/referral/walletwithdraw/dto"
	paydto "hostsent/backend/internal/modules/admin/system/payment/dto"
)

// TestCustomerMethodOptionsDropsInternalFields 用户中心支付方式不得下发平台内部字段：
// 渠道备注（如分级/兜底标记）、凭证、接口地址、回调地址、费率与健康状态。
func TestCustomerMethodOptionsDropsInternalFields(t *testing.T) {
	in := &paydto.MethodOptionsResponse{
		Scene: "native",
		Channels: []paydto.ChannelInfo{{
			ID:           1,
			ChannelCode:  "manual_main",
			Name:         "对公转账-主账户",
			Type:         "manual",
			TypeName:     "人工",
			Mode:         "manual",
			Credentials:  map[string]string{"app_id": "ak_1234****"},
			Endpoint:     "https://openapi.alipay.com/gateway.do",
			NotifyURL:    "https://internal.example.com/notify",
			FeeRate:      0.006,
			SettleMode:   "t1",
			MinAmountFen: 100,
			MaxAmountFen: 0,
			HealthStatus: "healthy",
			LastError:    "上游超时",
			Remark:       "S1 兜底渠道",
			IsDefault:    true,
		}},
		Default: "manual_main",
	}

	raw, err := json.Marshal(customerMethodOptionsFrom(in))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)

	for _, field := range []string{
		"credentials", "endpoint", "notify_url", "return_url", "fee_rate",
		"settle_mode", "health_status", "last_error", "remark", "is_default",
		"capabilities", "priority", "weight",
	} {
		if strings.Contains(body, `"`+field+`"`) {
			t.Errorf("用户侧支付方式不应包含内部字段 %q，实际: %s", field, body)
		}
	}
	for _, leak := range []string{"S1 兜底渠道", "openapi.alipay.com", "internal.example.com", "上游超时"} {
		if strings.Contains(body, leak) {
			t.Errorf("用户侧支付方式泄露内部取值 %q，实际: %s", leak, body)
		}
	}
	// 下单必需字段必须保留，否则用户选不了支付方式。
	for _, keep := range []string{"channel_code", "name", "min_amount_fen", "max_amount_fen"} {
		if !strings.Contains(body, `"`+keep+`"`) {
			t.Errorf("用户侧支付方式缺少必需字段 %q，实际: %s", keep, body)
		}
	}
}

// TestCustomerPaymentOrderDropsInternalFields 支付单不得下发渠道流水号、手续费与平台备注。
func TestCustomerPaymentOrderDropsInternalFields(t *testing.T) {
	in := &paydto.OrderInfo{
		ID:           7,
		PaymentNo:    "PAY20261013001",
		UserID:       2,
		Amount:       100,
		ChannelCode:  "manual_main",
		Status:       "pending",
		PayURL:       "https://pay.example.com/x",
		QRCode:       "https://pay.example.com/qr.png",
		Instructions: "请转账至 6222****1234",
		ChannelTx:    "420000123420261013",
		PrepayParams: `{"appId":"ak_live_secret"}`,
		Fee:          0.6,
		Remark:       "S1 兜底渠道",
	}

	raw, err := json.Marshal(customerPaymentOrderFrom(in))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)

	for _, field := range []string{"user_id", "channel_tx", "prepay_params", "fee", "remark", "channel_id"} {
		if strings.Contains(body, `"`+field+`"`) {
			t.Errorf("用户侧支付单不应包含内部字段 %q，实际: %s", field, body)
		}
	}
	for _, leak := range []string{"420000123420261013", "ak_live_secret", "S1 兜底渠道"} {
		if strings.Contains(body, leak) {
			t.Errorf("用户侧支付单泄露内部取值 %q，实际: %s", leak, body)
		}
	}
	// 收银台展示必需字段。
	for _, keep := range []string{"payment_no", "amount", "pay_url", "qrcode", "instructions", "status"} {
		if !strings.Contains(body, `"`+keep+`"`) {
			t.Errorf("用户侧支付单缺少必需字段 %q，实际: %s", keep, body)
		}
	}
}

// TestCustomerWithdrawHidesAuditor 提现单不下发审核人身份与渠道流水号。
func TestCustomerWithdrawHidesAuditor(t *testing.T) {
	in := withdrawdto.WithdrawInfo{
		ID:          9,
		WithdrawNo:  "WD20261013001",
		UserID:      2,
		Amount:      50,
		Channel:     "bank",
		ChannelName: "银行卡",
		Account:     "6222****1234",
		AccountName: "张三",
		Status:      "paid",
		AuditBy:     88,
		AuditByName: "财务-李四",
		PayoutNo:    "PO20261013001",
		ChannelTx:   "420000999",
		Remark:      "内部备注：走加急",
	}

	raw, err := json.Marshal(customerWithdrawFrom(in))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)

	for _, field := range []string{"user_id", "audit_by", "audit_by_name", "channel_tx", "remark"} {
		if strings.Contains(body, `"`+field+`"`) {
			t.Errorf("用户侧提现单不应包含内部字段 %q，实际: %s", field, body)
		}
	}
	for _, leak := range []string{"财务-李四", "420000999", "内部备注"} {
		if strings.Contains(body, leak) {
			t.Errorf("用户侧提现单泄露内部取值 %q，实际: %s", leak, body)
		}
	}
	for _, keep := range []string{"withdraw_no", "amount", "account", "status", "payout_no"} {
		if !strings.Contains(body, `"`+keep+`"`) {
			t.Errorf("用户侧提现单缺少必需字段 %q，实际: %s", keep, body)
		}
	}
}

// TestCustomerWithdrawListHidesAuditor 列表视图与单条视图保持同一脱敏口径。
func TestCustomerWithdrawListHidesAuditor(t *testing.T) {
	resp := &withdrawdto.WithdrawListResponse{
		Items: []withdrawdto.WithdrawInfo{{
			ID: 1, WithdrawNo: "WD1", AuditByName: "财务-王五", ChannelTx: "420000111",
		}},
		Meta: withdrawdto.ListMeta{Total: 1},
	}
	raw, err := json.Marshal(customerWithdrawListFrom(resp))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	if strings.Contains(body, "财务-王五") || strings.Contains(body, "420000111") {
		t.Errorf("提现列表泄露内部信息: %s", body)
	}
	if !strings.Contains(body, `"meta"`) || !strings.Contains(body, `"total":1`) {
		t.Errorf("提现列表缺少分页信息: %s", body)
	}
}
