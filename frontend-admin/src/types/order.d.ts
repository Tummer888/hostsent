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
  /** 申请来源：admin / user */
  origin?: string
  /** 复核状态：pending（待复核）/ done（已复核） */
  review_state?: string
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
  /** 申请来源：admin=运营发起 / user=客户自助申请 */
  origin: string
  /** 审核备注与退款凭据外部单号 */
  audit_remark: string
  evidence_no: string
  /** 事后复核（免审核自动退款路径）：''（无需）/pending（待复核）/done（已复核） */
  review_state: string
  review_remark: string
  reviewed_by: number
  reviewed_by_name: string
  reviewed_at: string
  /** 凭据附件（详情接口返回） */
  attachments?: RefundAttachmentInfo[]
  created_at: string
  updated_at: string
}

export interface RefundListResponse {
  items: RefundInfo[]
  meta: ListMeta
}

export interface RefundApproveRequest {
  remark?: string
  /** 退款凭据外部单号（人工退款的转账/渠道流水号） */
  evidence_no?: string
  /** 本次审核上传的凭据附件 ID 列表 */
  attachment_ids?: number[]
}

/** 退款策略（按商品来源 + 分类配置可退时限与免审开关） */
export interface RefundPolicyInfo {
  id: number
  /** self / upstream / ''（不限来源） */
  source_mode: string
  /** 0 = 该来源下全部商品（含子孙分类） */
  category_id: number
  refundable: boolean
  /** 可退时限（小时，自开通成功起算）；0 = 不限 */
  window_hours: number
  /** 免人工审核自动退款（仅退回余额 + 全额时生效） */
  auto_approve: boolean
  status: string
  sort_order: number
  remark: string
  category_name?: string
  created_at: string
  updated_at: string
}

export interface RefundPolicyListResponse {
  items: RefundPolicyInfo[]
}

export interface RefundPolicyRequest {
  source_mode: string
  category_id: number
  refundable: boolean
  window_hours: number
  auto_approve: boolean
  status: string
  sort_order: number
  remark: string
}

/** 退款凭据附件 */
export interface RefundAttachmentInfo {
  id: number
  refund_id: number
  file_name: string
  file_size: number
  file_type: string
  uploader_id: number
  created_at: string
}

export interface RefundReviewRequest {
  remark?: string
}
