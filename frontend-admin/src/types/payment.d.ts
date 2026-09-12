// 类型定义（拆分自 interface.d.ts：payment 域 · 支付中心）
// 对应后端 internal/modules/admin/payment/dto/payment.go 与 pkg/payment 契约层。
import type { ListMeta } from './common'

// ===== 契约层：能力描述符 =====

/** 凭证/端点字段描述（对应后端 pkg/integration.Field，驱动动态表单 + 字段级加密） */
export interface PaymentField {
  key: string
  label: string
  type: 'string' | 'password' | 'number' | 'select' | 'bool' | 'textarea'
  required: boolean
  secret: boolean
  placeholder?: string
  help?: string
  default?: string
  options?: Array<{ label: string; value: string }>
}

/** 支付渠道能力描述符（对应后端 payment.CapabilityDescriptor） */
export interface PaymentCapabilityDescriptor {
  /** api（接口）/ manual（线下人工） */
  mode: string
  scenes: string[] | null
  /** collect/query/close/refund/payout */
  operations: string[] | null
  credential_schema: PaymentField[] | null
  endpoint_schema: PaymentField[] | null
  cert_mode: string
  signer_type: string
  supports_payout: boolean
  supports_partial_refund: boolean
  supports_query: boolean
  currency: string
  fee_rate: number
  settle_mode: string
  implemented: boolean
  doc_url: string
}

// ===== 渠道类型 =====

export interface ChannelTypeItem {
  type: string
  name: string
  mode: string
  implemented: boolean
  adapter_version: string
  doc_url: string
  icon: string
  capabilities: PaymentCapabilityDescriptor
}

export interface ChannelTypeListResponse {
  items: ChannelTypeItem[]
}

// ===== 渠道实例 =====

export interface ChannelListQuery {
  type?: string
  status?: number
  keyword?: string
  page?: number
  page_size?: number
}

export interface ChannelCreateRequest {
  channel_code: string
  name: string
  type: string
  credentials?: Record<string, string>
  endpoint?: string
  notify_url?: string
  return_url?: string
  scenes?: string[]
  priority?: number
  weight?: number
  fee_rate?: number
  settle_mode?: string
  min_amount_fen?: number
  max_amount_fen?: number
  environment?: string
  is_default?: boolean
  remark?: string
}

export type ChannelUpdateRequest = Omit<ChannelCreateRequest, 'channel_code' | 'type'>

export interface ChannelInfo {
  id: number
  channel_code: string
  name: string
  type: string
  type_name: string
  mode: string
  /** 脱敏回显：secret 字段值为掩码，提交时原样回传表示不修改 */
  credentials: Record<string, string>
  endpoint: string
  notify_url: string
  return_url: string
  scenes: string[]
  priority: number
  weight: number
  fee_rate: number
  settle_mode: string
  min_amount_fen: number
  max_amount_fen: number
  environment: string
  health_status: string
  last_error: string
  last_check_at: string
  status: number
  is_default: boolean
  remark: string
  capabilities: PaymentCapabilityDescriptor
  created_at: string
  updated_at: string
}

export interface ChannelListResponse {
  items: ChannelInfo[]
  meta: ListMeta
}

export interface ChannelTestResponse {
  ok: boolean
  message: string
}

// ===== 支付方式路由（场景 → 渠道 + 优先级） =====

export interface MethodOptionsResponse {
  scene: string
  channels: ChannelInfo[]
  default: string
}

// ===== 支付单 =====

export interface PaymentOrderListQuery {
  user_id?: number
  payment_no?: string
  biz_type?: string
  channel_code?: string
  status?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface PaymentOrderInfo {
  id: number
  payment_no: string
  user_id: number
  biz_type: string
  biz_id: number
  biz_no: string
  amount: number
  amount_fen: number
  currency: string
  channel_id: number
  channel_code: string
  channel_type: string
  channel_name: string
  scene: string
  status: string
  channel_tx: string
  pay_url: string
  qrcode: string
  prepay_params: string
  instructions: string
  subject: string
  fee: number
  expire_at: string
  paid_at: string
  remark: string
  created_at: string
  updated_at: string
}

export interface PaymentOrderListResponse {
  items: PaymentOrderInfo[]
  meta: ListMeta
}

export interface OrderConfirmRequest {
  channel_tx?: string
  remark?: string
}

// ===== 回调日志 =====

export interface CallbackListQuery {
  channel_code?: string
  payment_no?: string
  verify_ok?: boolean
  page?: number
  page_size?: number
}

export interface CallbackInfo {
  id: number
  channel_id: number
  channel_code: string
  notify_id: string
  payment_no: string
  raw_body: string
  verify_ok: boolean
  amount: number
  handle_status: string
  handle_msg: string
  source_ip: string
  created_at: string
}

export interface CallbackListResponse {
  items: CallbackInfo[]
  meta: ListMeta
}

// ===== 渠道退款单 =====

export interface PaymentRefundListQuery {
  payment_no?: string
  user_id?: number
  status?: string
  page?: number
  page_size?: number
}

export interface PaymentRefundInfo {
  id: number
  refund_no: string
  payment_no: string
  order_refund_no: string
  user_id: number
  amount: number
  status: string
  reason: string
  fail_reason: string
  refunded_at: string
  created_at: string
}

export interface PaymentRefundListResponse {
  items: PaymentRefundInfo[]
  meta: ListMeta
}

// ===== 打款单 =====

export interface PayoutListQuery {
  user_id?: number
  status?: string
  mode?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface PayoutInfo {
  id: number
  payout_no: string
  withdraw_id: number
  withdraw_no: string
  user_id: number
  amount: number
  channel_id: number
  channel_code: string
  mode: string
  status: string
  channel_tx: string
  receipt_url: string
  fail_reason: string
  attempts: number
  operator_id: number
  paid_at: string
  remark: string
  created_at: string
  updated_at: string
}

export interface PayoutListResponse {
  items: PayoutInfo[]
  meta: ListMeta
}

export interface PayoutMarkPaidRequest {
  channel_tx?: string
  receipt_url?: string
  remark?: string
}

// ===== 渠道对账 =====

export interface ReconRequest {
  channel_code?: string
  period?: string
}

export interface ReconResponse {
  recon_no: string
  channel_code: string
  period: string
  local_amount: number
  local_count: number
  channel_amount: number
  channel_count: number
  diff: number
  status: string
}

export interface ReconRecordInfo extends ReconResponse {
  id: number
  created_at: string
}

export interface ReconRecordListResponse {
  items: ReconRecordInfo[]
  meta: ListMeta
}

// ===== 用户端收银台（管理端仅用于展示口径，实际由 frontend-user 调用） =====

export interface PreferenceItem {
  scene: string
  channel_code: string
  priority: number
  is_default: boolean
}
