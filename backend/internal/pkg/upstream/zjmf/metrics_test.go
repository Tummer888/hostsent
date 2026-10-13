package zjmf

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hostsent/backend/internal/pkg/upstream"
)

// 财务型上游的监控是**透传**：目录来自 host/header.module_chart，
// 数据来自 provision/chart/{host_id}。这里钉住的是「原样转发 + 形状归一」，
// 因为它对上游来说是只读契约，写错不会报错、只会拿到空图表。

func newChartProvider(t *testing.T, handler http.HandlerFunc) *ZJMFProvider {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return NewZJMFProvider(&upstream.ProviderConfig{
		APIEndpoint:  ts.URL,
		UpstreamType: "zjmf_api",
		Timeout:      5,
	})
}

func loginOK(w http.ResponseWriter) {
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 200, "msg": "ok", "jwt": "jwt-1"})
}

func TestListMetricCharts_PassesThroughModuleChart(t *testing.T) {
	var sawPath string
	p := newChartProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zjmf_api_login":
			loginOK(w)
		case "/host/header":
			sawPath = r.URL.Path
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200, "msg": "ok",
				"data": map[string]interface{}{
					"module_chart": []map[string]interface{}{
						{"title": "CPU使用量", "type": "cpu"},
						{"title": "硬盘IO", "type": "disk", "select": []map[string]interface{}{{"label": "vda", "value": "vda"}}},
						{"title": "内存用量", "type": "mem"},
						{"title": "网卡", "type": "flow"},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	charts, err := p.ListMetricCharts(context.Background(), &upstream.MetricsCatalogRequest{ProviderInstanceID: "77"})
	if err != nil {
		t.Fatalf("ListMetricCharts failed: %v", err)
	}
	if sawPath != "/host/header" {
		t.Errorf("path = %q, want /host/header", sawPath)
	}
	if len(charts) != 4 {
		t.Fatalf("charts = %+v, want 4", charts)
	}
	// Type 必须**原样透传**（请求 provision/chart 时要用它）；Kind 才是归一化语义。
	wantType := []string{"cpu", "disk", "mem", "flow"}
	wantKind := []string{upstream.MetricCPU, upstream.MetricDisk, upstream.MetricMemory, upstream.MetricNetwork}
	for i := range wantType {
		if charts[i].Type != wantType[i] {
			t.Errorf("charts[%d].Type = %q, want %q（必须原样透传）", i, charts[i].Type, wantType[i])
		}
		if charts[i].Kind != wantKind[i] {
			t.Errorf("charts[%d].Kind = %q, want %q", i, charts[i].Kind, wantKind[i])
		}
	}
	if charts[0].Title != "CPU使用量" {
		t.Errorf("title = %q", charts[0].Title)
	}
	if len(charts[1].Select) != 1 || charts[1].Select[0].Value != "vda" {
		t.Errorf("select = %+v", charts[1].Select)
	}
}

func TestQueryMetrics_PassesThroughSeries(t *testing.T) {
	var gotQuery map[string]string
	p := newChartProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zjmf_api_login":
			loginOK(w)
		case "/provision/chart/77":
			q := r.URL.Query()
			gotQuery = map[string]string{"type": q.Get("type"), "select": q.Get("select"), "start": q.Get("start"), "end": q.Get("end")}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200, "msg": "ok",
				"data": map[string]interface{}{
					"unit":       "Mbps",
					"chart_type": "line",
					"label":      []string{"进(Mbps)", "出(Mbps)"},
					"list": []interface{}{
						[]map[string]interface{}{{"time": "2026-01-01 00:00:00", "value": "1.5"}},
						[]map[string]interface{}{{"time": "2026-01-01 00:00:00", "value": 2}},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	s, err := p.QueryMetrics(context.Background(), &upstream.MetricsQuery{
		ProviderInstanceID: "77", Type: upstream.MetricNetwork, Select: "eth0",
		StartMillis: 1000, EndMillis: 2000,
	})
	if err != nil {
		t.Fatalf("QueryMetrics failed: %v", err)
	}
	if gotQuery["type"] != upstream.MetricNetwork || gotQuery["select"] != "eth0" ||
		gotQuery["start"] != "1000" || gotQuery["end"] != "2000" {
		t.Errorf("query = %+v", gotQuery)
	}
	if s.Unit != "Mbps" || s.ChartType != upstream.MetricChartLine {
		t.Errorf("series head = %+v", s)
	}
	if len(s.Label) != 2 || len(s.List) != 2 {
		t.Fatalf("series = %+v", s)
	}
	// value 兼容字符串与数字：两条曲线都要拿到点。
	if s.List[0][0].Value != 1.5 || s.List[1][0].Value != 2 {
		t.Errorf("values = %+v", s.List)
	}
	if s.List[0][0].Time != "2026-01-01 00:00:00" {
		t.Errorf("time = %q", s.List[0][0].Time)
	}
}

func TestQueryMetrics_EmptyLabelStillReturnsPoints(t *testing.T) {
	p := newChartProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zjmf_api_login":
			loginOK(w)
		case "/provision/chart/77":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": 200, "msg": "ok",
				"data": map[string]interface{}{
					"chart_type": "area",
					"list":       []interface{}{[]map[string]interface{}{{"time": "t1", "value": 3}}},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	s, err := p.QueryMetrics(context.Background(), &upstream.MetricsQuery{ProviderInstanceID: "77", Type: upstream.MetricCPU})
	if err != nil {
		t.Fatalf("QueryMetrics failed: %v", err)
	}
	if len(s.Label) != 1 || len(s.List) != 1 || s.List[0][0].Value != 3 {
		t.Errorf("series = %+v", s)
	}
}

func TestImplementsInstanceMetrics(t *testing.T) {
	var _ upstream.InstanceMetrics = (*ZJMFProvider)(nil)
}
