import request from '@/utils/request'

// 用户中心 · 积分接口（baseURL=/api/v1，路径拼接为 /api/v1/uc/points/*）
// 积分与余额账本完全独立：积分不可用于支付、不可提现，仅作活动与权益用途。

export interface ListMeta {
  page: number
  page_size: number
  total: number
}

/** 积分规则（说明积分如何获得，只读展示）。 */
export interface PointRuleInfo {
  id: number
  code: string
  name: string
  scene: string
  earn_mode: string
  fixed_points: number
  points_per_yuan: number
  min_amount: number
  max_points_per_order: number
  valid_days: number
  status: number
  remark: string
}

/** 积分流水。 */
export interface PointTransactionInfo {
  id: number
  tx_no: string
  user_id: number
  type: string
  direction: number
  points: number
  balance_before: number
  balance_after: number
  biz_type: string
  ref_no: string
  remark: string
  expire_at: string
  created_at: string
}

export interface PointTransactionListResponse {
  items: PointTransactionInfo[]
  meta: ListMeta
}

/** 我的积分概览。 */
export interface PointOverview {
  user_id: number
  balance: number
  frozen: number
  total_earned: number
  total_spent: number
  rules: PointRuleInfo[]
  recent: PointTransactionInfo[]
}

export interface PointTransactionQuery {
  type?: string
  direction?: number
  page?: number
  page_size?: number
}

// 我的积分概览（余额 / 生效规则 / 最近流水）
export function getMyPoints() {
  return request.get<any, { data: PointOverview }>('/uc/points')
}

// 我的积分流水
export function getMyPointTransactions(params: PointTransactionQuery = {}) {
  return request.get<any, { data: PointTransactionListResponse }>('/uc/points/transactions', { params })
}
