// Package dto 提供规格管理（spec）子域的数据传输结构。
package dto

import (
	"encoding/json"

	"hostsent/backend/internal/pkg/specatom"
)

// ListMeta 通用分页元信息
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// SpecTemplateQuery 规格模板列表查询
type SpecTemplateQuery struct {
	Keyword    string `form:"keyword" json:"keyword"`
	SpecFamily string `form:"spec_family" json:"spec_family"`
	// Status 指针区分「未传」与「显式筛 status=0（停用）」
	Status *int `form:"status" json:"status"`
	Page       int    `form:"page" json:"page"`
	PageSize   int    `form:"page_size" json:"page_size"`
}

// SpecTemplateRequest 创建/更新规格模板
type SpecTemplateRequest struct {
	Name        string  `json:"name" binding:"required"`
	SpecFamily  string  `json:"spec_family"`
	CPU         int     `json:"cpu"`
	Memory      float64 `json:"memory"`
	Disk        int     `json:"disk"`
	DiskType    string  `json:"disk_type"`
	Bandwidth   int     `json:"bandwidth"`
	OS          string  `json:"os"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	SortOrder   int     `json:"sort_order"`
	Status      int     `json:"status"`
}

// SpecTemplateInfo 规格模板信息
type SpecTemplateInfo struct {
	ID          uint64  `json:"id"`
	Name        string  `json:"name"`
	SpecFamily  string  `json:"spec_family"`
	CPU         int     `json:"cpu"`
	Memory      float64 `json:"memory"`
	Disk        int     `json:"disk"`
	DiskType    string  `json:"disk_type"`
	Bandwidth   int     `json:"bandwidth"`
	OS          string  `json:"os"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	SortOrder   int     `json:"sort_order"`
	Status      int     `json:"status"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// SpecTemplateListResponse 规格模板列表响应
type SpecTemplateListResponse struct {
	Items []SpecTemplateInfo `json:"items"`
	Meta  ListMeta           `json:"meta"`
}

// SpecMappingQuery 规格映射列表查询
type SpecMappingQuery struct {
	ProviderType string `form:"provider_type" json:"provider_type"`
	Keyword      string `form:"keyword" json:"keyword"`
	// Status 指针区分「未传」与「显式筛 status=0（停用）」
	Status   *int `form:"status" json:"status"`
	Page     int  `form:"page" json:"page"`
	PageSize int  `form:"page_size" json:"page_size"`
}

// SpecMappingRequest 创建/更新规格映射
type SpecMappingRequest struct {
	ProviderType   string  `json:"provider_type" binding:"required"`
	UpstreamSpecID string  `json:"upstream_spec_id" binding:"required"`
	UpstreamName   string  `json:"upstream_name"`
	PlatformSpecID uint64  `json:"platform_spec_id"`
	PlatformName   string  `json:"platform_name"`
	CPU            int     `json:"cpu"`
	Memory         float64 `json:"memory"`
	Disk           int     `json:"disk"`
	Status         int     `json:"status"`
}

// SpecMappingBindRequest 规格映射绑定参数（设置映射到的平台模板）
type SpecMappingBindRequest struct {
	PlatformSpecID uint64 `json:"platform_spec_id" binding:"required"`
	PlatformName   string `json:"platform_name"`
}

// SpecMappingInfo 规格映射信息
type SpecMappingInfo struct {
	ID             uint64  `json:"id"`
	ProviderType   string  `json:"provider_type"`
	UpstreamSpecID string  `json:"upstream_spec_id"`
	UpstreamName   string  `json:"upstream_name"`
	PlatformSpecID uint64  `json:"platform_spec_id"`
	PlatformName   string  `json:"platform_name"`
	CPU            int     `json:"cpu"`
	Memory         float64 `json:"memory"`
	Disk           int     `json:"disk"`
	Status         int     `json:"status"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// SpecMappingListResponse 规格映射列表响应
type SpecMappingListResponse struct {
	Items []SpecMappingInfo `json:"items"`
	Meta  ListMeta          `json:"meta"`
}

// ===== 规格契约（P2/T2.5、T2.6）=====

// SpecAtomInfo 规格原子字典项（含各平台字段映射）。
type SpecAtomInfo struct {
	ID             uint64          `json:"id"`
	Key            string          `json:"key"`
	Name           string          `json:"name"`
	Unit           string          `json:"unit"`
	ValueType      string          `json:"value_type"`
	EnumValues     json.RawMessage `json:"enum_values"`
	MinValue       *float64        `json:"min_value"`
	MaxValue       *float64        `json:"max_value"`
	StepValue      *float64        `json:"step_value"`
	Required       bool            `json:"required"`
	Configurable   bool            `json:"configurable"`
	AppliesTo      string          `json:"applies_to"`
	PlatformFields json.RawMessage `json:"platform_fields"`
	Description    string          `json:"description"`
	SortOrder      int             `json:"sort_order"`
	Status         int             `json:"status"`
}

// SpecValidateRequest 规格校验请求（Extra 受字典约束的试算入口）。
type SpecValidateRequest struct {
	CPU       int                    `json:"cpu"`
	Memory    int                    `json:"memory"`
	Disk      int                    `json:"disk"`
	DiskType  string                 `json:"disk_type"`
	Bandwidth int                    `json:"bandwidth"`
	OS        string                 `json:"os"`
	Region    string                 `json:"region"`
	Zone      string                 `json:"zone"`
	Extra     map[string]interface{} `json:"extra"`
}

// SpecValidateResponse 规格校验结果。
type SpecValidateResponse struct {
	Valid  bool             `json:"valid"`
	Issues []specatom.Issue `json:"issues"`
	Atoms  int              `json:"atom_count"`
}

// ExternalSpecInfo 外部规格快照信息。
type ExternalSpecInfo struct {
	ID           uint64          `json:"id"`
	ProviderID   uint64          `json:"provider_id"`
	ProviderType string          `json:"provider_type"`
	ExternalID   string          `json:"external_id"`
	ExternalName string          `json:"external_name"`
	ExternalKind string          `json:"external_kind"`
	Raw          json.RawMessage `json:"raw"`
	Normalized   json.RawMessage `json:"normalized"`
	Fingerprint  string          `json:"fingerprint"`
	Status       string          `json:"status"`
	SyncedAt     *string         `json:"synced_at"`
	CreatedAt    string          `json:"created_at"`
}

// ExternalSpecUpsertRequest 登记/刷新外部规格快照。
type ExternalSpecUpsertRequest struct {
	ProviderID   uint64          `json:"provider_id"`
	ProviderType string          `json:"provider_type" binding:"required"`
	ExternalID   string          `json:"external_id" binding:"required"`
	ExternalName string          `json:"external_name"`
	ExternalKind string          `json:"external_kind"`
	Raw          json.RawMessage `json:"raw"`
	Normalized   json.RawMessage `json:"normalized"`
}

// SpecBindingInfo 规格绑定信息。
// ExternalSpecID 与 ProductSpecID 二选一非零：前者为代理链路（上游规格镜像），
// 后者为自营链路（我方 SKU，T4.2）。
type SpecBindingInfo struct {
	ID              uint64          `json:"id"`
	ExternalSpecID  uint64          `json:"external_spec_id,omitempty"`
	ProductSpecID   uint64          `json:"product_spec_id,omitempty"`
	ProductSpecCode string          `json:"product_spec_code,omitempty"` // SKU 编码（仅自营绑定回填展示）
	SpecTemplateID  uint64          `json:"spec_template_id"`
	Direction       string          `json:"direction"`
	PlatformParams  json.RawMessage `json:"platform_params"`
	MatchType       string          `json:"match_type"`
	Status          string          `json:"status"`
	Confidence      int             `json:"confidence"`
	ConfirmedBy     uint64          `json:"confirmed_by"`
	ConfirmedAt     *string         `json:"confirmed_at"`
	Remark          string          `json:"remark"`
	Priority        int             `json:"priority"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
}

// SpecBindingUpsertRequest 建立/更新规格绑定。
// 自营链路填 product_spec_id，代理链路填 external_spec_id（至少一个）。
type SpecBindingUpsertRequest struct {
	ExternalSpecID uint64          `json:"external_spec_id"`
	ProductSpecID  uint64          `json:"product_spec_id"`
	SpecTemplateID uint64          `json:"spec_template_id"`
	Direction      string          `json:"direction"`
	PlatformParams json.RawMessage `json:"platform_params"`
	MatchType      string          `json:"match_type"`
	Status         string          `json:"status"`
	Confidence     int             `json:"confidence"`
	Remark         string          `json:"remark"`
	Priority       int             `json:"priority"`
}

// SpecBindingConfirmRequest 人工确认绑定（状态机 → confirmed）。
type SpecBindingConfirmRequest struct {
	SpecTemplateID uint64          `json:"spec_template_id"`
	PlatformParams json.RawMessage `json:"platform_params"`
	Remark         string          `json:"remark"`
}
