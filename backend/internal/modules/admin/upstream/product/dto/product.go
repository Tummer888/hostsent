package dto

import commondto "hostsent/backend/internal/modules/admin/upstream/common/dto"

// ListMeta 通用分页元信息（复用 common 包定义）
type ListMeta = commondto.ListMeta

// ProductListQuery 商品列表查询
type ProductListQuery struct {
	Keyword    string `form:"keyword" json:"keyword"`
	ProviderID uint64 `form:"provider_id" json:"provider_id"`
	Status     int    `form:"status" json:"status"`
	Page       int    `form:"page" json:"page"`
	PageSize   int    `form:"page_size" json:"page_size"`
}

// ProductPriceRequest 更新商品价格
type ProductPriceRequest struct {
	CostPrice float64 `json:"cost_price"`
	SalePrice float64 `json:"sale_price"`
}

// ProductInfo 商品信息
type ProductInfo struct {
	ID         uint64  `json:"id"`
	ProviderID uint64  `json:"provider_id"`
	UpstreamID string  `json:"upstream_id"`
	Name       string  `json:"name"`
	CPU        int     `json:"cpu"`
	Memory     int     `json:"memory"`
	Disk       int     `json:"disk"`
	DiskType   string  `json:"disk_type"`
	Bandwidth  int     `json:"bandwidth"`
	OS         string  `json:"os"`
	Region     string  `json:"region"`
	Zone       string  `json:"zone"`
	Specs      string  `json:"specs"`
	RawSpecs   string  `json:"raw_specs"`
	CostPrice  float64 `json:"cost_price"`
	SalePrice  float64 `json:"sale_price"`
	Status     int     `json:"status"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

// ProductListResponse 商品列表响应
type ProductListResponse struct {
	Items []ProductInfo `json:"items"`
	Meta  ListMeta      `json:"meta"`
}

// SyncResult 同步结果
type SyncResult struct {
	TaskID  uint64 `json:"task_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}
