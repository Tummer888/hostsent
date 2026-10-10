package mofangyun

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

// 文档口径配置项键 → /clouds 实际写键的翻译。
//
// 官方参数表（docs/魔方云对接魔方财务.html §五）里给运营看的键名与面板接口
// 真正接收的字段并不一致：表里是 IP_MACBond / resource_package / traffic_bill_type /
// flow_way / flow_limit / system_disk_io_limit / data_disk_io_limit / data_disk_size，
// 而 /clouds 收的是 bind_mac / rid / reset_flow_day / traffic_type / traffic_quota /
// system_read_* 四件套 / data_read_* 四件套 / other_data_disk 数组。
//
// 保留文档键名（运营照着官方文档填，不用记平台内部叫法），由这里翻译。
// 翻译是幂等的：目标键已显式给出时不覆盖（调用方显式传平台写键优先）。
func applyOptionAliases(opts map[string]string) {
	// 带宽：文档口径的 `bw` 在 /clouds 上**不被接收**（实测：只传 bw=10 建出来的
	// 云主机 in_bw/out_bw 均为 0，面板带宽组是空值）。平台真正读的是 in_bw/out_bw
	// 两个方向，官方财务插件也是把 bw 同时写进这两个字段。因此这里双向展开：
	//   bw=10        → in_bw=10, out_bw=10
	//   bw=10,in_bw=100 → in_bw=100（显式进带宽优先）, out_bw=10
	if v := strings.TrimSpace(opts["bw"]); v != "" {
		if opts["in_bw"] == "" {
			opts["in_bw"] = v
		}
		if opts["out_bw"] == "" {
			opts["out_bw"] = v
		}
	}
	// 只给了进带宽时补出带宽，避免上行带宽落成面板默认 0。
	if v := strings.TrimSpace(opts["in_bw"]); v != "" && opts["out_bw"] == "" {
		opts["out_bw"] = v
	}

	// 流量方向 → traffic_type（1 进 / 2 出 / 3 进出汇总）。
	if opts["traffic_type"] == "" && opts["flow_way"] != "" {
		switch strings.ToLower(opts["flow_way"]) {
		case "in":
			opts["traffic_type"] = "1"
		case "out":
			opts["traffic_type"] = "2"
		default:
			opts["traffic_type"] = "3"
		}
	}
	// 流量 → traffic_quota（单位 GB，原样透传）。
	if opts["traffic_quota"] == "" && opts["flow_limit"] != "" {
		opts["traffic_quota"] = opts["flow_limit"]
	}

	// 资源包 → rid。
	if opts["rid"] == "" && opts["resource_package"] != "" {
		opts["rid"] = opts["resource_package"]
	}

	// IP-MAC 绑定 → bind_mac（0 关 / 1 开）。
	if opts["bind_mac"] == "" && opts["IP_MACBond"] != "" {
		opts["bind_mac"] = opts["IP_MACBond"]
	}

	// 流量计费周期 → reset_flow_day：选「订购日至下月」时把清零日设为开通当日。
	// 自然月（month）不传 reset_flow_day，平台默认每月 1 日清零。
	if opts["traffic_bill_type"] == "last_30days" && opts["reset_flow_day"] == "" {
		opts["reset_flow_day"] = strconv.Itoa(time.Now().Day())
	}

	// 磁盘性能四段值 a,b,c,d = 读MB/s,写MB/s,读IOPS,写IOPS → 4 个平台参数。
	// 段数不对时不删除原键：宁可留下一个平台不认识的值被上游拒绝，
	// 也好过静默丢弃运营填错的性能限制（面板上看不出任何异常）。
	ioConsumed := []string{}
	if expandIOLimit(opts, "system_disk_io_limit", "system_read_bytes_sec", "system_write_bytes_sec", "system_read_iops_sec", "system_write_iops_sec") {
		ioConsumed = append(ioConsumed, "system_disk_io_limit")
	}
	if expandIOLimit(opts, "data_disk_io_limit", "data_read_bytes_sec", "data_write_bytes_sec", "data_read_iops_sec", "data_write_iops_sec") {
		ioConsumed = append(ioConsumed, "data_disk_io_limit")
	}

	// 系统盘 "lin:30,win:50[,store]" / "30[,store]" → system_disk_size + store。
	// 按操作系统选值：Windows 镜像走 win，其余走 lin。
	if raw := strings.TrimSpace(opts["system_disk_size"]); raw != "" {
		size, store := resolveSystemDisk(raw, isWindowsOS(opts))
		if size != "" {
			opts["system_disk_size"] = size
		}
		if store != "" && opts["store"] == "" {
			opts["store"] = store
		}
	}

	// 数据盘 "30[,store]" → 归一为 size，store 另存供展开。
	if raw := strings.TrimSpace(opts["data_disk_size"]); raw != "" {
		size, store := splitSizeStore(raw)
		if size != "" {
			opts["data_disk_size"] = size
		}
		if store != "" {
			opts["data_disk_store"] = store
		}
	}

	// 清掉只用于翻译的文档键：让平台写键保持唯一，避免同一语义下发两次
	// （cloudOptionKeys 同时含 flow_way 与 traffic_type，留着会重复下发）。
	for _, key := range append([]string{
		"flow_way", "flow_limit", "IP_MACBond", "resource_package", "traffic_bill_type",
	}, ioConsumed...) {
		delete(opts, key)
	}
}

// expandIOLimit 把 "a,b,c,d" 展开成四个平台参数；目标键已存在时不覆盖。
// 值的顺序与官方文档一致：读MB/s, 写MB/s, 读IOPS, 写IOPS。
// 返回 true 表示已成功展开（原键可安全删除）；段数不符返回 false 并保留原键。
func expandIOLimit(opts map[string]string, src, readBW, writeBW, readIOPS, writeIOPS string) bool {
	raw := strings.TrimSpace(opts[src])
	if raw == "" {
		return false
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return false
	}
	targets := []string{readBW, writeBW, readIOPS, writeIOPS}
	for i, key := range targets {
		v := strings.TrimSpace(parts[i])
		if v != "" && opts[key] == "" {
			opts[key] = v
		}
	}
	return true
}

// resolveSystemDisk 解析系统盘取值：
//
//	"30,1"          → size=30, store=1
//	"lin:30,win:50,1" → 按 OS 选 30 或 50，store=1
//	"lin:30,win:50"   → 按 OS 选，store 空
//	"50"              → size=50
func resolveSystemDisk(raw string, windows bool) (size, store string) {
	linSet, winSet := "", ""
	for _, seg := range strings.Split(raw, ",") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		switch {
		case strings.HasPrefix(seg, "lin:"):
			linSet = strings.TrimPrefix(seg, "lin:")
		case strings.HasPrefix(seg, "win:"):
			winSet = strings.TrimPrefix(seg, "win:")
		default:
			// 无前缀：单值即 size；若已出现 lin/win 则视为 store。
			if linSet == "" && winSet == "" && size == "" {
				size = seg
			} else if store == "" {
				store = seg
			}
		}
	}
	if linSet != "" || winSet != "" {
		if windows && winSet != "" {
			size = winSet
		} else if linSet != "" {
			size = linSet
		}
	}
	return size, store
}

// splitSizeStore 解析 "30,1" → ("30","1")，单值 → ("30","")。
func splitSizeStore(raw string) (size, store string) {
	parts := strings.Split(raw, ",")
	size = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		store = strings.TrimSpace(parts[1])
	}
	return size, store
}

// isWindowsOS 判断本次开通是否 Windows 镜像。
// 优先看显式提示字段（os_name / os_type），都没有时无法判定——按 Linux 处理。
func isWindowsOS(opts map[string]string) bool {
	for _, key := range []string{"os_name", "os_type"} {
		if v := strings.ToLower(opts[key]); strings.Contains(v, "win") {
			return true
		}
	}
	return false
}

// expandDataDisk 把归一后的数据盘参数展开成 /clouds 的数组表单语法
// other_data_disk[0][size] / [store]。
func expandDataDisk(form url.Values, opts map[string]string) {
	size := strings.TrimSpace(opts["data_disk_size"])
	if size == "" || size == "0" {
		return
	}
	form.Set("other_data_disk[0][size]", size)
	if store := strings.TrimSpace(opts["data_disk_store"]); store != "" {
		form.Set("other_data_disk[0][store]", store)
	}
}
