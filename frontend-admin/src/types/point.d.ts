// 类型定义（docs/实施计划/36：积分体系）
// 边界：积分与余额（资金账本）完全独立，不可抵扣、不可提现。
import type { ListMeta } from './common'

// ===== 规则 =====

export interface PointRuleListQuery {
  scene?: string
  status?: number
  page?: number
  page_size?: number
}

export interface PointRuleInfo {
  id: number
  code: string
  name: string
  scene: string
  /** rate=按金额比例 fixed=固定积分 */
  earn_mode: string
  fixed_points: number
  points_per_yuan: number
  /** 触发门槛（0=不限） */
  min_amount: number
  /** 单笔积分上限（0=不限） */
  max_points_per_order: number
  /** 有效期天数（0=永久） */
  valid_days: number
  status: number
  sort_order: number
  remark: string
  created_at: string
  updated_at: string
}

export interface PointRuleListResponse {
  items: PointRuleInfo[]
  meta: ListMeta
}

export interface PointRuleSaveRequest {
  code?: string
  name: string
  scene: string
  earn_mode: string
  fixed_points?: number
  points_per_yuan?: number
  min_amount?: number
  max_points_per_order?: number
  valid_days?: number
  status?: number
  sort_order?: number
  remark?: string
}

// ===== 账户 =====

export interface PointAccountListQuery {
  user_id?: number
  user_keyword?: string
  min_points?: number
  page?: number
  page_size?: number
}

export interface PointAccountInfo {
  id: number
  user_id: number
  username: string
  email: string
  balance: number
  frozen: number
  total_earned: number
  total_spent: number
  updated_at: string
}

export interface PointAccountListResponse {
  items: PointAccountInfo[]
  meta: ListMeta
}

export interface PointAdjustRequest {
  user_id: number
  /** 正数=发放，负数=扣减 */
  points: number
  remark: string
}

// ===== 流水 =====

export interface PointTransactionListQuery {
  user_id?: number
  type?: string
  direction?: number
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface PointTransactionInfo {
  id: number
  tx_no: string
  user_id: number
  username: string
  type: string
  /** 1=获得 -1=消耗 */
  direction: number
  points: number
  balance_before: number
  balance_after: number
  biz_type: string
  ref_no: string
  remark: string
  operator_id: number
  expire_at: string
  created_at: string
}

export interface PointTransactionListResponse {
  items: PointTransactionInfo[]
  meta: ListMeta
}

// ===== 概览 =====

export interface PointOverviewResponse {
  total_balance: number
  total_earned: number
  total_spent: number
  account_count: number
  rules: PointRuleInfo[]
  recent: PointTransactionInfo[]
}
