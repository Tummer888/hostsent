package dto

import "time"

// QuotaListQuery 定义资源配额列表的筛选条件。
type QuotaListQuery struct {
	Page            int    `form:"page"`
	PageSize        int    `form:"page_size"`
	UserID          uint64 `form:"user_id"`
	Username        string `form:"username"`
	QuotaType       string `form:"quota_type"`
	Source          string `form:"source"`
	Status          string `form:"status"`
	IsOverallocated string `form:"is_overallocated"`
	Keyword         string `form:"keyword"`
}

type QuotaAdjustRequest struct {
	LimitValue float64 `json:"limit_value" binding:"required"`
	Reason     string  `json:"reason"`
	TicketNo   string  `json:"ticket_no"`
}

type QuotaInfo struct {
	ID              uint64    `json:"id"`
	UserID          uint64    `json:"user_id"`
	Username        string    `json:"username"`
	QuotaCode       string    `json:"quota_code"`
	QuotaName       string    `json:"quota_name"`
	QuotaType       string    `json:"quota_type"`
	LimitValue      float64   `json:"limit_value"`
	UsedValue       float64   `json:"used_value"`
	AvailableValue  float64   `json:"available_value"`
	Unit            string    `json:"unit"`
	Status          string    `json:"status"`
	Source          string    `json:"source"`
	TemplateID      *uint64   `json:"template_id,omitempty"`
	LevelID         *uint64   `json:"level_id,omitempty"`
	IsOverallocated bool      `json:"is_overallocated"`
	UpdatedBy       uint64    `json:"updated_by"`
	LastAdjustedAt  time.Time `json:"last_adjusted_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// QuotaListMeta 描述分页元信息。
type QuotaListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// QuotaListResponse 表示资源配额列表返回结果。
type QuotaListResponse struct {
	Items []QuotaInfo    `json:"items"`
	Meta  QuotaListMeta  `json:"meta"`
}

type QuotaTemplateListQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
	Scope    string `form:"scope"`
	Keyword  string `form:"keyword"`
}

type QuotaTemplateItemPayload struct {
	QuotaCode  string  `json:"quota_code" binding:"required"`
	QuotaName  string  `json:"quota_name" binding:"required"`
	QuotaType  string  `json:"quota_type" binding:"required"`
	LimitValue float64 `json:"limit_value"`
	Unit       string  `json:"unit"`
	Sort       int     `json:"sort"`
}

type QuotaTemplateCreateRequest struct {
	Name        string                     `json:"name" binding:"required"`
	Code        string                     `json:"code" binding:"required"`
	Scope       string                     `json:"scope"`
	Status      string                     `json:"status"`
	Description string                     `json:"description"`
	Items       []QuotaTemplateItemPayload `json:"items"`
}

// QuotaTemplateUpdateRequest 表示更新配额模板时提交的参数。
type QuotaTemplateUpdateRequest struct {
	Name        string                     `json:"name" binding:"required"`
	Code        string                     `json:"code" binding:"required"`
	Scope       string                     `json:"scope"`
	Status      string                     `json:"status" binding:"required"`
	Description string                     `json:"description"`
	Items       []QuotaTemplateItemPayload `json:"items"`
}

type QuotaTemplateInfo struct {
	ID            uint64                     `json:"id"`
	Name          string                     `json:"name"`
	Code          string                     `json:"code"`
	Scope         string                     `json:"scope"`
	Status        string                     `json:"status"`
	Description   string                     `json:"description"`
	Version       int                        `json:"version"`
	CreatedBy     uint64                     `json:"created_by"`
	UpdatedBy     uint64                     `json:"updated_by"`
	BindingLevels int                        `json:"binding_levels"`
	BindingUsers  int                        `json:"binding_users"`
	Items         []QuotaTemplateItemPayload `json:"items"`
	CreatedAt     time.Time                  `json:"created_at"`
	UpdatedAt     time.Time                  `json:"updated_at"`
}

// QuotaTemplateListResponse 表示配额模板列表返回结果。
type QuotaTemplateListResponse struct {
	Items []QuotaTemplateInfo `json:"items"`
	Meta  QuotaListMeta       `json:"meta"`
}

type UserLevelListQuery struct {
	Page              int    `form:"page"`
	PageSize          int    `form:"page_size"`
	Status            string `form:"status"`
	DefaultTemplateID uint64 `form:"default_template_id"`
	Keyword           string `form:"keyword"`
}

// UserLevelCreateRequest 表示创建用户等级时提交的参数。
type UserLevelCreateRequest struct {
	Name              string  `json:"name" binding:"required"`
	Code              string  `json:"code" binding:"required"`
	Weight            int     `json:"weight"`
	Status            string  `json:"status"`
	DefaultTemplateID *uint64 `json:"default_template_id"`
	MaxInstanceCount  int     `json:"max_instance_count"`
	MaxCPUCores       int     `json:"max_cpu_cores"`
	MaxMemoryGB       int     `json:"max_memory_gb"`
	MaxDiskGB         int     `json:"max_disk_gb"`
	FeatureFlags      string  `json:"feature_flags"`
	UpgradeCondition  string  `json:"upgrade_condition"`
	Description       string  `json:"description"`
}

type UserLevelUpdateRequest struct {
	Name              string  `json:"name" binding:"required"`
	Code              string  `json:"code" binding:"required"`
	Weight            int     `json:"weight"`
	Status            string  `json:"status" binding:"required"`
	DefaultTemplateID *uint64 `json:"default_template_id"`
	MaxInstanceCount  int     `json:"max_instance_count"`
	MaxCPUCores       int     `json:"max_cpu_cores"`
	MaxMemoryGB       int     `json:"max_memory_gb"`
	MaxDiskGB         int     `json:"max_disk_gb"`
	FeatureFlags      string  `json:"feature_flags"`
	UpgradeCondition  string  `json:"upgrade_condition"`
	Description       string  `json:"description"`
}

// UserLevelBindTemplateRequest 表示为用户等级绑定默认模板时提交的参数。
type UserLevelBindTemplateRequest struct {
	DefaultTemplateID uint64 `json:"default_template_id" binding:"required"`
}

// UserLevelInfo 描述用户等级及其默认模板绑定信息。
type UserLevelInfo struct {
	ID                uint64    `json:"id"`
	Name              string    `json:"name"`
	Code              string    `json:"code"`
	Weight            int       `json:"weight"`
	Status            string    `json:"status"`
	DefaultTemplateID *uint64   `json:"default_template_id,omitempty"`
	DefaultTemplateName string  `json:"default_template_name"`
	MaxInstanceCount  int       `json:"max_instance_count"`
	MaxCPUCores       int       `json:"max_cpu_cores"`
	MaxMemoryGB       int       `json:"max_memory_gb"`
	MaxDiskGB         int       `json:"max_disk_gb"`
	FeatureFlags      string    `json:"feature_flags"`
	UpgradeCondition  string    `json:"upgrade_condition"`
	Description       string    `json:"description"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type UserLevelListResponse struct {
	Items []UserLevelInfo `json:"items"`
	Meta  QuotaListMeta   `json:"meta"`
}

type QuotaAdjustmentListQuery struct {
	Page           int    `form:"page"`
	PageSize       int    `form:"page_size"`
	UserID         uint64 `form:"user_id"`
	Username       string `form:"username"`
	QuotaCode      string `form:"quota_code"`
	AdjustmentType string `form:"adjustment_type"`
	Source         string `form:"source"`
	OperatorName   string `form:"operator_name"`
}

type QuotaAdjustmentInfo struct {
	ID             uint64    `json:"id"`
	UserID         uint64    `json:"user_id"`
	Username       string    `json:"username"`
	QuotaCode      string    `json:"quota_code"`
	QuotaName      string    `json:"quota_name"`
	BeforeValue    float64   `json:"before_value"`
	AfterValue     float64   `json:"after_value"`
	DeltaValue     float64   `json:"delta_value"`
	AdjustmentType string    `json:"adjustment_type"`
	Source         string    `json:"source"`
	TemplateID     *uint64   `json:"template_id,omitempty"`
	LevelID        *uint64   `json:"level_id,omitempty"`
	OperatorID     uint64    `json:"operator_id"`
	OperatorName   string    `json:"operator_name"`
	Reason         string    `json:"reason"`
	TicketNo       string    `json:"ticket_no"`
	BatchNo        string    `json:"batch_no"`
	CreatedAt      time.Time `json:"created_at"`
}

// QuotaAdjustmentListResponse 表示配额调整记录列表返回结果。
type QuotaAdjustmentListResponse struct {
	Items []QuotaAdjustmentInfo `json:"items"`
	Meta  QuotaListMeta         `json:"meta"`
}

// APIResponse 表示通用接口响应结构。
type APIResponse[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      T      `json:"data"`
	Timestamp int64  `json:"timestamp"`
}
