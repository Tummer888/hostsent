// Package dto 提供规格管理（spec）子域的数据传输结构。
package dto

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
	Status     int    `form:"status" json:"status"`
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
	ID          uint64    `json:"id"`
	Name        string    `json:"name"`
	SpecFamily  string    `json:"spec_family"`
	CPU         int       `json:"cpu"`
	Memory      float64   `json:"memory"`
	Disk        int       `json:"disk"`
	DiskType    string    `json:"disk_type"`
	Bandwidth   int       `json:"bandwidth"`
	OS          string    `json:"os"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	SortOrder   int       `json:"sort_order"`
	Status      int       `json:"status"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}

// SpecTemplateListResponse 规格模板列表响应
type SpecTemplateListResponse struct {
	Items []SpecTemplateInfo `json:"items"`
	Meta  ListMeta            `json:"meta"`
}

// SpecMappingQuery 规格映射列表查询
type SpecMappingQuery struct {
	ProviderType string `form:"provider_type" json:"provider_type"`
	Keyword      string `form:"keyword" json:"keyword"`
	Status       int    `form:"status" json:"status"`
	Page         int    `form:"page" json:"page"`
	PageSize     int    `form:"page_size" json:"page_size"`
}

// SpecMappingRequest 创建/更新规格映射
type SpecMappingRequest struct {
	ProviderType   string `json:"provider_type" binding:"required"`
	UpstreamSpecID string `json:"upstream_spec_id" binding:"required"`
	UpstreamName   string `json:"upstream_name"`
	PlatformSpecID uint64 `json:"platform_spec_id"`
	PlatformName   string `json:"platform_name"`
	CPU            int    `json:"cpu"`
	Memory         float64 `json:"memory"`
	Disk           int    `json:"disk"`
	Status         int    `json:"status"`
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
