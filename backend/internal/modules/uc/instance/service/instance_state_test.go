package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"hostsent/backend/internal/modules/uc/instance/model"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
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
		// 已开通的实例（provider_instance_id 非空）才参与账期判定。
		{"正常在期 → active", model.Instance{ProviderInstanceID: "1", LifecycleStage: "active"}, expireStateNormal, 60, "active"},
		{"即将到期 → expiring", model.Instance{ProviderInstanceID: "1", LifecycleStage: "active"}, expireStateExpiring, 5, "expiring"},
		{"已过期 → expired", model.Instance{ProviderInstanceID: "1", LifecycleStage: "active"}, expireStateExpired, -3, "expired"},
		{"宽限期内 → grace", model.Instance{ProviderInstanceID: "1", LifecycleStage: "grace"}, expireStateExpired, -3, "grace"},
		{"已暂停 → suspended", model.Instance{ProviderInstanceID: "1", LifecycleStage: "suspended"}, expireStateExpired, -10, "suspended"},
		{"已销毁 → destroyed", model.Instance{ProviderInstanceID: "1", LifecycleStage: "destroyed"}, expireStateExpired, -30, "destroyed"},
		// 核心：**关机不影响账期** —— 客户自己关机，服务仍在有效期内。
		{"关机但未到期 → 仍是 active", model.Instance{ProviderInstanceID: "1", Status: "stopped", LifecycleStage: "active"}, expireStateNormal, 60, "active"},
		// 第十九轮新增：服务中 = 未到期 且 未暂停。
		//
		// 占位行（尚未开通）不得显示「服务中」：它没有交付物，界面此前把一台
		// 还没开出来的机器说成"服务在有效期内"（用户口径）。
		{"占位行（未开通）→ pending", model.Instance{LifecycleStage: "active"}, expireStateNormal, 60, "pending"},
		{"订单未付款超时 → cancelled", model.Instance{LifecycleStage: "cancelled"}, expireStateNormal, 60, "cancelled"},
		// 违规暂停与到期暂停必须分开：续费解决不了违规，客户该联系客服。
		{"违规暂停 → suspended_violation", model.Instance{ProviderInstanceID: "1", LifecycleStage: "suspended",
			SuspendReason: pkgmodel.SuspendReasonViolation}, expireStateNormal, 60, "suspended_violation"},
		{"到期自动暂停 → suspended", model.Instance{ProviderInstanceID: "1", LifecycleStage: "suspended",
			SuspendReason: pkgmodel.SuspendReasonExpired}, expireStateExpired, -10, "suspended"},
		{"人工暂停 → suspended（不冒充违规）", model.Instance{ProviderInstanceID: "1", LifecycleStage: "suspended",
			SuspendReason: pkgmodel.SuspendReasonManual}, expireStateNormal, 60, "suspended"},
		{"暂停但原因未知（存量行）→ suspended", model.Instance{ProviderInstanceID: "1", LifecycleStage: "suspended"}, expireStateNormal, 60, "suspended"},
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
		// 已开通（provider_instance_id 非空）：没有这一列会被判成"待开通"，
		// 服务维度就不会走账期计算（那是本轮修的另一条判据）。
		ProviderInstanceID: "i-1",
		LifecycleStage:     "active", ExpireAt: timePtr(time.Now().Add(60 * 24 * time.Hour)),
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

// TestCustomerApplyLiveStatusWaitsForTarget 客户侧回源也必须"到目标才算完成"。
//
// 与运维台同口径（见 admin 侧 TestLiveDetailPollFinishesTransitionOnlyOnTarget）。
// 回归背景：详情/列表轮询每几秒回源一次，上游还没开始执行时回的正是**动作前**的旧值；
// 若按"上游给了非空状态就算完成"，客户点完关机第一次轮询就把「关机中」清掉，
// 还把刚写入的前瞻状态冲回 running —— 客户以为关机没生效。
func TestCustomerApplyLiveStatusWaitsForTarget(t *testing.T) {
	// 上游仍报 stopped（动作前的旧值），本地刚下发"开机"乐观写入 running。
	upstreamStopped := &pkgmodel.StandardInstance{Status: pkgmodel.InstanceStatusStopped}
	prov := &slowControlProvider{}
	started := time.Now().Add(-2 * time.Minute) // 已越过 25s 最短窗口，隔离"收尾判据"这一个变量
	it := &model.Instance{
		ID: 1, Status: string(pkgmodel.InstanceStatusRunning), ProviderID: 7,
		LifecycleStage:      "active",
		TransitionState:     pkgmodel.InstanceTransitionStarting,
		TransitionStartedAt: &started,
	}
	info := toInfo(it) // 与生产路径一致：Transition 由 toInfo 填充后再交给 applyLiveStatus
	svc := &instanceService{
		repo:    &fakeRepo{items: []model.Instance{*it}},
		resolve: func(context.Context, uint64) (upstream.Provider, error) { return prov, nil },
	}
	svc.applyLiveStatus(context.Background(), it, upstreamStopped, &info)

	if info.Status != string(pkgmodel.InstanceStatusRunning) {
		t.Errorf("过渡未完成时应保留前瞻状态 running，实际 %q（上游旧值把它冲掉了）", info.Status)
	}
	if info.Transition == nil {
		t.Error("上游仍在停机状态时过渡态必须保留，否则界面提前结束「开机中」")
	}

	// 上游到达目标状态 running：这次才允许收尾。
	upstreamRunning := &pkgmodel.StandardInstance{Status: pkgmodel.InstanceStatusRunning}
	svc.applyLiveStatus(context.Background(), it, upstreamRunning, &info)
	if info.Status != string(pkgmodel.InstanceStatusRunning) {
		t.Errorf("收尾后状态应为 running，实际 %q", info.Status)
	}
	if info.Transition != nil {
		t.Errorf("上游已到 running（开机目标）应收尾，实际仍为 %+v", info.Transition)
	}
}

// TestTransitionJSONKeySurvivesClearing 收尾后 transition 键必须仍存在（值为 null）。
//
// 回归背景（用户实测反馈）：关机指令执行完、库里过渡态已清，详情页轮询每 4 秒
// 拿到的新数据里**没有** transition 键 —— 前端是增量合并（{...旧, ...新}），
// 键不存在在合并里什么都不覆盖，于是旧的「关机中」对象一直留在界面上：
// status 已随回源更新（概览显示已关机），头部动画却继续转，
// 要手动点「刷新状态」（整对象替换）才消失。
//
// 这条用例钉住的是**序列化契约**：字段带 omitempty 时会被整键省略，
// 任何人改回去都会在这里失败。
func TestTransitionJSONKeySurvivesClearing(t *testing.T) {
	idle := toInfo(&model.Instance{ID: 1, InstanceID: "x", Status: "running"})
	idle.Transition = nil
	b, err := json.Marshal(idle)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	raw, ok := m["transition"]
	if !ok {
		t.Fatal("transition 键必须始终出现（收尾后为 null）：键被省略时前端增量合并无法清除旧的过渡态，界面会一直显示「关机中」")
	}
	if string(raw) != "null" {
		t.Errorf("空闲时 transition 应为 null，实际 %s", raw)
	}

	// 有过渡态时必须是对象，且带 state（前端据此决定显示动画）。
	started := time.Now()
	busy := toInfo(&model.Instance{
		ID: 1, InstanceID: "x", Status: "running",
		TransitionState: pkgmodel.InstanceTransitionStopping, TransitionStartedAt: &started,
	})
	b2, _ := json.Marshal(busy)
	var m2 map[string]json.RawMessage
	_ = json.Unmarshal(b2, &m2)
	if got := string(m2["transition"]); got == "null" || got == "" {
		t.Errorf("有过渡态时 transition 必须是对象，实际 %s", got)
	}
}
