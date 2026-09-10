// Package dto 提供用户中心代理专区（P6-03）的数据传输结构。
package dto

// ProfileInfo 代理概览：等级、推广信息与业绩汇总。
type ProfileInfo struct {
	AgentID          uint64  `json:"agent_id"`
	LevelName        string  `json:"level_name"`
	LevelCode        string  `json:"level_code"`
	InviteCode       string  `json:"invite_code"`
	InviteLink       string  `json:"invite_link"`
	Status           string  `json:"status"`
	DirectSubCount   int     `json:"direct_sub_count"`
	TeamSubCount     int     `json:"team_sub_count"`
	TotalCommission  float64 `json:"total_commission"`
	AvailableBalance float64 `json:"available_balance"`
}

// ListQuery 代理专区分页查询。
type ListQuery struct {
	Status   string `form:"status"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	// LevelDepth 下级层级筛选（P7，0/空为全部）。
	LevelDepth int `form:"level_depth"`
}

// StatsInfo 代理后台看板汇总（P7）。
type StatsInfo struct {
	DirectSubCount    int64   `json:"direct_sub_count"`
	TeamSubCount      int64   `json:"team_sub_count"`
	PendingCommission float64 `json:"pending_commission"`
	SettledCommission float64 `json:"settled_commission"`
	PaidSettlement    float64 `json:"paid_settlement"`
	PendingSettlement float64 `json:"pending_settlement"`
}

// TeamMemberInfo 下级成员。
type TeamMemberInfo struct {
	ID                 uint64  `json:"id"`
	UserID             uint64  `json:"user_id"`
	Username           string  `json:"username"`
	Name               string  `json:"name"`
	Email              string  `json:"email"`
	LevelDepth         int     `json:"level_depth"`
	RelationType       string  `json:"relation_type"`
	ContributionAmount float64 `json:"contribution_amount"`
	CommissionAmount   float64 `json:"commission_amount"`
	Status             string  `json:"status"`
	JoinedAt           string  `json:"joined_at"`
}

// TeamListResponse 下级列表响应。
type TeamListResponse struct {
	Items    []TeamMemberInfo `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Total    int64            `json:"total"`
}

// CommissionInfo 佣金记录。
type CommissionInfo struct {
	ID             uint64  `json:"id"`
	OrderNo        string  `json:"order_no"`
	CommissionType string  `json:"commission_type"`
	SourceType     string  `json:"source_type"`
	BaseAmount     float64 `json:"base_amount"`
	Rate           float64 `json:"rate"`
	Amount         float64 `json:"amount"`
	Status         string  `json:"status"`
	Remark         string  `json:"remark"`
	CreatedAt      string  `json:"created_at"`
}

// CommissionListResponse 佣金列表响应。
type CommissionListResponse struct {
	Items    []CommissionInfo `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Total    int64            `json:"total"`
}

// SettlementInfo 结算单（代理视角）。
type SettlementInfo struct {
	ID              uint64  `json:"id"`
	SettlementNo    string  `json:"settlement_no"`
	PeriodStart     string  `json:"period_start"`
	PeriodEnd       string  `json:"period_end"`
	CommissionTotal float64 `json:"commission_total"`
	DeductionTotal  float64 `json:"deduction_total"`
	PayableTotal    float64 `json:"payable_total"`
	Status          string  `json:"status"`
	PaidAt          string  `json:"paid_at"`
	CreatedAt       string  `json:"created_at"`
}

// SettlementListResponse 结算单列表响应。
type SettlementListResponse struct {
	Items    []SettlementInfo `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Total    int64            `json:"total"`
}
