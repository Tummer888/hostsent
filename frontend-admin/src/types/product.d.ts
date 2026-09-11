// 类型定义（拆分自 interface.d.ts：product 域）
import type { ListMeta } from './common'

export interface SaleProductListQuery {
  keyword?: string
  category_id?: number
  status?: number
  provision_mode?: string
  source_mode?: string
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
  provision_mode?: string
  config_options?: string
  stock?: number
  sort_order?: number
  status?: number
  // 上游加价规则（T4.3）：percent 按成本百分比、fixed 加固定额；空串表示不配置。
  upstream_markup_type?: string
  upstream_markup_value?: number
  // 代理商品「仅透传」标记：上游规格未归一确认时放行上架。
  spec_passthrough?: boolean
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
  provision_mode: string
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
}

export type SaleProductCategoryUpdateRequest = SaleProductCategoryCreateRequest

export interface SaleProductCategoryInfo {
  id: number
  parent_id: number
  name: string
  icon: string
  sort_order: number
  status: number
  children?: SaleProductCategoryInfo[]
}

export interface SaleProductCategoryListResponse {
  items: SaleProductCategoryInfo[]
}

export interface SpecTemplateQuery {
  [key: string]: unknown
  keyword?: string
  spec_family?: string
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
  created_at: string
  updated_at: string
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
