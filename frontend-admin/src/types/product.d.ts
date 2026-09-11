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
  stock?: number
  sort_order?: number
  status?: number
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
