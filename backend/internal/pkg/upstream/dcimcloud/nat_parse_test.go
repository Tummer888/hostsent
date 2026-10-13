package dcimcloud

import (
	"encoding/json"
	"testing"
)

// TestNatAclResponseParse NAT 端口转发响应解析。
//
// 关键结构：nat_host_ip 在**外层 data 上**，不在列表项里——面板模板渲染
// {$nat_host_ip}:{$vo.ext_port} 就是这个形状。把 IP 解析错（比如当成列表项字段）
// 会得到空地址，客户拿到一台连不上的机器。
//
// 用例里的报文按面板实测口径构造；id 两种类型都覆盖（面板其它接口已实测把 id
// 返回成字符串，这里两种都可能）。
func TestNatAclResponseParse(t *testing.T) {
	raw := `{
	  "data": {
	    "nat_host_ip": "203.0.113.7",
	    "data": [
	      {"id": 12, "name": "ssh", "ext_port": 20022, "int_port": 22, "protocol": 1},
	      {"id": "13", "name": "web", "ext_port": 20080, "int_port": 80, "protocol": 3}
	    ]
	  }
	}`
	var resp natAclResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if resp.Data.NatHostIP != "203.0.113.7" {
		t.Fatalf("nat_host_ip 应在 data 外层，实际 %q", resp.Data.NatHostIP)
	}
	if len(resp.Data.Data) != 2 {
		t.Fatalf("期望 2 条规则，实际 %d", len(resp.Data.Data))
	}
	// 数字 id 与字符串 id 都要正确取出。
	if got := resp.Data.Data[0].ID.String(); got != "12" {
		t.Errorf("数字 id 解析错误：%q", got)
	}
	if got := resp.Data.Data[1].ID.String(); got != "13" {
		t.Errorf("字符串 id 解析错误：%q", got)
	}
}

// TestNatWebResponseParse 共享建站响应解析（无 protocol 字段）。
func TestNatWebResponseParse(t *testing.T) {
	raw := `{
	  "data": {
	    "nat_host_ip": "203.0.113.7",
	    "data": [{"id": "5", "domain": "abc.example.com", "ext_port": 20080, "int_port": 80}]
	  }
	}`
	var resp natWebResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if resp.Data.NatHostIP != "203.0.113.7" {
		t.Fatalf("nat_host_ip 错误：%q", resp.Data.NatHostIP)
	}
	if len(resp.Data.Data) != 1 || resp.Data.Data[0].Domain != "abc.example.com" {
		t.Fatalf("域名解析错误：%+v", resp.Data.Data)
	}
}

// TestNatResponseNullData 无规则时面板返回 data=null（VPS 实例实测如此）。
// 必须解析成空列表而不是报错——"没有规则"是正常状态。
func TestNatResponseNullData(t *testing.T) {
	for _, body := range []string{`null`, `{"data":null}`, `{"data":{"data":null}}`} {
		var resp natAclResponse
		if err := json.Unmarshal([]byte(body), &resp); err != nil {
			t.Fatalf("%s 应能解析为空，实际报错 %v", body, err)
		}
		if resp.Data.NatHostIP != "" {
			t.Errorf("%s 的 nat_host_ip 应为空", body)
		}
	}
}

// TestNatProtocolText 协议枚举归一：面板 1=tcp / 2=udp / 其它=tcp,udp。
func TestNatProtocolText(t *testing.T) {
	cases := map[int]string{1: "tcp", 2: "udp", 3: "tcp,udp", 0: "tcp,udp", 9: "tcp,udp"}
	for code, want := range cases {
		if got := natProtocolText(code); got != want {
			t.Errorf("protocol=%d 期望 %q，实际 %q", code, want, got)
		}
	}
}

// TestFormatNatForward 地址拼接：HostIP 或端口缺失时不能拼出半截地址。
//
// 面板返回空 nat_host_ip（或端口为 0）时，拼成 ":0" / "1.2.3.4:0" 会让客户
// 以为拿到地址但连不上，不如返回空让上层显示"暂未分配"。
func TestFormatNatForward(t *testing.T) {
	cases := []struct {
		hostIP string
		port   int
		want   string
	}{
		{hostIP: "203.0.113.7", port: 20022, want: "203.0.113.7:20022"},
		{hostIP: "", port: 20022, want: ""},
		{hostIP: "203.0.113.7", port: 0, want: ""},
		{hostIP: "", port: 0, want: ""},
		{hostIP: "203.0.113.7", port: -1, want: ""},
	}
	for _, tc := range cases {
		if got := FormatNatForward(tc.hostIP, tc.port); got != tc.want {
			t.Errorf("FormatNatForward(%q,%d) 期望 %q，实际 %q", tc.hostIP, tc.port, tc.want, got)
		}
	}
}
