// Package dto 提供实名认证模块的请求与响应数据结构。
package dto

import (
	"time"

	"hostsent/backend/internal/pkg/integration"
)

// ListMeta 表示分页元信息。
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// ListResponse 表示分页列表响应。
type ListResponse[T any] struct {
	Items []T      `json:"items"`
	Meta  ListMeta `json:"meta"`
}

// APIResponse 表示统一 API 响应格式。
type APIResponse[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      T      `json:"data"`
	Timestamp int64  `json:"timestamp"`
}

// VerificationInfo 表示实名认证记录详情。
type VerificationInfo struct {
	ID               uint64     `json:"id"`
	UserID           uint64     `json:"user_id"`
	Username         string     `json:"username"`
	VerificationType string     `json:"verification_type"`
	Status           string     `json:"status"`
	RealName         string     `json:"real_name"`
	SubjectName      string     `json:"subject_name"`
	IDType           string     `json:"id_type"`
	IDNumberMasked   string     `json:"id_number_masked"`
	MobileMasked     string     `json:"mobile_masked"`
	RiskFlags        string     `json:"risk_flags"`
	SubmittedAt      time.Time  `json:"submitted_at"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	ReviewedBy       *uint64    `json:"reviewed_by,omitempty"`
	ReviewerName     string     `json:"reviewer_name"`
	RejectReasonCode string     `json:"reject_reason_code,omitempty"`
	RejectReason     string     `json:"reject_reason,omitempty"`
	ReviewNote       string     `json:"review_note,omitempty"`
	// —— 迁移 051 新增（doc104 §5.2）——
	Provider          string     `json:"provider"`
	ProviderResult    string     `json:"provider_result"`
	ProviderMessage   string     `json:"provider_message"`
	ProviderCheckedAt *time.Time `json:"provider_checked_at,omitempty"`
	ReviewRound       int        `json:"review_round"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// DocumentInfo 申请附件。
type DocumentInfo struct {
	ID           uint64 `json:"id"`
	DocumentType string `json:"document_type"`
	FileURL      string `json:"file_url"`
	Sort         int    `json:"sort"`
}

// EnterpriseInfo 企业认证扩展信息（个人认证为空）。
type EnterpriseInfo struct {
	CompanyName      string `json:"company_name"`
	CreditCodeMasked string `json:"credit_code_masked"`
	LegalPersonName  string `json:"legal_person_name"`
	ContactName      string `json:"contact_name"`
	BusinessLicense  string `json:"business_license"`
}

// ReviewLogInfo 审核轨迹条目。
type ReviewLogInfo struct {
	ID               uint64    `json:"id"`
	FromStatus       string    `json:"from_status"`
	ToStatus         string    `json:"to_status"`
	Action           string    `json:"action"`
	OperatorID       uint64    `json:"operator_id"`
	OperatorName     string    `json:"operator_name"`
	Note             string    `json:"note"`
	RejectReasonCode string    `json:"reject_reason_code"`
	RejectReason     string    `json:"reject_reason"`
	CreatedAt        time.Time `json:"created_at"`
}

// VerificationDetail 申请详情（主记录 + 附件 + 企业信息 + 审核轨迹）。
type VerificationDetail struct {
	VerificationInfo
	Documents    []DocumentInfo            `json:"documents"`
	Enterprise   *EnterpriseInfo           `json:"enterprise,omitempty"`
	Logs         []ReviewLogInfo           `json:"logs"`
	CertifyModes []integration.FieldOption `json:"certify_modes"`
}

// ReviewRequest 整单审核请求（通过 / 驳回 / 撤销共用）。
//
// 驳回时 RejectReason 必填（服务层与前端各校验一次）；通过/撤销时忽略。
type ReviewRequest struct {
	RejectReasonCode string `json:"reject_reason_code"`
	RejectReason     string `json:"reject_reason"`
	Note             string `json:"note"`
}

// ProviderCheckResult 手动触发三方核验的结果。
//
// OK=false 不等于出错：支付宝属跳转式核验，手动触发会返回
// 「该服务商不支持无跳转核验」——这是业务状态，HTTP 仍是 200。
type ProviderCheckResult struct {
	OK           bool   `json:"ok"`
	Provider     string `json:"provider"`
	Passed       bool   `json:"passed"`
	BizCode      string `json:"biz_code"`
	Message      string `json:"message"`
	TxnNo        string `json:"txn_no"`
	AutoApproved bool   `json:"auto_approved"`
	AutoRejected bool   `json:"auto_rejected"`
}

// ProviderTypeInfo 核验服务商类型元数据（描述符驱动前端动态表单）。
type ProviderTypeInfo struct {
	Type             string                    `json:"type"`
	Name             string                    `json:"name"`
	Mode             string                    `json:"mode"`
	Icon             string                    `json:"icon"`
	DocURL           string                    `json:"doc_url"`
	AdapterVersion   string                    `json:"adapter_version"`
	Implemented      bool                      `json:"implemented"`
	Builtin          bool                      `json:"builtin"`
	CredentialSchema []integration.Field       `json:"credential_schema"`
	CertifyModes     []integration.FieldOption `json:"certify_modes"`
}

// ProviderInfo 核验服务商配置（凭证只回脱敏值）。
type ProviderInfo struct {
	ID             uint64                    `json:"id"`
	ProviderType   string                    `json:"provider_type"`
	Name           string                    `json:"name"`
	Mode           string                    `json:"mode"`
	Endpoint       string                    `json:"endpoint"`
	Priority       int64                     `json:"priority"`
	HealthStatus   string                    `json:"health_status"`
	LastError      string                    `json:"last_error"`
	LastCheckAt    string                    `json:"last_check_at"`
	Status         int                       `json:"status"`
	IsDefault      bool                      `json:"is_default"`
	Remark         string                    `json:"remark"`
	Supported      bool                      `json:"supported"`
	Implemented    bool                      `json:"implemented"`
	Builtin        bool                      `json:"builtin"`
	CertifyModes   []integration.FieldOption `json:"certify_modes"`
	Credentials    map[string]string         `json:"credentials"`
	CredentialKeys []string                  `json:"credential_keys"`
}

// ProviderUpsertRequest 核验服务商新建 / 更新请求。
//
// ID=0 为新建（ProviderType 必填且必须已登记描述符），ID>0 为更新。
type ProviderUpsertRequest struct {
	ID           uint64            `json:"id"`
	ProviderType string            `json:"provider_type"`
	Name         string            `json:"name"`
	Endpoint     string            `json:"endpoint"`
	Priority     int64             `json:"priority"`
	Status       *int              `json:"status"`
	IsDefault    *bool             `json:"is_default"`
	Remark       string            `json:"remark"`
	Credentials  map[string]string `json:"credentials"`
}

// ConfigUpsertRequest 配置项新增 / 更新请求（按 config_key 定位）。
type ConfigUpsertRequest struct {
	ConfigKey   string  `json:"config_key" binding:"required"`
	ConfigValue *string `json:"config_value"`
	ValueType   string  `json:"value_type"`
	Status      string  `json:"status"`
	Description *string `json:"description"`
}

// SubmitRequest 用户端提交实名申请。
//
// IDNumber 是明文（服务层立刻加密落库，绝不回显）；IDNumberMasked 由服务端生成。
// VerifyTicket 为 realname_submit 场景的关键操作票据（策略要求时必填）。
type SubmitRequest struct {
	VerificationType string `json:"verification_type"`
	RealName         string `json:"real_name" binding:"required"`
	SubjectName      string `json:"subject_name"`
	IDType           string `json:"id_type"`
	IDNumber         string `json:"id_number" binding:"required"`
	Mobile           string `json:"mobile"`
	CountryCode      string `json:"country_code"`
	// —— 企业认证扩展 ——
	CompanyName     string `json:"company_name"`
	CreditCode      string `json:"credit_code"`
	LegalPersonName string `json:"legal_person_name"`
	ContactName     string `json:"contact_name"`
	// —— 二次验证 ——
	CaptchaKey   string `json:"captcha_key"`
	CaptchaCode  string `json:"captcha_code"`
	OTPCode      string `json:"otp_code"`
	VerifyTicket string `json:"verify_ticket"`
}

// UserStatusResponse 用户端实名状态卡。
type UserStatusResponse struct {
	// Status none / pending / approved / rejected
	Status           string     `json:"status"`
	Enabled          bool       `json:"enabled"`
	AllowedTypes     []string   `json:"allowed_types"`
	ApplicationID    uint64     `json:"application_id"`
	VerificationType string     `json:"verification_type"`
	RealName         string     `json:"real_name"`
	IDNumberMasked   string     `json:"id_number_masked"`
	MobileMasked     string     `json:"mobile_masked"`
	SubmittedAt      *time.Time `json:"submitted_at,omitempty"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	RejectReason     string     `json:"reject_reason"`
	ReviewNote       string     `json:"review_note"`
	ProviderResult   string     `json:"provider_result"`
	ReviewRound      int        `json:"review_round"`
	// CooldownHours 驳回后需等待的小时数；CooldownRemainSeconds 为剩余秒数（0 表示可提交）。
	CooldownHours         int `json:"cooldown_hours"`
	CooldownRemainSeconds int `json:"cooldown_remain_seconds"`
}

// AuthorizeResponse 跳转式核验的认证入口。
type AuthorizeResponse struct {
	AuthURL  string    `json:"auth_url"`
	TxnNo    string    `json:"txn_no"`
	Provider string    `json:"provider"`
	ExpireAt time.Time `json:"expire_at"`
}

// VerificationConfigInfo 表示实名认证配置详情。
type VerificationConfigInfo struct {
	ID          uint64    `json:"id"`
	ConfigKey   string    `json:"config_key"`
	ConfigGroup string    `json:"config_group"`
	ConfigValue string    `json:"config_value"`
	ValueType   string    `json:"value_type"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	UpdatedBy   uint64    `json:"updated_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
