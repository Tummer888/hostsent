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
	ID                uint64 `json:"id"`
	UserID            uint64 `json:"user_id"`
	Username          string `json:"username"`
	LoginType         string `json:"login_type"`
	Result            string `json:"result"`
	FailureReason     string `json:"failure_reason,omitempty"`
	IP                string `json:"ip"`
	UserAgent         string `json:"user_agent"`
	DeviceFingerprint string `json:"device_fingerprint"`
	Platform          string `json:"platform"`
	// SubjectType 登录主体域：user（客户）/ admin（员工后台）。
	// 两个 ID 空间共用 user_id 列且有撞号，前端展示必须带上它才不会把
	// 「员工后台登录」读成「同 ID 客户的登录」。
	SubjectType string    `json:"subject_type"`
	RiskFlag    string    `json:"risk_flag"`
	CreatedAt   time.Time `json:"created_at"`
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
	ID        uint64 `json:"id"`
	RiskType  string `json:"risk_type"`
	RiskLevel string `json:"risk_level"`
	UserID    uint64 `json:"user_id"`
	Username  string `json:"username"`
	// SubjectType 事件主体域：user（客户）/ admin（员工后台）。
	// 与登录日志同口径 —— user_id 列承载两个 ID 空间，不带上它就无法区分
	// 「客户 7 号」与「员工 7 号」。
	SubjectType       string    `json:"subject_type"`
	IP                string    `json:"ip"`
	DeviceFingerprint string    `json:"device_fingerprint"`
	RuleCode          string    `json:"rule_code"`
	Summary           string    `json:"summary"`
	DetailPayload     string    `json:"detail_payload"`
	OccurCount        int       `json:"occur_count"`
	FirstOccurredAt   time.Time `json:"first_occurred_at"`
	LastOccurredAt    time.Time `json:"last_occurred_at"`
	Status            string    `json:"status"`
	HandledBy         uint64    `json:"handled_by"`
	// HandledByName 处置人账号名（由服务层从 admins 表补全）。
	// 只回 ID 的话页面上只能显示一个数字，运营无法判断是谁处置的。
	HandledByName string     `json:"handled_by_name"`
	HandledAt     *time.Time `json:"handled_at,omitempty"`
	HandleNote    string     `json:"handle_note,omitempty"`
	// Actions 该事件已执行过的处置动作（去重后的动作码）。
	//
	// 与 Status 是两个维度：Status 只说明「待办关掉没有」，而拉黑、失效会话这类
	// 管控动作不改变待办状态，却必须让运营一眼看到「这条已经管控过了」。
	// 此前这些动作没有任何回显，点完页面上什么都不变，看起来像没生效。
	Actions []string `json:"actions"`
	// ActionSummary 处置动作的中文摘要（如「已拉黑 IP 1.2.3.4」），列表页直接展示。
	ActionSummary string    `json:"action_summary"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// RiskEventActionInfo 一条处置流水（详情抽屉的处置时间线）。
type RiskEventActionInfo struct {
	ID         uint64 `json:"id"`
	Action     string `json:"action"`
	OperatorID uint64 `json:"operator_id"`
	// OperatorName 操作人账号名（服务层从 admins 表补全）。
	OperatorName string    `json:"operator_name"`
	Note         string    `json:"note"`
	Detail       string    `json:"detail"`
	CreatedAt    time.Time `json:"created_at"`
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
	// CreatedByName / UpdatedByName 操作人账号名（服务层补全，同 HandledByName）。
	CreatedByName string `json:"created_by_name"`
	UpdatedByName string `json:"updated_by_name"`
	UpdatedBy     uint64 `json:"updated_by"`
	// RuntimeStatus 运行态：active / inactive / expired / pending。
	//
	// 与 status 分开：status 是运营开关（启用/停用），RuntimeStatus 叠加了
	// 生效时间与失效时间。只显示 status 的话，一条限时黑名单到期后页面上
	// 仍写「启用」，运营会以为还在拦人 —— 而它早就放行了。
	RuntimeStatus string    `json:"runtime_status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type SessionInfo struct {
	ID                uint64    `json:"id"`
	SessionID         string    `json:"session_id"`
	UserID            uint64    `json:"user_id"`
	Username          string    `json:"username"`
	Platform          string    `json:"platform"`
	IP                string    `json:"ip"`
	UserAgent         string    `json:"user_agent"`
	DeviceFingerprint string    `json:"device_fingerprint"`
	LoginAt           time.Time `json:"login_at"`
	LastActiveAt      time.Time `json:"last_active_at"`
	ExpiredAt         time.Time `json:"expired_at"`
	Status            string    `json:"status"`
	// SubjectType 会话主体域：user（客户）/ admin（员工后台），语义同 LoginLogInfo。
	SubjectType   string     `json:"subject_type"`
	RiskFlag      string     `json:"risk_flag"`
	RevokedReason string     `json:"revoked_reason,omitempty"`
	RevokedBy     uint64     `json:"revoked_by"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
