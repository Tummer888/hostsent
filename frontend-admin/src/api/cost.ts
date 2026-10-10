import { request } from '@/utils/request'

import type {
  BalanceAlert,
  CostBalanceRecordQuery,
  CostLedgerEntryListQuery,
  CostLedgerEntryListResponse,
  CostLedgerSyncRequest,
  CostLedgerSyncResponse,
  CostItemInfo,
  CostItemListQuery,
  CostItemListResponse,
  CostItemRequest,
  CostOverviewResponse,
  CostSnapshotInfo,
  CostSnapshotListResponse,
  CostSnapshotRequest,
  CostTopupInfo,
  CostTopupListResponse,
  CostTopupRequest,
  UpstreamLedgerResponse,
} from '@/types/interface'

// ===== 成本管理（doc111）=====
//
// 口径：收入=服务收入（消费 − 退款）；成本=上游余额消耗（期初+充值−期末）+ 成本项配置
// + 用户佣金入账 + 推广返现计提；利润=收入−成本。周期按自然月。
// 页面页脚展示后端返回的 caliber 原文，前端不另写一份口径文案。

/** 成本总览：月度收入/成本/利润与利润率、成本构成、上游台账、近 12 月趋势。 */
export function getCostOverview(params: { month?: string } = {}): Promise<CostOverviewResponse> {
  return request.get<CostOverviewResponse>({
    url: '/cost/overview',
    params: { month: params.month },
  })
}

/** 成本项列表（含按月计入合计，不受分页影响）。 */
export function getCostItems(params: CostItemListQuery): Promise<CostItemListResponse> {
  return request.get<CostItemListResponse>({
    url: '/cost/items',
    params: {
      keyword: params.keyword,
      category: params.category,
      status: params.status,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function createCostItem(data: CostItemRequest): Promise<CostItemInfo> {
  return request.post<CostItemInfo>({ url: '/cost/items', data })
}

export function updateCostItem(id: number, data: CostItemRequest): Promise<CostItemInfo> {
  return request.put<CostItemInfo>({ url: `/cost/items/${id}`, data })
}

export function deleteCostItem(id: number): Promise<void> {
  return request.delete<void>({ url: `/cost/items/${id}` })
}

/** 上游余额台账：各渠道期初/充值/消耗/期末 + 支持抓取的渠道清单。 */
export function getCostLedger(params: { month?: string } = {}): Promise<UpstreamLedgerResponse> {
  return request.get<UpstreamLedgerResponse>({
    url: '/cost/balances',
    params: { month: params.month },
  })
}

/** 录入余额快照（同一渠道同日重复录入会覆盖）。 */
export function saveCostSnapshot(data: CostSnapshotRequest): Promise<CostSnapshotInfo> {
  return request.post<CostSnapshotInfo>({ url: '/cost/balances/snapshot', data })
}

export function deleteCostSnapshot(id: number): Promise<void> {
  return request.delete<void>({ url: `/cost/balances/snapshots/${id}` })
}

/** 录入上游充值/退还记录（正=充值，负=渠道退款冲正）。 */
export function saveCostTopup(data: CostTopupRequest): Promise<CostTopupInfo> {
  return request.post<CostTopupInfo>({ url: '/cost/balances/topup', data })
}

export function deleteCostTopup(id: number): Promise<void> {
  return request.delete<void>({ url: `/cost/balances/topups/${id}` })
}

/** 余额快照明细（分页，台账核对与纠错用）。 */
export function getCostSnapshots(params: CostBalanceRecordQuery): Promise<CostSnapshotListResponse> {
  return request.get<CostSnapshotListResponse>({
    url: '/cost/balances/snapshots',
    params: {
      provider_id: params.provider_id,
      month: params.month,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

/** 上游充值明细（分页）。 */
export function getCostTopups(params: CostBalanceRecordQuery): Promise<CostTopupListResponse> {
  return request.get<CostTopupListResponse>({
    url: '/cost/balances/topups',
    params: {
      provider_id: params.provider_id,
      month: params.month,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

/**
 * 同步上游账本（消费/充值流水）。
 * provider_id=0 同步全部支持账本的渠道；full=true 全量回填（首次接入/数据修复）。
 * 增量模式按上游自增 ID 断点续传，日常每天由调度器自动执行一次。
 */
export function syncCostLedger(data: CostLedgerSyncRequest = {}): Promise<CostLedgerSyncResponse> {
  return request.post<CostLedgerSyncResponse>({ url: '/cost/balances/sync-ledger', data })
}

/** 上游账本流水分页（kind: consume 消费 / topup 充值）。 */
export function getCostLedgerEntries(params: CostLedgerEntryListQuery): Promise<CostLedgerEntryListResponse> {
  return request.get<CostLedgerEntryListResponse>({
    url: '/cost/balances/entries',
    params: {
      provider_id: params.provider_id,
      kind: params.kind,
      month: params.month,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

/** 上游余额水位告警：余额 < 未来 30 天到期金额（critical）或 < 配置阈值（warning）。 */
export function getCostBalanceAlerts(): Promise<BalanceAlert[]> {
  return request.get<BalanceAlert[]>({ url: '/cost/balances/alerts' })
}

/**
 * 抓取渠道余额并落当日快照。
 * 渠道适配器未实现账户余额读取时后端返回 30008 与可读提示（请手工录入快照），
 * 页面只对「支持抓取」的渠道开放该按钮，此接口用于该白名单内的渠道。
 */
export function fetchCostBalance(providerId: number): Promise<CostSnapshotInfo> {
  return request.post<CostSnapshotInfo>({
    url: '/cost/balances/fetch',
    data: { provider_id: providerId },
  })
}
