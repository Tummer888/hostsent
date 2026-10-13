package dcimcloud

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"hostsent/backend/internal/pkg/upstream"
)

// 监控能力契约测试：校验 /statistics 的调用参数与四种语义的列位解析。
//
// 这些断言的价值在于「列位」：面板四种图表共用同一个 /statistics 接口，
// 差别只在 type 与「第几列是什么量纲」。写错列位不会报错，只会安静地
// 把内存画成 CPU，因此必须逐个钉住。

func TestListMetricCharts_SkipsHyperv(t *testing.T) {
	cases := []struct {
		name     string
		platform string
		wantLen  int
	}{
		{"kvm 提供四张图", "kvm", 4},
		{"hyperv 不提供", "hyperv", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/login" {
					_, _ = w.Write([]byte("test-token"))
					return
				}
				if r.URL.Path != "/v1/clouds/1934" {
					http.NotFound(w, r)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"id": "1934", "configoptions": map[string]interface{}{"type": tc.platform},
					"disk": []interface{}{
						map[string]interface{}{"dev_name": "vda", "type": "system"},
						map[string]interface{}{"dev_name": "vdb", "type": "data"},
					},
				})
			})
			charts, err := p.ListMetricCharts(context.Background(), &upstream.MetricsCatalogRequest{ProviderInstanceID: "1934"})
			if err != nil {
				t.Fatalf("ListMetricCharts failed: %v", err)
			}
			if len(charts) != tc.wantLen {
				t.Fatalf("charts len = %d, want %d", len(charts), tc.wantLen)
			}
			if tc.wantLen == 0 {
				return
			}
			wantOrder := []string{upstream.MetricCPU, upstream.MetricDisk, upstream.MetricMemory, upstream.MetricNetwork}
			for i, want := range wantOrder {
				if charts[i].Type != want {
					t.Errorf("charts[%d].Type = %q, want %q", i, charts[i].Type, want)
				}
			}
			if len(charts[1].Select) != 2 || charts[1].Select[0].Value != "vda" {
				t.Errorf("disk select = %+v, want vda/vdb", charts[1].Select)
			}
		})
	}
}

// statisticsServer 按 type 返回固定采样行，并记录收到的 query。
func statisticsServer(t *testing.T, rows []interface{}) (*DCIMCloudProvider, *map[string]string) {
	t.Helper()
	got := map[string]string{}
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/login":
			_, _ = w.Write([]byte("test-token"))
		case "/v1/clouds/1934":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "1934", "node_id": "7", "kvmid": "55",
				"configoptions": map[string]interface{}{"type": "kvm"},
			})
		case "/v1/statistics":
			q := r.URL.Query()
			got["type"] = q.Get("type")
			got["node_id"] = q.Get("node_id")
			got["kvm"] = q.Get("kvm")
			got["dev_name"] = q.Get("dev_name")
			got["kvm_ifname"] = q.Get("kvm_ifname")
			got["st"] = q.Get("st")
			got["et"] = q.Get("et")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": rows})
		default:
			http.NotFound(w, r)
		}
	})
	return p, &got
}

func queryOne(t *testing.T, p *DCIMCloudProvider, typ string) *upstream.MetricSeries {
	t.Helper()
	s, err := p.QueryMetrics(context.Background(), &upstream.MetricsQuery{
		ProviderInstanceID: "1934", Type: typ, StartMillis: 1000, EndMillis: 2000,
	})
	if err != nil {
		t.Fatalf("QueryMetrics(%s) failed: %v", typ, err)
	}
	return s
}

func TestQueryMetrics_CPU(t *testing.T) {
	p, got := statisticsServer(t, []interface{}{
		[]interface{}{"2026-01-01 00:00:00", 12.345, 8192, 4096},
		[]interface{}{"2026-01-01 00:05:00", 40, 8192, 4096},
	})
	// 时间列之外的列都保留，cpu 只取第 1 列（值列下标 0）。
	s := queryOne(t, p, upstream.MetricCPU)
	if (*got)["type"] != "kvm_info" {
		t.Errorf("type = %q, want kvm_info", (*got)["type"])
	}
	if (*got)["node_id"] != "7" || (*got)["kvm"] != "55" {
		t.Errorf("node/kvm = %q/%q, want 7/55", (*got)["node_id"], (*got)["kvm"])
	}
	if (*got)["st"] != "1000" || (*got)["et"] != "2000" {
		t.Errorf("st/et = %q/%q, want 1000/2000", (*got)["st"], (*got)["et"])
	}
	if s.Unit != "%" || s.ChartType != upstream.MetricChartArea || len(s.Label) != 1 {
		t.Fatalf("series head = %+v, want unit=%% area 1 label", s)
	}
	if len(s.List) != 1 || len(s.List[0]) != 2 {
		t.Fatalf("list = %+v, want 1x2", s.List)
	}
	if v := s.List[0][0].Value; v != 12.35 {
		t.Errorf("first value = %v, want 12.35 (round2)", v)
	}
	if s.List[0][0].Time != "2026-01-01 00:00:00" {
		t.Errorf("time = %q", s.List[0][0].Time)
	}
}

func TestQueryMetrics_MemoryUsesTotalUsedColumns(t *testing.T) {
	// 总量 8192B、已用 4096B：都 < 1024*1024 但 >= 1024 → 单位落到 KB。
	p, _ := statisticsServer(t, []interface{}{
		[]interface{}{"2026-01-01 00:00:00", 1.0, 2048, 1024},
	})
	s := queryOne(t, p, upstream.MetricMemory)
	if s.Unit != "KB" {
		t.Fatalf("unit = %q, want KB", s.Unit)
	}
	if len(s.Label) != 2 || len(s.List) != 2 {
		t.Fatalf("want 2 series, got %+v", s)
	}
	if got := s.List[0][0].Value; got != 2 {
		t.Errorf("total = %v, want 2 (2048/1024)", got)
	}
	if got := s.List[1][0].Value; got != 1 {
		t.Errorf("used = %v, want 1 (1024/1024)", got)
	}
}

func TestQueryMetrics_DiskUsesReadWriteAndDevName(t *testing.T) {
	p, got := statisticsServer(t, []interface{}{
		[]interface{}{"2026-01-01 00:00:00", 1024, 2048},
	})
	s, err := p.QueryMetrics(context.Background(), &upstream.MetricsQuery{
		ProviderInstanceID: "1934", Type: upstream.MetricDisk, Select: "vdb", StartMillis: 1, EndMillis: 2,
	})
	if err != nil {
		t.Fatalf("QueryMetrics failed: %v", err)
	}
	if (*got)["type"] != "disk_io" || (*got)["dev_name"] != "vdb" {
		t.Errorf("type/dev_name = %q/%q, want disk_io/vdb", (*got)["type"], (*got)["dev_name"])
	}
	if s.Unit != "KB/s" {
		t.Errorf("unit = %q, want KB/s", s.Unit)
	}
	if len(s.List) != 2 || s.List[0][0].Value != 1 || s.List[1][0].Value != 2 {
		t.Errorf("read/write = %+v, want 1/2", s.List)
	}
}

func TestQueryMetrics_NetworkUsesKvmIfname(t *testing.T) {
	p, got := statisticsServer(t, []interface{}{
		[]interface{}{"2026-01-01 00:00:00", 512, 256},
	})
	s := queryOne(t, p, upstream.MetricNetwork)
	if (*got)["type"] != "net_adapter" {
		t.Errorf("type = %q, want net_adapter", (*got)["type"])
	}
	if (*got)["kvm_ifname"] != "55.0" {
		t.Errorf("kvm_ifname = %q, want 55.0", (*got)["kvm_ifname"])
	}
	if s.Unit != "bps" || len(s.List) != 2 {
		t.Errorf("series = %+v", s)
	}
}

func TestQueryMetrics_EmptyWhenNoNode(t *testing.T) {
	// 未分配节点（创建中）：没有采样可查，返回空序列而不是报错。
	p := newMockProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/login" {
			_, _ = w.Write([]byte("test-token"))
			return
		}
		if r.URL.Path != "/v1/clouds/1934" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "1934"})
	})
	s, err := p.QueryMetrics(context.Background(), &upstream.MetricsQuery{
		ProviderInstanceID: "1934", Type: upstream.MetricCPU,
	})
	if err != nil {
		t.Fatalf("QueryMetrics failed: %v", err)
	}
	if len(s.List) != 1 || len(s.List[0]) != 0 {
		t.Errorf("want one empty series, got %+v", s.List)
	}
}

func TestImplementsInstanceMetrics(t *testing.T) {
	var _ upstream.InstanceMetrics = (*DCIMCloudProvider)(nil)
}
