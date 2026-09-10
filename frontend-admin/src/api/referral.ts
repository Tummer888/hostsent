import { request } from '@/utils/request'

import type {
  ReferralCashbackListResponse,
  ReferralInviteeListResponse,
  ReferralLedgerQuery,
  ReferralWithdrawAuditRequest,
  ReferralWithdrawInfo,
  ReferralWithdrawListQuery,
  ReferralWithdrawListResponse,
} from '@/types/interface'

// ===== 返现台账 =====

export function getReferralCashbacks(params: ReferralLedgerQuery): Promise<ReferralCashbackListResponse> {
  return request.get<ReferralCashbackListResponse>({
    url: '/referral/cashbacks',
    params: {
      user_id: params.user_id,
      type: params.type,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

// ===== 邀请关系 =====

export function getReferralInvitees(params: {
  inviter_user_id: number
  page?: number
  page_size?: number
}): Promise<ReferralInviteeListResponse> {
  return request.get<ReferralInviteeListResponse>({
    url: '/referral/invitees',
    params: {
      inviter_user_id: params.inviter_user_id,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

// ===== 提现审核 =====

export function getReferralWithdrawals(
  params: ReferralWithdrawListQuery,
): Promise<ReferralWithdrawListResponse> {
  return request.get<ReferralWithdrawListResponse>({
    url: '/referral/withdrawals',
    params: {
      user_id: params.user_id,
      status: params.status,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function approveReferralWithdrawal(
  id: number,
  data: ReferralWithdrawAuditRequest,
): Promise<ReferralWithdrawInfo> {
  return request.post<ReferralWithdrawInfo>({
    url: `/referral/withdrawals/${id}/approve`,
    data,
  })
}

export function rejectReferralWithdrawal(
  id: number,
  data: ReferralWithdrawAuditRequest,
): Promise<ReferralWithdrawInfo> {
  return request.post<ReferralWithdrawInfo>({
    url: `/referral/withdrawals/${id}/reject`,
    data,
  })
}
