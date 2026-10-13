package service

import (
	"context"
	"strings"
	"testing"

	"hostsent/backend/internal/modules/uc/instance/model"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// TestPowerUpdatesLocalStatus 电源操作下发成功后必须把预期状态落到本地。
//
// 背景（用户实测）：点关机后接口返回成功，但页面状态不变 —— 因为用户侧 Power
// 只调了上游、没写本地；而列表/详情读的是本地 instances.status。
// 上游状态有延迟，不落预期状态的话用户界面上"什么都没发生"。
func TestPowerUpdatesLocalStatus(t *testing.T) {
	prov := &slowControlProvider{delay: 0}
	repo := &fakeRepo{items: []model.Instance{{
		ID: 7, InstanceID: "i-7", ProviderID: 1, ProviderInstanceID: "up-7",
		UserID: 2, Status: "running",
	}}}
	svc := &instanceService{
		repo:    repo,
		resolve: func(context.Context, uint64) (upstream.Provider, error) { return prov, nil },
	}

	if err := svc.Power(context.Background(), 2, 7, "off"); err != nil {
		t.Fatalf("Power: %v", err)
	}
	if len(repo.statusWrites) == 0 {
		t.Fatal("电源操作后未写本地状态（界面不会变化）")
	}
	// 首次写入应为预期状态 stopped；随后的回源对齐可能再写一次（桩返回 running）。
	if repo.statusWrites[0].Status != string(pkgmodel.InstanceStatusStopped) {
		t.Errorf("关机后首个状态写入应为 stopped，实际 %q", repo.statusWrites[0].Status)
	}
	if repo.statusWrites[0].ID != 7 {
		t.Errorf("状态写入目标错：%d", repo.statusWrites[0].ID)
	}
}

// TestPowerStartsUpdatesToRunning 开机同理写 running。
func TestPowerStartsUpdatesToRunning(t *testing.T) {
	prov := &slowControlProvider{delay: 0}
	repo := &fakeRepo{items: []model.Instance{{
		ID: 8, InstanceID: "i-8", ProviderID: 1, ProviderInstanceID: "up-8",
		UserID: 2, Status: "stopped",
	}}}
	svc := &instanceService{
		repo:    repo,
		resolve: func(context.Context, uint64) (upstream.Provider, error) { return prov, nil },
	}
	if err := svc.Power(context.Background(), 2, 8, "on"); err != nil {
		t.Fatalf("Power: %v", err)
	}
	if len(repo.statusWrites) == 0 || repo.statusWrites[0].Status != string(pkgmodel.InstanceStatusRunning) {
		t.Errorf("开机后应写 running，实际 %+v", repo.statusWrites)
	}
}

// TestPowerUpstreamFailureDoesNotWriteStatus 上游失败时不得改本地状态：
// 否则会出现"界面显示已关机、实际还在运行"的假象，比状态不变更危险。
func TestPowerUpstreamFailureDoesNotWriteStatus(t *testing.T) {
	prov := &failPowerProvider{}
	repo := &fakeRepo{items: []model.Instance{{
		ID: 9, InstanceID: "i-9", ProviderID: 1, ProviderInstanceID: "up-9",
		UserID: 2, Status: "running",
	}}}
	svc := &instanceService{
		repo:    repo,
		resolve: func(context.Context, uint64) (upstream.Provider, error) { return prov, nil },
	}
	if err := svc.Power(context.Background(), 2, 9, "off"); err == nil {
		t.Fatal("上游失败应返回错误")
	}
	if len(repo.statusWrites) != 0 {
		t.Errorf("上游失败不应写本地状态，实际写了 %+v", repo.statusWrites)
	}
}

// failPowerProvider：电源指令一律失败（模拟上游拒绝）。
type failPowerProvider struct{}

func (p *failPowerProvider) GetType() string                   { return "fake" }
func (p *failPowerProvider) GetName() string                   { return "fake" }
func (p *failPowerProvider) HealthCheck(context.Context) error { return nil }
func (p *failPowerProvider) Capabilities() upstream.CapabilityDescriptor {
	return upstream.CapabilityDescriptor{}
}
func (p *failPowerProvider) GetInstance(context.Context, string) (*pkgmodel.StandardInstance, error) {
	return nil, context.DeadlineExceeded
}
func (p *failPowerProvider) StartInstance(context.Context, string) error {
	return context.DeadlineExceeded
}
func (p *failPowerProvider) StopInstance(context.Context, string, bool) error {
	return context.DeadlineExceeded
}
func (p *failPowerProvider) RestartInstance(context.Context, string) error {
	return context.DeadlineExceeded
}
func (p *failPowerProvider) VNC(context.Context, string) (upstream.VNCResult, error) {
	return upstream.VNCResult{}, context.DeadlineExceeded
}

// TestCustomerProvisionErrorHidesUpstreamDetails 面向客户的失败原因不得含对接细节。
//
// provision_tasks.last_error 是上游 API 原文（含上游路径、HTTP 状态、上游业务码），
// 直接透给客户等于暴露供货链路；这里锁定"翻译后不含上游字样"。
func TestCustomerProvisionErrorHidesUpstreamDetails(t *testing.T) {
	cases := []struct {
		name          string
		raw           string
		mustNotHave   []string
		mustHaveChars bool
	}{
		{
			name:          "上游原文带接口路径与错误码",
			raw:           `upstream POST /index.php?m=api&a=cart: http=500 code=123 msg=余额不足`,
			mustNotHave:   []string{"upstream", "index.php", "http=500", "code=123"},
			mustHaveChars: true,
		},
		{
			name:          "主机名冲突",
			raw:           `upstream POST /provision: code=400 msg=主机名已存在`,
			mustNotHave:   []string{"upstream", "provision", "code=400"},
			mustHaveChars: true,
		},
		{
			name:          "未知错误一律中性结论",
			raw:           `upstream GET /secret/endpoint: code=999 msg=内部异常`,
			mustNotHave:   []string{"upstream", "secret", "code=999", "内部异常"},
			mustHaveChars: true,
		},
		{
			name:          "空错误原样为空",
			raw:           "",
			mustNotHave:   nil,
			mustHaveChars: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := customerProvisionError(tc.raw)
			for _, bad := range tc.mustNotHave {
				if strings.Contains(got, bad) {
					t.Errorf("客户可见原因仍含内部信息 %q：%q", bad, got)
				}
			}
			if tc.mustHaveChars && len(got) == 0 {
				t.Error("应有可读的原因说明，实际为空")
			}
			if !tc.mustHaveChars && got != "" {
				t.Errorf("空输入应返回空串，实际 %q", got)
			}
		})
	}
}

// TestExpectedPowerStatus 状态映射：开关机有明确落库值，重启不落（转场态无意义）。
func TestExpectedPowerStatus(t *testing.T) {
	cases := []struct {
		action, current, want string
	}{
		{"off", "running", string(pkgmodel.InstanceStatusStopped)},
		{"hard_off", "running", string(pkgmodel.InstanceStatusStopped)},
		{"on", "stopped", string(pkgmodel.InstanceStatusRunning)},
		{"reboot", "running", ""},
		{"hard_reboot", "running", ""},
	}
	for _, tc := range cases {
		if got := expectedPowerStatus(tc.action, tc.current); got != tc.want {
			t.Errorf("expectedPowerStatus(%q) = %q, want %q", tc.action, got, tc.want)
		}
	}
}
