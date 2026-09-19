package dto

type LoginLogListQuery struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	UserID    uint64 `form:"user_id"`
	Username  string `form:"username"`
	Result    string `form:"result"`
	LoginType string `form:"login_type"`
	IP        string `form:"ip"`
	RiskFlag  string `form:"risk_flag"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
}

type AuditLogListQuery struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"page_size"`
	Operator     string `form:"operator"`
	Module       string `form:"module"`
	Action       string `form:"action"`
	Result       string `form:"result"`
	ResourceType string `form:"resource_type"`
	ResourceID   string `form:"resource_id"`
	StartTime    string `form:"start_time"`
	EndTime      string `form:"end_time"`
}

type RiskEventListQuery struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
	// UserID 按用户过滤；缺这一项时 /security/risk-events?user_id=N 会被静默忽略，
	// 返回全量数据 —— 详情页「安全」Tab 因此拿到的是别人的风险事件。
	UserID    uint64 `form:"user_id"`
	RiskType  string `form:"risk_type"`
	RiskLevel string `form:"risk_level"`
	Status    string `form:"status"`
	Keyword   string `form:"keyword"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
}

type BlacklistListQuery struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	Type      string `form:"type"`
	Status    string `form:"status"`
	Source    string `form:"source"`
	Keyword   string `form:"keyword"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
}

type SessionListQuery struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	UserID    uint64 `form:"user_id"`
	Username  string `form:"username"`
	Status    string `form:"status"`
	Platform  string `form:"platform"`
	IP        string `form:"ip"`
	RiskFlag  string `form:"risk_flag"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
}

// BlacklistHitListQuery 黑名单命中记录的分页参数。
//
// 命中记录是「该黑名单在登录日志里命中的行」，与登录日志同构，
// 因此分页参数独立成一个小结构，而不是硬编码在仓储里（旧实现写死 1,10，
// 前端翻页翻不动、总数也不对）。
type BlacklistHitListQuery struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

type RiskEventHandleRequest struct {
	Note string `json:"note"`
}

type BlacklistCreateRequest struct {
	Type        string `json:"type" binding:"required"`
	TargetValue string `json:"target_value" binding:"required"`
	Status      string `json:"status"`
	Source      string `json:"source"`
	Reason      string `json:"reason"`
	ExpiredAt   string `json:"expired_at"`
}

type BlacklistUpdateRequest struct {
	Status    string `json:"status"`
	Reason    string `json:"reason"`
	ExpiredAt string `json:"expired_at"`
}

type BlacklistStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type SessionRevokeRequest struct {
	Reason string `json:"reason"`
}

type SessionBatchRevokeRequest struct {
	IDs    []uint64 `json:"ids" binding:"required"`
	Reason string   `json:"reason"`
}

type SessionRevokeUserAllRequest struct {
	UserID uint64 `json:"user_id" binding:"required"`
	Reason string `json:"reason"`
}
