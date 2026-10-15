package service

import (
	"testing"
	"time"

	"hostsent/backend/internal/modules/uc/instance/model"
	pkgmodel "hostsent/backend/internal/pkg/model"
)

// TestPowerStateOf 客户侧电源维度映射：只回答"机器通电了吗"。
func TestPowerStateOf(t *testing.T) {
	future := time.Now().Add(2 * time.Minute)
	past := time.Now().Add(-5 * time.Minute)

	cases := []struct {
		name string
		it   model.Instance
		want string
	}{
		{"运行中 → on", model.Instance{Status: "running"}, "on"},
		{"已关机 → off", model.Instance{Status: "stopped"}, "off"},
		{"创建中 → unknown", model.Instance{Status: "creating"}, "unknown"},
		{
			"开机中（窗口内）→ transitioning",
			model.Instance{Status: "running", TransitionState: pkgmodel.InstanceTransitionStarting, TransitionUntil: &future},
			"transitioning",
		},
		{
			"重启中（窗口内）→ transitioning",
			model.Instance{Status: "running", TransitionState: pkgmodel.InstanceTransitionRebooting, TransitionUntil: &future},
			"transitioning",
		},
		{
			// 窗口已过：结果未知，回落到真实状态判断，界面不再转圈。
			"过渡态超时 → 回落真实状态",
			model.Instance{Status: "stopped", TransitionState: pkgmodel.InstanceTransitionStopping, TransitionUntil: &past},
			"off",
		},
		{
			// 重置密码不改电源语义（PowerHint 为空）：不该显示成"电源变化中"。
			"重置密码中 → 不算电源过渡",
			model.Instance{Status: "running", TransitionState: pkgmodel.InstanceTransitionResettingPassword, TransitionUntil: &future},
			"on",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := powerStateOf(&tc.it); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestServiceStateOf 客户侧账期维度映射：关机不影响账期。
func TestServiceStateOf(t *testing.T) {
	cases := []struct {
		name     string
		it       model.Instance
		expireSt string
		daysLeft int
		want     string
	}{
		{"正常在期 → active", model.Instance{LifecycleStage: "active"}, expireStateNormal, 60, "active"},
		{"即将到期 → expiring", model.Instance{LifecycleStage: "active"}, expireStateExpiring, 5, "expiring"},
		{"已过期 → expired", model.Instance{LifecycleStage: "active"}, expireStateExpired, -3, "expired"},
		{"宽限期内 → grace", model.Instance{LifecycleStage: "grace"}, expireStateExpired, -3, "grace"},
		{"已暂停 → suspended", model.Instance{LifecycleStage: "suspended"}, expireStateExpired, -10, "suspended"},
		{"已销毁 → destroyed", model.Instance{LifecycleStage: "destroyed"}, expireStateExpired, -30, "destroyed"},
		// 核心：**关机不影响账期** —— 客户自己关机，服务仍在有效期内。
		{"关机但未到期 → 仍是 active", model.Instance{Status: "stopped", LifecycleStage: "active"}, expireStateNormal, 60, "active"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := serviceStateOf(&tc.it, tc.expireSt, tc.daysLeft); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestToInfoCarriesTwoDimensions 下发结构必须同时带两个维度与过渡态。
//
// 客户端据此渲染两个独立标签并驱动动画；缺任何一个都会退回旧的启发式，
// 也就是"关机显示成已暂停"那类问题。
func TestToInfoCarriesTwoDimensions(t *testing.T) {
	future := time.Now().Add(2 * time.Minute)
	started := time.Now().Add(-20 * time.Second)
	it := &model.Instance{
		ID: 1, InstanceID: "i-1", Name: "h", Status: "stopped",
		LifecycleStage: "active", ExpireAt: timePtr(time.Now().Add(60 * 24 * time.Hour)),
		TransitionState:     pkgmodel.InstanceTransitionStarting,
		TransitionStartedAt: &started,
		TransitionUntil:     &future,
	}
	info := toInfo(it)
	if info.PowerState != "transitioning" {
		t.Errorf("power_state 应为 transitioning，实际 %q", info.PowerState)
	}
	if info.ServiceState != "active" {
		t.Errorf("service_state 应为 active（关机不影响账期），实际 %q", info.ServiceState)
	}
	if info.Transition == nil {
		t.Fatal("过渡态必须下发（客户端据此显示动画并禁用按钮）")
	}
	if info.Transition.Label != "开机中" {
		t.Errorf("过渡态文案应为「开机中」，实际 %q", info.Transition.Label)
	}
	if info.Transition.StartedAt == "" {
		t.Error("过渡态必须带开始时间（客户端据此显示已等待时长）")
	}
}

// TestToInfoTransitionExpiredNotOverdue 窗口未过时 Overdue 为 false。
func TestToInfoTransitionOverdue(t *testing.T) {
	past := time.Now().Add(-5 * time.Minute)
	it := &model.Instance{
		ID: 2, Status: "running", LifecycleStage: "active",
		TransitionState: pkgmodel.InstanceTransitionRebooting, TransitionUntil: &past,
	}
	info := toInfo(it)
	if info.Transition == nil {
		t.Fatal("超时的过渡态仍要下发（界面提示「结果未确认，请刷新」）")
	}
	if !info.Transition.Overdue {
		t.Error("窗口已过时 Overdue 应为 true，否则客户端会一直转圈")
	}
}

func timePtr(t time.Time) *time.Time { return &t }
