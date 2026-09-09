package mofangyun

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// newMockProvider 起一个 httptest 服务模拟魔方云（管理员账号），返回 server 与指向它的适配器。
func newMockProvider(t *testing.T, handler http.HandlerFunc) *MoFangYunProvider {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return NewMoFangYunProvider(&upstream.ProviderConfig{
		APIEndpoint: ts.URL,
		Secure:      false,
		Timeout:     5,
	})
}

// TestContractLogin_TokenFlow 校验：管理员账号登录走 POST /v1/login（响应体即 token），
// 业务调用携带 access-token 头；登录成功后 token 被缓存，业务调用不重复打登录。
func TestContractLogin_TokenFlow(t *testing.T) {
	var logins, business apiCalls
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/login":
			logins.add(r.Method + " " + r.URL.Path)
			// 管理员登录：响应体即 token
			if r.Method != http.MethodPost {
				t.Errorf("login method = %s, want POST", r.Method)
			}
			_, _ = w.Write([]byte("test-token"))
			return
		case "/v1/clouds/1934":
			business.add(r.Header.Get("access-token"))
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "1934", "hostname": "hs-abc", "mainip": "1.2.3.4", "status": "on",
			})
			return
		case "/v1/clouds/1934/status":
			business.add(r.Header.Get("access-token"))
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "on", "task_name": ""})
			return
		default:
			http.NotFound(w, r)
		}
	})

	inst, err := p.GetInstance(context.Background(), "1934")
	if err != nil {
		t.Fatalf("GetInstance failed: %v", err)
	}
	if inst.UpstreamID != "1934" || inst.Name != "hs-abc" {
		t.Fatalf("unexpected instance: %+v", inst)
	}
	if inst.Status != pkgmodel.InstanceStatusRunning {
		t.Errorf("status = %s, want running", inst.Status)
	}
	if logins.count() != 1 {
		t.Errorf("login called %d times, want 1 (token should be cached)", logins.count())
	}
	if business.count() != 2 {
		t.Errorf("business calls = %d, want 2 (detail + status)", business.count())
	}
}

// TestContract_ListInstances_Filters 校验：列表接口 GET /v1/clouds 透传分页与 area/node 过滤器，
// 且能解析裸数组形态的分页响应。
func TestContract_ListInstances_Filters(t *testing.T) {
	var gotQuery string
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/login" && r.URL.Path != "/v1/clouds" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/v1/login" {
			_, _ = w.Write([]byte("token"))
			return
		}
		gotQuery = r.URL.RawQuery
		// 裸数组形态（非 {data:[...]}）
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "1", "hostname": "a", "mainip": "10.0.0.1", "status": "on"},
			{"id": "2", "hostname": "b", "mainip": "10.0.0.2", "status": "off"},
		})
	})

	list, err := p.ListInstances(context.Background(), map[string]string{
		"per_page": "50", "page": "2", "area": "hk", "node": "n1",
	})
	if err != nil {
		t.Fatalf("ListInstances failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d instances, want 2", len(list))
	}
	for _, want := range []string{"per_page=50", "page=2", "area=hk", "node=n1"} {
		if !contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}

// apiCalls 简单计数辅助。
type apiCalls struct{ n int64 }

func (a *apiCalls) add(v string) { atomic.AddInt64(&a.n, 1); _ = v }
func (a *apiCalls) count() int   { return int(atomic.LoadInt64(&a.n)) }

func contains(s, sub string) bool {
	if s == "" || sub == "" {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
