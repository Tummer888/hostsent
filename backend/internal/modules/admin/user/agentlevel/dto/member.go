// Package dto：代理分组成员（归属）的数据传输结构。
//
// 「代理分组」= 现有的 agent_levels（doc108 的代理等级）。归属关系就是
// users.agent_level_id —— 一个代理只属于一个分组，这正是二维结构里的纵向轴：
// 行 = 折扣组（绑定商品分组），列 = 代理分组，格 = 折扣率。
package dto

// MemberListQuery 成员列表查询。
//
// unassigned=true 时列出「尚未归属任何代理分组」的用户，供批量纳入分组；
// 否则列出该分组当前的成员。
type MemberListQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Keyword  string `form:"keyword"`
	// Unassigned 仅列未归属任何代理分组的用户（跨分组取候选，忽略分组 ID）。
	Unassigned bool `form:"unassigned"`
}

// AgentMemberInfo 代理分组成员（一个已归属/待归属的代理账号）。
type AgentMemberInfo struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	RealName string `json:"real_name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Status   string `json:"status"`
	// AgentLevelID nil = 未归属任何代理分组；未归属候选列表里恒为 nil。
	AgentLevelID   *uint64 `json:"agent_level_id"`
	AgentLevelName string  `json:"agent_level_name"`
	CreatedAt      string  `json:"created_at"`
}

// MemberListResponse 成员列表响应。
type MemberListResponse struct {
	Items []AgentMemberInfo `json:"items"`
	Meta  ListMeta          `json:"meta"`
	// UnassignedTotal 未归属任何代理分组的账号总数（弹窗里显示"还有 N 人可纳入"）。
	UnassignedTotal int64 `json:"unassigned_total"`
}

// 成员归属动作。
const (
	// MemberActionAssign 把选中账号纳入本分组（覆盖原有归属，含从别组转过来）。
	MemberActionAssign = "assign"
	// MemberActionRemove 把选中账号移出代理体系（agent_level_id 置 NULL）。
	MemberActionRemove = "remove"
)

// MemberAssignRequest 批量调整归属：一次把若干账号纳入或移出该代理分组。
//
// 批量是刻意的：逐个改用户要走两次请求且中途失败会留下"一半入组"的状态，
// 而归属是拿货价的依据，半途而废的可见性很差。
type MemberAssignRequest struct {
	UserIDs []uint64 `json:"user_ids" binding:"required,min=1"`
	// Action assign / remove。
	Action string `json:"action" binding:"required,oneof=assign remove"`
}

// MemberAssignSkip 被跳过的一条及原因（账号不存在/已注销/本就不在该组等）。
type MemberAssignSkip struct {
	UserID uint64 `json:"user_id"`
	Reason string `json:"reason"`
}

// MemberAssignResponse 批量调整结果。
type MemberAssignResponse struct {
	// Changed 实际被改动的账号数。
	Changed int `json:"changed"`
	// Skipped 未改动的一条条原因，前端逐条回显（与用户注销的批量语义一致）。
	Skipped []MemberAssignSkip `json:"skipped"`
}
