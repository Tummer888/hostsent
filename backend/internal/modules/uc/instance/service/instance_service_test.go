package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"hostsent/backend/internal/modules/uc/instance/dto"
	"hostsent/backend/internal/modules/uc/instance/model"
	"hostsent/backend/internal/modules/uc/instance/repository"
	"hostsent/backend/internal/pkg/upstream"
)

// fakeRepo 最小实例仓储桩：只实现本文件用到的读取。
type fakeRepo struct {
	items     []model.Instance
	ops       []repository.OperationEntry
	provision string
	// statusWrites 记录 UpdateStatus 调用（顺序即调用顺序）。
	statusWrites []statusWrite
	updateErr    error
}

// statusWrite 一次状态写入（供断言）。
type statusWrite struct {
	ID     uint64
	Status string
}

func (f *fakeRepo) ListByUser(context.Context, uint64) ([]model.Instance, error) { return f.items, nil }

func (f *fakeRepo) FindByUser(_ context.Context, _, id uint64) (*model.Instance, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			return &f.items[i], nil
		}
	}
	return nil, repository.ErrNotFound
}

func (f *fakeRepo) Provision(context.Context, uint64) (string, string, error) {
	return f.provision, "", nil
}

func (f *fakeRepo) Operations(context.Context, uint64, int) ([]repository.OperationEntry, error) {
	return f.ops, nil
}

// UpdateStatus 记录状态写入，供「电源操作后本地状态即时更新」类用例断言。
func (f *fakeRepo) UpdateStatus(_ context.Context, id uint64, status string) error {
	f.statusWrites = append(f.statusWrites, statusWrite{ID: id, Status: status})
	for i := range f.items {
		if f.items[i].ID == id {
			f.items[i].Status = status
		}
	}
	return f.updateErr
}

// fakeMetricsProvider 只实现 InstanceMetrics：验证监控链路不依赖其它能力。
type fakeMetricsProvider struct {
	charts []upstream.MetricChartInfo
	series *upstream.MetricSeries
	err    error
	// wantStart/wantEnd 记录收到的窗口，用于断言默认时间窗。
	wantStart, wantEnd int64
}

func (f *fakeMetricsProvider) GetType() string                   { return "fake" }
func (f *fakeMetricsProvider) GetName() string                   { return "fake" }
func (f *fakeMetricsProvider) HealthCheck(context.Context) error { return nil }
func (f *fakeMetricsProvider) Capabilities() upstream.CapabilityDescriptor {
	return upstream.CapabilityDescriptor{}
}
func (f *fakeMetricsProvider) ListMetricCharts(context.Context, *upstream.MetricsCatalogRequest) ([]upstream.MetricChartInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.charts, nil
}
func (f *fakeMetricsProvider) QueryMetrics(_ context.Context, q *upstream.MetricsQuery) (*upstream.MetricSeries, error) {
	f.wantStart, f.wantEnd = q.StartMillis, q.EndMillis
	if f.err != nil {
		return nil, f.err
	}
	return f.series, nil
}

// 说明：GetInstance 的返回类型在 upstream.InstanceControl 里是 *model.StandardInstance，
// 这里用不到，因此不实现该接口 —— 断言只落在 InstanceMetrics 上。

func TestListSummaryCountsByVisibleState(t *testing.T) {
	// 80 小时 → 3 天（整数截断，与管理端 expireStateOf 同口径）。
	expireSoon := time.Now().Add(80 * time.Hour)
	expired := time.Now().Add(-2 * 24 * time.Hour)
	repo := &fakeRepo{items: []model.Instance{
		{ID: 1, ProviderInstanceID: "a", Status: "running", ExpireAt: &expireSoon},
		{ID: 2, ProviderInstanceID: "b", Status: "stopped"},
		{ID: 3, ProviderInstanceID: "", Status: "creating"},                     // 待开通
		{ID: 4, ProviderInstanceID: "d", Status: "running", ExpireAt: &expired}, // 已过期但仍在运行
		{ID: 5, ProviderInstanceID: "e", Status: "deleted", LifecycleStage: "destroyed"},
	}}
	svc := NewInstanceService(repo, nil)
	res, err := svc.List(context.Background(), 1, dto.ListQuery{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	want := dto.ListSummary{Total: 5, Expiring: 1, Stopped: 1, Creating: 1, Expired: 1, Destroyed: 1}
	if res.Summary != want {
		t.Errorf("summary = %+v, want %+v", res.Summary, want)
	}
	if res.Items[0].ExpireState != expireStateExpiring || res.Items[0].DaysLeft != 3 {
		t.Errorf("item0 expire = %q/%d", res.Items[0].ExpireState, res.Items[0].DaysLeft)
	}
	if res.Items[3].ExpireState != expireStateExpired {
		t.Errorf("item3 expire = %q, want expired (running 也要标已过期)", res.Items[3].ExpireState)
	}
	if res.Items[4].LifecycleStage != "destroyed" {
		t.Errorf("item4 stage = %q", res.Items[4].LifecycleStage)
	}
}

func TestMetricsCatalog_UnsupportedWhenNoCharts(t *testing.T) {
	repo := &fakeRepo{items: []model.Instance{{ID: 1, InstanceID: "i-1", ProviderInstanceID: "9", ProviderID: 3}}}
	svc := NewInstanceService(repo, func(context.Context, uint64) (upstream.Provider, error) {
		return &fakeMetricsProvider{}, nil
	})
	res, err := svc.MetricsCatalog(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("MetricsCatalog failed: %v", err)
	}
	if res.Supported || res.Reason == "" {
		t.Errorf("res = %+v, want supported=false with reason", res)
	}
	if res.Charts == nil {
		t.Errorf("Charts 应为空切片而不是 nil（前端 v-for 更稳）")
	}
}

func TestMetrics_RejectsMissingType(t *testing.T) {
	repo := &fakeRepo{items: []model.Instance{{ID: 1, ProviderInstanceID: "9", ProviderID: 3}}}
	svc := NewInstanceService(repo, func(context.Context, uint64) (upstream.Provider, error) {
		return &fakeMetricsProvider{}, nil
	})
	if _, err := svc.Metrics(context.Background(), 1, 1, &dto.MetricsQuery{}); !errors.Is(err, ErrMetricsTypeRequired) {
		t.Errorf("err = %v, want ErrMetricsTypeRequired", err)
	}
}

func TestMetrics_DefaultWindowIsSixHours(t *testing.T) {
	repo := &fakeRepo{items: []model.Instance{{ID: 1, InstanceID: "i-1", ProviderInstanceID: "9", ProviderID: 3}}}
	fake := &fakeMetricsProvider{series: &upstream.MetricSeries{Type: upstream.MetricCPU, Label: []string{"cpu"}}}
	svc := NewInstanceService(repo, func(context.Context, uint64) (upstream.Provider, error) { return fake, nil })
	before := time.Now().UnixMilli()
	got, err := svc.Metrics(context.Background(), 1, 1, &dto.MetricsQuery{Type: upstream.MetricCPU})
	if err != nil {
		t.Fatalf("Metrics failed: %v", err)
	}
	span := fake.wantEnd - fake.wantStart
	if span != (6 * time.Hour).Milliseconds() {
		t.Errorf("window = %d ms, want 6h", span)
	}
	if fake.wantEnd < before {
		t.Errorf("end = %d, want >= %d", fake.wantEnd, before)
	}
	if got.Type != upstream.MetricCPU || len(got.Label) != 1 {
		t.Errorf("series = %+v", got)
	}
}

func TestMetrics_UnknownProviderTypeUnsupported(t *testing.T) {
	repo := &fakeRepo{items: []model.Instance{{ID: 1, InstanceID: "i-1", ProviderInstanceID: "9", ProviderID: 3}}}
	// 解析出的 provider 不实现 InstanceMetrics（如仅电源能力）。
	svc := NewInstanceService(repo, func(context.Context, uint64) (upstream.Provider, error) {
		return &plainProvider{}, nil
	})
	if _, err := svc.Metrics(context.Background(), 1, 1, &dto.MetricsQuery{Type: upstream.MetricCPU}); !errors.Is(err, ErrMetricsUnsupported) {
		t.Errorf("err = %v, want ErrMetricsUnsupported", err)
	}
	cat, err := svc.MetricsCatalog(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("MetricsCatalog failed: %v", err)
	}
	if cat.Supported {
		t.Errorf("cat = %+v, want supported=false", cat)
	}
}

func TestOperations_MapsEntries(t *testing.T) {
	repo := &fakeRepo{
		items: []model.Instance{{ID: 7, ProviderInstanceID: "9"}},
		ops: []repository.OperationEntry{{
			ID: 11, Action: "power_off", OperatorType: "user", OperatorName: "alice",
			Result: "success", CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		}},
	}
	svc := NewInstanceService(repo, nil)
	rows, err := svc.Operations(context.Background(), 1, 7, 20)
	if err != nil {
		t.Fatalf("Operations failed: %v", err)
	}
	if len(rows) != 1 || rows[0].Action != "power_off" || rows[0].OperatorName != "alice" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].CreatedAt == "" {
		t.Errorf("CreatedAt 未格式化")
	}
}

// plainProvider 只有基础能力，用于验证「上游不支持监控」的分支。
type plainProvider struct{}

func (p *plainProvider) GetType() string                   { return "plain" }
func (p *plainProvider) GetName() string                   { return "plain" }
func (p *plainProvider) HealthCheck(context.Context) error { return nil }
func (p *plainProvider) Capabilities() upstream.CapabilityDescriptor {
	return upstream.CapabilityDescriptor{}
}
