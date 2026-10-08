package model

import "time"

type RiskEvent struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement"`
	RiskType  string `gorm:"column:risk_type;size:64;not null;index"`
	RiskLevel string `gorm:"column:risk_level;size:32;not null;index"`
	UserID    uint64 `gorm:"column:user_id;not null;index"`
	Username  string `gorm:"size:64;not null;index"`
	// SubjectType 事件主体域（user/admin），语义同 login_logs.subject_type：
	// user_id 列承载 users.id 与 admins.id 两个 ID 空间，天然会撞号，不区分域会把
	// 「某个员工后台的异常登录」记到 ID 相同的客户头上。默认 user 保证既有写入方
	// 不关心本列时天然正确。
	SubjectType       string     `gorm:"column:subject_type;size:16;not null;default:user;index:idx_risk_events_subject_user,priority:1"`
	IP                string     `gorm:"column:ip;size:64;index"`
	DeviceFingerprint string     `gorm:"column:device_fingerprint;size:255"`
	RuleCode          string     `gorm:"column:rule_code;size:128;not null;index"`
	Summary           string     `gorm:"size:255;not null"`
	DetailPayload     string     `gorm:"column:detail_payload;type:text"`
	OccurCount        int        `gorm:"column:occur_count;not null;default:1"`
	FirstOccurredAt   time.Time  `gorm:"column:first_occurred_at;not null;index"`
	LastOccurredAt    time.Time  `gorm:"column:last_occurred_at;not null;index"`
	Status            string     `gorm:"size:32;not null;index"`
	HandledBy         *uint64    `gorm:"column:handled_by;index"`
	HandledAt         *time.Time `gorm:"column:handled_at"`
	HandleNote        string     `gorm:"column:handle_note;size:255"`
	CreatedAt         time.Time  `gorm:"autoCreateTime"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime"`
}

func (RiskEvent) TableName() string {
	return "risk_events"
}

// 风险等级（doc06 §4.3 建议值；critical 为严重级，此前只有枚举、无写入方）。
// 主体域常量 SubjectTypeUser / SubjectTypeAdmin 见 login_log.go（同包内唯一定义）。
const (
	RiskLevelLow      = "low"
	RiskLevelMedium   = "medium"
	RiskLevelHigh     = "high"
	RiskLevelCritical = "critical"
)

// 处置状态。
const (
	RiskStatusPending = "pending"
	RiskStatusIgnored = "ignored"
	RiskStatusHandled = "handled"
)

// RiskLevels 全部合法等级（「手动升级风险等级」接口的入参校验基准）。
func RiskLevels() []string {
	return []string{RiskLevelLow, RiskLevelMedium, RiskLevelHigh, RiskLevelCritical}
}

// Blacklist 状态与来源（与前端枚举对齐；expired 不是落库状态 ——
// 「已过期」由 effective/expired_at 与当前时间推导，存状态就必然要有人定时改）。
const (
	BlacklistStatusActive   = "active"
	BlacklistStatusInactive = "inactive"

	BlacklistSourceManual    = "manual"
	BlacklistSourceSystem    = "system"
	BlacklistSourceRiskEvent = "risk_event"
)

// BlacklistTypes 全部合法黑名单类型（新增/编辑校验基准，doc06 §4.4）。
func BlacklistTypes() []string {
	return []string{"ip", "user", "device", "phone", "email"}
}

// IsBlacklistType 校验类型是否受支持。
func IsBlacklistType(t string) bool {
	for _, v := range BlacklistTypes() {
		if v == t {
			return true
		}
	}
	return false
}
