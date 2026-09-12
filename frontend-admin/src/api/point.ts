import { request } from '@/utils/request'

import type {
  PointAccountInfo,
  PointAccountListQuery,
  PointAccountListResponse,
  PointAdjustRequest,
  PointOverviewResponse,
  PointRuleInfo,
  PointRuleListQuery,
  PointRuleListResponse,
  PointRuleSaveRequest,
  PointTransactionListQuery,
  PointTransactionListResponse,
  PointTransactionInfo,
} from '@/types/interface'

// ===== 概览（积分中心首页） =====

export function getPointOverview(): Promise<PointOverviewResponse> {
  return request.get<PointOverviewResponse>({
    url: '/points/overview',
  })
}

// ===== 积分规则 =====

export function getPointRules(params: PointRuleListQuery): Promise<PointRuleListResponse> {
  return request.get<PointRuleListResponse>({
    url: '/points/rules',
    params: {
      scene: params.scene,
      status: params.status,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function createPointRule(data: PointRuleSaveRequest): Promise<PointRuleInfo> {
  return request.post<PointRuleInfo>({
    url: '/points/rules',
    data,
  })
}

export function updatePointRule(id: number, data: PointRuleSaveRequest): Promise<PointRuleInfo> {
  return request.put<PointRuleInfo>({
    url: `/points/rules/${id}`,
    data,
  })
}

// ===== 积分账户 =====

export function getPointAccounts(params: PointAccountListQuery): Promise<PointAccountListResponse> {
  return request.get<PointAccountListResponse>({
    url: '/points/accounts',
    params: {
      user_id: params.user_id,
      user_keyword: params.user_keyword,
      min_points: params.min_points,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getPointAccount(userId: number): Promise<PointAccountInfo> {
  return request.get<PointAccountInfo>({
    url: `/points/accounts/${userId}`,
  })
}

export function adjustPoints(data: PointAdjustRequest): Promise<PointTransactionInfo> {
  return request.post<PointTransactionInfo>({
    url: '/points/accounts/adjust',
    data,
  })
}

// ===== 积分流水 =====

export function getPointTransactions(params: PointTransactionListQuery): Promise<PointTransactionListResponse> {
  return request.get<PointTransactionListResponse>({
    url: '/points/transactions',
    params: {
      user_id: params.user_id,
      type: params.type,
      direction: params.direction,
      start_time: params.start_time,
      end_time: params.end_time,
      page: params.page,
      page_size: params.page_size,
    },
  })
}
