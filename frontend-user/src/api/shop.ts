import request from '@/utils/request'

/** 规格变体（SKU）：下单时把 spec_code 传给 POST /uc/orders。 */
export interface SkuInfo {
  spec_code: string
  name: string
  /** 原子取值 JSON 字符串，前端按需解析渲染规格参数 */
  specs: string
  price: number
  price_model: string
  /** -1 表示不限库存 */
  stock: number
  /** 该 SKU 可售周期（doc25）；为空表示未维护周期矩阵，下单只需 spec_code */
  cycles: string[]
}

export interface ProductInfo {
  id: number
  name: string
  description: string
  cover_image: string
  category_id: number
  product_type: string
  price: number
  price_model: string
  specs: string
  /** self 自营 / upstream 上游转售 */
  source_mode: string
  config_options: string
  featured: boolean
  created_at: string
  /** 商品级可售周期；空数组表示未维护周期价格矩阵 */
  cycles: string[]
  /** 商品下挂的规格；空数组表示未拆 SKU，按商品级价格下单 */
  skus: SkuInfo[]
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
  /** 所选 SKU 编码 / 数量 / 计费周期 */
  spec_code: string
  quantity: number
  cycle: string
  /** 支付时间（RFC3339，未支付为空串）；列表不保证填充，详情页展示 */
  pay_time: string
  /** 算价明细（P5-06）：原价 / 优惠 / 实付与折扣来源 */
  original_amount: number
  discount_amount: number
  final_amount: number
  discount_source: string
  created_at: string
  /** 开通履约状态（T5.1）：pending/running/success/failed/manual */
  provision_status?: string
  /** 开通失败原因 */
  provision_error?: string
  /** 命中规则明细 JSON（P5-04），详情页展示优惠构成 */
  price_snapshot?: string
  /** 计费到期时间（RFC3339，无则空串） */
  expire_time?: string
  /** 订单备注（用户侧只读） */
  remark?: string
}

/** 预结算价格明细（P5-05）。 */
export interface QuoteInfo {
  spec_code: string
  cycle: string
  original_amount: number
  discount_amount: number
  final_amount: number
  price_policy_id: number | null
  discount_source: string
  price_snapshot: Array<{ source: string; code: string; type: string; value: number }>
}

export interface OrderListResponse {
  items: OrderInfo[]
  page: number
  page_size: number
  total: number
}

/** 下单参数：规格与周期都为空时按商品级价格下单（存量行为）。 */
export interface CreateOrderParams {
  productId: number
  specCode?: string
  cycle?: string
  quantity?: number
}

/**
 * 归一化商品字段。
 *
 * 后端未拆 SKU / 未维护周期矩阵时返回的是 Go nil 切片，JSON 里是 `null` 而不是 `[]`
 * （列表接口按设计不下发这两项，恒为 null）。页面按数组使用（`skus.length`、
 * `cycles.includes()`），直接消费 null 会抛 TypeError，因此在 API 层收敛成空数组。
 */
function normalizeProduct(p: ProductInfo): ProductInfo {
  return {
    ...p,
    cycles: Array.isArray(p.cycles) ? p.cycles : [],
    skus: Array.isArray(p.skus) ? p.skus.map((s) => ({ ...s, cycles: Array.isArray(s.cycles) ? s.cycles : [] })) : [],
  }
}

export function getProducts(
  params: { keyword?: string; featured?: boolean; page?: number; page_size?: number } = {},
) {
  return request
    .get<any, { data: ProductListResponse }>('/uc/products', { params })
    .then((res) => ({ ...res, data: { ...res.data, items: (res.data?.items ?? []).map(normalizeProduct) } }))
}

export function getProductDetail(id: number) {
  return request
    .get<any, { data: ProductInfo }>(`/uc/products/${id}`)
    .then((res) => ({ ...res, data: normalizeProduct(res.data) }))
}

export function createOrder({ productId, specCode = '', cycle = '', quantity = 1 }: CreateOrderParams) {
  return request.post<any, { data: OrderInfo }>('/uc/orders', {
    product_id: productId,
    spec_code: specCode,
    cycle,
    quantity,
  })
}

/** 预结算：算价明细，不落库、不扣款（P5-05）。 */
export function quoteOrder({
  productId,
  specCode = '',
  cycle = '',
  quantity = 1,
}: CreateOrderParams) {
  return request.post<any, { data: QuoteInfo }>('/uc/orders/quote', {
    product_id: productId,
    spec_code: specCode,
    cycle,
    quantity,
  })
}

export function getMyOrders(params: { status?: string; page?: number; page_size?: number } = {}) {
  return request.get<any, { data: OrderListResponse }>('/uc/orders', { params })
}

/** 订单详情：他人订单后端按「不存在」处理，前端不需要区分。 */
export function getOrderDetail(id: number) {
  return request.get<any, { data: OrderInfo }>(`/uc/orders/${id}`)
}
