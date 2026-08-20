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
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	RiskType  string `form:"risk_type"`
	RiskLevel string `form:"risk_level"`
	Status    string `form:"status"`
	Keyword   string `form:"keyword"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
}

type BlacklistListQuery struct {
	Page        int    `form:"page"`
	PageSize    int    `form:"page_size"`
	Type        string `form:"type"`
	Status      string `form:"status"`
	Source      string `form:"source"`
	Keyword     string `form:"keyword"`
	StartTime   string `form:"start_time"`
	EndTime     string `form:"end_time"`
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
