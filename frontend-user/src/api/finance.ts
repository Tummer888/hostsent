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
  created_at: string
}

export interface BillListResponse {
  items: BillInfo[]
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

// 查询我的账单
export function getMyBills(params: BillListQuery = {}) {
  return request.get<any, { data: BillListResponse }>('/uc/finance/bills', { params })
}
