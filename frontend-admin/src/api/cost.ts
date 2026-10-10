import { request } from '@/utils/request'

import type {
  BalanceAlert,
  CostBalanceSnapshotInfo,
  CostLedgerEntryListQuery,
  CostLedgerEntryListResponse,
  CostLedgerSyncRequest,
  CostLedgerSyncResponse,
  CostItemInfo,
  CostItemListQuery,
  CostItemListResponse,
  CostItemRequest,
  CostOverviewResponse,
  UpstreamLedgerResponse,
} from '@/types/interface'

// ===== 成本管理（doc111）=====
//
// 口径：收入=服务收入（消费 − 退款）；成本=上游账本消费流水 + 成本项配置
// + 用户佣金入账 + 推广返现计提；利润=收入−成本。周期按自然月。
// 上游成本与余额全部自动取数（账本同步 + 每日抓取快照），没有手工录入入口。
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

/** 上游余额台账：各渠道最新余额 + 期间消耗/充值（全部自动取数）。 */
export function getCostLedger(params: { month?: string } = {}): Promise<UpstreamLedgerResponse> {
  return request.get<UpstreamLedgerResponse>({
    url: '/cost/balances',
    params: { month: params.month },
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
 * 抓取渠道余额并落当日快照（余额展示与水位告警的数据源，每日也会自动抓一次）。
 * 渠道适配器未实现账户余额读取时后端返回 30008；页面只对「支持抓取」的渠道开放该按钮。
 */
export function fetchCostBalance(providerId: number): Promise<CostBalanceSnapshotInfo> {
  return request.post<CostBalanceSnapshotInfo>({
    url: '/cost/balances/fetch',
    data: { provider_id: providerId },
  })
}
