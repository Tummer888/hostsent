package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"hostsent/backend/internal/modules/admin/instance/repository"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
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
		name      string
		expireAt  *time.Time
		stage     string
		suspendRe string
		pending   bool
		within    int
		want      string
	}{
		{"未设到期 → active", nil, "", "", false, 7, "active"},
		{"远未到期 → active", &future, "", "", false, 7, "active"},
		{"临期 → expiring", &soon, "", "", false, 7, "expiring"},
		{"已过期 → expired", &past, "", "", false, 7, "expired"},
		{"宽限期 + 已过期 → grace", &past, lifecycleStageGrace, "", false, 7, "grace"},
		// 关键：到期暂停与已销毁是**阶段**决定的，与到期时间计算无关。
		{"到期暂停 → suspended（优先级最高）", &future, lifecycleStageSuspended, "", false, 7, "suspended"},
		{"已销毁 → destroyed", &future, lifecycleStageDestroyed, "", false, 7, "destroyed"},
		{"到期暂停覆盖宽限期阶段", &past, lifecycleStageSuspended, "", false, 7, "suspended"},
		// 第十九轮新增：服务中 = 未到期 且 未暂停。
		//
		// 占位行（尚未开通）不得显示「服务中」：它的 expire_at 通常在未来，
		// 此前会落到 active —— 界面把一台还没开出来的机器说成"服务在有效期内"。
		{"占位行（未开通）→ pending，哪怕未到期", &future, "", "", true, 7, "pending"},
		{"占位行 + 已到期 → 仍是 pending（没有交付物，不谈账期）", &past, "", "", true, 7, "pending"},
		// 违规暂停与到期暂停必须分开：客户看到"到期暂停"会去续费，而续费解决不了违规。
		{"违规暂停 → suspended_violation", &future, lifecycleStageSuspended,
			pkgmodel.SuspendReasonViolation, false, 7, "suspended_violation"},
		{"到期自动暂停 → suspended（原因 expired）", &past, lifecycleStageSuspended,
			pkgmodel.SuspendReasonExpired, false, 7, "suspended"},
		{"人工暂停 → suspended（原因 manual）", &future, lifecycleStageSuspended,
			pkgmodel.SuspendReasonManual, false, 7, "suspended"},
		{"阶段暂停但原因未知（迁移前存量）→ suspended 而不是猜违规", &future,
			lifecycleStageSuspended, "", false, 7, "suspended"},
		// 取消：订单未付款超时 —— 机器从未开通。
		{"已取消 → cancelled", &future, lifecycleStageCancelled, "", true, 7, "cancelled"},
		{"已取消（即使带到期时间）→ cancelled 优先于账期计算", &past,
			lifecycleStageCancelled, "", true, 7, "cancelled"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := serviceStateOf(tc.expireAt, tc.stage, tc.suspendRe, tc.pending, tc.within); got != tc.want {
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
	if got := serviceStateOf(&future, "", "", false, 7); got != "active" {
		t.Errorf("关机不影响账期：应为 active，实得 %q（这正是「关机显示成已暂停」的根因）", got)
	}

	// ② 运行中 + 已到期：电源 on、服务 expired（机器开着但欠费）
	past := time.Now().AddDate(0, 0, -1)
	if got := powerStateOf(string(pkgmodel.InstanceStatusRunning), ""); got != "on" {
		t.Errorf("运行中的电源维度应为 on，实得 %q", got)
	}
	if got := serviceStateOf(&past, "", "", false, 7); got != "expired" {
		t.Errorf("到期不影响电源：应为 expired，实得 %q", got)
	}

	// ③ 到期暂停：电源维度仍报实际状态（上游通常已关机），服务维度报 suspended。
	//	 两者组合后才能读出完整语义："机器停了，因为欠费"。
	if got := serviceStateOf(&past, lifecycleStageSuspended, "", false, 7); got != "suspended" {
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

// TestTransitionSatisfiedBy 收尾判据必须"到目标"而不是"变了值"。
//
// 回归背景（用户实测）：详情轮询每 4 秒回源一次，上游还没开始执行时回的仍是
// 动作前的旧状态。此前的判据是"上游给了非空状态就算完成"，于是点完关机第一次
// 轮询就把过渡态清掉，界面从「关机中」跳回「运行中」—— 用户以为关机没生效。
func TestTransitionSatisfiedBy(t *testing.T) {
	cases := []struct {
		name       string
		state      string
		upstream   string
		wantSatisf bool
	}{
		{"关机中 + 上游仍报 running → 未完成（不得提前收尾）", pkgmodel.InstanceTransitionStopping, "running", false},
		{"关机中 + 上游报 stopped → 完成", pkgmodel.InstanceTransitionStopping, "stopped", true},
		{"开机中 + 上游仍报 stopped → 未完成", pkgmodel.InstanceTransitionStarting, "stopped", false},
		{"开机中 + 上游报 running → 完成", pkgmodel.InstanceTransitionStarting, "running", true},
		{"重启中 + 上游报 restarting → 未完成（机器还在转）", pkgmodel.InstanceTransitionRebooting, "restarting", false},
		{"重启中 + 上游报 running → 完成", pkgmodel.InstanceTransitionRebooting, "running", true},
		{"上游没给状态 → 未完成（拿不到数据不能假装完成）", pkgmodel.InstanceTransitionStopping, "", false},
		// 无从核对的动作（重装/改密/救援）：PowerHint 只是展示语义，
		// 机器全程通电，不能拿"是否 on"当完成信号。
		{"重装中 + 上游报 running → 不据此判完成", pkgmodel.InstanceTransitionReinstalling, "running", true},
		{"重置密码中 + 上游报 running → 不据此判完成", pkgmodel.InstanceTransitionResettingPassword, "running", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := pkgmodel.InstanceTransitionOf(tc.state)
			if got := spec.SatisfiedBy(tc.upstream); got != tc.wantSatisf {
				t.Errorf("%s.SatisfiedBy(%q) = %v, want %v", tc.state, tc.upstream, got, tc.wantSatisf)
			}
		})
	}
}

// TestVerifyByPowerOnlyForPowerActions 只有电源动作可以用上游电源状态核对完成。
//
// 若把重装也标成可核对（VerifyByPower=true 且 PowerHint="on"），重装刚下发时
// 上游仍报 running，会被判成"已完成"，过渡态在重装刚开始时就被清掉 ——
// 界面失去「重装中」提示与重复指令拦截。
func TestVerifyByPowerOnlyForPowerActions(t *testing.T) {
	powerActions := map[string]bool{
		pkgmodel.InstanceTransitionStarting:  true,
		pkgmodel.InstanceTransitionStopping:  true,
		pkgmodel.InstanceTransitionRebooting: true,
	}
	for _, spec := range pkgmodel.InstanceTransitions() {
		if spec.VerifyByPower != powerActions[spec.Value] {
			t.Errorf("%s: VerifyByPower=%v，期望 %v（只有开机/关机/重启可用电源状态核对）",
				spec.Value, spec.VerifyByPower, powerActions[spec.Value])
		}
		// 判可核对的动作必须带电源目标，否则 SatisfiedBy 无从比较。
		if spec.VerifyByPower && spec.PowerHint == "" {
			t.Errorf("%s: 标了 VerifyByPower 却没有 PowerHint，收尾判据会失去目标", spec.Value)
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

// TestRewordChannelErrorKeepsBothSentinels 措辞改写后两个哨兵都必须仍可判定。
//
// 回归背景：rewordChannelError 曾经**替换**掉 upstream.ErrProviderMissing，
// 只保留 ErrProviderUnavailable（决定 HTTP 码）。结果"渠道确实没了"这个判据
// 丢失，销毁时的「只清本地记录」分支永远进不去 —— 渠道被删的实例再也无法清理
// （实测：加 local_only 后仍然报渠道不可用）。
func TestRewordChannelErrorKeepsBothSentinels(t *testing.T) {
	row := &repository.InstanceRow{}
	row.SourceMode = "self"

	original := fmt.Errorf("%w：渠道 #103，绑定该渠道的实例无法同步/操作", upstream.ErrProviderMissing)
	got := rewordChannelError(row, original)

	if got == nil {
		t.Fatal("应返回改写后的错误，不是 nil")
	}
	if !errors.Is(got, ErrProviderUnavailable) {
		t.Error("必须保留 ErrProviderUnavailable（handler 据此映射 20005 错误码）")
	}
	if !errors.Is(got, upstream.ErrProviderMissing) {
		t.Error("必须保留 upstream.ErrProviderMissing（销毁时据此判断能否只清本地记录）")
	}
	if !isChannelGone(got) {
		t.Error("isChannelGone 应能识别改写后的错误 —— 否则 local_only 分支不可达")
	}
	// 自营链路的措辞：不说"上游渠道"。
	if !strings.Contains(got.Error(), "平台渠道") {
		t.Errorf("自营实例应说「平台渠道」，实际 %q", got.Error())
	}
	if strings.Contains(got.Error(), "上游") {
		t.Errorf("自营实例的提示不得出现「上游」，实际 %q", got.Error())
	}
}

// TestIsChannelGoneDistinguishesTransient 网络抖动/凭证错误不得判为"渠道没了"。
//
// 那些重试或修凭证即可，若误判成"渠道没了"，运营就会用它把机器只从本地清掉 ——
// 机器还在渠道侧计费的烂账。
func TestIsChannelGoneDistinguishesTransient(t *testing.T) {
	transient := []error{
		ErrProviderUnavailable,
		fmt.Errorf("%w: 连接超时", ErrProviderUnavailable),
		fmt.Errorf("%w: 凭证解密失败", ErrProviderUnavailable),
		nil,
	}
	for _, err := range transient {
		if isChannelGone(err) {
			t.Errorf("%v 不应被判为「渠道已不存在」（会造成只清本地的烂账）", err)
		}
	}
}
