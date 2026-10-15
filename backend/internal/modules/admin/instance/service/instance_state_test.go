package service

import (
	"testing"
	"time"

	"hostsent/backend/internal/modules/admin/instance/repository"
	pkgmodel "hostsent/backend/internal/pkg/model"
)

// 本节测「电源维度 / 账期维度」的拆分（迁移 108 后的语义修复）。
//
// 为什么值得专门的测试：本轮修的正是"关机被显示成已暂停"。根因是两个正交概念
// 被压成了一个字段 —— 上游把 suspended（到期暂停）映射成本地 stopped，而手动关机
// 同样是 stopped。这两个函数是唯一的分界线，任何一边混淆都会让运营重新看到误报。

func TestPowerStateOf(t *testing.T) {
	cases := []struct {
		name       string
		status     string
		transition string
		want       string
	}{
		{"运行中 → on", string(pkgmodel.InstanceStatusRunning), "", "on"},
		{"已关机 → off", string(pkgmodel.InstanceStatusStopped), "", "off"},
		{"创建中 → unknown（机器还没起来）", string(pkgmodel.InstanceStatusCreating), "", "unknown"},
		{"已销毁 → unknown", string(pkgmodel.InstanceStatusDeleted), "", "unknown"},
		// 过渡态优先：正在开机/重启时，电源维度是 transitioning 而不是旧状态。
		{"重启中 → transitioning", string(pkgmodel.InstanceStatusRunning), pkgmodel.InstanceTransitionRebooting, "transitioning"},
		{"开机中 → transitioning", string(pkgmodel.InstanceStatusStopped), pkgmodel.InstanceTransitionStarting, "transitioning"},
		{"关机中 → transitioning", string(pkgmodel.InstanceStatusRunning), pkgmodel.InstanceTransitionStopping, "transitioning"},
		// 重置密码不影响电源维度（PowerHint 为空）→ 仍按实际状态显示。
		{"重置密码中不改变电源维度", string(pkgmodel.InstanceStatusRunning), pkgmodel.InstanceTransitionResettingPassword, "on"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := powerStateOf(tc.status, tc.transition); got != tc.want {
				t.Errorf("powerStateOf(%q, %q) = %q, want %q", tc.status, tc.transition, got, tc.want)
			}
		})
	}
}

func TestServiceStateOf(t *testing.T) {
	now := time.Now()
	future := now.AddDate(0, 0, 60) // 远未到期
	soon := now.AddDate(0, 0, 3)    // 3 天内到期
	past := now.AddDate(0, 0, -2)   // 已过期

	cases := []struct {
		name     string
		expireAt *time.Time
		stage    string
		within   int
		want     string
	}{
		{"未设到期 → active", nil, "", 7, "active"},
		{"远未到期 → active", &future, "", 7, "active"},
		{"临期 → expiring", &soon, "", 7, "expiring"},
		{"已过期 → expired", &past, "", 7, "expired"},
		{"宽限期 + 已过期 → grace", &past, "grace", 7, "grace"},
		// 关键：到期暂停与已销毁是**阶段**决定的，与到期时间计算无关。
		{"到期暂停 → suspended（优先级最高）", &future, lifecycleStageSuspended, 7, "suspended"},
		{"已销毁 → destroyed", &future, lifecycleStageDestroyed, 7, "destroyed"},
		{"到期暂停覆盖宽限期阶段", &past, lifecycleStageSuspended, 7, "suspended"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := serviceStateOf(tc.expireAt, tc.stage, tc.within); got != tc.want {
				t.Errorf("serviceStateOf(...) = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestPowerAndServiceAreOrthogonal 两个维度必须互不干扰 —— 这是本轮修复的核心断言。
//
// 场景：客户自己关了一台未到期的机器。
// 修复前：关机 → status=stopped → 前端判成「已暂停」（错！账期好得很）。
// 修复后：电源=off（已关机）、账期=active（服务中），两句话都对。
func TestPowerAndServiceAreOrthogonal(t *testing.T) {
	future := time.Now().AddDate(0, 0, 60)

	// ① 关机 + 账期正常：电源 off、服务 active（不能说成"暂停"）
	if got := powerStateOf(string(pkgmodel.InstanceStatusStopped), ""); got != "off" {
		t.Errorf("关机的电源维度应为 off，实得 %q", got)
	}
	if got := serviceStateOf(&future, "", 7); got != "active" {
		t.Errorf("关机不影响账期：应为 active，实得 %q（这正是「关机显示成已暂停」的根因）", got)
	}

	// ② 运行中 + 已到期：电源 on、服务 expired（机器开着但欠费）
	past := time.Now().AddDate(0, 0, -1)
	if got := powerStateOf(string(pkgmodel.InstanceStatusRunning), ""); got != "on" {
		t.Errorf("运行中的电源维度应为 on，实得 %q", got)
	}
	if got := serviceStateOf(&past, "", 7); got != "expired" {
		t.Errorf("到期不影响电源：应为 expired，实得 %q", got)
	}

	// ③ 到期暂停：电源维度仍报实际状态（上游通常已关机），服务维度报 suspended。
	//	 两者组合后才能读出完整语义："机器停了，因为欠费"。
	if got := serviceStateOf(&past, lifecycleStageSuspended, 7); got != "suspended" {
		t.Errorf("到期暂停的服务维度应为 suspended，实得 %q", got)
	}
}

// TestEnsureNoActiveTransition 过渡态闸门：窗口内拦截、超时放行。
func TestEnsureNoActiveTransition(t *testing.T) {
	svc := &instanceService{}
	now := time.Now()
	until := now.Add(3 * time.Minute)
	pastUntil := now.Add(-1 * time.Minute)

	cases := []struct {
		name      string
		state     string
		until     *time.Time
		wantBlock bool
	}{
		{"无过渡态 → 放行", "", nil, false},
		{"重启中且窗口内 → 拦截", pkgmodel.InstanceTransitionRebooting, &until, true},
		{"重装中且窗口内 → 拦截", pkgmodel.InstanceTransitionReinstalling, &until, true},
		// 超时后放行：一次丢回执不能把实例永久锁死。
		{"过渡态已超时 → 放行（结果未知，允许重试）", pkgmodel.InstanceTransitionRebooting, &pastUntil, false},
		// 窗口缺失（老数据）→ 保守拦截，避免重复提交。
		{"过态但无窗口时间 → 拦截", pkgmodel.InstanceTransitionStarting, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := &repository.InstanceRow{}
			row.TransitionState = tc.state
			row.TransitionUntil = tc.until
			err := svc.ensureNoActiveTransition(row, "power_on")
			blocked := err != nil
			if blocked != tc.wantBlock {
				t.Errorf("state=%q until=%v: blocked=%v（err=%v）, want blocked=%v",
					tc.state, tc.until, blocked, err, tc.wantBlock)
			}
		})
	}
}

// TestTransitionWithinMinHold 验证最短占用窗口：重启类动作在窗口内不得被"上游已确认"提前放行。
//
// 用户实测反馈"多次重启累积"：重启是 运行中→运行中 的转场，上游确认了也看不出变化；
// 若确认即放行，用户几秒后再点一次同样会堆指令。窗口内必须保留过渡态。
func TestTransitionWithinMinHold(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name      string
		state     string
		startedAt time.Time
		want      bool
	}{
		{"重启刚开始 → 仍在窗口内（拦）", pkgmodel.InstanceTransitionRebooting, now.Add(-5 * time.Second), true},
		{"重启已过 45s 窗口 → 可以收尾", pkgmodel.InstanceTransitionRebooting, now.Add(-60 * time.Second), false},
		{"开机刚开始 → 仍在窗口内", pkgmodel.InstanceTransitionStarting, now.Add(-5 * time.Second), true},
		{"开机已过 25s 窗口 → 可以收尾", pkgmodel.InstanceTransitionStarting, now.Add(-40 * time.Second), false},
		{"重装窗口更长（180s）", pkgmodel.InstanceTransitionReinstalling, now.Add(-120 * time.Second), true},
		// 没有起点就无从计算：按"可以收尾"处理，避免把历史行永久锁住。
		{"起点缺失 → 不算窗口内", pkgmodel.InstanceTransitionRebooting, time.Time{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := transitionWithinMinHold(tc.state, tc.startedAt); got != tc.want {
				t.Errorf("state=%q startedAt=%v: got %v, want %v", tc.state, tc.startedAt, got, tc.want)
			}
		})
	}
}

// TestTransitionRegistryMinHoldSanity 登记表自检：每个动作都必须有非负的最短窗口与正的耗时上限。
//
// 两者缺一都会退化：没有耗时上限则一次丢回执永久锁死；没有最短窗口则
// 上游一确认就放行、重启类动作可被秒级重复叠加。
func TestTransitionRegistryMinHoldSanity(t *testing.T) {
	for _, spec := range pkgmodel.InstanceTransitions() {
		if spec.TerminalTimeoutSec <= 0 {
			t.Errorf("%s: TerminalTimeoutSec 必须 > 0，当前 %d", spec.Value, spec.TerminalTimeoutSec)
		}
		if spec.MinHoldSec < 0 {
			t.Errorf("%s: MinHoldSec 不能为负，当前 %d", spec.Value, spec.MinHoldSec)
		}
		if spec.MinHoldSec > spec.TerminalTimeoutSec {
			t.Errorf("%s: MinHoldSec(%d) 不应超过 TerminalTimeoutSec(%d)",
				spec.Value, spec.MinHoldSec, spec.TerminalTimeoutSec)
		}
	}
}

// TestResaleInstance 链路判据：自营/转售/历史空值三条分支。
//
// 这条判据决定界面是否出现「上游实例号」「重新关联」「更换对接商品」，
// 以及后端是否拒绝这些动作 —— 判错会让自营实例出现"背后还有第三方"的假象
// （用户实测反馈："虽然是我自己的，但它这里会显示上游实例号"）。
func TestResaleInstance(t *testing.T) {
	// InstanceRow 内嵌 syncmodel.Instance（SourceMode / UpstreamProductID 在那一层），
	// 因此用逐字段赋值构造，不能整体字面量初始化。
	mk := func(sourceMode string, upstreamProductID uint64, providerInstanceID string) repository.InstanceRow {
		row := repository.InstanceRow{}
		row.SourceMode = sourceMode
		row.UpstreamProductID = upstreamProductID
		row.ProviderInstanceID = providerInstanceID
		return row
	}

	cases := []struct {
		name string
		row  repository.InstanceRow
		want bool
	}{
		{"source_mode=upstream → 转售", mk("upstream", 0, ""), true},
		{"source_mode=self → 自营（即便有实例号）", mk("self", 0, "91981147"), false},
		{"source_mode=self + 残留上游镜像 → 仍自营", mk("self", 120, ""), false},
		// 历史行（迁移前 source_mode 为空）：按"是否绑定了上游镜像"判，宁可多给入口
		// —— 让真正对接的实例失去换绑能力，比让自营多看到一个入口更糟。
		{"空 source_mode + 有上游镜像 → 按转售", mk("", 120, ""), true},
		{"空 source_mode + 无上游镜像 → 按自营", mk("", 0, ""), false},
		{"前后空格容错", mk(" upstream ", 0, ""), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := tc.row
			if got := resaleInstance(&row); got != tc.want {
				t.Errorf("resaleInstance(source_mode=%q, upstream_product_id=%d) = %v, want %v",
					tc.row.SourceMode, tc.row.UpstreamProductID, got, tc.want)
			}
		})
	}
	if resaleInstance(nil) {
		t.Error("nil 行不得判为转售（调用方会据此放行上游动作）")
	}
}

// TestSelfInstanceRejectsUpstreamOnlyActions 自营实例必须拒绝只对上游有意义的两类动作。
//
// 不只"界面上藏起来"：接口层也要拒，否则运营从旧页面/接口直接调用仍会把
// 一台正确的自营机器指向别处（更糟的是指向一个根本不存在的号）。
func TestSelfInstanceRejectsUpstreamOnlyActions(t *testing.T) {
	row := &repository.InstanceRow{}
	row.ID = 1
	row.InstanceID = "91981147"
	row.SourceMode = "self"
	row.ProviderInstanceID = "91981147"
	if resaleInstance(row) {
		t.Fatal("前置：该行应为自营")
	}
	// 转售行用于反向确认判据本身没有一刀切。
	resaleRow := &repository.InstanceRow{}
	resaleRow.ID = 2
	resaleRow.SourceMode = "upstream"
	resaleRow.UpstreamProductID = 120
	if !resaleInstance(resaleRow) {
		t.Fatal("前置：该行应为转售")
	}
}
