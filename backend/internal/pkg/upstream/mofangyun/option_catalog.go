package mofangyun

import (
	"strconv"

	"hostsent/backend/internal/pkg/integration"
	"hostsent/backend/internal/pkg/upstream"
)

// 魔方云可配置项目录（34 项）。
//
// 来源：官方对接文档《魔方云对接魔方财务》§（五）可配置选项 参数表
// （docs/魔方云对接魔方财务.html）。字段的中文名、必选标记、未传递默认值，
// 以及每个取值的含义（OptionsHelp），都与该表逐条对齐；数量型区间按表里的
// 「配置项参数」列标注。
//
// 改这份目录时要守住三条约定：
//
//  1. **Default 只放「可以直接下发给平台的值」**。文档里的「不传递默认不支持」
//     「不传递默认以平台设置为准」是说明文字，不是值 —— 落进 default_value 会被
//     当默认选中项，数量型还会被当数字解析（cpu_limit 原先填「以平台设置为准」，
//     前台取它当默认值，客户一下单就报「取值必须是数字」）。这类平台侧根本没有
//     对应取值的情况一律留空，原话写进 Help。枚举型只有取值就在 Options 里时才填。
//  2. **每个特殊取值都要在 OptionsHelp 里给一句话**。-1 / 0 / auto 这类值光看
//     取值看不出含义，必须把文档原文挂到值上。
//  3. **Default 必须落在 MinValue~MaxValue 内**，否则前台步进器一打开就是非法值。
//     catalog_test.go 逐项校验第 1、3 条。
//
// 另有一组「文档口径键 → /clouds 实际写键」的翻译，见 option_alias.go。
// 目录保留文档里的键名（运营照着官方文档填），由 collectOptions 翻译成平台写键。
func optionCatalog() []upstream.ConfigOptionSpec {
	num := func(v float64) *float64 { return &v }
	static := func(pairs ...string) []integration.FieldOption {
		out := make([]integration.FieldOption, 0, len(pairs)/2)
		for i := 0; i+1 < len(pairs); i += 2 {
			out = append(out, integration.FieldOption{Label: pairs[i+1], Value: pairs[i]})
		}
		return out
	}
	// optsHelp 把「值|含义」对拼成 OptionsHelp（JSON 对象文本）。
	// 手拼而不走 encoding/json：这份目录是逐字对齐官方表格的静态声明，
	// 手拼能保持「值 → 一句话」一行一对，读起来和表格一致。
	optsHelp := func(pairs ...string) string {
		out := "{"
		for i := 0; i+1 < len(pairs); i += 2 {
			if i > 0 {
				out += ","
			}
			out += strconv.Quote(pairs[i]) + ":" + strconv.Quote(pairs[i+1])
		}
		return out + "}"
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
			// cpu/memory 是仅有的「取值本身就是含义」的两项：2 就是 2 核、4096 就是 4G，
			// 显示名已经把话说完了。这里再挂一份 OptionsHelp 只会在配置档页多出一行
			// 「2 2核」的重复。特殊值才需要 OptionsHelp，见本文件头部约定。
			Help: "为该云主机分配的 CPU 核心数。必选：不传平台会拒绝开通。" +
				"用「多选」把 2核/4核/8核 同时开放给客户选。",
		},
		{
			Key: "memory", Label: "内存", Group: "基础配置", Required: true,
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic,
			Unit: "MB", MultiValue: true, SortOrder: 20,
			Options: static(
				"1024", "1G", "2048", "2G", "4096", "4G", "6144", "6G", "8192", "8G",
				"12288", "12G", "16384", "16G", "24576", "24G", "32768", "32G",
				"65536", "64G", "131072", "128G",
			),
			// 同 cpu：显示名已是「4G」这种自解释的写法，不另挂取值含义（见 cpu 处的说明）。
			Help: "为该云主机分配的内存大小。平台单位是 MB（1024=1G），下发给平台时按 MB 传；" +
				"必选，不传平台会拒绝开通。",
		},
		{
			Key: "system_disk_size", Label: "系统盘", Group: "基础配置",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "GB", MinValue: num(30), MaxValue: num(1000), Step: num(10), SortOrder: 30,
			Default: "50",
			Help: "系统盘大小（GB）。不传递默认 50G；Windows 系统盘最小 30G，所以下限取 30。" +
				"存储 ID 由「存储 store」另传（也可写进本项取值：30,1 表示 30G 放在存储 1；" +
				"lin:30,win:50,1 表示 Linux 取 30G、Windows 取 50G，放在存储 1）。" +
				"注意：本项填的是数值，lin:30,win:50,1 这种复合写法属于高级用法。",
		},
		{
			Key: "store", Label: "存储", Group: "基础配置",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceStores,
			SortOrder: 40,
			Help: "系统盘/数据盘所在的存储（平台「区域节点-存储管理」的存储 ID）。" +
				"不传则平台随机选一个存储。取值来自平台的存储管理。",
		},
		{
			Key: "data_disk_size", Label: "数据盘", Group: "基础配置",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "GB", MinValue: num(0), MaxValue: num(1000), Step: num(10), SortOrder: 50,
			Default: "0",
			Help: "数据盘大小（GB），传参单位 G。0 表示不分配数据盘（不传递即默认无数据盘）。" +
				"传值时平台按 [size, store] 组装 other_data_disk；存储 ID 也可跟在后面传：30,1。",
		},
		{
			Key: "os", Label: "操作系统", Group: "基础配置", Required: true,
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceImages,
			MultiValue: true, SortOrder: 60,
			Help: "平台「镜像管理」的操作系统镜像 ID。必选：不传平台会拒绝开通。" +
				"取值来自平台可下载镜像（在「平台配置项」页点「从平台刷新取值」批量入库，" +
				"镜像会按 Ubuntu/Windows/CentOS 分组）。",
		},
		{
			Key: "type", Label: "云节点类型", Group: "基础配置",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceStatic, SortOrder: 70,
			Options: static("host", "KVM 加强版", "lightHost", "KVM 轻量版", "hyperv", "Hyper-V", "adsl", "拨号云"),
			OptionsHelp: optsHelp(
				"host", "KVM 加强版节点类型",
				"lightHost", "KVM 轻量版节点类型",
				"hyperv", "Hyper-V 节点类型",
				"adsl", "拨号云节点类型",
			),
			Default: "host", Help: "不传递默认 KVM 加强版。",
		},

		// ===== 位置与调度 =====
		{
			Key: "area", Label: "数据中心", Group: "位置与调度", Required: true,
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceAreas, SortOrder: 110,
			Help: "平台「区域管理」的区域 ID。必选，且数据中心与节点至少传一个（两个都不传平台拒绝开通）。" +
				"取值来自平台区域管理，可点「从平台刷新取值」入库。",
		},
		{
			Key: "node", Label: "节点", Group: "位置与调度",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceNodes, SortOrder: 120,
			Help: "平台「节点管理」的节点 ID。数据中心与节点至少传一个；不传递由平台自动分配。" +
				"取值来自所选区域下的节点。",
		},
		{
			Key: "node_group", Label: "节点分组", Group: "位置与调度",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceManual, SortOrder: 130,
			Help: "平台「区域管理 → 节点管理 → 节点分组」的 ID。在平台面板复制 ID 后手工填写。",
		},
		{
			Key: "node_priority", Label: "节点选择优先级", Group: "位置与调度",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic, SortOrder: 140,
			Options: static("1", "数量平均", "2", "负载最低", "3", "内存占用最低", "4", "填满一个"),
			OptionsHelp: optsHelp(
				"1", "优先分到实例数最少的节点",
				"2", "优先分到 CPU 负载最低的节点",
				"3", "优先分到内存占用最低的节点",
				"4", "按节点 ID 从小到大填满一个再换下一个（该节点资源达上限后才开在其它节点）",
			),
			Default: "1", Help: "创建实例时的节点选择策略。不传递默认数量平均。",
		},
		{
			Key: "ip_group", Label: "IP 分组", Group: "位置与调度",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceManual, SortOrder: 150,
			Help: "平台「IP 管理 → IP 分组」的 ID。在平台面板复制 ID 后手工填写。",
		},

		// ===== 网络 =====
		{
			Key: "network_type", Label: "网络类型", Group: "网络", Required: true,
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic, SortOrder: 210,
			Options: static("normal", "经典网络", "vpc", "VPC 网络"),
			OptionsHelp: optsHelp(
				"normal", "分配的 IP 可直接配置在机器里",
				"vpc", "支持浮动 IP，机器内不再配置公网 IP",
			),
			Help: "必选：不传平台会拒绝开通。",
		},
		{
			Key: "bw", Label: "带宽（上下行）", Group: "网络",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "Mbps", MinValue: num(0), MaxValue: num(100), Step: num(1), SortOrder: 220,
			Default: "0",
			Help: "上下行带宽，同一数值同时作为上行（进）与下行（出）带宽下发" +
				"（平台实际读 in_bw / out_bw 两个字段，详见「流入带宽 in_bw」）。" +
				"不传递默认 0Mbps。",
		},
		{
			Key: "in_bw", Label: "流入带宽（上行）", Group: "网络",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "Mbps", MinValue: num(0), MaxValue: num(100), Step: num(1), SortOrder: 230,
			Default: "0",
			Help: "单独指定流入（上行）带宽；配置了 in_bw 时优先使用它，出带宽仍取「带宽 bw」的值。" +
				"两项都不传时平台按 0Mbps 处理（面板带宽组为空）。",
		},
		{
			Key: "flow_limit", Label: "流量", Group: "网络",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "GB", MinValue: num(0), MaxValue: num(1000000), Step: num(100), SortOrder: 240,
			Default: "0",
			Help: "为该云主机分配的流量大小，单位 G。0 表示不限量（不传递即默认不限量）。" +
				"平台按 traffic_quota 接收，面板侧等效于带宽流量上限（bwlimit）。",
		},
		{
			Key: "flow_way", Label: "流量方向", Group: "网络",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic, SortOrder: 250,
			Options: static("in", "进方向", "out", "出方向", "all", "进出汇总"),
			OptionsHelp: optsHelp(
				"in", "只计进方向流量",
				"out", "只计出方向流量",
				"all", "进出方向流量汇总计算",
			),
			Default: "all", Help: "不传递默认进出汇总；平台按 traffic_type(1 进 / 2 出 / 3 汇总) 接收。",
		},
		{
			Key: "traffic_bill_type", Label: "流量计费周期", Group: "网络",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic, SortOrder: 260,
			Options: static("month", "自然月（每月 1 日清零）", "last_30days", "订购日至下月"),
			OptionsHelp: optsHelp(
				"month", "每月 1 日流量清零（不传递即默认此项）",
				"last_30days", "产品开通日至下月同日清零（开通时按当天日号下发 reset_flow_day）",
			),
			Default: "month", Help: "选「订购日至下月」时平台按 reset_flow_day=当天日 接收。",
		},
		{
			Key: "ip_num", Label: "IP 数量", Group: "网络", Required: true,
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "个", MinValue: num(1), MaxValue: num(20), Step: num(1), SortOrder: 270,
			Help: "为该云主机分配的 IP 数量，范围 1~20。必选：不传平台会拒绝开通。",
		},
		{
			Key: "ipv6_num", Label: "IPv6 数量", Group: "网络",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "个", MinValue: num(0), MaxValue: num(20), Step: num(1), SortOrder: 280,
			Default: "0",
			Help:    "分配的 IPv6 地址数量，0 表示不分配。",
		},
		{
			Key: "IP_MACBond", Label: "IP-MAC 绑定", Group: "网络",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceStatic, SortOrder: 290,
			Options: static("0", "关闭绑定", "1", "开启绑定"),
			OptionsHelp: optsHelp(
				"0", "关闭 IP-MAC 绑定",
				"1", "开启 IP-MAC 绑定（不传递即默认此项）",
			),
			Default: "1", Help: "不传递默认开启绑定；平台按 bind_mac 接收（0 关 / 1 开）。" +
				"文档里这一项的中文名写作「嵌套虚拟化」，实际作用就是 IP-MAC 绑定。",
		},
		{
			Key: "port", Label: "端口", Group: "网络",
			// manual 而不是 static：除了 auto 还要能填具体端口号（1024~30000），
			// manual 让配置档里的取值选择器可创建新值。
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceManual, SortOrder: 300,
			Options: static("auto", "随机端口"),
			OptionsHelp: optsHelp(
				"auto", "由平台随机分配端口",
			),
			Default: "auto",
			Help: "选「随机端口」下发 auto；也可新建取值直接填指定端口号（1024~30000）。" +
				"不传递视为不支持指定端口。",
		},

		// ===== 高级 =====
		{
			Key: "snap_num", Label: "快照数量", Group: "高级",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceManual, SortOrder: 410,
			Options: static("-1", "不开通", "0", "不限量", "5", "5 个"),
			OptionsHelp: optsHelp(
				"-1", "该云主机不能创建快照",
				"0", "可以不限量创建快照",
				"5", "最多只能创建 5 个快照",
			),
			Default: "2",
			Help: "不传递默认 2 个（可创建 2 个快照）。" +
				"取值来源是 manual：除文档给的 -1/0/5 外可自行新建具体数量（如 20）。",
		},
		{
			Key: "backup_num", Label: "备份数量", Group: "高级",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceManual, SortOrder: 420,
			Options: static("-1", "不开通", "0", "不限量", "5", "5 个"),
			OptionsHelp: optsHelp(
				"-1", "该云主机不能创建备份",
				"0", "可以不限量创建备份",
				"5", "最多只能创建 5 个备份",
			),
			Default: "2",
			Help: "不传递默认 2 个（可创建 2 个备份）。" +
				"取值来源是 manual：除文档给的 -1/0/5 外可自行新建具体数量（如 20）。",
		},
		{
			Key: "nat_acl_limit", Label: "NAT 转发", Group: "高级",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceManual, SortOrder: 430,
			Options: static("-1", "不支持", "0", "不限制", "10", "10 个"),
			OptionsHelp: optsHelp(
				"-1", "该云主机不能设置 NAT 转发（不传递即默认此项）",
				"0", "可以不限量 NAT 转发",
				"10", "最多只能创建 10 个 NAT 转发",
			),
			Default: "-1",
			Help:    "不传递默认不支持（等价 -1）。取值来源是 manual：除 -1/0/10 外可自行新建具体数量。",
		},
		{
			Key: "nat_web_limit", Label: "共享建站", Group: "高级",
			Widget: upstream.ConfigWidgetRadio, ValueSource: upstream.ValueSourceManual, SortOrder: 440,
			Options: static("-1", "不支持", "0", "不限制", "10", "10 个"),
			OptionsHelp: optsHelp(
				"-1", "该云主机不能 NAT 建站（不传递即默认此项）",
				"0", "可以不限量 NAT 建站",
				"10", "最多只能创建 10 个 NAT 建站",
			),
			Default: "-1",
			Help:    "不传递默认不支持（等价 -1）。取值来源是 manual：除 -1/0/10 外可自行新建具体数量。",
		},
		{
			Key: "system_disk_io_limit", Label: "系统盘性能", Group: "高级",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceStatic, SortOrder: 450,
			Options: static("0,0,0,0", "不限制性能", "500,500,2000,2000", "读500/写500MBps、IOPS 2000"),
			OptionsHelp: optsHelp(
				"0,0,0,0", "不限制系统盘性能（不传递即默认此项）",
				"500,500,2000,2000", "读取限制 500MB/s、写入限制 500MB/s、读取限制 2000 IOPS/s、写入限制 2000 IOPS/s",
			),
			Default: "0,0,0,0",
			Help:    "四段值 a,b,c,d = 读 MB/s、写 MB/s、读 IOPS、写 IOPS；平台展开为 system_read_* 等 4 个参数。",
		},
		{
			Key: "data_disk_io_limit", Label: "数据盘性能", Group: "高级",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceStatic, SortOrder: 460,
			Options: static("0,0,0,0", "不限制性能", "500,500,2000,2000", "读500/写500MBps、IOPS 2000"),
			OptionsHelp: optsHelp(
				"0,0,0,0", "不限制数据盘性能（不传递即默认此项）",
				"500,500,2000,2000", "读取限制 500MB/s、写入限制 500MB/s、读取限制 2000 IOPS/s、写入限制 2000 IOPS/s",
			),
			Default: "0,0,0,0",
			Help:    "同系统盘性能，作用于数据盘；平台展开为 data_read_* 等 4 个参数。",
		},
		{
			Key: "cpu_limit", Label: "CPU 限制", Group: "高级",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "%", MinValue: num(0), MaxValue: num(100), Step: num(5), SortOrder: 470,
			Help: "限制实例 CPU 占用百分比（如 50 表示限制为 50%）。" +
				"不传递由魔方云系统设置为准 —— 平台侧这个「默认」没有可下发的具体数值，" +
				"所以本项默认值留空，客户不改就不下发。",
		},
		{
			Key: "advanced_cpu", Label: "智能 CPU", Group: "高级",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceManual, SortOrder: 480,
			Help: "平台「监控 → 智能规则 → 智能 CPU」的 ID（复制后填写）。平台侧没有对应规则时不要开这一项。" +
				"不传即不使用智能 CPU 规则。",
		},
		{
			Key: "advanced_bw", Label: "智能带宽", Group: "高级",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceManual, SortOrder: 490,
			Help: "平台「监控 → 智能规则 → 智能带宽」的 ID（复制后填写）。平台侧没有对应规则时不要开这一项。" +
				"不传即不使用智能带宽规则。",
		},
		{
			Key: "resource_package", Label: "资源包", Group: "高级",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceManual, SortOrder: 500,
			Help: "魔方云侧的资源包 ID（后面显示的名称可自定义，只是展示用）。" +
				"不传递视为不支持；平台按 rid 接收。",
		},
		{
			Key: "gpu_num", Label: "GPU 数量", Group: "高级",
			Widget: upstream.ConfigWidgetQty, ValueSource: upstream.ValueSourceStatic,
			Unit: "个", MinValue: num(0), MaxValue: num(8), Step: num(1), SortOrder: 510,
			Help: "GPU 显卡个数。0 或不下发表示不配 GPU；平台侧没有 GPU 资源时不传即视为不支持。",
		},
		{
			Key: "niccard", Label: "网卡驱动", Group: "高级",
			Widget: upstream.ConfigWidgetSelect, ValueSource: upstream.ValueSourceStatic, SortOrder: 520,
			Options: static("1", "Realtek 8139", "2", "Intel PRO/1000", "3", "Virtio"),
			OptionsHelp: optsHelp(
				"1", "Realtek 8139 网卡",
				"2", "Intel PRO/1000 网卡",
				"3", "Virtio 半虚拟化网卡",
			),
			Help: "选择虚拟网卡驱动型号。不传递默认不支持指定（由平台按宿主机决定）。",
		},
	}
}
