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
	ID          uint64  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	CoverImage  string  `json:"cover_image"`
	CategoryID  uint64  `json:"category_id"`
	ProductType string  `json:"product_type"`
	Price       float64 `json:"price"`
	PriceModel  string  `json:"price_model"`
	Specs       string  `json:"specs"` // JSON 字符串（wire 层 `specs` 类型统一为 string；内容为商品展示规格）
	// 注意：这里刻意**不下发** source_mode（自营/上游转售）与 config_options
	// （平台原始写参数 JSON）。两者都是内部供货与对接口径，客户只需知道"能买什么、
	// 多少钱、怎么开通"，看到"这台是转售的、那台是自营的"只会有两个坏结果：
	// 挑渠道（自营被冷落）或发现转售关系。
	// 需要该信息的内部链路（下单投递履约任务）走 admin 侧 catalogdto.ProductInfo，
	// 与本 DTO 无关。
	Featured  bool   `json:"featured"`
	CreatedAt string `json:"created_at"`
	// Cycles 商品级可售周期（doc25）：monthly/quarterly/annually 等规范值。
	// 空数组表示未维护周期价格矩阵，按商品级单价下单（存量行为）。
	Cycles []string `json:"cycles"`
	// Skus 商品下挂的可售规格（T4.1）。空数组表示该商品未拆 SKU，按商品级价格下单。
	// 下单时把选中项的 spec_code 传给 POST /uc/orders 的 spec_code 字段。
	Skus []SkuInfo `json:"skus"`
	// OptionGroups 客户可选配置项（T4.5）：档位之外可自选的参数（操作系统/CPU 档/带宽…）。
	// 前端按每个 option 的 widget 渲染控件，选中加价并入预结算金额。
	OptionGroups []OptionGroup `json:"option_groups"`
	// Upstream 商品的固定配置信息（整机锁定项 / 是否可续费 / 下单说明）。
	// 只含展示口径：不含上游供应商名称、上游商品号、成本与代理价。
	// 字段名保留 upstream_chain 属历史命名（前端已按"固定配置"呈现，不出现上游字样）。
	Upstream *UpstreamChain `json:"upstream_chain,omitempty"`
	// Stock 商品级可售库存（-1=不限量，>=0=剩余可售数量）。
	// 上游转售商品没有 SKU，库存挂在商品行上：卖到 0 时前端应展示"已售罄"并禁用购买，
	// 后端下单同样会拦（避免超卖上游限量商品）。
	Stock int `json:"stock"`
}

// UpstreamChain 对接商品的客户可见链路信息。
type UpstreamChain struct {
	// FixedConfig 是否整机固定配置（上游全部配置项都只有一个取值）。
	FixedConfig bool `json:"fixed_config"`
	// Renewable 是否可续费；false 时前端不展示续费入口，RenewNote 说明原因。
	Renewable bool   `json:"renewable"`
	RenewNote string `json:"renew_note"`
	// Options 上游配置项（展示名 + 是否锁定 + 锁定值）。
	Options []UpstreamChainOption `json:"options"`
	// OrderNotes 下单说明（主机名可自定义 / 密码可指定 / 端口由系统分配等），购买页展示。
	OrderNotes []string `json:"order_notes"`
}

// UpstreamChainOption 上游配置项的客户可见信息。
type UpstreamChainOption struct {
	// Name 上游配置项键（cpu/memory/os…）；Label 中文名；Locked 是否固定不可改。
	Key    string `json:"key"`
	Name   string `json:"name"`
	Locked bool   `json:"locked"`
	Value  string `json:"value"` // 锁定时的唯一取值（展示用）
}

// OptionGroup 一组可选配置项（当前只有一个分组，保留结构便于以后分区）。
type OptionGroup struct {
	Options []Option `json:"options"`
}

// Option 客户可见的一个可选配置项。
type Option struct {
	OptionKey    string   `json:"option_key"` // 平台参数名，下单时作为 config_selections 的键
	Name         string   `json:"name"`
	Widget       string   `json:"widget"` // select|radio|qty|bool|group_select
	Required     bool     `json:"required"`
	DefaultValue string   `json:"default_value"`
	Unit         string   `json:"unit"`
	Help         string   `json:"help"`
	MinValue     *float64 `json:"min_value"`
	MaxValue     *float64 `json:"max_value"`
	// UnitPrice 数量型选项的每单位加价（元，月度口径）；枚举型加价在 Values 里逐项给出。
	//
	// 长周期必须用下面三档：上游每个周期各有单价（系统盘 0.8/月、2.4/季、9.6/年），
	// 前端按当前所选周期取值估算，只按月度估会让年付预览远低于实付
	// （用户反馈的"加配硬盘不随年付叠加"）。
	UnitPrice          float64      `json:"unit_price"`
	UnitPriceQuarterly float64      `json:"unit_price_quarterly"`
	UnitPriceAnnually  float64      `json:"unit_price_annually"`
	UnitPriceOnetime   float64      `json:"unit_price_onetime"`
	Values             []OptionItem `json:"values"`
}

// OptionItem 一个可选值及其加价。
type OptionItem struct {
	Value      string `json:"value"`
	Label      string `json:"label"`
	GroupLabel string `json:"group_label,omitempty"` // 分组（Ubuntu/Windows），前端做分组下拉
	// Help 该取值的一句话含义（-1 不开通 / 0 不限量 / auto 随机端口）。
	// -1/0/auto 这类取值光看取值读不出含义，购买页必须在选中后把履约结果讲清楚。
	Help           string  `json:"help,omitempty"`
	Default        bool    `json:"is_default"`
	PriceMonthly   float64 `json:"price_monthly"`
	PriceQuarterly float64 `json:"price_quarterly"`
	PriceAnnually  float64 `json:"price_annually"`
	PriceOnetime   float64 `json:"price_onetime"`
}

// SkuInfo 用户可见的规格变体（不含成本价）。
type SkuInfo struct {
	SpecCode   string  `json:"spec_code"`
	Name       string  `json:"name"`
	Specs      string  `json:"specs"` // 原子取值 JSON，便于前端渲染规格参数
	Price      float64 `json:"price"`
	PriceModel string  `json:"price_model"`
	Stock      int     `json:"stock"` // -1 表示不限
	// Cycles 该 SKU 可售周期（doc25）；SKU 无自有矩阵行时回落商品级周期。
	Cycles []string `json:"cycles"`
}

// ListResponse 商品列表响应
type ListResponse struct {
	Items    []ProductInfo `json:"items"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Total    int64         `json:"total"`
}
