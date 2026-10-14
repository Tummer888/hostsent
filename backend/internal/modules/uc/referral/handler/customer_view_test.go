package handler

import (
	"encoding/json"
	"strings"
	"testing"

	referraldto "hostsent/backend/internal/modules/admin/finance/referral/dto"
)

// TestCustomerWithdrawalHidesAuditor 返现提现不得下发审核人姓名与用户 ID：
// 「谁审核的」属于平台内部经办信息，用户只需看到审核结果与时间。
func TestCustomerWithdrawalHidesAuditor(t *testing.T) {
	in := referraldto.WithdrawalInfo{
		ID:          9,
		WithdrawNo:  "RW20261013001",
		UserID:      2,
		Username:    "tester01",
		Amount:      88,
		Channel:     "bank",
		Account:     "6222****1234",
		Status:      "approved",
		AuditBy:     88,
		AuditByName: "财务-李四",
		AuditedAt:   "2026-10-13 10:00:00",
		Remark:      "内部备注：已核对",
	}

	raw, err := json.Marshal(customerWithdrawalFrom(in))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)

	for _, field := range []string{"user_id", "username", "audit_by", "audit_by_name", "remark"} {
		if strings.Contains(body, `"`+field+`"`) {
			t.Errorf("用户侧返现提现不应包含内部字段 %q，实际: %s", field, body)
		}
	}
	if strings.Contains(body, "财务-李四") {
		t.Errorf("用户侧返现提现泄露审核人: %s", body)
	}
	for _, keep := range []string{"withdraw_no", "amount", "status", "audited_at", "account"} {
		if !strings.Contains(body, `"`+keep+`"`) {
			t.Errorf("用户侧返现提现缺少必需字段 %q，实际: %s", keep, body)
		}
	}
}

// TestCustomerWithdrawalListHidesAuditor 列表视图与单条视图同一口径。
func TestCustomerWithdrawalListHidesAuditor(t *testing.T) {
	resp := &referraldto.WithdrawalListResponse{
		Items: []referraldto.WithdrawalInfo{{
			ID: 1, WithdrawNo: "RW1", AuditByName: "财务-王五", Username: "tester01",
		}},
		Meta: referraldto.ListMeta{Total: 1},
	}
	raw, err := json.Marshal(customerWithdrawalListFrom(resp))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	if strings.Contains(body, "财务-王五") || strings.Contains(body, "tester01") {
		t.Errorf("返现提现列表泄露内部信息: %s", body)
	}
	if !strings.Contains(body, `"total":1`) {
		t.Errorf("返现提现列表缺少分页信息: %s", body)
	}
}
