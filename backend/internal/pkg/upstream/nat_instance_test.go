package upstream

import "testing"

// TestIsNatInstance NAT 机型判定（doc122）。
//
// 判定错的方向有两种，都有实际损失：
//   - 把 NAT 判成独立 IP：前端不给转发地址，客户拿到机器连不上；
//   - 把独立 IP 判成 NAT：前端显示"没有独立 IP"，客户以为买错了。
//
// 因此这里把各种 raw_data 形态都钉住，尤其是 ip_num=0 这个"0 是有效值"的判据
// （用真值判断会把 0 当成缺失，直接判错）。
func TestIsNatInstance(t *testing.T) {
	cases := []struct {
		name    string
		rawData string
		want    bool
	}{
		{
			name:    "is_nat_host=true 最权威",
			rawData: `{"is_nat_host":true}`,
			want:    true,
		},
		{
			name:    "is_nat_host=false 且无转发能力 → 独立 IP",
			rawData: `{"is_nat_host":false,"nat_acl_limit":-1,"nat_web_limit":-1,"ip_num":1}`,
			want:    false,
		},
		{
			// 面板实测的独立 IP 实例形态：is_nat_host=false、nat_*=-1、ip_num=0（历史数据）。
			name:    "独立 IP 实例（实测形态）",
			rawData: `{"is_nat_host":false,"ip_num":0,"nat_acl_limit":-1,"nat_web_limit":-1}`,
			want:    false,
		},
		{
			name:    "nat_acl_limit=0（开转发）→ NAT",
			rawData: `{"nat_acl_limit":0,"nat_web_limit":-1}`,
			want:    true,
		},
		{
			name:    "nat_web_limit=0（共享建站）→ NAT",
			rawData: `{"nat_acl_limit":-1,"nat_web_limit":0}`,
			want:    true,
		},
		{
			name:    "无 is_nat_host，靠 ip_num=0 兜底",
			rawData: `{"ip_num":0}`,
			want:    true,
		},
		{
			name:    "ip_num=0 是字符串也要认（0 是有效值，不能真值判断）",
			rawData: `{"ip_num":"0"}`,
			want:    true,
		},
		{
			name:    "ip_num=1 → 独立 IP",
			rawData: `{"ip_num":1}`,
			want:    false,
		},
		{
			name:    "is_nat_host 为字符串 true",
			rawData: `{"is_nat_host":"true"}`,
			want:    true,
		},
		{
			name:    "is_nat_host 为字符串 false 且无转发 → 独立 IP",
			rawData: `{"is_nat_host":"false","nat_acl_limit":-1}`,
			want:    false,
		},
		{
			name:    "nat_* 为 -1 且 ip_num 缺失 → 非 NAT（不该猜）",
			rawData: `{"nat_acl_limit":-1,"nat_web_limit":-1}`,
			want:    false,
		},
		{
			name:    "空 raw_data",
			rawData: ``,
			want:    false,
		},
		{
			name:    "JSON null",
			rawData: `null`,
			want:    false,
		},
		{
			name:    "非法 JSON 不 panic",
			rawData: `{not json`,
			want:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsNatInstance(tc.rawData); got != tc.want {
				t.Fatalf("IsNatInstance(%s) = %v，期望 %v", tc.rawData, got, tc.want)
			}
		})
	}
}

// TestNatLimitsFromRaw 转发能力上限读取。
func TestNatLimitsFromRaw(t *testing.T) {
	acl, web, ok := NatLimitsFromRaw(`{"nat_acl_limit":0,"nat_web_limit":10}`)
	if !ok || acl != 0 || web != 10 {
		t.Fatalf("期望 (0,10,true)，实际 (%d,%d,%v)", acl, web, ok)
	}
	acl, web, ok = NatLimitsFromRaw(`{"nat_acl_limit":-1,"nat_web_limit":-1}`)
	if !ok || acl != -1 || web != -1 {
		t.Fatalf("期望 (-1,-1,true)，实际 (%d,%d,%v)", acl, web, ok)
	}
	if _, _, ok := NatLimitsFromRaw(`{"ip_num":0}`); ok {
		t.Fatal("无 nat_* 字段时应返回 ok=false")
	}
}

// TestFormatNatForwardAddr 地址拼接：任一侧缺失都不产出半截地址。
func TestFormatNatForwardAddr(t *testing.T) {
	if got := FormatNatForwardAddr("1.2.3.4", 20022); got != "1.2.3.4:20022" {
		t.Fatalf("正常拼接错误：%q", got)
	}
	for _, tc := range []struct {
		ip   string
		port int
	}{
		{"", 20022}, {"1.2.3.4", 0}, {"1.2.3.4", -1}, {"", 0},
	} {
		if got := FormatNatForwardAddr(tc.ip, tc.port); got != "" {
			t.Errorf("FormatNatForwardAddr(%q,%d) 应返回空，实际 %q", tc.ip, tc.port, got)
		}
	}
}
