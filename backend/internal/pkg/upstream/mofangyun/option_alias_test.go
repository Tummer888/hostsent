package mofangyun

import (
	"net/url"
	"strconv"
	"testing"
	"time"
)

// TestApplyOptionAliasesDocKeys 校验文档口径键名被翻译成 /clouds 实际写键。
// 官方参数表给运营看的键与平台接口接收的字段不一致，翻译错了就是"配置项填了没生效"。
func TestApplyOptionAliasesDocKeys(t *testing.T) {
	opts := map[string]string{
		"flow_way":             "out",
		"flow_limit":           "2048",
		"resource_package":     "7",
		"IP_MACBond":           "0",
		"traffic_bill_type":    "last_30days",
		"system_disk_io_limit": "500,600,2000,2400",
		"data_disk_io_limit":   "0,0,0,0",
	}
	applyOptionAliases(opts)

	cases := map[string]string{
		"traffic_type":           "2",    // flow_way=out → 出方向
		"traffic_quota":          "2048", // flow_limit → traffic_quota
		"rid":                    "7",    // resource_package → rid
		"bind_mac":               "0",    // IP_MACBond → bind_mac
		"system_read_bytes_sec":  "500",
		"system_write_bytes_sec": "600",
		"system_read_iops_sec":   "2000",
		"system_write_iops_sec":  "2400",
		"data_read_bytes_sec":    "0",
		"data_write_iops_sec":    "0",
	}
	for key, want := range cases {
		if got := opts[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	// last_30days 应落 reset_flow_day = 当天日。
	if got := opts["reset_flow_day"]; got != itoa(time.Now().Day()) {
		t.Errorf("reset_flow_day = %q, want %q", got, itoa(time.Now().Day()))
	}
	// 只用于翻译的文档键必须被清掉：flow_way 与 traffic_type 同在云 option 白名单里，
	// 留着会同一语义下发两次。
	for _, key := range []string{"flow_way", "flow_limit", "IP_MACBond", "resource_package",
		"traffic_bill_type", "system_disk_io_limit", "data_disk_io_limit"} {
		if _, ok := opts[key]; ok {
			t.Errorf("文档键 %s 应被清理，实际仍在", key)
		}
	}
}

// TestApplyOptionAliasesNaturalMonth 自然月计费不传 reset_flow_day（平台默认每月 1 日清零）。
func TestApplyOptionAliasesNaturalMonth(t *testing.T) {
	opts := map[string]string{"traffic_bill_type": "month"}
	applyOptionAliases(opts)
	if _, ok := opts["reset_flow_day"]; ok {
		t.Errorf("month 计费不应写 reset_flow_day")
	}
}

// TestApplyOptionAliasesBandwidth 带宽键展开。
//
// 实测教训：只传文档口径的 `bw` 建出来的云主机 in_bw/out_bw 都是 0（面板带宽组为空），
// 平台 /clouds 读的是 in_bw/out_bw 两个方向字段，官方财务插件也是把 bw 同时写进这两列。
func TestApplyOptionAliasesBandwidth(t *testing.T) {
	// 只给 bw：两个方向都补上。
	opts := map[string]string{"bw": "20"}
	applyOptionAliases(opts)
	if opts["in_bw"] != "20" || opts["out_bw"] != "20" {
		t.Errorf("bw=20 应展开为 in_bw=out_bw=20，实际 in=%q out=%q", opts["in_bw"], opts["out_bw"])
	}

	// 显式进带宽优先，出带宽回落 bw。
	opts2 := map[string]string{"bw": "20", "in_bw": "100"}
	applyOptionAliases(opts2)
	if opts2["in_bw"] != "100" {
		t.Errorf("显式 in_bw 应优先，实际 %q", opts2["in_bw"])
	}
	if opts2["out_bw"] != "20" {
		t.Errorf("out_bw 应回落 bw=20，实际 %q", opts2["out_bw"])
	}

	// 只给 in_bw：补出带宽，避免上行带宽落成面板默认 0。
	opts3 := map[string]string{"in_bw": "50"}
	applyOptionAliases(opts3)
	if opts3["out_bw"] != "50" {
		t.Errorf("只给 in_bw 时应补 out_bw，实际 %q", opts3["out_bw"])
	}
}

// TestApplyOptionAliasesExplicitWins 显式给了平台写键时不被文档键覆盖。
func TestApplyOptionAliasesExplicitWins(t *testing.T) {
	opts := map[string]string{"traffic_type": "1", "flow_way": "all", "rid": "9", "resource_package": "7"}
	applyOptionAliases(opts)
	if opts["traffic_type"] != "1" {
		t.Errorf("显式 traffic_type 应保留，实际 %q", opts["traffic_type"])
	}
	if opts["rid"] != "9" {
		t.Errorf("显式 rid 应保留，实际 %q", opts["rid"])
	}
}

// TestResolveSystemDisk 系统盘三种写法与 OS 分支。
func TestResolveSystemDisk(t *testing.T) {
	cases := []struct {
		raw     string
		windows bool
		size    string
		store   string
	}{
		{"30,1", false, "30", "1"},            // 单值 + 存储
		{"30", false, "30", ""},               // 单值
		{"lin:30,win:50", false, "30", ""},    // Linux 取 lin
		{"lin:30,win:50", true, "50", ""},     // Windows 取 win
		{"lin:30,win:50,1", true, "50", "1"},  // Windows + 存储
		{"lin:30,win:50,2", false, "30", "2"}, // Linux + 存储
		{" 40 ,  3 ", false, "40", "3"},       // 容忍空格
	}
	for _, c := range cases {
		size, store := resolveSystemDisk(c.raw, c.windows)
		if size != c.size || store != c.store {
			t.Errorf("resolveSystemDisk(%q, win=%v) = (%q,%q), want (%q,%q)",
				c.raw, c.windows, size, store, c.size, c.store)
		}
	}
}

// TestSystemDiskWindowsPicksWin 端到端：os_name 是 Windows 时系统盘取 win 值。
func TestSystemDiskWindowsPicksWin(t *testing.T) {
	opts := map[string]string{"system_disk_size": "lin:30,win:60,5", "os_name": "Windows2022.qcow2"}
	applyOptionAliases(opts)
	if opts["system_disk_size"] != "60" {
		t.Errorf("Windows 系统盘应为 60，实际 %q", opts["system_disk_size"])
	}
	if opts["store"] != "5" {
		t.Errorf("存储应为 5，实际 %q", opts["store"])
	}
}

// TestDataDiskSplit 数据盘 "60,3" 拆成 size + store（供 other_data_disk 展开）。
func TestDataDiskSplit(t *testing.T) {
	opts := map[string]string{"data_disk_size": "60,3"}
	applyOptionAliases(opts)
	if opts["data_disk_size"] != "60" {
		t.Errorf("data_disk_size = %q, want 60", opts["data_disk_size"])
	}
	if opts["data_disk_store"] != "3" {
		t.Errorf("data_disk_store = %q, want 3", opts["data_disk_store"])
	}
}

// TestExpandDataDiskSkipsZero 数据盘为 0 时不展开（无数据盘）。
func TestExpandDataDiskSkipsZero(t *testing.T) {
	form := url.Values{}
	expandDataDisk(form, map[string]string{"data_disk_size": "0"})
	if len(form) != 0 {
		t.Errorf("size=0 不应写 other_data_disk，实际 %v", form)
	}
	form2 := url.Values{}
	expandDataDisk(form2, map[string]string{"data_disk_size": "60", "data_disk_store": "3"})
	if form2.Get("other_data_disk[0][size]") != "60" || form2.Get("other_data_disk[0][store]") != "3" {
		t.Errorf("展开结果不对：%v", form2)
	}
}

// TestExpandIOLimitBadFormat 段数不对时不猜测，原键不动（交给上游报错）。
func TestExpandIOLimitBadFormat(t *testing.T) {
	opts := map[string]string{"system_disk_io_limit": "500,500"}
	applyOptionAliases(opts)
	if opts["system_disk_io_limit"] != "500,500" {
		t.Errorf("格式不符应保留原值，实际 %q", opts["system_disk_io_limit"])
	}
}

func itoa(v int) string { return strconv.Itoa(v) }
