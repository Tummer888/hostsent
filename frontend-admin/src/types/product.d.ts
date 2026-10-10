// 类型定义（拆分自 interface.d.ts：product 域）
import type { ListMeta } from './common'

export interface SaleProductListQuery {
  keyword?: string
  category_id?: number
  status?: number
  /** 链路判据（D6）：self 自营 / upstream 上游转售 */
  source_mode?: string
  /** 推荐位过滤：不传=不过滤，true/false=精确筛选（服务端支持） */
  featured?: boolean
  page?: number
  page_size?: number
}

export interface SaleProductCreateRequest {
  code: string
  name: string
  category_id?: number
  product_type?: string
  description?: string
  cover_image?: string
  specs?: string
  price_model?: string
  price?: number
  cost_price?: number
  source_product_id?: number
  source_provider_id?: number
  /** 链路判据（D6）：self 自营 / upstream 上游转售；省略视为 self */
  source_mode?: string
  config_options?: string
  stock?: number
  sort_order?: number
  status?: number
  // 上游加价规则（T4.3）：percent 按成本百分比、fixed 加固定额；空串表示不配置。
  upstream_markup_type?: string
  upstream_markup_value?: number
  // 代理商品「仅透传」标记：上游规格未归一确认时放行上架。
  spec_passthrough?: boolean
  /**
   * 建品同时选定的规格模板（自营链路）：每个模板生成一个带平台绑定的 SKU。
   * 这是「新建商品就能预选配好的规格」的入口。
   */
  spec_templates?: SaleProductSpecTemplateSelection[]
}

/** 建品时选定一个规格模板（含就地调整后的参数） */
export interface SaleProductSpecTemplateSelection {
  spec_template_id: number
  /** 覆盖从模板派生的 SKU 编码与名称（留空自动派生） */
  spec_code?: string
  name?: string
  /** 售价；为 0 时回落模板参考售价，再回落商品售价 */
  price?: number
  cost_price?: number
  stock?: number
  /** 就地调整后的原子取值 JSON（compute.cpu / compute.memory(MB) / storage.system.size …） */
  spec_values?: Record<string, unknown> | null
  /** 就地调整后的平台写参数 JSON（area/node/os/store）；留空用模板值 */
  platform_params?: Record<string, unknown> | null
  /** 就地覆盖的客户选配项（T4.5）：留空按档位 option_selections 自动生成 */
  option_overrides?: SaleProductOptionOverride[]
}

export interface SaleProductCloneRequest {
  source_product_id: number
  source_provider_id: number
  code: string
  name?: string
  category_id?: number
  price?: number
  cost_price?: number
  config_options?: string
  stock?: number
  status?: number
}

export interface SaleProductBatchCloneRequest {
  source_provider_id: number
  source_product_ids: number[]
  category_id?: number
  price_percent?: number
  status?: number
}

export interface SaleProductUpdateRequest {
  name: string
  category_id?: number
  /** 自营商品的平台渠道（改绑）：省略=保持原值；代理商品不允许改 */
  source_provider_id?: number
  product_type?: string
  description?: string
  cover_image?: string
  specs?: string
  price_model?: string
  price?: number
  cost_price?: number
  config_options?: string
  stock?: number
  sort_order?: number
  status?: number
  /** 链路判据（D6）：省略=保持原值，显式传则改链路 */
  source_mode?: string
  // 加价规则与透传标记（T4.3）：指针语义——省略保持原值。
  upstream_markup_type?: string
  upstream_markup_value?: number
  spec_passthrough?: boolean
}

export interface SaleProductPriceRequest {
  price: number
  cost_price: number
  remark?: string
}

export interface SaleProductInfo {
  id: number
  code: string
  name: string
  category_id: number
  product_type: string
  description: string
  cover_image: string
  specs: string
  price_model: string
  price: number
  cost_price: number
  source_product_id: number
  source_provider_id: number
  /** 链路判据（D6 单一判据）：self 自营 / upstream 上游转售 */
  source_mode: string
  config_options: string
  /** 上游加价规则（T4.3）：percent / fixed，空串表示未配置 */
  upstream_markup_type: string
  upstream_markup_value: number
  /** 代理商品「仅透传」标记（T4.3） */
  spec_passthrough: boolean
  featured: boolean
  stock: number
  sort_order: number
  status: number
  created_at: string
  updated_at: string
  /** 建品时按模板生成的 SKU（仅创建响应带值） */
  generated_specs?: SaleProductSpecInfo[]
  /** 建品时规格生成的部分失败原因（商品已建，规格可到详情页重试） */
  spec_template_notice?: string
}

export interface SaleProductListResponse {
  items: SaleProductInfo[]
  meta: ListMeta
}

/** SKU 出站平台绑定状态（T4.2）：空串表示未建立绑定 */
export type SaleProductSpecBindingStatus = '' | 'unmapped' | 'auto_mapped' | 'confirmed' | 'stale'

export interface SaleProductSpecInfo {
  id: number
  product_id: number
  spec_code: string
  name: string
  specs: string
  price_model: string
  price: number
  cost_price: number
  stock: number
  /** 引用的标准规格模板 ID（T4.2）；0 表示未引用 */
  spec_template_id: number
  sort_order: number
  status: number
  /** 该 SKU 的出站平台绑定状态（T4.2/T4.6） */
  binding_status: SaleProductSpecBindingStatus | string
  /** 已确认的出站平台参数 JSON（无绑定为空串） */
  platform_params?: string
}

export interface SaleProductSpecRequest {
  spec_code: string
  name: string
  specs?: string
  price_model?: string
  price?: number
  cost_price?: number
  stock?: number
  spec_template_id?: number
  sort_order?: number
  status?: number
}

/** 商品可配置项（T4.4）：source=upstream 走上游配置 id，source=self 直接下发平台参数名 */
export interface SaleProductConfigSub {
  option_name: string
  upstream_id?: number
  source?: string
  source_key?: string
  hidden?: number
  sort_order?: number
  /** 分组标签（Ubuntu/Windows/CentOS），用户侧分组下拉用（T4.5） */
  group_label?: string
  /** 该取值是否为客户默认选中项（T4.5） */
  is_default?: boolean
  pricings?: Array<{
    monthly?: number
    annually?: number
    quarterly?: number
    onetime?: number
  }>
}

export interface SaleProductConfigOption {
  option_name: string
  option_type?: number
  qty_minimum?: number
  qty_maximum?: number
  upstream_id?: number
  source?: string
  source_key?: string
  hidden?: number
  sort_order?: number
  // ---- T4.5 客户选配渲染与计价 ----
  provider_type?: string
  option_key?: string
  /** select | radio | qty | bool | group_select */
  widget?: string
  /** 客户下单必选 */
  required?: boolean
  default_value?: string
  /** 分组标签（同组下拉的分组名） */
  widget_group?: string
  min_value?: number | null
  max_value?: number | null
  unit?: string
  help?: string
  sub?: SaleProductConfigSub[]
}

export interface SaleProductConfigGroup {
  id?: number
  name?: string
  description?: string
  options: SaleProductConfigOption[]
}

/** 规格绑定（spec_bindings，T4.2/T4.3）：external_spec_id 与 product_spec_id 二选一 */
export interface SpecBindingInfo {
  id: number
  external_spec_id?: number
  product_spec_id?: number
  product_spec_code?: string
  spec_template_id: number
  direction: string
  platform_params?: Record<string, unknown> | string | null
  match_type: string
  status: string
  confidence: number
  confirmed_by: number
  confirmed_at?: string | null
  remark: string
  priority: number
  created_at: string
  updated_at: string
}

export interface SpecBindingUpsertRequest {
  external_spec_id?: number
  product_spec_id?: number
  spec_template_id?: number
  direction?: string
  platform_params?: Record<string, unknown> | string
  match_type?: string
  status?: string
  confidence?: number
  remark?: string
  priority?: number
}

export interface SpecBindingConfirmRequest {
  spec_template_id?: number
  platform_params?: Record<string, unknown> | string
  remark?: string
}

/** 外部规格快照（external_specs，T4.3） */
export interface ExternalSpecInfo {
  id: number
  provider_id: number
  provider_type: string
  external_id: string
  external_name: string
  external_kind: string
  raw?: unknown
  normalized?: unknown
  fingerprint: string
  status: string
  synced_at?: string | null
  created_at: string
}

/** 规格原子字典项（spec_atoms） */
export interface SpecAtomInfo {
  id: number
  key: string
  name: string
  unit: string
  value_type: string
  enum_values?: unknown
  min_value?: number | null
  max_value?: number | null
  step_value?: number | null
  required: boolean
  configurable: boolean
  applies_to: string
  platform_fields?: Record<string, { read?: string; write?: string; source?: string }> | null
  description: string
  sort_order: number
  status: number
}

export interface SaleProductHistoryInfo {
  id: number
  product_id: number
  change_type: string
  old_value: string
  new_value: string
  operator_name: string
  remark: string
  created_at: string
}

export interface SaleProductCategoryCreateRequest {
  parent_id?: number
  name: string
  icon?: string
  sort_order?: number
  status?: number
  /** 分类级拿货折扣率（doc108），如 0.6 = 六折进货；0 = 未配置（跳过毛利校验）。 */
  cost_rate?: number
}

export type SaleProductCategoryUpdateRequest = SaleProductCategoryCreateRequest

export interface SaleProductCategoryInfo {
  id: number
  parent_id: number
  name: string
  icon: string
  sort_order: number
  status: number
  /** 分类级拿货折扣率（doc108）；0 = 未配置。代理折扣的毛利校验基准。 */
  cost_rate?: number
  children?: SaleProductCategoryInfo[]
}

export interface SaleProductCategoryListResponse {
  items: SaleProductCategoryInfo[]
}

export interface SpecTemplateQuery {
  [key: string]: unknown
  keyword?: string
  spec_family?: string
  /** 按归属平台筛选（配置档改造后的主筛选维度） */
  provider_type?: string
  status?: number
  page?: number
  page_size?: number
}

export interface SpecTemplateRequest {
  name: string
  spec_family?: string
  cpu?: number
  memory?: number
  disk?: number
  disk_type?: string
  bandwidth?: number
  os?: string
  description?: string
  price?: number
  sort_order?: number
  status?: number
  /** 原子 key → 取值 JSON（自营规格模板核心）；留空由 CPU/内存/磁盘等推导 */
  spec_values?: Record<string, unknown> | null
  /** 平台写参数 JSON（魔方云 area/node/os/store），生成 SKU 时写入平台绑定 */
  platform_params?: Record<string, unknown> | null
  /** 档位归属的平台类型（mofangyun 等） */
  provider_type?: string
  /** 每参数勾选的可选值（T4.5）：建品时据此生成客户选配项 */
  option_selections?: SpecOptionSelections | null
  /** 商品名渲染模板，如 "{cpu}核{memory}G {os}" */
  name_template?: string
  /** 描述渲染模板 */
  description_template?: string
}

export interface SpecTemplateInfo {
  id: number
  name: string
  spec_family: string
  cpu: number
  memory: number
  disk: number
  disk_type: string
  bandwidth: number
  os: string
  description: string
  price: number
  sort_order: number
  status: number
  /** 原子取值 JSON（可能为 null：按 CPU/内存/磁盘推导展示） */
  spec_values: Record<string, unknown> | null
  /** 平台写参数 JSON（为 null 表示该模板尚未完成平台映射） */
  platform_params: Record<string, unknown> | null
  /** 归属平台类型 */
  provider_type: string
  /** 每参数勾选的可选值（可能为 null：老模板未配置） */
  option_selections: SpecOptionSelections | null
  /** 商品名/描述渲染模板 */
  name_template: string
  description_template: string
  /** 模板来源：self 自建 / imported 上游归一 */
  source: string
  created_at: string
  updated_at: string
}

/** 按规格模板为自营商品生成 SKU 并建立平台绑定 */
export interface SpecTemplateGenerateRequest {
  spec_template_id: number
  spec_code?: string
  name?: string
  price?: number
  cost_price?: number
  stock?: number
  platform_params?: Record<string, unknown> | null
  spec_values?: Record<string, unknown> | null
  confirm?: boolean
}

export interface SpecTemplateGenerateResult {
  spec: SaleProductSpecInfo
  bound_platform_params: Record<string, unknown> | null
  notice?: string
}

/** 平台可售资源项（区域/节点/存储/镜像） */
export interface PlatformResourceItem {
  value: string
  label: string
  parent_id?: string
  /** 资源自身分组名（镜像家族 Ubuntu/Windows/CentOS） */
  group?: string
  status?: string
}

export interface PlatformResources {
  areas: PlatformResourceItem[]
  nodes: PlatformResourceItem[]
  stores: PlatformResourceItem[]
  images: PlatformResourceItem[]
}

// ===== 平台配置项目录与取值库（T4.5 规格配置化）=====

/** 平台配置项的一个可选值 */
export interface OptionValueItem {
  value: string
  label: string
  group_label?: string
  parent_value?: string
  status?: string
  origin?: string
}

/** 平台配置项（目录项） */
export interface OptionSpecInfo {
  id: number
  provider_type: string
  option_key: string
  label: string
  group_name: string
  required: boolean
  default_value: string
  widget: string
  value_source: string
  options?: unknown
  min_value?: number | null
  max_value?: number | null
  step_value?: number | null
  unit: string
  help: string
  multi_value: boolean
  hidden: boolean
  sort_order: number
  source: string
  values?: OptionValueItem[]
}

export interface OptionSpecRequest {
  provider_type?: string
  option_key: string
  label?: string
  group_name?: string
  required?: boolean
  default_value?: string
  widget?: string
  value_source?: string
  options?: unknown
  min_value?: number | null
  max_value?: number | null
  step_value?: number | null
  unit?: string
  help?: string
  multi_value?: boolean
  hidden?: boolean
  sort_order?: number
}

export interface OptionCatalogSyncResult {
  provider_type: string
  declared: number
  created: number
  skipped: number
}

export interface OptionValueImportRequest {
  provider_type?: string
  option_key: string
  replace?: boolean
  items: OptionValueItem[]
}

export interface OptionValueImportResult {
  option_key: string
  created: number
  updated: number
  offlined: number
}

export interface OptionValueRefreshRequest {
  provider_type?: string
  provider_id: number
  option_keys?: string[]
}

/** 配置档里单个参数的勾选结果 */
export interface SpecOptionSelection {
  /** 离散多选取值 */
  values?: string[]
  /** 数量型区间 [min, max] */
  range?: number[]
  /** 默认值（与平台写参数口径一致） */
  default?: string
  group_label?: string
}

/** 配置档勾选的参数 → 勾选结果 */
export type SpecOptionSelections = Record<string, SpecOptionSelection>

/** 建品时就地覆盖的客户选配项 */
export interface SaleProductOptionOverride {
  option_key: string
  label?: string
  widget?: string
  required?: boolean
  default?: string
  unit?: string
  group_label?: string
  help?: string
  min_value?: number | null
  max_value?: number | null
  sort_order?: number
  values?: SaleProductOptionValue[]
}

export interface SaleProductOptionValue {
  value: string
  label?: string
  group_label?: string
  is_default?: boolean
  hidden?: boolean
  price_monthly?: number
  price_quarterly?: number
  price_annually?: number
  price_onetime?: number
}

export interface SpecTemplateListResponse {
  items: SpecTemplateInfo[]
  meta: ListMeta
}

export interface SpecMappingQuery {
  [key: string]: unknown
  provider_type?: string
  keyword?: string
  status?: number
  page?: number
  page_size?: number
}

export interface SpecMappingRequest {
  provider_type: string
  upstream_spec_id: string
  upstream_name?: string
  platform_spec_id?: number
  platform_name?: string
  cpu?: number
  memory?: number
  disk?: number
  status?: number
}

export interface SpecMappingBindRequest {
  platform_spec_id: number
  platform_name?: string
}

export interface SpecMappingInfo {
  id: number
  provider_type: string
  upstream_spec_id: string
  upstream_name: string
  platform_spec_id: number
  platform_name: string
  cpu: number
  memory: number
  disk: number
  status: number
  created_at: string
  updated_at: string
}

export interface SpecMappingListResponse {
  items: SpecMappingInfo[]
  meta: ListMeta
}

export interface PricingQuery {
  [key: string]: unknown
  product_id?: number
  billing_mode?: string
  status?: number
  page?: number
  page_size?: number
}

export interface PricingRequest {
  product_id: number
  billing_mode: string
  unit_price?: number
  min_billing_unit?: number
  tier_pricing?: string
  tier_discount?: string
  status?: number
}

export interface PricingInfo {
  id: number
  product_id: number
  billing_mode: string
  unit_price: number
  min_billing_unit: number
  tier_pricing: string
  tier_discount: string
  status: number
  created_at: string
  updated_at: string
}

export interface PricingListResponse {
  items: PricingInfo[]
  meta: ListMeta
}

/** 周期价格矩阵（doc25）：商品 × 规格 × 周期 一档价格 */
export type ProductPriceSource = 'upstream' | 'markup' | 'manual'

export interface ProductPriceItem {
  cycle: string
  cycle_name: string
  price: number
  cost_price: number
  setup_fee: number
  cost_setup_fee: number
  source: ProductPriceSource | string
  /** 1 启用可售 / 0 停用 */
  status: number
  /** 上游派生行只读：价格由上游成本 + 加价规则推导，只能改启停 */
  editable: boolean
  remark: string
}

export interface ProductPriceMatrix {
  product_id: number
  spec_id: number
  product_name: string
  currency: string
  /** self 自营 / upstream 上游转售 */
  source_mode: string
  /** 渠道能力声明的可售周期（上游商品据此限制可选档位） */
  upstream_cycles: string[]
  items: ProductPriceItem[]
}

export interface ProductPriceMatrixQuery {
  [key: string]: unknown
  product_id: number
  spec_id?: number
  currency?: string
}

export interface ProductPriceMatrixSaveItem {
  cycle: string
  price: number
  cost_price: number
  setup_fee: number
  cost_setup_fee: number
  status: number
}

export interface ProductPriceMatrixSaveRequest {
  product_id: number
  spec_id?: number
  currency?: string
  remark?: string
  items: ProductPriceMatrixSaveItem[]
}

export interface CouponQuery {
  [key: string]: unknown
  keyword?: string
  coupon_type?: string
  status?: number
  page?: number
  page_size?: number
}

export interface CouponRequest {
  coupon_code?: string
  name: string
  coupon_type: string
  amount?: number
  min_amount?: number
  discount?: number
  total_stock?: number
  per_user_limit?: number
  scope?: string
  valid_from?: string
  valid_to?: string
  status?: number
}

export interface CouponInfo {
  id: number
  coupon_code: string
  name: string
  coupon_type: string
  amount: number
  min_amount: number
  discount: number
  total_stock: number
  claimed_count: number
  per_user_limit: number
  scope: string
  valid_from: string
  valid_to: string
  status: number
  created_at: string
  updated_at: string
}

export interface CouponListResponse {
  items: CouponInfo[]
  meta: ListMeta
}

export interface CouponGrantQuery {
  [key: string]: unknown
  coupon_id?: number
  status?: string
  keyword?: string
  page?: number
  page_size?: number
}

export interface CouponGrantCreateRequest {
  coupon_id: number
  user_ids: number[]
}

export interface CouponGrantInfo {
  id: number
  coupon_id: number
  user_id: number
  user_name: string
  status: string
  valid_from: string
  valid_to: string
  used_at: string
  created_at: string
}

export interface CouponGrantListResponse {
  items: CouponGrantInfo[]
  meta: ListMeta
}

export interface PromotionQuery {
  [key: string]: unknown
  keyword?: string
  promotion_type?: string
  status?: number
  page?: number
  page_size?: number
}

export interface PromotionRequest {
  name: string
  promotion_type: string
  rule?: string
  start_time?: string
  end_time?: string
  sort_order?: number
  status?: number
}

export interface PromotionInfo {
  id: number
  name: string
  promotion_type: string
  rule: string
  start_time: string
  end_time: string
  sort_order: number
  status: number
  created_at: string
  updated_at: string
}

export interface PromotionListResponse {
  items: PromotionInfo[]
  meta: ListMeta
}

// ===== 折扣策略（P5 统一算价管线）=====

export interface PricePolicyQuery {
  [key: string]: unknown
  keyword?: string
  status?: string
  scope?: string
  page?: number
  page_size?: number
}

export interface PricePolicyItemRequest {
  target_type: string
  target_id: number
  discount_type: string
  discount_value: number
}

export interface PricePolicyRequest {
  name: string
  code: string
  discount_type: string
  discount_value: number
  scope?: string
  priority?: number
  effective_from?: string
  effective_to?: string
  status?: string
  remark?: string
  items?: PricePolicyItemRequest[]
}

export interface PricePolicyItemInfo {
  id: number
  target_type: string
  target_id: number
  discount_type: string
  discount_value: number
}

export interface PricePolicyInfo {
  id: number
  name: string
  code: string
  discount_type: string
  discount_value: number
  scope: string
  priority: number
  effective_from: string
  effective_to: string
  status: string
  remark: string
  items: PricePolicyItemInfo[]
  created_at: string
  updated_at: string
}

export interface PricePolicyListResponse {
  items: PricePolicyInfo[]
  meta: ListMeta
}
