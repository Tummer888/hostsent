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

/** 一个可选配置项的取值（含加价）。 */
export interface OptionItem {
  value: string
  label: string
  /** 分组标签（Ubuntu/Windows/CentOS）：有值时按分组渲染下拉 */
  group_label?: string
  /** 是否该配置项的默认取值 */
  is_default: boolean
  price_monthly: number
  price_quarterly: number
  price_annually: number
  price_onetime: number
}

/** 客户可选配置项：按 widget 渲染控件（select/group_select 下拉、radio 按钮组、qty 步进器、bool 开关）。 */
export interface ProductOption {
  /** 平台参数名，下单时作为 configSelections 的键 */
  option_key: string
  name: string
  widget: string
  required: boolean
  default_value: string
  unit: string
  help: string
  min_value: number | null
  max_value: number | null
  /** 数量型选项的每单位加价（元）；枚举型加价随 Values 逐项给出 */
  unit_price?: number
  values: OptionItem[]
}

export interface ProductOptionGroup {
  options: ProductOption[]
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
  /** 客户可选配置项（自营）：档位之外可自选的参数，选中加价并入结算金额 */
  option_groups: ProductOptionGroup[]
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
  /** 待支付订单的支付截止时间（RFC3339）：仅 pending 有值，用于提示「请在 xx 前完成支付」 */
  pay_expire_at?: string
  /** 订单备注（用户侧只读） */
  remark?: string
}

/** 单个已选配置项的加价明细（预结算返回）。 */
export interface SelectedOption {
  option_key: string
  option_name: string
  value: string
  value_name: string
  quantity: number
  unit_price: number
  amount: number
  is_defaulted: boolean
}

/** 预结算价格明细（P5-05）。 */
export interface QuoteInfo {
  spec_code: string
  cycle: string
  original_amount: number
  discount_amount: number
  final_amount: number
  /** 选配项加价小计，已含在 original_amount 内 */
  option_amount?: number
  /** 已选选项明细，用于逐项展示加价来源 */
  options?: SelectedOption[]
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
  /** 客户选配项选择：配置项参数名 → 选中取值（如 cpu=4、os=62）。 */
  configSelections?: Record<string, string>
  /** 支付方式：balance 余额支付（默认，下单即开通）/ channel 渠道支付（落待支付单去收银台）。 */
  payMode?: 'balance' | 'channel'
}

/** 订单支付结果：收银台渲染与跳转所需参数。 */
export interface OrderPayInfo {
  order_id: number
  order_no: string
  payment_order_id: number
  payment_no: string
  amount: number
  channel_code: string
  channel_name: string
  scene: string
  status: string
  pay_url: string
  qrcode: string
  instructions: string
  subject: string
  expire_at: string
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
    option_groups: (Array.isArray(p.option_groups) ? p.option_groups : [])
      .map((g) => ({ ...g, options: Array.isArray(g.options) ? g.options : [] }))
      .filter((g) => g.options.length > 0),
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

export function createOrder({
  productId,
  specCode = '',
  cycle = '',
  quantity = 1,
  configSelections,
  payMode,
}: CreateOrderParams) {
  return request.post<any, { data: OrderInfo }>('/uc/orders', {
    product_id: productId,
    spec_code: specCode,
    cycle,
    quantity,
    // 选配项选择：键为平台参数名。无选配时不传，保持存量请求体不变。
    config_selections: configSelections && Object.keys(configSelections).length ? configSelections : undefined,
    // 留空由后端按 balance 处理（存量调用方不变）；channel 时订单落待支付，去收银台付款。
    pay_mode: payMode || undefined,
  })
}

/** 待支付订单发起收银台支付：金额由后端取算价快照，前端只选渠道与场景。 */
export function payOrder(orderId: number, data: { channel_code?: string; scene?: string } = {}) {
  return request.post<any, { data: OrderPayInfo }>(`/uc/orders/${orderId}/pay`, data)
}

/** 取消待支付订单：后端同时关闭未支付支付单并回补库存。 */
export function cancelOrder(orderId: number) {
  return request.post<any, { data: OrderInfo }>(`/uc/orders/${orderId}/cancel`)
}

/** 预结算：算价明细，不落库、不扣款（P5-05）。 */
export function quoteOrder({
  productId,
  specCode = '',
  cycle = '',
  quantity = 1,
  configSelections,
}: CreateOrderParams) {
  return request.post<any, { data: QuoteInfo }>('/uc/orders/quote', {
    product_id: productId,
    spec_code: specCode,
    cycle,
    quantity,
    config_selections: configSelections && Object.keys(configSelections).length ? configSelections : undefined,
  })
}

export function getMyOrders(params: { status?: string; page?: number; page_size?: number } = {}) {
  return request.get<any, { data: OrderListResponse }>('/uc/orders', { params })
}

/** 订单详情：他人订单后端按「不存在」处理，前端不需要区分。 */
export function getOrderDetail(id: number) {
  return request.get<any, { data: OrderInfo }>(`/uc/orders/${id}`)
}
