package zjmf

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"hostsent/backend/internal/pkg/upstream"
)

// 实例性能监控（InstanceMetrics）实现：财务型上游自己就是一层下游聚合器，
// 图表的**目录**（module_chart）与**数据**都原样来自上游，本适配器只做两件事：
//
//  1. 目录：GET host/header 的 data.module_chart（与 PHP 下游 HostController::getChart
//     完全同源），把 title/type/select 归一化成 MetricChartInfo。
//  2. 数据：GET provision/chart/{host_id}?type=&select=&start=&end=
//     ——这是上游 home/ProvisionController::getChartData 对下游开放的读接口
//     （route/home.php: Route::get("provision/chart/:id", ...)）。
//     其中 type 家族示例：cpu / disk / memory / flow；select 可空。
//
// 不做单位换算：上游已经给出 unit 与 label，换算规则属于上游模块知识，
// 下游再算一遍只会与上游面板不一致（PHP 下游也是原样透传）。

// zjmfModuleChart host/header 里的 module_chart 条目。
//
// 形状取自上游 Provision::chart / DcimCloud::chart：
//
//	[{"title":"CPU使用量","type":"cpu"}, {"title":"硬盘IO","type":"disk","select":[...]}]
type zjmfModuleChart struct {
	Title  string        `json:"title"`
	Type   string        `json:"type"`
	Select []interface{} `json:"select"`
}

// zjmfChartData provision/chart 的 data 壳。
//
// list 是「按 label 分组的采样点」：list[i] 对应 label[i] 那条曲线。
type zjmfChartData struct {
	Unit      string             `json:"unit"`
	ChartType string             `json:"chart_type"`
	Label     []string           `json:"label"`
	List      [][]zjmfChartPoint `json:"list"`
}

// zjmfChartPoint 单个采样点。
type zjmfChartPoint struct {
	Time  string          `json:"time"`
	Value jsonNumberValue `json:"value"`
}

// jsonNumberValue 兼容数字与数字字符串（上游 value 可能是 "12.34"）。
type jsonNumberValue float64

// UnmarshalJSON 接受 json 数字或字符串。
func (v *jsonNumberValue) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*v = 0
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*v = 0
		return nil
	}
	*v = jsonNumberValue(f)
	return nil
}

// ListMetricCharts 读上游 host/header 的 module_chart 作为图表目录。
//
// 上游没装配图表模块（或该主机类型不提供）时 module_chart 为空数组，
// 此时返回空目录，前端显示「暂不支持监控」，而不是报一个让用户困惑的错误。
func (p *Provider) ListMetricCharts(ctx context.Context, req *upstream.MetricsCatalogRequest) ([]upstream.MetricChartInfo, error) {
	if req == nil || req.ProviderInstanceID == "" {
		return nil, nil
	}
	var header struct {
		ModuleChart []zjmfModuleChart `json:"module_chart"`
	}
	if err := p.call(ctx, "ListMetricCharts", http.MethodGet, "host/header", url.Values{
		"host_id": {req.ProviderInstanceID},
		"source":  {"API"},
	}, &header); err != nil {
		return nil, err
	}
	out := make([]upstream.MetricChartInfo, 0, len(header.ModuleChart))
	for _, c := range header.ModuleChart {
		if c.Type == "" {
			continue
		}
		out = append(out, upstream.MetricChartInfo{
			// Type 原样透传（请求 provision/chart 时必须用它）；
			// Kind 只用于前端配色，认不出时退化为原生 type。
			Type:   c.Type,
			Kind:   normalizeMetricType(c.Type),
			Title:  c.Title,
			Select: normalizeChartSelect(c.Select),
		})
	}
	return out, nil
}

// normalizeMetricType 把上游的图表 type 归一到本包的 Metric* 语义（仅用于前端配色）。
// 上游各模块用的名字不完全统一（cpu / cpu_usage / memory / mem / flow / bandwidth …），
// 认得的归一，认不得的原样返回 —— 前端对未知 kind 仍可画图，只是没有专属配色。
func normalizeMetricType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "cpu", "cpu_usage", "cpu_used":
		return upstream.MetricCPU
	case "memory", "mem", "memory_usage", "ram":
		return upstream.MetricMemory
	case "disk", "disk_io", "io", "harddisk":
		return upstream.MetricDisk
	case "flow", "network", "net", "net_adapter", "bandwidth", "bw":
		return upstream.MetricNetwork
	default:
		return t
	}
}

// normalizeChartSelect 把上游 select 的多种形状归一成 label/value。
// 上游可能给：[{"label":"vda","value":"vda"}]、["vda"]、{"vda":"系统盘"}。
func normalizeChartSelect(raw []interface{}) []upstream.MetricChartOption {
	if len(raw) == 0 {
		return nil
	}
	out := make([]upstream.MetricChartOption, 0, len(raw))
	for _, item := range raw {
		switch t := item.(type) {
		case string:
			if t != "" {
				out = append(out, upstream.MetricChartOption{Label: t, Value: t})
			}
		case map[string]interface{}:
			value := firstNonEmpty(str(t["value"]), str(t["val"]), str(t["id"]))
			label := firstNonEmpty(str(t["label"]), str(t["name"]), str(t["title"]), value)
			if value != "" {
				out = append(out, upstream.MetricChartOption{Label: label, Value: value})
			}
		}
	}
	return out
}

// QueryMetrics 查询单项监控序列（GET provision/chart/{host_id}）。
func (p *Provider) QueryMetrics(ctx context.Context, req *upstream.MetricsQuery) (*upstream.MetricSeries, error) {
	series := &upstream.MetricSeries{Label: []string{}, List: [][]upstream.MetricDataPoint{}}
	if req == nil || req.ProviderInstanceID == "" || req.Type == "" {
		return series, nil
	}
	series.Type = req.Type
	query := url.Values{
		"type":  {req.Type},
		"start": {strconv.FormatInt(req.StartMillis, 10)},
		"end":   {strconv.FormatInt(req.EndMillis, 10)},
	}
	if req.Select != "" {
		query.Set("select", req.Select)
	}
	var data zjmfChartData
	if err := p.call(ctx, "QueryMetrics", http.MethodGet, "provision/chart/"+req.ProviderInstanceID, query, &data); err != nil {
		return nil, fmt.Errorf("查询上游监控数据失败: %w", err)
	}
	series.Unit = data.Unit
	series.ChartType = normalizeChartType(data.ChartType)
	series.Label = data.Label
	if series.Label == nil {
		series.Label = []string{}
	}
	for i := range series.Label {
		points := make([]upstream.MetricDataPoint, 0)
		if i < len(data.List) {
			for _, pt := range data.List[i] {
				points = append(points, upstream.MetricDataPoint{Time: pt.Time, Value: float64(pt.Value)})
			}
		}
		series.List = append(series.List, points)
	}
	// 上游 label 为空但确有数据时，按 list 长度补出无名的曲线，避免前端拿不到点。
	if len(series.Label) == 0 {
		for i, rows := range data.List {
			points := make([]upstream.MetricDataPoint, 0, len(rows))
			for _, pt := range rows {
				points = append(points, upstream.MetricDataPoint{Time: pt.Time, Value: float64(pt.Value)})
			}
			series.Label = append(series.Label, "系列"+strconv.Itoa(i+1))
			series.List = append(series.List, points)
		}
	}
	return series, nil
}

// normalizeChartType 图表形态归一；上游没给（或给了未知值）时按折线处理。
func normalizeChartType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case upstream.MetricChartArea:
		return upstream.MetricChartArea
	case upstream.MetricChartBar:
		return upstream.MetricChartBar
	default:
		return upstream.MetricChartLine
	}
}
