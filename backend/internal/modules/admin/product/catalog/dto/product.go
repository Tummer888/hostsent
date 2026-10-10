package dto

import "encoding/json"

// ProductListQuery 产品列表查询
type ProductListQuery struct {
	Keyword    string `form:"keyword" json:"keyword"`
	CategoryID uint64 `form:"category_id" json:"category_id"`
	// Status 状态过滤：用指针区分「未传」与「显式筛 status=0（草稿）」。
	// 此前用 int + `!= 0` 判断，导致「草稿」筛选条件被当成未传，前端选草稿会返回全部商品。
	Status     *int   `form:"status" json:"status"`
	SourceMode string `form:"source_mode" json:"source_mode"` // self / upstream（双链路判据）
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
	ConfigOptions    string  `json:"config_options"`
	Stock            int     `json:"stock"`
	SortOrder        int     `json:"sort_order"`
	Status           int     `json:"status"`
	// SourceMode 链路判据（D6 单一判据）：self 自营 / upstream 上游转售；空值归一为 self。
	// 注意：本接口只接 self——上游转售商品必须走「导入上游商品」（见 service.Create 守卫）。
	SourceMode string `json:"source_mode"`
	// SpecTemplates 建品同时选定的规格模板（自营链路）：每个模板生成一个带平台绑定的 SKU。
	// 这是「新建商品就能预选配好的规格」的入口——运营不必先建商品再去详情页生成规格。
	SpecTemplates []ProductSpecTemplateSelection `json:"spec_templates"`
	// 上游加价规则与"仅透传"标记（T4.3）：代理商品改上游价后按此规则重算售价。
	UpstreamMarkupType  string  `json:"upstream_markup_type"`
	UpstreamMarkupValue float64 `json:"upstream_markup_value"`
	SpecPassthrough     bool    `json:"spec_passthrough"`
}

// ProductSpecTemplateSelection 建品时选定一个规格模板（含就地调整后的参数）。
type ProductSpecTemplateSelection struct {
	SpecTemplateID uint64 `json:"spec_template_id" binding:"required"`
	// SpecCode / Name 覆盖从模板派生的 SKU 编码与名称（留空自动派生）。
	SpecCode string `json:"spec_code"`
	Name     string `json:"name"`
	// Price / CostPrice 覆盖售价与成本价；Price 为 0 时回落模板参考售价，再回落商品售价。
	Price     float64 `json:"price"`
	CostPrice float64 `json:"cost_price"`
	Stock     int     `json:"stock"`
	// SpecValues 就地调整后的原子取值 JSON（如 {"compute.cpu":4,...}）；
	// 留空则原样用模板值（再空则按模板 CPU/内存/磁盘推导）。
	SpecValues json.RawMessage `json:"spec_values"`
	// PlatformParams 就地调整后的平台写参数 JSON（area/node/os/store 等）；
	// 留空则原样用模板值。为空（且模板也为空）时该 SKU 不带平台绑定，无法上架。
	PlatformParams json.RawMessage `json:"platform_params"`
	// OptionOverrides 就地覆盖的客户选配项（T4.5）：参数名 → 可选值及加价。
	// 留空时由模板 option_selections + 平台配置项目录自动生成。
	OptionOverrides []ProductOptionOverride `json:"option_overrides"`
}

// ProductOptionOverride 建品时就地调整某个客户选配项。
// 只覆盖"可选值与加价"；控件类型/必选/单位等默认取平台配置项目录。
type ProductOptionOverride struct {
	OptionKey  string               `json:"option_key" binding:"required"`
	Label      string               `json:"label"`
	Widget     string               `json:"widget"`
	Required   bool                 `json:"required"`
	Default    string               `json:"default"`
	Unit       string               `json:"unit"`
	GroupLabel string               `json:"group_label"`
	Help       string               `json:"help"`
	MinValue   *float64             `json:"min_value"`
	MaxValue   *float64             `json:"max_value"`
	SortOrder  int                  `json:"sort_order"`
	Values     []ProductOptionValue `json:"values"`
}

// ProductOptionValue 一个客户可选值及其加价。
type ProductOptionValue struct {
	Value   string `json:"value" binding:"required"`
	Label   string `json:"label"`
	Group   string `json:"group_label"`
	Default bool   `json:"is_default"`
	Hidden  bool   `json:"hidden"`
	// 四周期加价；均为 0 表示该取值不加价（不影响算价）。
	PriceMonthly   float64 `json:"price_monthly"`
	PriceQuarterly float64 `json:"price_quarterly"`
	PriceAnnually  float64 `json:"price_annually"`
	PriceOnetime   float64 `json:"price_onetime"`
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
	// SourceMode 链路判据（D6）：指针区分"未传"（保持原值）与"显式改链路"。
	SourceMode *string `json:"source_mode"`
	// SourceProviderID 自营商品绑定的平台渠道（T4.2）：指针区分"未传"与"显式改绑"。
	// 代理商品的 source_provider_id 由导入流程决定，不接受本字段改写。
	SourceProviderID *uint64 `json:"source_provider_id"`
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
	// GeneratedSpecs 建品时按模板生成的 SKU（仅 Create 响应带值，供前端提示"建了什么"）。
	GeneratedSpecs []ProductSpecInfo `json:"generated_specs,omitempty"`
	// SpecTemplateNotice 建品时规格生成的部分失败原因（商品已建，规格可到详情页重试）。
	SpecTemplateNotice string `json:"spec_template_notice,omitempty"`
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

// SpecTemplateGenerateRequest 按规格模板为自营商品生成 SKU 并建立平台绑定。
// 自营链路打通的关键动作：模板（含平台写参数）+ 商品 → 已确认绑定的 SKU。
type SpecTemplateGenerateRequest struct {
	SpecTemplateID uint64 `json:"spec_template_id" binding:"required"`
	// SpecCode / Name 覆盖从模板派生的 SKU 编码与名称；为空按模板自动生成。
	SpecCode string `json:"spec_code"`
	Name     string `json:"name"`
	// Price / CostPrice 覆盖 SKU 售价与成本价；为 0 时依次回落到模板参考售价、商品售价。
	Price     float64 `json:"price"`
	CostPrice float64 `json:"cost_price"`
	Stock     int     `json:"stock"`
	// PlatformParams 覆盖模板的平台参数（如换 area/node）；为空则原样用模板值。
	PlatformParams json.RawMessage `json:"platform_params"`
	// SpecValues 覆盖模板的原子取值；为空则原样用模板值（再空则按模板配置字段推导）。
	SpecValues json.RawMessage `json:"spec_values"`
	// Confirm 是否直接把绑定置为 confirmed（默认 true）。
	Confirm *bool `json:"confirm"`
}

// SpecTemplateGenerateResult 按模板生成 SKU 的结果。
type SpecTemplateGenerateResult struct {
	Spec ProductSpecInfo `json:"spec"`
	// BoundPlatformParams 实际写入绑定的平台参数 JSON（为空表示未建立绑定）。
	BoundPlatformParams json.RawMessage `json:"bound_platform_params"`
	// Notice 非空表示已生成 SKU 但需人工补充的事项（如平台绑定能力未装配、模板缺平台参数）。
	Notice string `json:"notice,omitempty"`
}
