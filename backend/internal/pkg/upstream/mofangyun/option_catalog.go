package mofangyun

import (
	"hostsent/backend/internal/pkg/integration"
	"hostsent/backend/internal/pkg/upstream"
)

// 魔方云可配置项目录（33 项）。
//
// 来源：官方对接文档《魔方云对接魔方财务》§（五）可配置选项 参数表
// （docs/魔方云对接魔方财务.html），字段的中文名、必选标记、未传递默认值
// 与该表逐条对齐；参数格式（静态枚举 / 数量区间 / 平台 ID）也按表里说明标注。
//
// 与旧文档表的差异（有意为之，见 collectOptions 的别名翻译）：
//   - 表里对外用 IP_MACBond / resource_package / traffic_bill_type / flow_way /
//     flow_limit / system_disk_io_limit / data_disk_io_limit / data_disk_size，
//     而 /clouds 实际接收 bind_mac / rid / reset_flow_day / traffic_type /
//     traffic_quota / system_read_* 等。目录保留**表里的键**（运营看得懂），
//     由 collectOptions 翻译成平台写键。
//   - store（存储）表里未单列但在 /clouds 可传，作为 system_disk_size 的配套项补入。
func optionCatalog() []upstream.ConfigOptionSpec {
	num := func(v float64) *float64 { return &v }
	static := func(pairs ...string) []integration.FieldOption {
		out := make([]integration.FieldOption, 0, len(pairs)/2)
		for i := 0; i+1 < len(pairs); i += 2 {
			out = append(out, integration.FieldOption{Label: pairs[i+1], Value: pairs[i]})
		}
		return out
	}

	return []upstream.ConfigOptionSpec{
		// ===== 基础配置 =====
		{
			Key: "cpu", Label: "CPU", Group: "基础配置", Required: true,
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic,
			Unit: "核", MultiValue: true, SortOrder: 10,
			Options: static(
				"1", "1核", "2", "2核", "4", "4核", "8", "8核",
				"12", "12核", "16", "16核", "24", "24核", "32", "32核",
			),
			Help: "为该云主机分配的 CPU 核心数；用「多选」把 2核/4核/8核 同时开放给客户选。",
		},
		{
			Key: "memory", Label: "内存", Group: "基础配置", Required: true,
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic,
			Unit: "GB", MultiValue: true, SortOrder: 20,
			Options: static(
				"1024", "1G", "2048", "2G", "4096", "4G", "6144", "6G", "8192", "8G",
				"12288", "12G", "16384", "16G", "24576", "24G", "32768", "32G",
				"65536", "64G", "131072", "128G",
			),
			Help: "平台按 MB 接收（1024=1G）。目录里按文档口径列 MB，前台展示按 GB 换算。",
		},
		{
			Key: "system_disk_size", Label: "系统盘", Group: "基础配置",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "GB", MinValue: num(30), MaxValue: num(1000), Step: num(10), SortOrder: 30,
			Default: "50", Help: "不传递默认 50G，Windows 系统盘最小 30G；存储 ID 由「存储 store」另传。",
		},
		{
			Key: "store", Label: "存储", Group: "基础配置",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceStores,
			SortOrder: 40,
			Help:      "系统盘所在存储，可不传（默认随机）。取值来自平台的存储管理。",
		},
		{
			Key: "data_disk_size", Label: "数据盘", Group: "基础配置",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "GB", MinValue: num(0), MaxValue: num(1000), Step: num(10), SortOrder: 50,
			Default: "0", Help: "0 表示无数据盘。传值时平台按 [size, store] 组装 other_data_disk。",
		},
		{
			Key: "os", Label: "操作系统", Group: "基础配置", Required: true,
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceImages,
			MultiValue: true, SortOrder: 60,
			Help: "取值来自平台可下载镜像（可在「平台配置项」页批量导入并按 Ubuntu/Windows/CentOS 分组）。",
		},
		{
			Key: "type", Label: "云节点类型", Group: "基础配置",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceStatic, SortOrder: 70,
			Options: static("host", "KVM 加强版", "lightHost", "KVM 轻量版", "hyperv", "Hyper-V", "adsl", "拨号云"),
			Default: "host", Help: "不传递默认 KVM 加强版。",
		},

		// ===== 位置与调度 =====
		{
			Key: "area", Label: "数据中心", Group: "位置与调度", Required: true,
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceAreas, SortOrder: 110,
			Help: "数据中心与节点至少传一个；区域取值来自平台区域管理。",
		},
		{
			Key: "node", Label: "节点", Group: "位置与调度",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceNodes, SortOrder: 120,
			Default: "自动分配", Help: "不传递由平台自动分配；取值来自所选区域下的节点。",
		},
		{
			Key: "node_group", Label: "节点分组", Group: "位置与调度",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceManual, SortOrder: 130,
			Help: "平台「区域管理 → 节点管理 → 节点分组」的 ID，复制后填写。",
		},
		{
			Key: "node_priority", Label: "节点选择优先级", Group: "位置与调度",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic, SortOrder: 140,
			Options: static("1", "数量平均", "2", "负载最低", "3", "内存占用最低", "4", "填满一个"),
			Default: "1", Help: "不传递默认数量平均。",
		},
		{
			Key: "ip_group", Label: "IP 分组", Group: "位置与调度",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceManual, SortOrder: 150,
			Help: "平台「IP 管理 → IP 分组」的 ID，复制后填写。",
		},

		// ===== 网络 =====
		{
			Key: "network_type", Label: "网络类型", Group: "网络", Required: true,
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic, SortOrder: 210,
			Options: static("normal", "经典网络", "vpc", "VPC 网络"),
			Help:    "normal=IP 直接配置在机器内；vpc=支持浮动 IP，机器内不再配公网 IP。",
		},
		{
			Key: "bw", Label: "带宽", Group: "网络",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "Mbps", MinValue: num(0), MaxValue: num(100), Step: num(1), SortOrder: 220,
			Default: "0", Help: "上下行带宽；不传递默认 0Mbps。",
		},
		{
			Key: "in_bw", Label: "流入带宽", Group: "网络",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "Mbps", MinValue: num(0), MaxValue: num(100), Step: num(1), SortOrder: 230,
			Default: "0", Help: "进带宽；配置了 in_bw 时优先使用该参数。",
		},
		{
			Key: "flow_limit", Label: "流量", Group: "网络",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "GB", MinValue: num(0), MaxValue: num(1000000), Step: num(100), SortOrder: 240,
			Default: "不限量", Help: "0 表示不限量；平台按 traffic_quota 接收。",
		},
		{
			Key: "flow_way", Label: "流量方向", Group: "网络",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic, SortOrder: 250,
			Options: static("in", "进方向", "out", "出方向", "all", "进出汇总"),
			Default: "all", Help: "不传递默认进出汇总；平台按 traffic_type(1/2/3) 接收。",
		},
		{
			Key: "traffic_bill_type", Label: "流量计费周期", Group: "网络",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic, SortOrder: 260,
			Options: static("month", "自然月（每月 1 日清零）", "last_30days", "订购日至下月"),
			Default: "month", Help: "选「订购日至下月」时平台按 reset_flow_day=当天日 接收。",
		},
		{
			Key: "ip_num", Label: "IP 数量", Group: "网络", Required: true,
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "个", MinValue: num(1), MaxValue: num(20), Step: num(1), SortOrder: 270,
			Help: "为该云主机分配的 IP 数量。",
		},
		{
			Key: "ipv6_num", Label: "IPv6 数量", Group: "网络",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "个", MinValue: num(0), MaxValue: num(20), Step: num(1), SortOrder: 280,
			Help: "IPv6 地址数量，0 表示不分配。",
		},
		{
			Key: "IP_MACBond", Label: "IP-MAC 绑定", Group: "网络",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic, SortOrder: 290,
			Options: static("0", "关闭绑定", "1", "开启绑定"),
			Default: "开启绑定", Help: "不传递默认开启绑定；平台按 bind_mac 接收。",
		},
		{
			Key: "port", Label: "端口", Group: "网络",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceStatic, SortOrder: 300,
			Options: static("auto", "随机端口"),
			Default: "不支持", Help: "选「随机端口」或填写指定端口号（1024-30000）；不传递默认不支持。",
		},

		// ===== 高级 =====
		{
			Key: "snap_num", Label: "快照数量", Group: "高级",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic, SortOrder: 410,
			Options: static("-1", "不开通", "0", "不限量", "5", "5 个"),
			Default: "2", Help: "-1 不能创建快照，0 不限量；不传递默认 2 个。",
		},
		{
			Key: "backup_num", Label: "备份数量", Group: "高级",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic, SortOrder: 420,
			Options: static("-1", "不开通", "0", "不限量", "5", "5 个"),
			Default: "2", Help: "-1 不能创建备份，0 不限量；不传递默认 2 个。",
		},
		{
			Key: "nat_acl_limit", Label: "NAT 转发", Group: "高级",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic, SortOrder: 430,
			Options: static("-1", "不支持", "0", "不限制", "10", "10 个"),
			Default: "不支持", Help: "不传递默认不支持。",
		},
		{
			Key: "nat_web_limit", Label: "共享建站", Group: "高级",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic, SortOrder: 440,
			Options: static("-1", "不支持", "0", "不限制", "10", "10 个"),
			Default: "不支持", Help: "不传递默认不支持。",
		},
		{
			Key: "system_disk_io_limit", Label: "系统盘性能", Group: "高级",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceStatic, SortOrder: 450,
			Options: static("0,0,0,0", "不限制性能", "500,500,2000,2000", "读500/写500MBps、IOPS 2000"),
			Default: "不限制", Help: "四段值 a,b,c,d = 读MB/s,写MB/s,读IOPS,写IOPS；平台展开为 system_read_* 等 4 个参数。",
		},
		{
			Key: "data_disk_io_limit", Label: "数据盘性能", Group: "高级",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceStatic, SortOrder: 460,
			Options: static("0,0,0,0", "不限制性能", "500,500,2000,2000", "读500/写500MBps、IOPS 2000"),
			Default: "不限制", Help: "同系统盘性能，作用于数据盘。",
		},
		{
			Key: "cpu_limit", Label: "CPU 限制", Group: "高级",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "%", MinValue: num(0), MaxValue: num(100), Step: num(5), SortOrder: 470,
			Default: "以平台设置为准", Help: "限制实例 CPU 占用百分比；不传递以魔方云系统设置为准。",
		},
		{
			Key: "advanced_cpu", Label: "智能 CPU", Group: "高级",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceManual, SortOrder: 480,
			Help: "平台「监控 → 智能规则 → 智能 CPU」的 ID，复制后填写（如 1）。",
		},
		{
			Key: "advanced_bw", Label: "智能带宽", Group: "高级",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceManual, SortOrder: 490,
			Help: "平台「监控 → 智能规则 → 智能带宽」的 ID，复制后填写（如 1）。",
		},
		{
			Key: "resource_package", Label: "资源包", Group: "高级",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceManual, SortOrder: 500,
			Default: "不支持", Help: "平台的资源包 ID；平台按 rid 接收。不传递默认不支持。",
		},
		{
			Key: "gpu_num", Label: "GPU 数量", Group: "高级",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "个", MinValue: num(0), MaxValue: num(8), Step: num(1), SortOrder: 510,
			Default: "不支持", Help: "GPU 显卡个数；不传递默认不支持。",
		},
		{
			Key: "niccard", Label: "网卡驱动", Group: "高级",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceStatic, SortOrder: 520,
			Options: static("1", "Realtek 8139", "2", "Intel PRO/1000", "3", "Virtio"),
			Default: "不支持", Help: "不传递默认不支持（由平台决定）。",
		},
	}
}
