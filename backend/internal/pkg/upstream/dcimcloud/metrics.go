package dcimcloud

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"hostsent/backend/internal/pkg/upstream"
)

// 实例性能监控（InstanceMetrics）实现。
//
// 协议取自面板源码 app/common/logic/DcimCloud.php：
//   - chart()       → 图表目录（CPU使用量 / 硬盘IO / 内存用量 / 网卡）
//   - getChartData()→ GET /statistics，四种语义分别对应不同的 type 与字段位:
//       cpu    : type=kvm_info    → data[i][1] = CPU 使用率(%)
//       memory : type=kvm_info    → data[i][2] = 总量, data[i][3] = 已用（字节）
//       disk   : type=disk_io     → data[i][1] = 读, data[i][2] = 写（字节/秒）
//       flow   : type=net_adapter → data[i][1] = 进, data[i][2] = 出（bps）
//
// 单位缩放与图表形态（area/line/bar）也在本文件内完成：放在适配器而不是前端，
// 是因为「哪个平台的第几列是什么量纲」属于平台知识，前端只该拿到单位与曲线。
//
// hyperv 类型的面板不提供统计（源码 chart() 直接返回空），因此该类型目录为空，
// 前端据此显示「该类型不支持监控」而不是画一张永远没有数据的图。

// metricSelectDevice 磁盘 IO 的默认设备名（源码 getChartData 的 select 回退值）。
const metricSelectDevice = "vda"

// ListMetricCharts 返回图表目录。null 表示平台不支持（hyperv）。
func (p *Provider) ListMetricCharts(ctx context.Context, req *upstream.MetricsCatalogRequest) ([]upstream.MetricChartInfo, error) {
	_ = ctx
	if req == nil || req.ProviderInstanceID == "" {
		return nil, nil
	}
	// 是否 hyperv：读 /clouds/{id} 的 configoptions.type（源码 getProvisionParams 同源）。
	var detail map[string]interface{}
	if err := p.call(ctx, "ListMetricCharts", http.MethodGet, "/clouds/"+req.ProviderInstanceID, nil, &detail); err != nil {
		return nil, err
	}
	if metricPlatformType(detail) == "hyperv" {
		return nil, nil
	}
	// Type 与 Kind 同值：魔方云的原生 type 就是本包的四种语义，无需映射。
	return []upstream.MetricChartInfo{
		{Type: upstream.MetricCPU, Kind: upstream.MetricCPU, Title: "CPU使用量"},
		{Type: upstream.MetricDisk, Kind: upstream.MetricDisk, Title: "硬盘IO", Select: metricDiskSelects(detail)},
		{Type: upstream.MetricMemory, Kind: upstream.MetricMemory, Title: "内存用量"},
		{Type: upstream.MetricNetwork, Kind: upstream.MetricNetwork, Title: "网卡"},
	}, nil
}

// metricPlatformType 取实例的虚拟化类型（configoptions.type）；取不到留空。
func metricPlatformType(detail map[string]interface{}) string {
	if opts, ok := detail["configoptions"].(map[string]interface{}); ok {
		if t := jsonStr(opts, "type"); t != "" {
			return t
		}
	}
	return ""
}

// metricDiskSelects 从磁盘列表生成「设备名」子项（系统盘在前，数据盘其次）。
// 面板按 dev_name 区分曲线，取不到列表时给默认 vda，保持与源码一致。
func metricDiskSelects(detail map[string]interface{}) []upstream.MetricChartOption {
	raw, ok := detail["disk"].([]interface{})
	if !ok {
		return []upstream.MetricChartOption{{Label: metricSelectDevice, Value: metricSelectDevice}}
	}
	out := make([]upstream.MetricChartOption, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		dev := jsonStr(m, "dev_name")
		if dev == "" {
			dev = jsonStr(m, "name")
		}
		if dev == "" {
			continue
		}
		label := dev
		switch jsonStr(m, "type") {
		case "system":
			label = dev + "（系统盘）"
		case "data":
			label = dev + "（数据盘）"
		}
		out = append(out, upstream.MetricChartOption{Label: label, Value: dev})
	}
	if len(out) == 0 {
		out = append(out, upstream.MetricChartOption{Label: metricSelectDevice, Value: metricSelectDevice})
	}
	return out
}

// QueryMetrics 查询单项监控序列。
func (p *Provider) QueryMetrics(ctx context.Context, req *upstream.MetricsQuery) (*upstream.MetricSeries, error) {
	if req == nil || req.ProviderInstanceID == "" {
		return emptySeries(req), nil
	}
	// node_id / kvmid 只能从实例详情取：统计接口按「节点 + KVM ID」定位采样。
	var detail map[string]interface{}
	if err := p.call(ctx, "QueryMetrics", http.MethodGet, "/clouds/"+req.ProviderInstanceID, nil, &detail); err != nil {
		return nil, err
	}
	nodeID := jsonStr(detail, "node_id")
	kvmID := jsonStr(detail, "kvmid")
	if nodeID == "" || kvmID == "" {
		// 面板还没分配节点（创建中/迁移中）：没有采样可查，返回空序列而非报错。
		return emptySeries(req), nil
	}
	query := url.Values{
		"node_id": {nodeID},
		"kvm":     {kvmID},
		"st":      {strconv.FormatInt(req.StartMillis, 10)},
		"et":      {strconv.FormatInt(req.EndMillis, 10)},
	}
	series := &upstream.MetricSeries{Type: req.Type, Label: []string{}, List: [][]upstream.MetricDataPoint{}}
	switch req.Type {
	case upstream.MetricCPU:
		query.Set("type", "kvm_info")
		rows, err := p.queryStatistics(ctx, query)
		if err != nil {
			return nil, err
		}
		series.Unit = "%"
		series.ChartType = upstream.MetricChartArea
		series.Label = []string{"CPU使用率(%)"}
		points := make([]upstream.MetricDataPoint, 0, len(rows))
		for _, row := range rows {
			points = append(points, upstream.MetricDataPoint{Time: row.at, Value: round2(row.value(1))})
		}
		series.List = append(series.List, points)
	case upstream.MetricMemory:
		query.Set("type", "kvm_info")
		rows, err := p.queryStatistics(ctx, query)
		if err != nil {
			return nil, err
		}
		unit, pow := byteUnit(maxOf(rows, 2))
		series.Unit = unit
		series.ChartType = upstream.MetricChartBar
		series.Label = []string{"总量(" + unit + ")", "已用(" + unit + ")"}
		total := make([]upstream.MetricDataPoint, 0, len(rows))
		used := make([]upstream.MetricDataPoint, 0, len(rows))
		for _, row := range rows {
			total = append(total, upstream.MetricDataPoint{Time: row.at, Value: round2(row.value(2) / pow)})
			used = append(used, upstream.MetricDataPoint{Time: row.at, Value: round2(row.value(3) / pow)})
		}
		series.List = append(series.List, total, used)
	case upstream.MetricDisk:
		dev := req.Select
		if dev == "" {
			dev = metricSelectDevice
		}
		query.Set("type", "disk_io")
		query.Set("dev_name", dev)
		rows, err := p.queryStatistics(ctx, query)
		if err != nil {
			return nil, err
		}
		unit, pow := bitScaledUnit(maxOf(rows, 1, 2), "B/s", "KB/s", "MB/s", "GB/s", "TB/s")
		series.Unit = unit
		series.ChartType = upstream.MetricChartLine
		series.Label = []string{"读取速度(" + unit + ")", "写入速度(" + unit + ")"}
		read := make([]upstream.MetricDataPoint, 0, len(rows))
		write := make([]upstream.MetricDataPoint, 0, len(rows))
		for _, row := range rows {
			read = append(read, upstream.MetricDataPoint{Time: row.at, Value: round2(row.value(1) / pow)})
			write = append(write, upstream.MetricDataPoint{Time: row.at, Value: round2(row.value(2) / pow)})
		}
		series.List = append(series.List, read, write)
	case upstream.MetricNetwork:
		query.Set("type", "net_adapter")
		query.Set("kvm_ifname", kvmID+".0")
		rows, err := p.queryStatistics(ctx, query)
		if err != nil {
			return nil, err
		}
		unit, pow := bitScaledUnit(maxOf(rows, 1, 2), "bps", "Kbps", "Mbps", "Gbps", "Tbps")
		series.Unit = unit
		series.ChartType = upstream.MetricChartLine
		series.Label = []string{"进(" + unit + ")", "出(" + unit + ")"}
		in := make([]upstream.MetricDataPoint, 0, len(rows))
		out := make([]upstream.MetricDataPoint, 0, len(rows))
		for _, row := range rows {
			in = append(in, upstream.MetricDataPoint{Time: row.at, Value: round2(row.value(1) / pow)})
			out = append(out, upstream.MetricDataPoint{Time: row.at, Value: round2(row.value(2) / pow)})
		}
		series.List = append(series.List, in, out)
	default:
		return emptySeries(req), nil
	}
	return series, nil
}

// emptySeries 返回空序列（无采样），曲线上限按语义给足，前端图例不塌。
func emptySeries(req *upstream.MetricsQuery) *upstream.MetricSeries {
	s := &upstream.MetricSeries{Type: "", List: [][]upstream.MetricDataPoint{}}
	if req != nil {
		s.Type = req.Type
	}
	switch s.Type {
	case upstream.MetricCPU:
		s.Unit, s.ChartType, s.Label = "%", upstream.MetricChartArea, []string{"CPU使用率(%)"}
	case upstream.MetricMemory:
		s.Unit, s.ChartType, s.Label = "MB", upstream.MetricChartBar, []string{"总量(MB)", "已用(MB)"}
	case upstream.MetricDisk:
		s.Unit, s.ChartType, s.Label = "KB/s", upstream.MetricChartLine, []string{"读取速度(KB/s)", "写入速度(KB/s)"}
	case upstream.MetricNetwork:
		s.Unit, s.ChartType, s.Label = "Kbps", upstream.MetricChartLine, []string{"进(Kbps)", "出(Kbps)"}
	}
	for range s.Label {
		s.List = append(s.List, []upstream.MetricDataPoint{})
	}
	return s
}

// statisticRow 一行统计采样。
//
// cells 保留**面板原始列位**（下标 0 是时间），value 的下标因此与
// DcimCloud::getChartData 里的 $v[1]/$v[2]/$v[3] 完全一致 —— 把时间列剥掉
// 会让每个语义的列号都要 -1，是这类代码最容易写错的地方。
type statisticRow struct {
	at    string
	cells []float64
}

// value 取第 idx 列（与面板原始列位一致；越界返回 0）。
func (r statisticRow) value(idx int) float64 {
	if idx < 0 || idx >= len(r.cells) {
		return 0
	}
	return r.cells[idx]
}

// queryStatistics 调 GET /statistics 并把行数组转成 statisticRow。
//
// 面板返回的 data 是「数组的数组」：[["2026-01-01 00:00:00", 12.3, ...], ...]；
// 第 0 列总是时间（GMT 字符串），其余按 type 语义解释。时间统一转成本地时区
// 的 "2006-01-02 15:04:05"（与源码 date("Y-m-d H:i:s", strtotime($v[0])) 一致）。
func (p *Provider) queryStatistics(ctx context.Context, query url.Values) ([]statisticRow, error) {
	var resp struct {
		Data []interface{} `json:"data"`
	}
	if err := p.call(ctx, "QueryMetrics", http.MethodGet, "/statistics", query, &resp); err != nil {
		return nil, err
	}
	rows := make([]statisticRow, 0, len(resp.Data))
	for _, item := range resp.Data {
		arr, ok := item.([]interface{})
		if !ok || len(arr) == 0 {
			continue
		}
		row := statisticRow{cells: make([]float64, 0, len(arr))}
		for _, cell := range arr {
			row.cells = append(row.cells, cellFloat(cell))
		}
		row.at = formatPanelTime(rawTimeString(arr[0]))
		rows = append(rows, row)
	}
	return rows, nil
}

// rawTimeString 把时间列转成字符串（面板可能给字符串或数字时间戳）。
func rawTimeString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}

// formatPanelTime 面板时间串 → "2006-01-02 15:04:05"（本地时区）。
// 面板给的是 "2026-01-01 00:00:00" 这类可直接解析的格式（PHP strtotime 口径）。
func formatPanelTime(raw string) string {
	if raw == "" {
		return ""
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
		time.RFC3339,
		"2006-01-02 15:04",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t.Format("2006-01-02 15:04:05")
		}
	}
	return raw
}

// cellFloat 统计列的数值（面板可能给数字或数字字符串）。
func cellFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return f
		}
	}
	return 0
}

// maxOf 取所有行在给定列上的最大值（用于选单位缩放指数）。
func maxOf(rows []statisticRow, idxs ...int) float64 {
	peak := 0.0
	for _, row := range rows {
		for _, idx := range idxs {
			if v := row.value(idx); v > peak {
				peak = v
			}
		}
	}
	return peak
}

// byteUnit 按 1024 进制选二进制单位（B/KB/MB/GB/TB），返回单位名与除数。
func byteUnit(maxN float64) (string, float64) {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	return pickUnit(maxN, units)
}

// bitScaledUnit 按 1024 进制选单位，单位名由调用方给出（如磁盘 B/s 系列）。
func bitScaledUnit(maxN float64, units ...string) (string, float64) {
	return pickUnit(maxN, units)
}

// pickUnit 与源码同逻辑：max >= 1024 就升一级，直到 < 1024 或单位用尽。
func pickUnit(maxN float64, units []string) (string, float64) {
	pow := 1.0
	idx := 0
	for idx < len(units) {
		if maxN < 1024 {
			break
		}
		maxN /= 1024
		pow *= 1024
		idx++
	}
	if idx >= len(units) {
		idx = len(units) - 1
	}
	return units[idx], pow
}

// round2 保留两位小数（与源码 round($v, 2) 一致）。
func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
