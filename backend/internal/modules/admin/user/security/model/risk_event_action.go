package model

import "time"

// RiskEventAction 风险事件的一条处置流水（doc06 §4.3）。
//
// 与 risk_events.status 的分工：
//   - status 只回答「这条待办关掉没有」（pending / handled / ignored）；
//   - 本表回答「对这条事件做了什么管控」（调等级、拉黑、踢会话…）。
//
// 合并到 status 一列是不行的（改造前就是这么做的）：拉黑与踢会话不属于「关掉待办」，
// 但运营点完必须看到状态变化与留痕，否则会以为没生效。分开存之后，
// 一条事件可以既是 pending（还要盯）又已经拉黑（已经管控）。
type RiskEventAction struct {
	ID      uint64 `gorm:"primaryKey;autoIncrement"`
	EventID uint64 `gorm:"column:event_id;not null;index:idx_risk_event_actions_event,priority:1"`
	Action  string `gorm:"size:32;not null"`
	// OperatorID 操作人（admins.id）。0 表示系统动作（如规则引擎自动拉黑）。
	OperatorID uint64    `gorm:"column:operator_id;not null;default:0"`
	Note       string    `gorm:"size:255;not null;default:''"`
	Detail     string    `gorm:"type:text;not null;default:''"`
	CreatedAt  time.Time `gorm:"autoCreateTime;index:idx_risk_event_actions_event,priority:2,sort:desc"`
}

func (RiskEventAction) TableName() string {
	return "risk_event_actions"
}

// 处置动作码（落 risk_event_actions.action，同时是前端时间线的图标/文案键）。
const (
	RiskActionLevel          = "level"
	RiskActionHandle         = "handle"
	RiskActionIgnore         = "ignore"
	RiskActionBlacklist      = "blacklist"
	RiskActionRevokeSessions = "revoke_sessions"
)
