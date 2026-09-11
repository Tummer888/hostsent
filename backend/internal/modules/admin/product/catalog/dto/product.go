package dto

import "encoding/json"

// ProductListQuery 产品列表查询
type ProductListQuery struct {
	Keyword       string `form:"keyword" json:"keyword"`
	CategoryID    uint64 `form:"category_id" json:"category_id"`
	Status        int    `form:"status" json:"status"`
	ProvisionMode string `form:"provision_mode" json:"provision_mode"` // self / clone
	SourceMode    string `form:"source_mode" json:"source_mode"`       // self / upstream（双链路判据）
	// Featured 推荐位过滤：nil=不过滤，true/false=按推荐位精确筛选。
	// 用指针而非 bool，以区分「未传」与「显式传 false」。
	Featured *bool `form:"featured" json:"featured"`
	Page     int   `form:"page" json:"page"`
	PageSize int   `form:"page_size" json:"page_size"`
}

// ProductCreateRequest 创建产品
type ProductCreateRequest struct {
	Code             string  `json:"code" binding:"required"`
	Name             string  `json:"name" binding:"required"`
	CategoryID       uint64  `json:"category_id"`
	ProductType      string  `json:"product_type"`
	Description      string  `json:"description"`
	CoverImage       string  `json:"cover_image"`
	Specs            string  `json:"specs"`
	PriceModel       string  `json:"price_model"`
	Price            float64 `json:"price"`
	CostPrice        float64 `json:"cost_price"`
	SourceProductID  uint64  `json:"source_product_id"`
	SourceProviderID uint64  `json:"source_provider_id"`
	ProvisionMode    string  `json:"provision_mode"` // self / clone
	ConfigOptions    string  `json:"config_options"`
	Stock            int     `json:"stock"`
	SortOrder        int     `json:"sort_order"`
	Status           int     `json:"status"`
	// 上游加价规则与"仅透传"标记（T4.3）：代理商品改上游价后按此规则重算售价。
	UpstreamMarkupType  string  `json:"upstream_markup_type"`
	UpstreamMarkupValue float64 `json:"upstream_markup_value"`
	SpecPassthrough     bool    `json:"spec_passthrough"`
}

// ProductCloneRequest 从上游商品克隆创建销售商品
type ProductCloneRequest struct {
	SourceProductID  uint64  `json:"source_product_id" binding:"required"`  // 上游资源商品 ID
	SourceProviderID uint64  `json:"source_provider_id" binding:"required"` // 上游提供商 ID
	Code             string  `json:"code" binding:"required"`
	Name             string  `json:"name"`
	CategoryID       uint64  `json:"category_id"`
	Price            float64 `json:"price"`
	CostPrice        float64 `json:"cost_price"`
	ConfigOptions    string  `json:"config_options"` // 可覆盖上游默认规格
	Stock            int     `json:"stock"`
	Status           int     `json:"status"`
}

// ProductBatchCloneRequest 批量从上游商品克隆创建销售商品（按百分比定价）。
type ProductBatchCloneRequest struct {
	SourceProviderID uint64   `json:"source_provider_id" binding:"required"` // 上游提供商 ID
	SourceProductIDs []uint64 `json:"source_product_ids" binding:"required"` // 待导入的上游商品 ID 列表
	CategoryID       uint64   `json:"category_id"`                           // 归入的商品分类
	PricePercent     float64  `json:"price_percent"`                         // 定价百分比：售价 = 上游售价 × percent/100（0 或 100 表示按原价）
	Status           int      `json:"status"`                                // 默认草稿
}

// ProductUpdateRequest 更新产品
type ProductUpdateRequest struct {
	Name          string  `json:"name" binding:"required"`
	CategoryID    uint64  `json:"category_id"`
	ProductType   string  `json:"product_type"`
	Description   string  `json:"description"`
	CoverImage    string  `json:"cover_image"`
	Specs         string  `json:"specs"`
	PriceModel    string  `json:"price_model"`
	Price         float64 `json:"price"`
	CostPrice     float64 `json:"cost_price"`
	ConfigOptions string  `json:"config_options"`
	Stock         int     `json:"stock"`
	SortOrder     int     `json:"sort_order"`
	Status        int     `json:"status"`
	// 上游加价规则与"仅透传"标记（T4.3）；省略则保持原值（指针区分"未传"与"清空"）。
	UpstreamMarkupType  *string  `json:"upstream_markup_type"`
	UpstreamMarkupValue *float64 `json:"upstream_markup_value"`
	SpecPassthrough     *bool    `json:"spec_passthrough"`
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
	CoverImage       string  `json:"cover_image"`
	Specs            string  `json:"specs"`
	PriceModel       string  `json:"price_model"`
	Price            float64 `json:"price"`
	CostPrice        float64 `json:"cost_price"`
	SourceProductID  uint64  `json:"source_product_id"`
	SourceProviderID uint64  `json:"source_provider_id"`
	ProvisionMode    string  `json:"provision_mode"`
	SourceMode       string  `json:"source_mode"`
	ConfigOptions    string  `json:"config_options"`
	// UpstreamMarkupType/Value 上游加价规则（T4.3）：percent 按成本百分比、fixed 加固定额。
	UpstreamMarkupType  string  `json:"upstream_markup_type"`
	UpstreamMarkupValue float64 `json:"upstream_markup_value"`
	// SpecPassthrough 代理商品"仅透传"标记（T4.3）：未归一确认时放行上架。
	SpecPassthrough bool   `json:"spec_passthrough"`
	Featured        bool   `json:"featured"`
	Stock           int    `json:"stock"`
	SortOrder       int    `json:"sort_order"`
	Status          int    `json:"status"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
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
	// SpecTemplateID 引用的标准规格模板（T4.2）；0 表示未绑定模板。
	SpecTemplateID uint64 `json:"spec_template_id"`
	SortOrder      int    `json:"sort_order"`
	Status         int    `json:"status"`
	// BindingStatus 该 SKU 出站平台绑定的状态（confirmed / auto_mapped / unmapped / stale）；
	// 空串表示未建立绑定。供上架门禁与前端提示使用（T4.2/T4.6）。
	BindingStatus string `json:"binding_status"`
	// PlatformParams 该 SKU 已确认的出站平台参数 JSON（无绑定为空）。
	PlatformParams string `json:"platform_params"`
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

// ProductSpecRequest 创建/更新 SKU 规格变体（T4.1）。
// Specs 为"原子 key → 取值"的 JSON 字符串（如 {"compute.cpu":2,...}），写库前按 spec_atoms 校验。
type ProductSpecRequest struct {
	SpecCode   string  `json:"spec_code" binding:"required"`
	Name       string  `json:"name" binding:"required"`
	Specs      string  `json:"specs"`
	PriceModel string  `json:"price_model"`
	Price      float64 `json:"price"`
	CostPrice  float64 `json:"cost_price"`
	Stock      int     `json:"stock"`
	// SpecTemplateID 引用标准规格模板（可选，T4.2）；0 清除引用。
	SpecTemplateID uint64 `json:"spec_template_id"`
	SortOrder      int    `json:"sort_order"`
	Status         int    `json:"status"`
}

// ProductConfigOptionsRequest 保存商品配置项（T4.4）。
// groups 的结构与上游 config_groups 对齐（name/description/options[option_name/upstream_id/sub[...]]），
// 每个配置项/子项可显式带 source/source_key 覆盖来源：
//   - source=upstream（默认）：source_key 为上游配置项 id（缺省由 upstream_id 派生）；
//   - source=self：source_key 为平台参数名（如 area/os/store），开通时直接作为写参数下发。
type ProductConfigOptionsRequest struct {
	Groups []json.RawMessage `json:"groups"`
}
