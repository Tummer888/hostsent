package mofangfinance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// newMockProvider 起一个 httptest 服务模拟魔方财务（财务型 zjmf_api），返回指向它的适配器。
func newMockProvider(t *testing.T, handler http.HandlerFunc) *MoFangFinanceProvider {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return NewMoFangFinanceProvider(&upstream.ProviderConfig{
		APIEndpoint:  ts.URL,
		Secure:       false,
		UpstreamType: "zjmf_api",
		Timeout:      5,
	})
}

// TestContract_GetInstanceHostHeader 校验：登录走 POST /zjmf_api_login（返回 jwt），
// 业务调用 GET /host/header 携带 `Authorization: Bearer <jwt>`，且 assignedips/域名映射正确。
func TestContract_GetInstanceHostHeader(t *testing.T) {
	var authHeader string
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zjmf_api_login":
			if r.Method != http.MethodPost {
				t.Errorf("login method = %s, want POST", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "jwt": "TESTJWT"})
		case "/host/header":
			authHeader = r.Header.Get("Authorization")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200,
				"data": map[string]interface{}{
					"host_data": map[string]interface{}{
						"id": 1934, "domain": "hs-abc", "dedicatedip": "1.2.3.4",
						"domainstatus": "Active", "assignedips": "1.2.3.4,5.6.7.8",
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	inst, err := p.GetInstance(context.Background(), "1934")
	if err != nil {
		t.Fatalf("GetInstance failed: %v", err)
	}
	if authHeader != "Bearer TESTJWT" {
		t.Errorf("Authorization header = %q, want \"Bearer TESTJWT\"", authHeader)
	}
	if inst.UpstreamID != "1934" || inst.Name != "hs-abc" || inst.PublicIP != "1.2.3.4" {
		t.Fatalf("unexpected instance: %+v", inst)
	}
	if inst.Status != pkgmodel.InstanceStatusRunning {
		t.Errorf("status = %s, want running (Active)", inst.Status)
	}
	if got, ok := inst.RawData["assignedips"].(StringList); !ok || got != "1.2.3.4,5.6.7.8" {
		t.Errorf("assignedips = %#v, want \"1.2.3.4,5.6.7.8\"", inst.RawData["assignedips"])
	}
}
