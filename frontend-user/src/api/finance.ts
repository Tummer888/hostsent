import request from '@/utils/request'

// 用户中心 · 财务接口（baseURL=/api/v1，路径拼接为 /api/v1/uc/finance/*）
// 注意：本项目 request 拦截器返回的是完整信封 { code, message, data }，因此各函数返回 .data。

export interface ListMeta {
  page: number
  page_size: number
  total: number
}

export interface WalletInfo {
  user_id: number
  balance: number
  frozen: number
  total_income: number
  total_expense: number
}

export interface TransactionInfo {
  id: number
  tx_no: string
  user_id: number
  type: string
  direction: number
  amount: number
  balance_before: number
  balance_after: number
  order_id?: number
  order_no?: string
  ref_no?: string
  biz_type?: string
  remark?: string
  created_at: string
}

export interface TransactionListResponse {
  items: TransactionInfo[]
  meta: ListMeta
}

export interface RechargeInfo {
  id: number
  recharge_no: string
  user_id: number
  amount: number
  method: string
  status: string
  channel_tx?: string
  /** 支付中心渠道编码与支付单 ID（在线充值经收银台下单后回填） */
  channel_code?: string
  payment_order_id?: number
  remark?: string
  paid_at?: string
  created_at: string
}

export interface RechargeListResponse {
  items: RechargeInfo[]
  meta: ListMeta
}

export interface BillInfo {
  id: number
  bill_no: string
  user_id: number
  period: string
  total_amount: number
  refund_amount: number
  status: string
  // 支付方式描述（doc34 F-11）：结清时记录实收金额与所用方式/渠道。
  paid_amount?: number
  paid_method?: string
  paid_channel_id?: number
  created_at: string
  updated_at?: string
}

export interface BillListResponse {
  items: BillInfo[]
  meta: ListMeta
}

/** 我的提现记录（含收款账户与打款方式，来自支付中心） */
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
  paid_at?: string
  created_at: string
}

export interface WithdrawListResponse {
  items: WithdrawInfo[]
  meta: ListMeta
}

export interface RechargeCreateRequest {
  amount: number
  method: string
  remark?: string
}

export interface TransactionListQuery {
  type?: string
  direction?: number
  page?: number
  page_size?: number
}

export interface BillListQuery {
  period?: string
  status?: string
  page?: number
  page_size?: number
}

// 查询我的余额
export function getBalance() {
  return request.get<any, { data: WalletInfo }>('/uc/finance/balance')
}

// 查询我的资金流水
export function getMyTransactions(params: TransactionListQuery = {}) {
  return request.get<any, { data: TransactionListResponse }>('/uc/finance/transactions', { params })
}

// 发起充值
export function createRecharge(data: RechargeCreateRequest) {
  return request.post<any, { data: RechargeInfo }>('/uc/finance/recharge', data)
}

// 查询我的充值单（doc34 F-07：此前以流水冒充，充值单号为空）
export function getMyRecharges(params: { status?: string; page?: number; page_size?: number } = {}) {
  return request.get<any, { data: RechargeListResponse }>('/uc/finance/recharges', { params })
}

// 查询我的账单
export function getMyBills(params: BillListQuery = {}) {
  return request.get<any, { data: BillListResponse }>('/uc/finance/bills', { params })
}

// 查询我的提现记录（支付中心）
export function getMyWithdrawals(params: { page?: number; page_size?: number } = {}) {
  return request.get<any, { data: WithdrawListResponse }>('/uc/payment/withdrawals', { params })
}
