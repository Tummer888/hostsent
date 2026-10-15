package model

import "strings"

// 实例的**过渡态**：上游正在执行某个动作，结果尚未确认（迁移 112）。
//
// 为什么需要这一层：上游的电源/维护指令是异步的 —— 接口只回"已受理"，真正生效
// 要等十几秒到几分钟。这段窗口里 instances.status 还是**操作前**的旧值，
// 界面上看不出任何变化。后果有两个（用户实测反馈）：
//  1. 运营以为指令没生效，反复点同一个按钮，同一台机器堆了多次重启指令；
//  2. 刷新页面后连"刚才那个操作还在跑"都看不到。
//
// 因此在实例行上记录"进行中的动作"，直到回源确认新状态或被超时判定为结果未知。
const (
	InstanceTransitionStarting          = "starting"           // 开机中
	InstanceTransitionStopping          = "stopping"           // 关机中（含强制关机）
	InstanceTransitionRebooting         = "rebooting"          // 重启中（含强制重启）
	InstanceTransitionReinstalling      = "reinstalling"       // 重装系统中
	InstanceTransitionResettingPassword = "resetting_password" // 重置密码中
	InstanceTransitionRescueEntering    = "rescue_entering"    // 进入救援系统中
	InstanceTransitionRescueExiting     = "rescue_exiting"     // 退出救援系统中
)

// InstanceTransition 一条过渡态定义。
type InstanceTransition struct {
	// Value 落库值（instances.transition_state）。
	Value string `json:"value"`
	// Label 进行中的展示文案（"重启中"）。
	Label string `json:"label"`
	// Verb 下发给上游的动作名（用于提示语"正在执行：强制重启"）。
	Verb string `json:"verb"`
	// Idle 空闲态：该过渡态结束时电源维度应该回到什么状态（on/off/unknown）。
	//
	// 用于过渡窗口内的界面展示：重启中仍然是"通电"的（机器在跑），
	// 判定按钮可用性时不能因为"状态未知"就把开机/关机都禁掉。
	PowerHint string `json:"power_hint"`
	// TerminalTimeoutSec 该动作的典型耗时上限（秒）。
	//
	// 超过这个窗口仍未回源确认，就认为"结果未知"：不再拦截新指令
	// （否则一次丢回执会把实例永久锁死），但界面会提示上次结果未确认。
	// 取值按实测经验：电源动作 1-3 分钟；重装要重建系统盘，给到 15 分钟。
	TerminalTimeoutSec int `json:"terminal_timeout_sec"`
	// MinHoldSec 最短占用窗口（秒）：这段时间内**一定**拦住重复指令，界面显示「进行中」+动画。
	//
	// 为什么不能只靠"上游确认就放行"：重启的口径是 运行中→运行中，
	// 上游即使确认了也拿不到"变了的状态"，若立刻放行，用户几秒后再点一次
	// 同样会堆一条重启指令（用户实测反馈"多次重启累积"）。
	//
	// 对**不报电源状态**的渠道（如魔方财务）本值就是过渡态的全部意义：
	// 那种渠道永远无法确认，到期即按"指令已受理"收尾，窗口本身就是锁。
	//
	// 取值按"用户要等多久才合理"：机器重启/开关机的实际耗时通常在 20-60 秒；
	// 取比实际略长一点，避免动画刚停用户就以为可以再点。
	MinHoldSec int `json:"min_hold_sec"`
}

// instanceTransitionRegistry 过渡态登记表（唯一权威定义）。
var instanceTransitionRegistry = []InstanceTransition{
	{Value: InstanceTransitionStarting, Label: "开机中", Verb: "开机", PowerHint: "on", TerminalTimeoutSec: 180, MinHoldSec: 25},
	{Value: InstanceTransitionStopping, Label: "关机中", Verb: "关机", PowerHint: "off", TerminalTimeoutSec: 180, MinHoldSec: 25},
	{Value: InstanceTransitionRebooting, Label: "重启中", Verb: "重启", PowerHint: "on", TerminalTimeoutSec: 300, MinHoldSec: 40},
	{Value: InstanceTransitionReinstalling, Label: "重装中", Verb: "重装系统", PowerHint: "on", TerminalTimeoutSec: 900, MinHoldSec: 180},
	{Value: InstanceTransitionResettingPassword, Label: "重置密码中", Verb: "重置密码", PowerHint: "", TerminalTimeoutSec: 300, MinHoldSec: 45},
	{Value: InstanceTransitionRescueEntering, Label: "进入救援中", Verb: "进入救援系统", PowerHint: "", TerminalTimeoutSec: 300, MinHoldSec: 30},
	{Value: InstanceTransitionRescueExiting, Label: "退出救援中", Verb: "退出救援系统", PowerHint: "", TerminalTimeoutSec: 300, MinHoldSec: 30},
}

// InstanceTransitions 返回登记的全部过渡态（副本）。
func InstanceTransitions() []InstanceTransition {
	out := make([]InstanceTransition, len(instanceTransitionRegistry))
	copy(out, instanceTransitionRegistry)
	return out
}

// InstanceTransitionOf 按值取过渡态定义；未登记返回值本身与保守默认（3 分钟窗口）。
//
// 兜底而不是返回空：未知过渡值意味着"某个操作在进行"，宁可按最保守的窗口拦截一会儿，
// 也不要当作空闲放行 —— 那正是重复提交的来源。
func InstanceTransitionOf(value string) InstanceTransition {
	v := strings.TrimSpace(value)
	for _, t := range instanceTransitionRegistry {
		if t.Value == v {
			return t
		}
	}
	return InstanceTransition{Value: v, Label: v, Verb: v, TerminalTimeoutSec: 180}
}

// IsKnownInstanceTransition 判断是否为登记在册的过渡态。
func IsKnownInstanceTransition(value string) bool {
	v := strings.TrimSpace(value)
	if v == "" {
		return false
	}
	for _, t := range instanceTransitionRegistry {
		if t.Value == v {
			return true
		}
	}
	return false
}

// IsTransitionActive 过渡是否**仍在窗口内**（未超时）。
//
// 语义：只有"状态还没被回源确认、且没超过该动作的典型耗时"时才算进行中。
// 超时后返回 false —— 调用方据此放行新指令，避免一次丢回执把实例永久锁死。
func IsTransitionActive(state string, startedAt, until *int64, nowUnix int64) bool {
	if strings.TrimSpace(state) == "" {
		return false
	}
	if until != nil && nowUnix > *until {
		return false
	}
	return true
}
