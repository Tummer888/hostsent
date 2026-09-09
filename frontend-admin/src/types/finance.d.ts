// 类型定义（拆分自 interface.d.ts：finance 域）
import type { ListMeta } from './common'

export interface WalletInfo {
  user_id: number
  balance: number
  frozen: number
  total_income: number
  total_expense: number
  version: number
}

export interface TransactionListQuery {
  user_id?: number
  type?: string
  direction?: number
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface TransactionInfo {
  id: number
  tx_no: string
  user_id: number
  type: string
  direction: number // 1=收入 -1=支出
  amount: number
  balance_before: number
  balance_after: number
  order_id: number
  order_no: string
  ref_no: string
  remark: string
  operator_id: number
  created_at: string
}

export interface TransactionListResponse {
  items: TransactionInfo[]
  meta: ListMeta
}

export interface AdjustRequest {
  user_id: number
  type?: string
  direction: number
  amount: number
  biz_key: string
  remark?: string
}

export interface RechargeCreateRequest {
  user_id: number
  amount: number
  method: string
  remark?: string
}

export interface RechargeApproveRequest {
  channel_tx?: string
  remark?: string
}

export interface RechargeInfo {
  id: number
  recharge_no: string
  user_id: number
  amount: number
  method: string
  status: string
  channel_tx: string
  paid_at: string
  remark: string
  created_at: string
  updated_at: string
}

export interface RechargeListQuery {
  user_id?: number
  status?: string
  method?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface RechargeListResponse {
  items: RechargeInfo[]
  meta: ListMeta
}

export interface WithdrawListQuery {
  user_id?: number
  status?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface WithdrawInfo {
  id: number
  withdraw_no: string
  user_id: number
  amount: number
  channel: string
  account: string
  status: string
  audit_by: number
  audit_by_name: string
  audited_at: string
  paid_at: string
  remark: string
  created_at: string
  updated_at: string
}

export interface WithdrawListResponse {
  items: WithdrawInfo[]
  meta: ListMeta
}

export interface WithdrawAuditRequest {
  remark?: string
}

export interface BillListQuery {
  user_id?: number
  period?: string
  status?: string
  page?: number
  page_size?: number
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
  updated_at: string
}

export interface BillListResponse {
  items: BillInfo[]
  meta: ListMeta
}

export interface ReconcileResponse {
  period: string
  income_total: number
  expense_total: number
  tx_count: number
  wallet_balance: number
  diff: number
  status: string
}
