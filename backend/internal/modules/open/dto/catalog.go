// Package dto 开放平台对外契约（P6）。只暴露标准模型字段，
// 平台字段映射、折扣率明细、成本价等内部信息不外泄（doc16 §8.1/§8.2）。
package dto

import (
	ucproductdto "hostsent/backend/internal/modules/uc/product/dto"
)

// SpecAtomItem 规格原子字典条目（doc16 §8.3 GET /spec-atoms）。
// 不下发 platform_fields（平台读写映射属内部契约，D4 规格单向只读）。
type SpecAtomItem struct {
	Key          string   `json:"key"`
	Name         string   `json:"name"`
	Unit         string   `json:"unit,omitempty"`
	ValueType    string   `json:"value_type"`
	EnumValues   []string `json:"enum_values,omitempty"`
	MinValue     *float64 `json:"min_value,omitempty"`
	MaxValue     *float64 `json:"max_value,omitempty"`
	StepValue    *float64 `json:"step_value,omitempty"`
	Required     bool     `json:"required"`
	Configurable bool     `json:"configurable"`
	AppliesTo    string   `json:"applies_to"`
	Description  string   `json:"description,omitempty"`
}

// ProductItem 可售商品（标准模型，与 uc 商品同构，剔除内部字段）。
type ProductItem struct {
	ID          uint64  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	CategoryID  uint64  `json:"category_id"`
	Price       float64 `json:"price"`
	PriceModel  string  `json:"price_model"`
	Featured    bool    `json:"featured"`
	CreatedAt   string  `json:"created_at,omitempty"`
}

// ProductDetail 商品详情（含 SKU 矩阵）。
type ProductDetail struct {
	ProductItem
	Skus []SkuItem `json:"skus"`
}

// SkuItem 可售规格变体（spec_code 为下单口径）。
type SkuItem struct {
	SpecCode   string  `json:"spec_code"`
	Name       string  `json:"name"`
	Specs      string  `json:"specs"` // 原子取值 JSON
	Price      float64 `json:"price"`
	PriceModel string  `json:"price_model"`
	Stock      int     `json:"stock"` // -1 表示不限
	// Cycles 该 SKU 可售计费周期（doc25）：下单 cycle 的合法取值；
	// SKU 无自有周期价时回落商品级可售周期，两者皆空表示按商品单价下单（无周期概念）。
	Cycles []string `json:"cycles"`
}

// RegionItem 标准区域（资源池），不含上游原始 ID 与容量细节。
type RegionItem struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// ImageItem 标准镜像（v1 由在售 SKU 的 os 原子取值聚合）。
type ImageItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// QuoteRequest 询价入参（doc16 §8.3 POST /quote）。
type QuoteRequest struct {
	ProductID uint64 `json:"product_id" binding:"required"`
	SpecCode  string `json:"spec_code"`
	Quantity  int    `json:"quantity"`
	// Cycle 计费周期（doc25）；与下单口径一致，留空回落商品 price_model。
	Cycle string `json:"cycle"`
}

// QuoteInfo 询价结果：只给零售原价与下游实付（成本价），
// 折扣率/策略 ID 等明细不外暴露（doc16 §8.2 下游定价口径）。
type QuoteInfo struct {
	ProductID      uint64  `json:"product_id"`
	SpecCode       string  `json:"spec_code,omitempty"`
	Quantity       int     `json:"quantity"`
	Cycle          string  `json:"cycle,omitempty"`
	OriginalAmount float64 `json:"original_amount"`
	FinalAmount    float64 `json:"final_amount"`
}

// ProductListResponse 商品分页。
type ProductListResponse struct {
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Items    []ProductItem `json:"items"`
}

// 复用 uc 商品 SKU 结构（字段一致，避免第二套映射）。
type SkuInfo = ucproductdto.SkuInfo
