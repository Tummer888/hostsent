import { request } from '@/utils/request'

import type {
  AdjustRequest,
  BillInfo,
  BillListQuery,
  BillListResponse,
  RechargeApproveRequest,
  RechargeCreateRequest,
  RechargeInfo,
  RechargeListQuery,
  RechargeListResponse,
  ReconcileResponse,
  TransactionInfo,
  TransactionListQuery,
  TransactionListResponse,
  WalletInfo,
  WithdrawAuditRequest,
  WithdrawInfo,
  WithdrawListQuery,
  WithdrawListResponse,
} from '@/types/interface'

// ===== 钱包 / 流水 =====

export function getWallet(userId: number): Promise<WalletInfo> {
  return request.get<WalletInfo>({
    url: `/finance/wallets/${userId}`,
  })
}

export function getTransactionList(params: TransactionListQuery): Promise<TransactionListResponse> {
  return request.get<TransactionListResponse>({
    url: '/finance/transactions',
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

export function adjustBalance(data: AdjustRequest): Promise<TransactionInfo> {
  return request.post<TransactionInfo>({
    url: '/finance/transactions/adjust',
    data,
  })
}

// ===== 充值 =====

export function createRecharge(data: RechargeCreateRequest): Promise<RechargeInfo> {
  return request.post<RechargeInfo>({
    url: '/finance/recharges',
    data,
  })
}

export function getRechargeList(params: RechargeListQuery): Promise<RechargeListResponse> {
  return request.get<RechargeListResponse>({
    url: '/finance/recharges',
    params: {
      user_id: params.user_id,
      status: params.status,
      method: params.method,
      start_time: params.start_time,
      end_time: params.end_time,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function approveRecharge(id: number, data: RechargeApproveRequest): Promise<RechargeInfo> {
  return request.post<RechargeInfo>({
    url: `/finance/recharges/${id}/approve`,
    data,
  })
}

// ===== 提现 =====

export function getWithdrawList(params: WithdrawListQuery): Promise<WithdrawListResponse> {
  return request.get<WithdrawListResponse>({
    url: '/finance/withdrawals',
    params: {
      user_id: params.user_id,
      status: params.status,
      start_time: params.start_time,
      end_time: params.end_time,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function approveWithdraw(id: number, data: WithdrawAuditRequest): Promise<WithdrawInfo> {
  return request.post<WithdrawInfo>({
    url: `/finance/withdrawals/${id}/approve`,
    data,
  })
}

export function rejectWithdraw(id: number, data: WithdrawAuditRequest): Promise<WithdrawInfo> {
  return request.post<WithdrawInfo>({
    url: `/finance/withdrawals/${id}/reject`,
    data,
  })
}

// ===== 账单 / 对账 =====

export function getBillList(params: BillListQuery): Promise<BillListResponse> {
  return request.get<BillListResponse>({
    url: '/finance/bills',
    params: {
      user_id: params.user_id,
      period: params.period,
      status: params.status,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function closeBill(id: number): Promise<string> {
  return request.post<string>({
    url: `/finance/bills/${id}/close`,
  })
}

export function reconcile(period?: string): Promise<ReconcileResponse> {
  return request.post<ReconcileResponse>({
    url: '/finance/bills/recon',
    data: { period },
  })
}

export type { BillInfo }
