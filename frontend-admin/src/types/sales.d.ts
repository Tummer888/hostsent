// 类型定义（销售中心：客户归属 / 提成台账 / 提成提现 / 业绩排行，doc86 §2.3）
import type { ListMeta } from './common'

// ===== 客户归属 =====

export interface SalesCustomerQuery {
  keyword?: string
  admin_id?: number
  department_id?: number
  /** 1 切到未归属池 */
  unassigned?: number
  status?: string
  page?: number
  page_size?: number
}

export interface SalesCustomerInfo {
  relation_id: number
  user_id: number
  username: string
  email: string
  phone: string
  admin_id: number
  admin_name: string
  admin_real_name: string
  department_id: number
  department_name: string
  status: string
  reason: string
  /** 保护期至；非空且晚于当前时间时不可改派 */
  protect_until: string
  protected: boolean
  effective_at: string
  released_at: string
  total_consume: number
  registered_at: string
}

export interface SalesCustomerListResponse {
  items: SalesCustomerInfo[]
  meta: ListMeta
}

export interface SalesUnassignedInfo {
  user_id: number
  username: string
  email: string
  phone: string
  total_consume: number
  registered_at: string
}

export interface SalesUnassignedListResponse {
  items: SalesUnassignedInfo[]
  meta: ListMeta
}

export interface SalesAssignRequest {
  user_id: number
  admin_id: number
  /** 变更原因，必填且 ≥5 字 */
  reason: string
}

export interface SalesReleaseRequest {
  user_id: number
  reason?: string
}

export interface SalesRelationInfo {
  id: number
  admin_id: number
  admin_name: string
  admin_real_name: string
  department_name: string
  status: string
  reason: string
  operator_id: number
  protect_until: string
  effective_at: string
  released_at: string
  created_at: string
}

export interface SalesRelationListResponse {
  items: SalesRelationInfo[]
}

export interface SalesCandidateInfo {
  admin_id: number
  username: string
  real_name: string
  department_id: number
  department_name: string
  active_customers: number
}

export interface SalesCandidateListResponse {
  items: SalesCandidateInfo[]
}

// ===== 提成台账 =====

export interface SalesCommissionQuery {
  admin_id?: number
  department_id?: number
  type?: string
  order_no?: string
  customer_id?: number
  start_at?: string
  end_at?: string
  page?: number
  page_size?: number
}

export interface SalesCommissionInfo {
  id: number
  tx_no: string
  admin_id: number
  admin_name: string
  admin_real_name: string
  department_name: string
  type: string
  type_label: string
  /** 1 入账 / -1 出账 */
  direction: number
  amount: number
  balance_after: number
  order_id: number
  order_no: string
  customer_user_id: number
  customer_name: string
  release_at: string
  remark: string
  created_at: string
}

export interface SalesCommissionListResponse {
  items: SalesCommissionInfo[]
  meta: ListMeta
}

export interface SalesCommissionSummary {
  admin_id: number
  admin_name: string
  admin_real_name: string
  department_name: string
  enabled: boolean
  balance: number
  pending_release: number
  frozen: number
  total_income: number
  total_out: number
  pending_withdraw: number
  withdrawable: number
  /** 可用余额为负（欠款） */
  debt: boolean
  min_withdraw: number
  release_days: number
  first_order_rate: number
  subsequent_rate: number
  renewal_rate: number
  active_customers: number
}

// ===== 提成提现 =====

export interface SalesWithdrawApplyRequest {
  amount: number
  channel?: string
  account?: string
  account_name?: string
  bank_name?: string
  remark?: string
}

export interface SalesWithdrawAuditRequest {
  remark?: string
}

export interface SalesWithdrawPayRequest {
  channel_tx?: string
  receipt_url?: string
  remark?: string
}

export interface SalesWithdrawalQuery {
  admin_id?: number
  department_id?: number
  status?: string
  page?: number
  page_size?: number
}

export interface SalesWithdrawalInfo {
  id: number
  withdraw_no: string
  admin_id: number
  admin_name: string
  admin_real_name: string
  department_name: string
  amount: number
  channel: string
  /** 已脱敏 */
  account: string
  account_name: string
  bank_name: string
  payout_mode: string
  payout_no: string
  status: string
  status_label: string
  audit_by: number
  audit_by_name: string
  audited_at: string
  paid_at: string
  remark: string
  created_at: string
}

export interface SalesWithdrawalListResponse {
  items: SalesWithdrawalInfo[]
  meta: ListMeta
}

// ===== 业绩与排行 =====

export interface SalesRankingQuery {
  period?: string
  department_id?: number
  sort_by?: string
}

export interface SalesRankingRow {
  rank: number
  admin_id: number
  admin_name: string
  admin_real_name: string
  department_id: number
  department_name: string
  amount: number
  orders: number
  target_amount: number
  target_orders: number
  achievement: number
}

export interface SalesRankingResponse {
  period: string
  items: SalesRankingRow[]
}

export interface SalesTargetUpsertItem {
  period: string
  scope?: string
  admin_id?: number
  department_id?: number
  target_amount?: number
  target_orders?: number
}

export interface SalesTargetInfo {
  id: number
  period: string
  scope: string
  admin_id: number
  admin_name: string
  admin_real_name: string
  department_id: number
  department_name: string
  target_amount: number
  target_orders: number
  actual_amount: number
  actual_orders: number
  achievement: number
}

export interface SalesTargetListResponse {
  period: string
  items: SalesTargetInfo[]
}

export interface SalesMyPerformance {
  period: string
  admin_id: number
  admin_name: string
  department_name: string
  amount: number
  orders: number
  target_amount: number
  target_orders: number
  achievement: number
  rank: number
  dept_members: number
  active_customers: number
  month_new_customers: number
}
