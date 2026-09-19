// Package model 提供实名认证模块的数据模型。
package model

import "time"

// 实名认证申请状态（doc104 §5.1）。
//
// 只有三个状态：整单通过 / 整单驳回是需求方的明确口径（D2），
// 不引入 supplement_required —— 需要补材料时驳回并说明即可。
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
)

// 认证主体类型。
const (
	TypePersonal   = "personal"
	TypeEnterprise = "enterprise"
)

// 三方核验结果。
const (
	ProviderResultPass  = "pass"
	ProviderResultFail  = "fail"
	ProviderResultError = "error"
)

// 审核动作（写入 verification_review_logs.action）。
const (
	ActionSubmit       = "submit"
	ActionApprove      = "approve"
	ActionReject       = "reject"
	ActionRevoke       = "revoke"
	ActionProviderPass = "provider_pass"
	ActionProviderFail = "provider_fail"
)

// IsValidStatus 判断申请状态是否在枚举内。
func IsValidStatus(s string) bool {
	switch s {
	case StatusPending, StatusApproved, StatusRejected:
		return true
	default:
		return false
	}
}

// VerificationApplication 表示实名认证申请主记录。
type VerificationApplication struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement"`
	UserID           uint64     `gorm:"column:user_id;not null;index"`
	Username         string     `gorm:"column:username;size:64;not null;index"`
	VerificationType string     `gorm:"column:verification_type;size:32;not null;index"`
	Status           string     `gorm:"column:status;size:32;not null;index"`
	RealName         string     `gorm:"column:real_name;size:128;not null"`
	SubjectName      string     `gorm:"column:subject_name;size:255;not null"`
	IDType           string     `gorm:"column:id_type;size:32;not null"`
	IDNumberMasked   string     `gorm:"column:id_number_masked;size:64;not null"`
	MobileMasked     string     `gorm:"column:mobile_masked;size:32"`
	CountryCode      string     `gorm:"column:country_code;size:16"`
	RiskFlags        string     `gorm:"column:risk_flags;size:255"`
	SubmittedAt      time.Time  `gorm:"column:submitted_at;not null;index"`
	ReviewedAt       *time.Time `gorm:"column:reviewed_at;index"`
	ReviewedBy       *uint64    `gorm:"column:reviewed_by;index"`
	ReviewerName     string     `gorm:"column:reviewer_name;size:64"`
	RejectReasonCode string     `gorm:"column:reject_reason_code;size:64"`
	RejectReason     string     `gorm:"column:reject_reason;size:255"`
	ReviewNote       string     `gorm:"column:review_note;size:255"`
	Version          int        `gorm:"column:version;not null;default:1"`
	// —— 迁移 051 新增（doc104 §5.2）——
	// IDNumberEncrypted / MobileEncrypted 证件号与手机号密文（pkg/credentials 字段级加密）。
	// 三方核验必须用原文，脱敏值不可用，因此密文与 masked 列并存。
	IDNumberEncrypted string `gorm:"column:id_number_encrypted;type:text;not null;default:''"`
	MobileEncrypted   string `gorm:"column:mobile_encrypted;type:text;not null;default:''"`
	// SubmittedBy 0 = 用户自助；>0 = 运营代提交的管理员 ID。
	SubmittedBy uint64 `gorm:"column:submitted_by;not null;default:0"`
	// Provider 生效的核验服务商（manual / alipay）；空表示未走三方核验。
	Provider string `gorm:"column:provider;size:32;not null;default:''"`
	// ProviderTxnNo 三方流水号（查结果用）。
	ProviderTxnNo string `gorm:"column:provider_txn_no;size:128;not null;default:''"`
	// ProviderResult pass / fail / error；空表示未核验。
	ProviderResult string `gorm:"column:provider_result;size:32;not null;default:''"`
	// ProviderMessage 三方返回摘要。**只存摘要，绝不存原始报文**（doc104 §5.5）。
	ProviderMessage   string     `gorm:"column:provider_message;size:255;not null;default:''"`
	ProviderCheckedAt *time.Time `gorm:"column:provider_checked_at"`
	// ReviewRound 第几次提交（同一用户可多次提交，驳回后冷却期满可再来）。
	ReviewRound int       `gorm:"column:review_round;not null;default:1"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TableName 返回实名认证申请主表名。
func (VerificationApplication) TableName() string {
	return "verification_applications"
}
