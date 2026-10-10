package mofangfinance

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"hostsent/backend/internal/pkg/upstream"
)

// 账本能力（消费/充值/到期）是成本管理「上游流水口径」的数据来源（doc111 §5.2）。
var _ upstream.FinanceLedgerReader = (*MoFangFinanceProvider)(nil)

// TestContract_ListConsumptionRecords 校验消费流水契约：
// GET /credit_record 携带 Bearer，data.accounts 的 relid/type/amount 映射为
// 账单号/类型/金额，秒级时间戳转 UTC，关联退款行（负数）累加为 RefundAmount。
func TestContract_ListConsumptionRecords(t *testing.T) {
	var gotPath, gotQuery, authHeader string
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zjmf_api_login":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
		case "/credit_record":
			gotPath, gotQuery, authHeader = r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"data": map[string]interface{}{
					"total": 2,
					"accounts": []map[string]interface{}{
						{
							"id": 9001, "relid": 807400, "create_time": 1789135688,
							"description": "余额支付", "type": "订购产品", "amount": "188.50",
							"refund": []map[string]interface{}{{"id": 1, "amount": "-88.50"}},
						},
						{
							// 上游无退款时返回的是空对象 {}（实测），不是 null/[]：
							// 早期写成 []RefundLine 会整页同步失败，这里锁死兼容。
							"id": 9002, "relid": 807371, "create_time": 1789130000,
							"description": "余额支付", "type": "续费", "amount": 16,
							"refund": map[string]interface{}{},
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	entries, total, err := p.ListConsumptionRecords(context.Background(), 2, 60)
	if err != nil {
		t.Fatalf("ListConsumptionRecords failed: %v", err)
	}
	if gotPath != "/credit_record" || authHeader != "Bearer TESTJWT" {
		t.Errorf("path=%q auth=%q, want /credit_record + Bearer TESTJWT", gotPath, authHeader)
	}
	if gotQuery != "limit=60&page=2" {
		t.Errorf("query = %q, want limit=60&page=2", gotQuery)
	}
	if total != 2 || len(entries) != 2 {
		t.Fatalf("total=%d entries=%d, want 2/2", total, len(entries))
	}
	first := entries[0]
	if first.Kind != upstream.LedgerKindConsume || first.ExternalID != "9001" || first.RefNo != "807400" {
		t.Errorf("unexpected entry identity: %+v", first)
	}
	if first.Amount != 188.5 {
		t.Errorf("amount = %v, want 188.5（字符串金额需解析）", first.Amount)
	}
	if first.RefundAmount != 88.5 {
		t.Errorf("refund = %v, want 88.5（负数取绝对值累计）", first.RefundAmount)
	}
	if first.Category != "订购产品" || entries[1].Category != "续费" {
		t.Errorf("category mapping wrong: %q / %q", first.Category, entries[1].Category)
	}
	if want := time.Unix(1789135688, 0).UTC(); !first.OccurredAt.Equal(want) {
		t.Errorf("occurred_at = %v, want %v", first.OccurredAt, want)
	}
	if entries[1].RefundAmount != 0 {
		t.Errorf("无退款行时 RefundAmount 应为 0，实际 %v", entries[1].RefundAmount)
	}
}

// TestContract_ListTopupRecords 校验充值流水契约：GET /v1/transactions/funds，
// amount_in 解析为金额、pay_time 转时间、交易号进 RefNo、描述与支付方式合并。
func TestContract_ListTopupRecords(t *testing.T) {
	var gotPath, authHeader string
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zjmf_api_login":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
		case "/v1/transactions/funds":
			gotPath, authHeader = r.URL.Path, r.Header.Get("Authorization")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"data": map[string]interface{}{
					"total": 1,
					"accounts": []map[string]interface{}{
						{
							"id": 2922, "invoice_id": 807181, "pay_time": 1788231074,
							"payment_zh": "支付宝支付", "description": "用户充值", "type": "recharge",
							"amount_in": "5000.00", "trans_id": "2026090110505985247",
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	entries, total, err := p.ListTopupRecords(context.Background(), 1, 60)
	if err != nil {
		t.Fatalf("ListTopupRecords failed: %v", err)
	}
	if gotPath != "/v1/transactions/funds" || authHeader != "Bearer TESTJWT" {
		t.Errorf("path=%q auth=%q", gotPath, authHeader)
	}
	if total != 1 || len(entries) != 1 {
		t.Fatalf("total=%d entries=%d, want 1/1", total, len(entries))
	}
	e := entries[0]
	if e.Kind != upstream.LedgerKindTopup || e.Amount != 5000 || e.RefNo != "2026090110505985247" {
		t.Errorf("unexpected topup entry: %+v", e)
	}
	if e.Category != "用户充值" {
		t.Errorf("category = %q, want 用户充值", e.Category)
	}
	if e.Description != "用户充值 支付宝支付" {
		t.Errorf("description = %q, want 描述+支付方式", e.Description)
	}
}

// TestContract_ListDueHosts 校验到期读取：GET /v1/hosts 分页拉全，
// 金额与到期时间解析正确（余额水位告警用）。
func TestContract_ListDueHosts(t *testing.T) {
	pages := 0
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zjmf_api_login":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
		case "/v1/hosts":
			pages++
			host := map[string]interface{}{
				"id": 1935, "domain": "hs-4-4-10mbps", "product_name": "香港|轻量 4-4 10Mbps",
				"amount": "16.00", "nextduedate": 1791520725, "domainstatus": "Suspended",
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"data":   map[string]interface{}{"total": 2, "host": []map[string]interface{}{host}},
			})
		default:
			http.NotFound(w, r)
		}
	})

	dues, err := p.ListDueHosts(context.Background())
	if err != nil {
		t.Fatalf("ListDueHosts failed: %v", err)
	}
	if pages != 2 {
		t.Errorf("分页次数 = %d，want 2（total=2 而每页 1 条）", pages)
	}
	if len(dues) != 2 {
		t.Fatalf("dues = %d, want 2", len(dues))
	}
	d := dues[0]
	if d.UpstreamID != "1935" || d.Amount != 16 || d.Status != "Suspended" {
		t.Errorf("unexpected due host: %+v", d)
	}
	if want := time.Unix(1791520725, 0).UTC(); !d.NextDueAt.Equal(want) {
		t.Errorf("next_due = %v, want %v", d.NextDueAt, want)
	}
}
