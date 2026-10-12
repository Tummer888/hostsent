package dto

import commondto "hostsent/backend/internal/modules/admin/resource/common/dto"

// ListMeta 通用分页元信息（复用 common 包定义）
type ListMeta = commondto.ListMeta

// ProductListQuery 商品列表查询
type ProductListQuery struct {
	Keyword    string `form:"keyword" json:"keyword"`
	ProviderID uint64 `form:"provider_id" json:"provider_id"`
	GroupID    int64  `form:"group_id" json:"group_id"`
	// Status 用指针区分「不筛选」与「筛未在售」：状态只有 1/0 两态，
	// 用 0 当"不筛选"会让「未在售」筛选失效（此前实测 status=0 返回全部 245 条）。
	Status   *int `form:"status" json:"status"`
	Page     int  `form:"page" json:"page"`
	PageSize int  `form:"page_size" json:"page_size"`
}

// ProductPriceRequest 更新商品价格
type ProductPriceRequest struct {
	CostPrice float64 `json:"cost_price"`
	SalePrice float64 `json:"sale_price"`
}

// ProductInfo 商品信息
type ProductInfo struct {
	ID         uint64 `json:"id"`
	ProviderID uint64 `json:"provider_id"`
	UpstreamID string `json:"upstream_id"`
	Name       string `json:"name"`
	GroupID    int64  `json:"group_id"`
	GroupName  string `json:"group_name"`
	// GroupNameDerived 分类名不是上游公示名，而是按商品名前缀派生（上游隐藏分组）。
	// 前端据此打「派生」标记，避免运营误当成上游正式分类。
	GroupNameDerived bool   `json:"group_name_derived"`
	CPU              int    `json:"cpu"`
	Memory           int    `json:"memory"`
	Disk             int    `json:"disk"`
	DiskType         string `json:"disk_type"`
	Bandwidth        int    `json:"bandwidth"`
	OS               string `json:"os"`
	Region           string `json:"region"`
	Zone             string `json:"zone"`
	Specs            string `json:"specs"`
	// RawSpecs 上游原始快照（含 config_groups/描述/双价周期，单商品 10KB 量级）。
	// 仅详情接口返回；列表接口留空，避免一次拉全量商品时把正文全带上。
	RawSpecs  string  `json:"raw_specs,omitempty"`
	CostPrice float64 `json:"cost_price"`
	SalePrice float64 `json:"sale_price"`
	Status    int     `json:"status"`
	// Visibility 上游前台在售判定：visible/group_hidden/product_hidden/retired。
	// status 只有 1/0 两态，这里给出"为何不在售"（上游隐藏分组/隐藏商品/已下架）。
	Visibility string `json:"visibility"`
	// PriceSource 价格来源：pricings=商品周期价；catalog=上游前台展示价回填；
	// none=上游未定价（成本与标价均为 0，导入前需人工确认）。
	PriceSource string `json:"price_source"`
	// Catalog* 上游前台展示价（cart/index）：价格来源为 catalog 时与之上同源，
	// 其余情况用于核对周期价与前台展示价是否一致。
	CatalogPrice     float64 `json:"catalog_price"`
	CatalogSalePrice float64 `json:"catalog_sale_price"`
	CatalogCycle     string  `json:"catalog_cycle"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
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
