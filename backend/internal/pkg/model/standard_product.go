package model

import "time"

// StandardProductSpec 统一商品规格
type StandardProductSpec struct {
	CPU       int                    `json:"cpu"`             // CPU核数
	Memory    int                    `json:"memory"`          // 内存大小(MB)
	Disk      int                    `json:"disk"`            // 系统盘大小(GB)
	DiskType  string                 `json:"disk_type"`       // ssd/premium/hdd
	Bandwidth int                    `json:"bandwidth"`       // 带宽(Mbps)
	OS        string                 `json:"os"`              // 操作系统
	Region    string                 `json:"region"`          // 区域
	Zone      string                 `json:"zone"`            // 可用区
	Extra     map[string]interface{} `json:"extra,omitempty"` // 扩展字段
}

// StandardProduct 统一商品
type StandardProduct struct {
	ID           uint                   `json:"id"`
	ProviderID   uint                   `json:"provider_id"`
	ProviderType string                 `json:"provider_type"`
	UpstreamID   string                 `json:"upstream_id"`
	Name         string                 `json:"name"`
	Specs        StandardProductSpec    `json:"specs"` // 上游标准化规格（StandardProductSpec）；wire 层商品 DTO 的 `specs` 为该结构的 JSON 字符串
	RawSpecs     map[string]interface{} `json:"raw_specs"`
	CostPrice    float64                `json:"cost_price"`
	SalePrice    float64                `json:"sale_price"`
	Status       string                 `json:"status"` // active/inactive
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}
