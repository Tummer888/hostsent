import request from '@/utils/request'

export interface ProductInfo {
  id: number
  name: string
  description: string
  category_id: number
  product_type: string
  price: number
  price_model: string
  specs: string
  provision_mode: string
  config_options: string
  featured: boolean
  created_at: string
}

export interface ProductListResponse {
  items: ProductInfo[]
  page: number
  page_size: number
  total: number
}

export interface OrderInfo {
  id: number
  order_no: string
  product_id: number
  product_name: string
  specs: string
  price_model: string
  total_amount: number
  paid_amount: number
  status: string
  pay_method: string
  /** 下单的真实操作人（子账号下单时有值，P4-09） */
  actor_user_id: number
  actor_name: string
  /** 算价明细（P5-06）：原价 / 优惠 / 实付与折扣来源 */
  original_amount: number
  discount_amount: number
  final_amount: number
  discount_source: string
  created_at: string
}

/** 预结算价格明细（P5-05）。 */
export interface QuoteInfo {
  original_amount: number
  discount_amount: number
  final_amount: number
  price_policy_id: number | null
  discount_source: string
  /** 命中的是代理价时为 true，前端展示「代理价」标识 */
  is_agent_price: boolean
  price_snapshot: Array<{ source: string; code: string; type: string; value: number }>
}

export interface OrderListResponse {
  items: OrderInfo[]
  page: number
  page_size: number
  total: number
}

export function getProducts(params: { keyword?: string; page?: number; page_size?: number } = {}) {
  return request.get<any, { data: ProductListResponse }>('/uc/products', { params })
}

export function getProductDetail(id: number) {
  return request.get<any, { data: ProductInfo }>(`/uc/products/${id}`)
}

export function createOrder(productId: number, quantity = 1) {
  return request.post<any, { data: OrderInfo }>('/uc/orders', { product_id: productId, quantity })
}

/** 预结算：算价明细，不落库、不扣款（P5-05）。 */
export function quoteOrder(productId: number, quantity = 1) {
  return request.post<any, { data: QuoteInfo }>('/uc/orders/quote', { product_id: productId, quantity })
}

export function getMyOrders(params: { status?: string; page?: number; page_size?: number } = {}) {
  return request.get<any, { data: OrderListResponse }>('/uc/orders', { params })
}
