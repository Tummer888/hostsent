package oauth

import "testing"

// JSONP 剥壳：QQ 的 /oauth2.0/me 返回 `callback( {...} );`，且外壳格式不稳定
// （有无分号、有无空格都可能），必须容错。
func TestParseJSONP(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"标准带分号", `callback( {"openid":"OID-1","unionid":"UID-1"} );`, "OID-1"},
		{"无空格", `callback({"openid":"OID-2"});`, "OID-2"},
		{"无分号", `callback( {"openid":"OID-3"} )`, "OID-3"},
		{"纯 JSON 兜底", `{"openid":"OID-4"}`, "OID-4"},
		{"多行", "callback(\n{\"openid\":\"OID-5\"}\n);", "OID-5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out struct {
				OpenID string `json:"openid"`
			}
			if err := ParseJSONP([]byte(tc.body), &out); err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if out.OpenID != tc.want {
				t.Fatalf("openid 应为 %s，实际 %s", tc.want, out.OpenID)
			}
		})
	}

	for _, bad := range []string{"", "   ", "callback();", "error=100016"} {
		var out map[string]any
		if err := ParseJSONP([]byte(bad), &out); err == nil {
			t.Fatalf("不可识别的响应 %q 必须报错", bad)
		}
	}
}

// 嵌套对象里的花括号不能影响剥壳（取最外层大括号范围）。
func TestParseJSONP_NestedObject(t *testing.T) {
	var out struct {
		OpenID string `json:"openid"`
		Extra  struct {
			N int `json:"n"`
		} `json:"extra"`
	}
	if err := ParseJSONP([]byte(`callback( {"openid":"OID","extra":{"n":7}} );`), &out); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if out.OpenID != "OID" || out.Extra.N != 7 {
		t.Fatalf("嵌套字段解析不符: %+v", out)
	}
}

// 微信/QQ 的 token 接口正常与出错时都返回 query string，必须能解且做 URL 解码。
func TestParseQueryString(t *testing.T) {
	kv := ParseQueryString([]byte("access_token=AT-1&expires_in=7200&refresh_token=RT%2B1%3D"))
	if kv["access_token"] != "AT-1" {
		t.Fatalf("access_token 解析错误: %q", kv["access_token"])
	}
	if kv["expires_in"] != "7200" {
		t.Fatalf("expires_in 解析错误: %q", kv["expires_in"])
	}
	if kv["refresh_token"] != "RT+1=" {
		t.Fatalf("值必须 URL 解码，实际 %q", kv["refresh_token"])
	}

	errKV := ParseQueryString([]byte("error=100016&error_description=access+token+expired"))
	if errKV["error"] != "100016" {
		t.Fatalf("error 解析错误: %q", errKV["error"])
	}
	if errKV["error_description"] != "access token expired" {
		t.Fatalf("error_description 应解码 + 为空格，实际 %q", errKV["error_description"])
	}

	if got := ParseQueryString([]byte("   ")); len(got) != 0 {
		t.Fatalf("空响应应返回空 map，实际 %v", got)
	}
}
