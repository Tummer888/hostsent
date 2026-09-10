// 类型定义（推广邀请返现域）
import type { ListMeta } from './common'

export interface ReferralLedgerQuery {
  user_id?: number
  type?: string
  page?: number
  page_size?: number
}

export interface ReferralCashbackInfo {
  id: number
  tx_no: string
  type: string
  direction: number
  amount: number
  balance_after: number
  order_id: number
  order_no: string
  invitee_user_id: number
  invitee_username: string
  inviter_user_id: number
  inviter_username: string
  ref_no: string
  remark: string
  created_at: string
}

export interface ReferralCashbackListResponse {
  items: ReferralCashbackInfo[]
  meta: ListMeta
}

export interface ReferralInviteeInfo {
  user_id: number
  username: string
  email: string
  invited_at: string
  cashback_total: number
  order_count: number
}

export interface ReferralInviteeListResponse {
  items: ReferralInviteeInfo[]
  meta: ListMeta
}

export interface ReferralWithdrawListQuery {
  user_id?: number
  status?: string
  page?: number
  page_size?: number
}

export interface ReferralWithdrawInfo {
  id: number
  withdraw_no: string
  user_id: number
  username: string
  amount: number
  channel: string
  account: string
  status: string
  audit_by: number
  audit_by_name: string
  audited_at: string
  remark: string
  created_at: string
}

export interface ReferralWithdrawListResponse {
  items: ReferralWithdrawInfo[]
  meta: ListMeta
}

export interface ReferralWithdrawAuditRequest {
  remark?: string
}
