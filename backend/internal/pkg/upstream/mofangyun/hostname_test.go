package mofangyun

import (
	"testing"

	pkgmodel "hostsent/backend/internal/pkg/model"
)

// TestBuildInstanceMapsReadFields 校验读取侧字段映射：面板读字段名与写字段名不对称
// （cpu/memory 用 GB，disk 在 disk[] 里，带宽在 bw_group[] 里），映射漏掉会让
// 实例列表与对账页的规格列全空。
func TestBuildInstanceMapsReadFields(t *testing.T) {
	detail := map[string]interface{}{
		"id":        "12",
		"hostname":  "hs-abc123",
		"status":    "on",
		"cpu":       4,
		"memory":    8,
		"area_name": "默认",
		"node_name": "节点一",
		"os_name":   "CentOS-7.9.2111-x64.qcow2",
		"ip":        []interface{}{map[string]interface{}{"ip": "1.2.3.4"}},
		"disk": []interface{}{
			map[string]interface{}{"type": "data", "size": 100, "fs_type": "xfs"},
			map[string]interface{}{"type": "system", "size": 30, "fs_type": "qcow2"},
		},
		"bw_group": []interface{}{map[string]interface{}{"in_bw": 10, "out_bw": 5}},
	}
	inst := buildInstance(detail)
	if inst.UpstreamID != "12" || inst.Name != "hs-abc123" {
		t.Errorf("基础字段映射错误：id=%q name=%q", inst.UpstreamID, inst.Name)
	}
	if inst.Status != pkgmodel.InstanceStatusRunning {
		t.Errorf("status=on 应映射为 running，得到 %q", inst.Status)
	}
	if inst.Specs.CPU != 4 {
		t.Errorf("cpu=%d，want 4", inst.Specs.CPU)
	}
	// 面板 memory 是 GB，标准规格统一 MB。
	if inst.Specs.Memory != 8*1024 {
		t.Errorf("memory=%d，want %d（GB→MB 换算）", inst.Specs.Memory, 8*1024)
	}
	if inst.Specs.Disk != 30 || inst.Specs.DiskType != "qcow2" {
		t.Errorf("系统盘应取 disk[] 中 type=system 的项：size=%d type=%q", inst.Specs.Disk, inst.Specs.DiskType)
	}
	if inst.Specs.Bandwidth != 10 {
		t.Errorf("bandwidth=%d，want 10（取 bw_group[0].in_bw）", inst.Specs.Bandwidth)
	}
	if inst.Specs.OS != "CentOS-7.9.2111-x64" {
		t.Errorf("os=%q，want 去掉 .qcow2 后缀", inst.Specs.OS)
	}
	if inst.Region != "默认" || inst.Zone != "节点一" {
		t.Errorf("region/zone 映射错误：%q/%q", inst.Region, inst.Zone)
	}
	if inst.PublicIP != "1.2.3.4" {
		t.Errorf("public_ip=%q，want 从 ip[] 回退取到 1.2.3.4", inst.PublicIP)
	}
}

// TestBuildInstanceEmptyDetail 空详情不该 panic，也不该产出伪造规格。
func TestBuildInstanceEmptyDetail(t *testing.T) {
	inst := buildInstance(map[string]interface{}{})
	if inst.UpstreamID != "" || inst.Specs.CPU != 0 || inst.Specs.Disk != 0 {
		t.Errorf("空详情应产出空实例，得到 %+v", inst)
	}
}

// TestSanitizeHostname 面板对主机名有强校验（字母开头 + 只允许字母数字/_-. + ≥6 位）：
// 中文商品名直接下发会被 400 拒绝，开通静默失败，因此必须有可预期的降级规则。
func TestSanitizeHostname(t *testing.T) {
	valid := []string{"hs-123456", "MyHost-01", "abc123", "a-b_c.d.1"}
	for _, in := range valid {
		if got := sanitizeHostname(in); got != in {
			t.Errorf("合法主机名不该被改写：%q → %q", in, got)
		}
	}
	invalid := []string{"", "abc", "魔方云测试 2核4G 30G", "1abcde", "_abcde", "a b c d e", "中文名"}
	for _, in := range invalid {
		got := sanitizeHostname(in)
		if !isValidHostname(got) {
			t.Errorf("非法主机名 %q 降级后仍不合法：%q", in, got)
		}
		if len(got) < 6 {
			t.Errorf("降级结果过短：%q → %q", in, got)
		}
	}
}

// TestRawIDString 面板 id 混用字符串与数字（登录返回 token、创建返回字符串 id），
// 统一转换是防止"开通成功但解析报错"的关键。
func TestRawIDString(t *testing.T) {
	cases := map[string]string{
		`"12"`:  `12`,
		`12`:    `12`,
		`"0"`:   `0`,
		`null`:  ``,
		``:      ``,
		`"abc"`: `abc`,
	}
	for in, want := range cases {
		if got := rawIDString([]byte(in)); got != want {
			t.Errorf("rawIDString(%s) = %q, want %q", in, got, want)
		}
	}
}

// TestRandPasswordPolicy 面板密码策略要求同时含大小写字母与数字（实测 400 报错文案）。
func TestRandPasswordPolicy(t *testing.T) {
	for i := 0; i < 50; i++ {
		pw := randPassword(12)
		if len(pw) < 8 {
			t.Fatalf("密码过短：%q", pw)
		}
		var hasLower, hasUpper, hasDigit bool
		for _, r := range pw {
			switch {
			case r >= 'a' && r <= 'z':
				hasLower = true
			case r >= 'A' && r <= 'Z':
				hasUpper = true
			case r >= '0' && r <= '9':
				hasDigit = true
			}
		}
		if !hasLower || !hasUpper || !hasDigit {
			t.Fatalf("密码不满足策略：%q（lower=%v upper=%v digit=%v）", pw, hasLower, hasUpper, hasDigit)
		}
	}
}

// TestPlatformImageStatus 镜像可用性取决于节点：仅顶层 status=1 不代表节点上已下载。
func TestPlatformImageStatus(t *testing.T) {
	if got := platformImageStatus(1, 1); got != "active" {
		t.Errorf("顶层与节点都可用应为 active，得到 %q", got)
	}
	if got := platformImageStatus(1, 0); got != "offline" {
		t.Errorf("节点未下载应为 offline，得到 %q", got)
	}
	if got := platformImageStatus(0, 1); got != "offline" {
		t.Errorf("顶层停用应为 offline，得到 %q", got)
	}
}
