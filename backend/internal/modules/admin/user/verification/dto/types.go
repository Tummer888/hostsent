// Package dto 提供实名认证模块的请求与响应数据结构。
package dto

import "time"

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
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
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
