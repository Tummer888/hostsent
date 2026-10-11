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
	Value string `json:"value" binding:"required"`
	Label string `json:"label"`
	Group string `json:"group_label"`
	// Help 该取值的一句话含义（特殊值如 -1 不开通 / 0 不限量 必须带给客户）。
	Help    string `json:"help"`
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

// ProductBatchCloneResponse 批量导入结果：成功项 + 失败项（逐条原因）。
//
// 为什么要有失败清单：导入按上游商品逐条建品，编码重复（同一上游商品已导入过）
// 等失败此前被静默跳过，接口仍返回成功，运营会以为"导进来了"。
type ProductBatchCloneResponse struct {
	Items  []ProductInfo              `json:"items"`
	Failed []ProductBatchCloneFailure `json:"failed"`
}

// ProductBatchCloneFailure 单条导入失败的原因。
type ProductBatchCloneFailure struct {
	SourceProductID uint64 `json:"source_product_id"`
	Name            string `json:"name"`
	Reason          string `json:"reason"`
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
	// Upstream 上游对接信息（仅 source_mode=upstream，且能读到上游镜像时有值）。
	// 详情页据此展示：渠道 / 上游商品与分类 / 标价与代理价 / 折扣 / 各周期价 / 固定配置与不可续费。
	Upstream *ProductUpstreamInfo `json:"upstream,omitempty"`
}

// ProductUpstreamInfo 对接商品的上游对接信息（商品详情页展示口径）。
type ProductUpstreamInfo struct {
	ProviderID   uint64 `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	ProviderType string `json:"provider_type"`
	// UpstreamID 上游商品 ID（resource_products.upstream_id）。
	UpstreamID string `json:"upstream_id"`
	// GroupID/GroupName 上游商品分组（分类）：与导入弹窗同一口径（上游 cart/all）。
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`
	// ListPrice 上游标价；CostPrice 代理价（我方采购价，已含上游给本账号的代理折扣）。
	ListPrice float64 `json:"list_price"`
	CostPrice float64 `json:"cost_price"`
	// DiscountDesc 折扣描述（如「代理折扣 70%（应付标价的 70%）」）。
	DiscountDesc string `json:"discount_desc"`
	// ListCycles/CostCycles 各周期标价与采购价（周期 → 金额），配置箱展示付费周期用。
	ListCycles map[string]float64 `json:"list_cycles"`
	CostCycles map[string]float64 `json:"cost_cycles"`
	// Renewable 是否可续费：上游一次性/免费商品不可续费，本地不提供续费入口。
	Renewable bool   `json:"renewable"`
	RenewNote string `json:"renew_note"`
	// Description 上游富文本商品描述（本地描述为空时展示）。
	Description string `json:"description"`
	// PayType 上游付费类型原文（recurring/onetime/free）。
	PayType string `json:"pay_type"`
	// FixedConfig 是否整机固定配置（上游全部配置项都只有一个可见取值）。
	FixedConfig bool `json:"fixed_config"`
	// Options 上游配置项清单（含锁定状态）：锁定项本地不可改，开通参数由上游商品决定。
	Options []ProductUpstreamOption `json:"options"`
}

// ProductUpstreamOption 上游配置项及其锁定状态。
// 锁定（Locked）＝上游该配置项只有一个可见取值（如「固定四核8G」），
// 本地任何修改都必然与上游商品不符 → 开通必然出错，故前端只读展示、履约侧拒绝覆盖。
type ProductUpstreamOption struct {
	// OptionName 上游配置项展示名（如 cpu|CPU）。
	OptionName string `json:"option_name"`
	// Key 归一化键（cpu/memory/os/bw/area/network_type…），履约守卫据此识别覆盖请求。
	Key string `json:"key"`
	// Locked 是否锁定；Value 为锁定时的唯一取值（展示用）。
	Locked   bool   `json:"locked"`
	Value    string `json:"value"`
	SubCount int    `json:"sub_count"`
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
