import request from '@/utils/request'

// 用户中心 · 支付接口（baseURL=/api/v1，拼接为 /api/v1/uc/payment/*）
// 注意：本项目 request 拦截器返回完整信封 { code, message, data }，因此各函数返回 .data。

export interface PaymentField {
  key: string
  label: string
  type: string
  required: boolean
  secret: boolean
  placeholder?: string
  help?: string
  default?: string
  options?: Array<{ label: string; value: string }>
}

export interface PaymentCapabilityDescriptor {
  mode: string
  scenes: string[] | null
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

/** 收银台可选支付渠道 */
export interface PaymentChannelInfo {
  id: number
  channel_code: string
  name: string
  type: string
  type_name: string
  mode: string
  scenes: string[]
  fee_rate: number
  settle_mode: string
  min_amount_fen: number
  max_amount_fen: number
  instructions?: string
  capabilities: PaymentCapabilityDescriptor
}

export interface MethodOptions {
  scene: string
  channels: PaymentChannelInfo[]
  default: string
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
  channel_code: string
  channel_type: string
  channel_name: string
  scene: string
  status: string
  channel_tx: string
  pay_url: string
  qrcode: string
  instructions: string
  subject: string
  expire_at: string
  paid_at: string
  created_at: string
}

export interface PreferenceItem {
  scene: string
  channel_code: string
  priority: number
  is_default: boolean
}

export interface PayoutAccountInfo {
  id: number
  channel: string
  account_no: string
  account_name: string
  bank_name: string
  branch: string
  is_default: boolean
  verified: boolean
  created_at: string
}

export interface PayoutAccountCreateRequest {
  channel: string
  account_no: string
  account_name: string
  bank_name?: string
  branch?: string
  is_default?: boolean
}

export interface WithdrawInfo {
  id: number
  withdraw_no: string
  user_id: number
  amount: number
  channel: string
  channel_name?: string
  account: string
  account_name: string
  bank_name: string
  status: string
  payout_no: string
  payout_mode: string
  channel_tx: string
  remark: string
  created_at: string
}

// ===== 支付方式与偏好 =====

/** 收银台可用支付方式（按我的偏好排序） */
export function getPaymentMethods(params: { scene?: string; amount?: number } = {}) {
  return request.get<any, { data: MethodOptions }>('/uc/payment/methods', { params })
}

/** 我的支付方式偏好（默认方式 + 优先级） */
export function getPaymentPreferences() {
  return request.get<any, { data: { items: PreferenceItem[] } }>('/uc/payment/preferences')
}

/** 保存支付方式偏好 */
export function savePaymentPreferences(data: {
  scene: string
  default?: string
  priorities: PreferenceItem[]
}) {
  return request.put<any, { data: { items: PreferenceItem[] } }>('/uc/payment/preferences', data)
}

// ===== 收款账户 =====

export function getPayoutAccounts() {
  return request.get<any, { data: { items: PayoutAccountInfo[] } }>('/uc/payment/accounts')
}

export function createPayoutAccount(data: PayoutAccountCreateRequest) {
  return request.post<any, { data: PayoutAccountInfo }>('/uc/payment/accounts', data)
}

export function setDefaultPayoutAccount(id: number) {
  return request.put<any, { data: unknown }>(`/uc/payment/accounts/${id}/default`)
}

// ===== 收银台下单 =====

/** 在线充值：创建充值单并发起渠道支付 */
export function payRecharge(data: { amount: number; channel_code?: string; scene?: string }) {
  return request.post<any, { data: PaymentOrderInfo }>('/uc/payment/recharge', data)
}

/** 账单支付：按账单应结金额发起渠道支付 */
export function payBill(data: { bill_id: number; bill_no?: string; amount?: number; channel_code?: string; scene?: string }) {
  return request.post<any, { data: PaymentOrderInfo }>('/uc/payment/bills/pay', data)
}

/** 申请提现：冻结余额，等待审核打款 */
export function applyWithdraw(data: { amount: number; account_id: number; payout_channel?: string; remark?: string }) {
  return request.post<any, { data: WithdrawInfo }>('/uc/payment/withdrawals', data)
}

/** 查询支付单（收银台轮询支付结果） */
export function getPaymentOrder(paymentNo: string) {
  return request.get<any, { data: PaymentOrderInfo }>(`/uc/payment/orders/${paymentNo}`)
}
