package dto

import "time"

type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type ListResponse[T any] struct {
	Items []T      `json:"items"`
	Meta  ListMeta `json:"meta"`
}

type APIResponse[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      T      `json:"data"`
	Timestamp int64  `json:"timestamp"`
}

type LoginLogInfo struct {
	ID                uint64    `json:"id"`
	UserID            uint64    `json:"user_id"`
	Username          string    `json:"username"`
	LoginType         string    `json:"login_type"`
	Result            string    `json:"result"`
	FailureReason     string    `json:"failure_reason,omitempty"`
	IP                string    `json:"ip"`
	IPRegion          string    `json:"ip_region"`
	UserAgent         string    `json:"user_agent"`
	DeviceFingerprint string    `json:"device_fingerprint"`
	Platform          string    `json:"platform"`
	RiskFlag          string    `json:"risk_flag"`
	CreatedAt         time.Time `json:"created_at"`
}

type AuditLogInfo struct {
	ID              uint64    `json:"id"`
	OperatorID      uint64    `json:"operator_id"`
	OperatorName    string    `json:"operator_name"`
	Module          string    `json:"module"`
	ResourceType    string    `json:"resource_type"`
	ResourceID      string    `json:"resource_id"`
	Action          string    `json:"action"`
	RequestMethod   string    `json:"request_method"`
	RequestPath     string    `json:"request_path"`
	RequestPayload  string    `json:"request_payload"`
	ResponseCode    int       `json:"response_code"`
	ResponseMessage string    `json:"response_message"`
	IP              string    `json:"ip"`
	UserAgent       string    `json:"user_agent"`
	TraceID         string    `json:"trace_id"`
	CreatedAt       time.Time `json:"created_at"`
}

type RiskEventInfo struct {
	ID                uint64     `json:"id"`
	RiskType          string     `json:"risk_type"`
	RiskLevel         string     `json:"risk_level"`
	UserID            uint64     `json:"user_id"`
	Username          string     `json:"username"`
	IP                string     `json:"ip"`
	DeviceFingerprint string     `json:"device_fingerprint"`
	RuleCode          string     `json:"rule_code"`
	Summary           string     `json:"summary"`
	DetailPayload     string     `json:"detail_payload"`
	OccurCount        int        `json:"occur_count"`
	FirstOccurredAt   time.Time  `json:"first_occurred_at"`
	LastOccurredAt    time.Time  `json:"last_occurred_at"`
	Status            string     `json:"status"`
	HandledBy         uint64     `json:"handled_by"`
	HandledAt         *time.Time `json:"handled_at,omitempty"`
	HandleNote        string     `json:"handle_note,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type BlacklistInfo struct {
	ID          uint64     `json:"id"`
	Type        string     `json:"type"`
	TargetValue string     `json:"target_value"`
	Status      string     `json:"status"`
	Source      string     `json:"source"`
	Reason      string     `json:"reason"`
	EffectiveAt time.Time  `json:"effective_at"`
	ExpiredAt   *time.Time `json:"expired_at,omitempty"`
	HitCount    int        `json:"hit_count"`
	CreatedBy   uint64     `json:"created_by"`
	UpdatedBy   uint64     `json:"updated_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type SessionInfo struct {
	ID                uint64     `json:"id"`
	SessionID         string     `json:"session_id"`
	UserID            uint64     `json:"user_id"`
	Username          string     `json:"username"`
	Platform          string     `json:"platform"`
	IP                string     `json:"ip"`
	IPRegion          string     `json:"ip_region"`
	UserAgent         string     `json:"user_agent"`
	DeviceFingerprint string     `json:"device_fingerprint"`
	LoginAt           time.Time  `json:"login_at"`
	LastActiveAt      time.Time  `json:"last_active_at"`
	ExpiredAt         time.Time  `json:"expired_at"`
	Status            string     `json:"status"`
	RiskFlag          string     `json:"risk_flag"`
	RevokedReason     string     `json:"revoked_reason,omitempty"`
	RevokedBy         uint64     `json:"revoked_by"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
