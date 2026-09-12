import { request } from '@/utils/request'

import type {
  AdjustRequest,
  BillGenerateRequest,
  BillInfo,
  BillListQuery,
  BillListResponse,
  InvoiceInfo,
  InvoiceIssueRequest,
  InvoiceListQuery,
  InvoiceListResponse,
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
      user_keyword: params.user_keyword,
      bill_no: params.bill_no,
      keyword: params.keyword,
      period: params.period,
      status: params.status,
      bill_type: params.bill_type,
      invoice_status: params.invoice_status,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

// generateBill 手动生成账单（doc36 §7-2）：按用户 + 账期归集消费/退款并分类。
export function generateBill(data: BillGenerateRequest): Promise<BillInfo> {
  return request.post<BillInfo>({
    url: '/finance/bills/generate',
    data,
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

// ===== 发票（doc36 §3.3） =====

export function getInvoiceList(params: InvoiceListQuery): Promise<InvoiceListResponse> {
  return request.get<InvoiceListResponse>({
    url: '/finance/invoices',
    params: {
      user_id: params.user_id,
      status: params.status,
      bill_no: params.bill_no,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function issueInvoice(id: number, data: InvoiceIssueRequest): Promise<InvoiceInfo> {
  return request.post<InvoiceInfo>({
    url: `/finance/invoices/${id}/issue`,
    data,
  })
}

export function rejectInvoice(id: number, reason: string): Promise<InvoiceInfo> {
  return request.post<InvoiceInfo>({
    url: `/finance/invoices/${id}/reject`,
    data: { reason },
  })
}

export type { BillInfo, InvoiceInfo }
