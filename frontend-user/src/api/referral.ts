import request from '@/utils/request'

/** 推广概览（用户端）。 */
export interface ReferralProfile {
  invite_code: string
  enabled: boolean
  balance: number
  frozen: number
  total_income: number
  total_out: number
  pending_withdraw: number
  invitee_count: number
  min_withdraw_amount: number
  first_order_rate: number
  subsequent_rate: number
  renewal_rate: number
}

/** 被邀请人贡献行。 */
export interface InviteeInfo {
  user_id: number
  username: string
  email: string
  invited_at: string
  cashback_total: number
  order_count: number
}

/** 返现台账行。 */
export interface CashbackInfo {
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

/** 提现单。 */
export interface WithdrawalInfo {
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

export interface ListMeta {
  page: number
  page_size: number
  total: number
}

interface Paged<T> {
  items: T[]
  meta: ListMeta
}

export interface WithdrawParams {
  amount: number
  channel?: string
  account?: string
}

export function getReferralProfile() {
  return request.get<any, { data: ReferralProfile }>('/uc/referral/profile')
}

export function getReferralInvitees(params: { page?: number; page_size?: number } = {}) {
  return request.get<any, { data: Paged<InviteeInfo> }>('/uc/referral/invitees', { params })
}

export function getReferralCashbacks(
  params: { type?: string; page?: number; page_size?: number } = {},
) {
  return request.get<any, { data: Paged<CashbackInfo> }>('/uc/referral/cashbacks', { params })
}

export function getReferralWithdrawals(
  params: { status?: string; page?: number; page_size?: number } = {},
) {
  return request.get<any, { data: Paged<WithdrawalInfo> }>('/uc/referral/withdrawals', { params })
}

/** 申请提现：冻结返现余额，待后台审核。 */
export function applyReferralWithdrawal(data: WithdrawParams) {
  return request.post<any, { data: WithdrawalInfo }>('/uc/referral/withdrawals', data)
}

/** 返现转入现金余额（即时到账）。 */
export function transferReferralToWallet(amount: number) {
  return request.post<any, void>('/uc/referral/transfer', { amount })
}
