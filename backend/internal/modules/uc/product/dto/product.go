// Package dto 定义用户中心商品模块的数据传输结构。
package dto

// ListQuery 商品列表查询
type ListQuery struct {
	Keyword  string `form:"keyword" json:"keyword"`
	Category uint64 `form:"category_id" json:"category_id"`
	// Featured 推荐位过滤：nil=不过滤（全部上架商品），true=仅推荐位商品。
	// 官网首页「热门产品」用它取运营勾选的推荐位，避免前端拉全量再本地筛选。
	Featured *bool `form:"featured" json:"featured"`
	Page     int   `form:"page" json:"page"`
	PageSize int   `form:"page_size" json:"page_size"`
}

// ProductInfo 用户可购商品信息（不含成本价等内部字段）。
type ProductInfo struct {
	ID            uint64  `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	CoverImage    string  `json:"cover_image"`
	CategoryID    uint64  `json:"category_id"`
	ProductType   string  `json:"product_type"`
	Price         float64 `json:"price"`
	PriceModel    string  `json:"price_model"`
	Specs         string  `json:"specs"` // JSON 字符串（wire 层 `specs` 类型统一为 string；内容为商品展示规格）
	SourceMode    string  `json:"source_mode"` // self 自营 / upstream 上游转售（D6）
	ConfigOptions string  `json:"config_options"`
	Featured      bool    `json:"featured"`
	CreatedAt     string  `json:"created_at"`
	// Skus 商品下挂的可售规格（T4.1）。空数组表示该商品未拆 SKU，按商品级价格下单。
	// 下单时把选中项的 spec_code 传给 POST /uc/orders 的 spec_code 字段。
	Skus []SkuInfo `json:"skus"`
}

// SkuInfo 用户可见的规格变体（不含成本价）。
type SkuInfo struct {
	SpecCode   string  `json:"spec_code"`
	Name       string  `json:"name"`
	Specs      string  `json:"specs"` // 原子取值 JSON，便于前端渲染规格参数
	Price      float64 `json:"price"`
	PriceModel string  `json:"price_model"`
	Stock      int     `json:"stock"` // -1 表示不限
}

// ListResponse 商品列表响应
type ListResponse struct {
	Items    []ProductInfo `json:"items"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Total    int64         `json:"total"`
}
