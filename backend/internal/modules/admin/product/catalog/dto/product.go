package dto

// ProductListQuery 产品列表查询
type ProductListQuery struct {
	Keyword    string `form:"keyword" json:"keyword"`
	CategoryID uint64 `form:"category_id" json:"category_id"`
	Status     int    `form:"status" json:"status"`
	Page       int    `form:"page" json:"page"`
	PageSize   int    `form:"page_size" json:"page_size"`
}

// ProductCreateRequest 创建产品
type ProductCreateRequest struct {
	Code             string  `json:"code" binding:"required"`
	Name             string  `json:"name" binding:"required"`
	CategoryID       uint64  `json:"category_id"`
	ProductType      string  `json:"product_type"`
	Description      string  `json:"description"`
	Specs            string  `json:"specs"`
	PriceModel       string  `json:"price_model"`
	Price            float64 `json:"price"`
	CostPrice        float64 `json:"cost_price"`
	SourceProductID  uint64  `json:"source_product_id"`
	SourceProviderID uint64  `json:"source_provider_id"`
	Stock            int     `json:"stock"`
	SortOrder        int     `json:"sort_order"`
	Status           int     `json:"status"`
}

// ProductUpdateRequest 更新产品
type ProductUpdateRequest struct {
	Name        string  `json:"name" binding:"required"`
	CategoryID  uint64  `json:"category_id"`
	ProductType string  `json:"product_type"`
	Description string  `json:"description"`
	Specs       string  `json:"specs"`
	PriceModel  string  `json:"price_model"`
	Price       float64 `json:"price"`
	CostPrice   float64 `json:"cost_price"`
	Stock       int     `json:"stock"`
	SortOrder   int     `json:"sort_order"`
	Status      int     `json:"status"`
}

// ProductPriceRequest 更新产品价格
type ProductPriceRequest struct {
	Price     float64 `json:"price"`
	CostPrice float64 `json:"cost_price"`
	Remark    string  `json:"remark"`
}

// ProductFeaturedRequest 设置前台推荐位
type ProductFeaturedRequest struct {
	Featured bool `json:"featured"`
}

// ProductInfo 产品信息
type ProductInfo struct {
	ID               uint64  `json:"id"`
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	CategoryID       uint64  `json:"category_id"`
	ProductType      string  `json:"product_type"`
	Description      string  `json:"description"`
	Specs            string  `json:"specs"`
	PriceModel       string  `json:"price_model"`
	Price            float64 `json:"price"`
	CostPrice        float64 `json:"cost_price"`
	SourceProductID  uint64  `json:"source_product_id"`
	SourceProviderID uint64  `json:"source_provider_id"`
	Featured         bool    `json:"featured"`
	Stock            int     `json:"stock"`
	SortOrder        int     `json:"sort_order"`
	Status           int     `json:"status"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

// ProductListResponse 产品列表响应
type ProductListResponse struct {
	Items []ProductInfo `json:"items"`
	Meta  ListMeta      `json:"meta"`
}

// ProductSpecInfo 产品规格信息
type ProductSpecInfo struct {
	ID         uint64  `json:"id"`
	ProductID  uint64  `json:"product_id"`
	SpecCode   string  `json:"spec_code"`
	Name       string  `json:"name"`
	Specs      string  `json:"specs"`
	PriceModel string  `json:"price_model"`
	Price      float64 `json:"price"`
	CostPrice  float64 `json:"cost_price"`
	Stock      int     `json:"stock"`
	SortOrder  int     `json:"sort_order"`
	Status     int     `json:"status"`
}

// ProductHistoryInfo 产品变更历史信息
type ProductHistoryInfo struct {
	ID           uint64 `json:"id"`
	ProductID    uint64 `json:"product_id"`
	ChangeType   string `json:"change_type"`
	OldValue     string `json:"old_value"`
	NewValue     string `json:"new_value"`
	OperatorName string `json:"operator_name"`
	Remark       string `json:"remark"`
	CreatedAt    string `json:"created_at"`
}
