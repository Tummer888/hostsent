import { request } from '@/utils/request'

import type {
  SalesAssignRequest,
  SalesCandidateListResponse,
  SalesCommissionListResponse,
  SalesCommissionQuery,
  SalesCommissionSummary,
  SalesCustomerListResponse,
  SalesCustomerQuery,
  SalesMyPerformance,
  SalesRankingQuery,
  SalesRankingResponse,
  SalesRelationListResponse,
  SalesReleaseRequest,
  SalesTargetListResponse,
  SalesTargetUpsertItem,
  SalesUnassignedListResponse,
  SalesWithdrawApplyRequest,
  SalesWithdrawAuditRequest,
  SalesWithdrawPayRequest,
  SalesWithdrawalInfo,
  SalesWithdrawalListResponse,
  SalesWithdrawalQuery,
} from '@/types/interface'

// ===== 客户归属（S4） =====

export function getSalesCustomers(params: SalesCustomerQuery): Promise<SalesCustomerListResponse> {
  return request.get<SalesCustomerListResponse>({
    url: '/sales/customers',
    params: {
      keyword: params.keyword,
      admin_id: params.admin_id,
      department_id: params.department_id,
      status: params.status,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getUnassignedCustomers(params: SalesCustomerQuery): Promise<SalesUnassignedListResponse> {
  return request.get<SalesUnassignedListResponse>({
    url: '/sales/customers/unassigned',
    params: {
      keyword: params.keyword,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function assignCustomer(data: SalesAssignRequest): Promise<{ ok: boolean }> {
  return request.post<{ ok: boolean }>({ url: '/sales/customers/assign', data })
}

export function releaseCustomer(data: SalesReleaseRequest): Promise<{ ok: boolean }> {
  return request.post<{ ok: boolean }>({ url: '/sales/customers/release', data })
}

export function getCustomerRelations(userId: number): Promise<SalesRelationListResponse> {
  return request.get<SalesRelationListResponse>({ url: `/sales/customers/${userId}/relations` })
}

export function getSalesCandidates(params?: { department_id?: number }): Promise<SalesCandidateListResponse> {
  return request.get<SalesCandidateListResponse>({
    url: '/sales/sales-candidates',
    params: { department_id: params?.department_id },
  })
}

// ===== 提成台账（S5） =====

export function getCommissions(params: SalesCommissionQuery): Promise<SalesCommissionListResponse> {
  return request.get<SalesCommissionListResponse>({
    url: '/sales/commissions',
    params: {
      admin_id: params.admin_id,
      department_id: params.department_id,
      type: params.type,
      order_no: params.order_no,
      customer_id: params.customer_id,
      start_at: params.start_at,
      end_at: params.end_at,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getCommissionSummary(params?: { admin_id?: number }): Promise<SalesCommissionSummary> {
  return request.get<SalesCommissionSummary>({
    url: '/sales/commissions/summary',
    params: { admin_id: params?.admin_id },
  })
}

// ===== 提成提现（S6） =====

export function getSalesWithdrawals(params: SalesWithdrawalQuery): Promise<SalesWithdrawalListResponse> {
  return request.get<SalesWithdrawalListResponse>({
    url: '/sales/withdrawals',
    params: {
      admin_id: params.admin_id,
      department_id: params.department_id,
      status: params.status,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function applySalesWithdrawal(data: SalesWithdrawApplyRequest): Promise<SalesWithdrawalInfo> {
  return request.post<SalesWithdrawalInfo>({ url: '/sales/withdrawals', data })
}

export function approveSalesWithdrawal(
  id: number,
  data: SalesWithdrawAuditRequest,
): Promise<SalesWithdrawalInfo> {
  return request.post<SalesWithdrawalInfo>({ url: `/sales/withdrawals/${id}/approve`, data })
}

export function rejectSalesWithdrawal(
  id: number,
  data: SalesWithdrawAuditRequest,
): Promise<SalesWithdrawalInfo> {
  return request.post<SalesWithdrawalInfo>({ url: `/sales/withdrawals/${id}/reject`, data })
}

export function paySalesWithdrawal(
  id: number,
  data: SalesWithdrawPayRequest,
): Promise<{ ok: boolean }> {
  return request.post<{ ok: boolean }>({ url: `/sales/withdrawals/${id}/pay`, data })
}

// ===== 业绩与排行（S5） =====

export function getPerformanceRanking(params: SalesRankingQuery): Promise<SalesRankingResponse> {
  return request.get<SalesRankingResponse>({
    url: '/sales/performance/ranking',
    params: {
      period: params.period,
      department_id: params.department_id,
      sort_by: params.sort_by,
    },
  })
}

export function getSalesTargets(params: { period?: string; department_id?: number }): Promise<SalesTargetListResponse> {
  return request.get<SalesTargetListResponse>({
    url: '/sales/performance/targets',
    params: { period: params.period, department_id: params.department_id },
  })
}

export function upsertSalesTargets(items: SalesTargetUpsertItem[]): Promise<{ ok: boolean }> {
  return request.put<{ ok: boolean }>({ url: '/sales/performance/targets', data: { items } })
}

export function getMyPerformance(params?: { period?: string; admin_id?: number }): Promise<SalesMyPerformance> {
  return request.get<SalesMyPerformance>({
    url: '/sales/performance/me',
    params: { period: params?.period, admin_id: params?.admin_id },
  })
}
