package netutil

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newIPProbe 构造一个只回显 ClientIP 的路由，按给定可信代理集合配置 gin。
func newIPProbe(t *testing.T, trusted []string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := r.SetTrustedProxies(trusted); err != nil {
		t.Fatalf("SetTrustedProxies(%v): %v", trusted, err)
	}
	r.GET("/ip", func(c *gin.Context) {
		c.String(http.StatusOK, ClientIP(c))
	})
	return r
}

func probeIP(t *testing.T, r *gin.Engine, remoteAddr string, headers map[string]string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Body.String()
}

// TestClientIPIgnoresForwardedHeadersFromUntrustedPeer 是这次 IP 修复的核心回归：
// 直连对端不在 trusted_proxies 里时，任何 X-Forwarded-For / X-Real-IP 都必须被忽略，
// 结果只能是对端地址本身。
//
// 旧实现无条件信任这三个头部，实测发 `X-Forwarded-For: 203.0.113.99` 就能让登录
// 日志记下任意 IP；同一个值还是 login_guard 的 failIPKey，等于可以直接绕过
// IP 维度的撞库锁定（doc91 §9.1）。所以这条断言不能只当「日志是否好看」来看。
func TestClientIPIgnoresForwardedHeadersFromUntrustedPeer(t *testing.T) {
	// 只信回环；请求来自 198.51.100.9（不可信）。
	r := newIPProbe(t, []string{"127.0.0.1/8", "::1"})

	cases := []struct {
		name    string
		headers map[string]string
	}{
		{"X-Forwarded-For 单值", map[string]string{"X-Forwarded-For": "203.0.113.99"}},
		{"X-Forwarded-For 链", map[string]string{"X-Forwarded-For": "203.0.113.99, 198.51.100.9"}},
		{"X-Real-IP", map[string]string{"X-Real-IP": "203.0.113.99"}},
		{"CF-Connecting-IP", map[string]string{"CF-Connecting-IP": "203.0.113.99"}},
		{"多个头部同时伪造", map[string]string{
			"X-Forwarded-For":  "203.0.113.99",
			"X-Real-IP":        "203.0.113.98",
			"CF-Connecting-IP": "203.0.113.97",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := probeIP(t, r, "198.51.100.9:45678", tc.headers)
			if got != "198.51.100.9" {
				t.Fatalf("不可信对端伪造头部后 ClientIP=%q，期望对端地址 198.51.100.9", got)
			}
		})
	}
}

// TestClientIPHonoursForwardedHeadersFromTrustedPeer 覆盖正常反代链路：
// 对端可信时，XFF 链里最右侧的不可信地址才是客户端。
func TestClientIPHonoursForwardedHeadersFromTrustedPeer(t *testing.T) {
	r := newIPProbe(t, []string{"127.0.0.1/8", "::1", "172.16.0.0/12"})

	cases := []struct {
		name       string
		remoteAddr string
		xff        string
		want       string
	}{
		{"可信对端 + 单个客户端", "127.0.0.1:5000", "203.0.113.99", "203.0.113.99"},
		{"可信对端 + 全部可信跳", "127.0.0.1:5000", "172.18.0.5, 172.18.0.1", "172.18.0.5"},
		{"可信对端 + 末跳不可信", "172.18.0.1:5000", "203.0.113.99, 172.18.0.7", "203.0.113.99"},
		{"可信对端但无 XFF", "127.0.0.1:5000", "", "127.0.0.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			headers := map[string]string{}
			if tc.xff != "" {
				headers["X-Forwarded-For"] = tc.xff
			}
			got := probeIP(t, r, tc.remoteAddr, headers)
			if got != tc.want {
				t.Fatalf("ClientIP=%q，期望 %q", got, tc.want)
			}
		})
	}
}

// TestClientIPWithoutTrustedProxiesFallsBackToPeer 锁住「默认保守」这条约束：
// 未声明任何可信代理时，头部一律不采信。config.setDefaults 的默认值就是
// 只有回环，因此这个用例同时是配置默认值的守卫。
func TestClientIPWithoutTrustedProxiesFallsBackToPeer(t *testing.T) {
	r := newIPProbe(t, nil)
	got := probeIP(t, r, "10.1.2.3:5000", map[string]string{"X-Forwarded-For": "203.0.113.99"})
	if got != "10.1.2.3" {
		t.Fatalf("无可信代理配置时 ClientIP=%q，期望对端地址 10.1.2.3", got)
	}
}

// TestClientIPWithoutPortInRemoteAddr 覆盖 ClientIP 的兜底分支：
// RemoteAddr 不是 host:port 形式时也不能退回请求头。
func TestClientIPWithoutPortInRemoteAddr(t *testing.T) {
	r := newIPProbe(t, []string{"127.0.0.1/8"})
	got := probeIP(t, r, "198.51.100.9", map[string]string{"X-Forwarded-For": "203.0.113.99"})
	if got != "198.51.100.9" {
		t.Fatalf("ClientIP=%q，期望 198.51.100.9", got)
	}
}

// TestClientIPNilContext 返回空串而不是 panic（handler 层防御性调用）。
func TestClientIPNilContext(t *testing.T) {
	if got := ClientIP(nil); got != "" {
		t.Fatalf("nil context 下 ClientIP=%q，期望空串", got)
	}
}
