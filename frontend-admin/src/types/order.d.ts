// 类型定义（拆分自 interface.d.ts：order 域）
import type { ListMeta } from './common'

export interface OrderListQuery {
  keyword?: string
  user_keyword?: string
  user_id?: number
  product_id?: number
  status?: string
  pay_method?: string
  /** 支付单号（doc36 §3.1）：经支付中心 payment_orders 反查，支持部分匹配 */
  payment_no?: string
  /** 渠道流水号（第三方交易号），对账排障用 */
  channel_tx?: string
  /** 订单类型：consume=普通购买 renewal=续费（renewal_id>0） */
  order_type?: string
  /** 核验状态筛选：pending=待核验（服务中未核验） verified=已核验 */
  verify_state?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

/** 批量操作单条结果：success=已执行 / skipped=状态不允许等可预期跳过 / failed=执行出错 */
export interface BatchItemResult {
  id: number
  status: 'success' | 'skipped' | 'failed'
  message?: string
}

export interface BatchActionResponse {
  items: BatchItemResult[]
  succeeded: number
  skipped: number
  failed: number
}

export interface BatchVerifyRequest {
  ids: number[]
  remark?: string
}

export interface BatchRefundAuditRequest {
  ids: number[]
  remark?: string
}

export interface OrderInfo {
  id: number
  order_no: string
  user_id: number
  product_id: number
  product_name: string
  specs: string
  quantity: number
  price_model: string
  total_amount: number
  paid_amount: number
  status: string
  pay_method: string
  pay_time: string
  expire_time: string
  remark: string
  /** 计费周期快照（月/季/年） */
  cycle: string
  /** 续费订单指向的原始订单 ID，>0 即续费单 */
  renewal_id: number
  /** 算价快照：原价 / 优惠 / 实付 */
  original_amount: number
  discount_amount: number
  final_amount: number
  discount_source: string
  /** 支付单号与渠道流水号（由支付中心反查填充） */
  payment_no: string
  channel_tx: string
  /** 开通履约状态：pending/running/success/failed/manual（manual=需人工处理） */
  provision_status?: string
  /** 开通失败原因（上游返回的原始信息，如"余额不足：账单需18.00，现余额5.08"） */
  provision_error?: string
  provision_attempts?: number
  /** 人工核验标记：verified_at 为空即未核验；verify_required 表示当前是否开启核验要求 */
  verified_at?: string
  verified_by?: number
  verified_by_name?: string
  verify_remark?: string
  verify_required?: boolean
  /** 核验状态（读取期推导）：pending=待核验 / verified=已核验 */
  verify_state?: string
  /** 关联实例（可跳转实例详情）；instance_pending 表示仍是待开通占位实例 */
  instance_id?: number
  instance_no?: string
  instance_pending?: boolean
  created_at: string
  updated_at: string
}

export interface OrderListResponse {
  items: OrderInfo[]
  meta: ListMeta
}

export interface OrderItemInfo {
  id: number
  order_id: number
  product_id: number
  product_name: string
  spec_code: string
  specs: string
  /** 行级计费周期快照 */
  cycle: string
  price: number
  quantity: number
  amount: number
  created_at: string
}

export interface OrderDetail extends OrderInfo {
  items: OrderItemInfo[]
  refunds: RefundInfo[]
}

export interface OrderRemarkUpdateRequest {
  remark: string
}

export interface RefundCreateRequest {
  amount: number
  reason?: string
  /** 退款去向（doc36 §3.2）：balance=退回余额（默认，消费口径不变）；channel=原路退回支付来源 */
  refund_mode?: string
  /** 渠道扣点（仅 channel 模式）：原路退回时渠道不退还的手续费，计入财务收入扣减 */
  fee_amount?: number
}

export interface OrderStatusStat {
  status: string
  count: number
}

export interface OrderTrendPoint {
  date: string
  sales: number
  orders: number
}

export interface OrderStatsResponse {
  today_sales: number
  today_orders: number
  avg_order_value: number
  refund_rate: number
  status_distribution: OrderStatusStat[]
  trend: OrderTrendPoint[]
}

export interface RefundListQuery {
  keyword?: string
  order_id?: number
  status?: string
  /** 退款去向：balance / channel */
  refund_mode?: string
  page?: number
  page_size?: number
}

export interface RefundInfo {
  id: number
  refund_no: string
  order_id: number
  order_no: string
  user_id: number
  amount: number
  reason: string
  status: string
  audit_by: number
  audit_by_name: string
  audited_at: string
  /** 退款去向：balance 退回余额 / channel 原路退回 */
  refund_mode: string
  /** 渠道扣点；net_amount = amount + fee_amount */
  fee_amount: number
  net_amount: number
  /** 原路退回时由支付中心回填的渠道退款单 */
  channel_refund_no: string
  channel_refund_status: string
  created_at: string
  updated_at: string
}

export interface RefundListResponse {
  items: RefundInfo[]
  meta: ListMeta
}

export interface RefundApproveRequest {
  remark?: string
}
