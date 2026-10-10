package mofangfinance

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"hostsent/backend/internal/pkg/upstream"
)

// 成本管理「上游余额台账」依赖本能力（doc111 §5）：实现 AccountReader 后，
// 资源渠道服务的能力探测（SupportsAccountBalance）会把它标记为「支持抓取」，
// 页面的「抓取余额」按钮与每日自动快照随即生效。
var _ upstream.AccountReader = (*MoFangFinanceProvider)(nil)

// TestContract_GetAccountInfo 校验取余额契约（对齐旧 PHP upstreamCredit）：
// POST /zjmf_api_login 换 JWT → GET /cart/credit 携带 `Authorization: Bearer <jwt>`；
// data.credit 为字符串 "199.00" 时解析为 199，currency.code 归一为大写。
func TestContract_GetAccountInfo(t *testing.T) {
	var creditPath, authHeader string
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zjmf_api_login":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
		case "/cart/credit":
			creditPath = r.URL.Path
			if r.Method != http.MethodGet {
				t.Errorf("cart/credit method = %s, want GET", r.Method)
			}
			authHeader = r.Header.Get("Authorization")
			// 上游 clients.credit 是 decimal，JSON 里是字符串（thinkphp 原样返回）。
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"data": map[string]interface{}{
					"credit":   "199.00",
					"currency": map[string]interface{}{"id": 1, "code": "cny", "prefix": "¥", "suffix": ""},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	info, err := p.GetAccountInfo(context.Background())
	if err != nil {
		t.Fatalf("GetAccountInfo failed: %v", err)
	}
	if creditPath != "/cart/credit" {
		t.Errorf("请求路径 = %q, want /cart/credit", creditPath)
	}
	if authHeader != "Bearer TESTJWT" {
		t.Errorf("Authorization = %q, want \"Bearer TESTJWT\"", authHeader)
	}
	if info.Balance != 199 {
		t.Errorf("balance = %v, want 199", info.Balance)
	}
	if info.Currency != "CNY" {
		t.Errorf("currency = %q, want CNY（ISO 码归一为大写）", info.Currency)
	}
}

// TestContract_GetAccountInfoCurrencyFallback 覆盖上游返回空币种（客户未设币种且无默认币种时
// getUserCurrency 返回 []）与数字型 credit 两个边界：币种回落 CNY，金额照常解析。
func TestContract_GetAccountInfoCurrencyFallback(t *testing.T) {
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zjmf_api_login":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
		case "/cart/credit":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"data":   map[string]interface{}{"credit": 12.5, "currency": []interface{}{}},
			})
		default:
			http.NotFound(w, r)
		}
	})

	info, err := p.GetAccountInfo(context.Background())
	if err != nil {
		t.Fatalf("GetAccountInfo failed: %v", err)
	}
	if info.Balance != 12.5 {
		t.Errorf("balance = %v, want 12.5", info.Balance)
	}
	if info.Currency != "CNY" {
		t.Errorf("currency = %q, want CNY（空币种回落）", info.Currency)
	}
}

// TestContract_GetAccountInfoMissingCredit 响应缺 credit 字段时必须报错而不是落 0：
// 静默返回 0 会被成本台账读成「钱花光了」，比接口失败更危险。
func TestContract_GetAccountInfoMissingCredit(t *testing.T) {
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zjmf_api_login":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
		case "/cart/credit":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"data":   map[string]interface{}{"currency": map[string]interface{}{"code": "CNY"}},
			})
		default:
			http.NotFound(w, r)
		}
	})

	_, err := p.GetAccountInfo(context.Background())
	if err == nil {
		t.Fatal("响应缺 credit 字段时应报错，实际成功")
	}
	if !strings.Contains(err.Error(), "credit") {
		t.Errorf("错误信息应点明缺少 credit，实际: %v", err)
	}
}
