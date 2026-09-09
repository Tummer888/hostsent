// Package dto 定义用户中心商品模块的数据传输结构。
package dto

// ListQuery 商品列表查询
type ListQuery struct {
	Keyword  string `form:"keyword" json:"keyword"`
	Category uint64 `form:"category_id" json:"category_id"`
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
}

// ProductInfo 用户可购商品信息（不含成本价等内部字段）。
type ProductInfo struct {
	ID            uint64  `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	CategoryID    uint64  `json:"category_id"`
	ProductType   string  `json:"product_type"`
	Price         float64 `json:"price"`
	PriceModel    string  `json:"price_model"`
	Specs         string  `json:"specs"`
	ProvisionMode string  `json:"provision_mode"`
	ConfigOptions string  `json:"config_options"`
	Featured      bool    `json:"featured"`
	CreatedAt     string  `json:"created_at"`
}

// ListResponse 商品列表响应
type ListResponse struct {
	Items    []ProductInfo `json:"items"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Total    int64         `json:"total"`
}
