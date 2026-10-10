// 类型定义（拆分自 interface.d.ts：finance 域）
import type { ListMeta } from './common'

export interface WalletInfo {
  user_id: number
  balance: number
  frozen: number
  total_income: number
  total_expense: number
  version: number
}

export interface TransactionListQuery {
  user_id?: number
  /** 关键词：流水号 / 订单号 / 关联单号 / 用户名（财务对账最常用入口） */
  keyword?: string
  type?: string
  direction?: number
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface TransactionInfo {
  id: number
  tx_no: string
  user_id: number
  username?: string
  type: string
  /** 业务标识（幂等键，排查重复记账用） */
  biz_type?: string
  direction: number // 1=收入 -1=支出
  amount: number
  balance_before: number
  balance_after: number
  order_id: number
  order_no: string
  ref_no: string
  remark: string
  operator_id: number
  created_at: string
}

/** 当前筛选条件下的汇总（全量、非当前页）：冻结/解冻不计入收支口径。 */
export interface TransactionSummary {
  income_total: number
  expense_total: number
  net_total: number
  tx_count: number
  internal_count: number
}

export interface TransactionListResponse {
  items: TransactionInfo[]
  meta: ListMeta
  summary: TransactionSummary
}

export interface AdjustRequest {
  user_id: number
  type?: string
  direction: number
  amount: number
  biz_key: string
  remark?: string
}

export interface RechargeCreateRequest {
  user_id: number
  amount: number
  method: string
  remark?: string
}

export interface RechargeApproveRequest {
  channel_tx?: string
  remark?: string
}

export interface RechargeInfo {
  id: number
  recharge_no: string
  user_id: number
  amount: number
  method: string
  status: string
  channel_tx: string
  paid_at: string
  remark: string
  created_at: string
  updated_at: string
}

export interface RechargeListQuery {
  user_id?: number
  status?: string
  method?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface RechargeListResponse {
  items: RechargeInfo[]
  meta: ListMeta
}

export interface WithdrawListQuery {
  user_id?: number
  status?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface WithdrawInfo {
  id: number
  withdraw_no: string
  user_id: number
  amount: number
  channel: string
  /** 收款渠道中文名（银行卡/支付宝） */
  channel_name?: string
  account: string
  account_name?: string
  bank_name?: string
  status: string
  /** 打款单号（审批通过后生成，登记打款后回填） */
  payout_no?: string
  /** 打款方式：manual 人工 / api 接口自动 */
  payout_mode?: string
  /** 渠道交易号（打款流水号） */
  channel_tx?: string
  audit_by: number
  audit_by_name: string
  audited_at: string
  paid_at: string
  remark: string
  created_at: string
  updated_at: string
}

export interface WithdrawListResponse {
  items: WithdrawInfo[]
  meta: ListMeta
}

export interface WithdrawAuditRequest {
  remark?: string
}

export interface BillListQuery {
  user_id?: number
  /** 用户账号（用户名/邮箱）模糊 */
  user_keyword?: string
  /** 账单号精确 */
  bill_no?: string
  /** 账单号模糊 */
  keyword?: string
  period?: string
  status?: string
  /** 账单来源：period 消费账单 / recharge 充值账单 */
  source_type?: string
  /** 账单分类：consumption / renewal / mixed / recharge */
  bill_type?: string
  /** 发票状态：none / applied / issued */
  invoice_status?: string
  page?: number
  page_size?: number
}

export interface BillInfo {
  id: number
  bill_no: string
  user_id: number
  period: string
  total_amount: number
  refund_amount: number
  status: string
  /** 账单来源：period 按期消费账单 / recharge 单笔充值账单 */
  source_type?: string
  /** 来源单据号：充值账单记充值单号（recharge_no） */
  source_no?: string
  /** 账单分类（doc36 §3.4） */
  bill_type: string
  /** 普通消费（正） */
  consume_amount: number
  /** 续费消费（正） */
  renewal_amount: number
  /** 原路退回本金（真金流出） */
  channel_refund_amount: number
  /** 原路退回渠道扣点（真金流出） */
  refund_fee_amount: number
  /** 该账单承载的充值金额（不参与应结口径）。按期账单恒为 0，充值账单等于该笔充值额 */
  recharge_amount: number
  /** 扣点后计入口径 = 应结 + 原路退款扣点 */
  net_amount: number
  /** 结清时记录的实收金额与支付方式 */
  paid_amount: number
  paid_method: string
  paid_channel_id: number
  paid_at: string
  /** 发票状态 */
  invoice_status: string
  invoice_no: string
  invoiced_at: string
  created_at: string
  updated_at: string
}

export interface BillListResponse {
  items: BillInfo[]
  meta: ListMeta
}

// ===== 发票（doc36 §3.3） =====

export interface InvoiceListQuery {
  user_id?: number
  status?: string
  bill_no?: string
  page?: number
  page_size?: number
}

export interface InvoiceInfo {
  id: number
  request_no: string
  bill_id: number
  bill_no: string
  user_id: number
  username: string
  /** normal=普票 special=专票 */
  invoice_type: string
  title: string
  tax_no: string
  amount: number
  email: string
  /** pending / issued / rejected */
  status: string
  /** manual=人工 / tax_api=税控（预埋） */
  channel: string
  /** 税控回执号（预埋） */
  external_no: string
  /** 发票文件地址（预埋下载） */
  file_url: string
  reject_reason: string
  operator_id: number
  issued_at: string
  created_at: string
  updated_at: string
}

export interface InvoiceListResponse {
  items: InvoiceInfo[]
  meta: ListMeta
}

export interface InvoiceIssueRequest {
  invoice_no: string
  /** 预埋：发票文件地址 */
  file_url?: string
  /** 预埋：manual / tax_api */
  channel?: string
  remark?: string
}

export interface BillGenerateRequest {
  user_id: number
  period: string
}

export interface ReconcileResponse {
  period: string
  income_total: number
  expense_total: number
  tx_count: number
  wallet_balance: number
  diff: number
  status: string
  /** 判定容差（元，来自财务配置 finance.recon_tolerance） */
  tolerance: number
}

// ===== 财务统计（总览 / 报表共用聚合） =====

export interface FinanceStatsQuery {
  start_time?: string
  end_time?: string
  /** day=按日 / month=按月（跨度 > 366 天时后端自动按月） */
  granularity?: 'day' | 'month'
}

export interface FinanceStatsRange {
  start_time: string
  end_time: string
  granularity: 'day' | 'month'
}

export interface FinanceStatsSummary {
  income_total: number
  expense_total: number
  net_total: number
  tx_count: number
  /** 笔均变动金额 */
  avg_amount: number
  /** 冻结/解冻内部划转笔数（不计入收支） */
  internal_count: number
}

export interface FinanceStatsTrendPoint {
  period: string
  income: number
  expense: number
  net: number
  count: number
}

export interface FinanceStatsTypeRow {
  type: string
  income: number
  expense: number
  net: number
  count: number
  /** 冻结/解冻：可用↔冻结内部划转，不计入收支口径 */
  internal: boolean
}

export interface FinanceStatsBillStatusRow {
  status: string
  count: number
  amount: number
}

export interface FinanceStatsBills {
  count: number
  total_amount: number
  refund_amount: number
  net_amount: number
  recharge_count: number
  recharge_amount: number
  by_status: FinanceStatsBillStatusRow[]
  /** 全量未结（不受区间限制） */
  unpaid_count: number
  unpaid_amount: number
}

export interface FinanceStatsPending {
  recharge_pending_count: number
  recharge_pending_amount: number
  withdraw_pending_count: number
  withdraw_pending_amount: number
  withdraw_paying_count: number
  withdraw_paying_amount: number
  invoice_pending_count: number
}

export interface FinanceStatsWallet {
  balance_total: number
  frozen_total: number
  count: number
  low_balance_threshold: number
  low_balance_count: number
  low_balance_amount: number
}

export interface FinanceStatsResponse {
  range: FinanceStatsRange
  summary: FinanceStatsSummary
  trend: FinanceStatsTrendPoint[]
  type_breakdown: FinanceStatsTypeRow[]
  bills: FinanceStatsBills
  pending: FinanceStatsPending
  wallet: FinanceStatsWallet
  /** 口径说明（页脚直接展示） */
  caliber: string
}

// ===== 财务参数（/finance/settings 白名单读写） =====

export interface FinanceSettingItem {
  key: string
  label: string
  description: string
  value_type: 'bool' | 'number'
  value: string
  default_value: string
  /** 该参数在哪里生效（写清消费点） */
  usage: string
  min: number
  max: number
}

export interface FinanceSettingGroup {
  /** 落库的 config_group（finance / referral） */
  key: string
  label: string
  hint: string
  items: FinanceSettingItem[]
}

export interface FinanceSettingsResponse {
  groups: FinanceSettingGroup[]
}

// ===== 成本管理（doc111） =====

/** 成本项分类。只影响展示分组，不影响合计口径。 */
export type CostCategory = 'self_hosted' | 'upstream_ops' | 'labor' | 'infra' | 'other'

/** 计费周期：monthly=按自然月计入；once=一次性，计入 occurred_on 所在自然月。 */
export type CostCycle = 'monthly' | 'once'

export interface CostItemInfo {
  id: number
  name: string
  category: string
  /** 分类中文名（后端一并给出，避免前后端两套文案漂移） */
  category_label: string
  amount: number
  cycle: string
  /** 一次性成本的发生日期（cycle=once 时非空） */
  occurred_on: string
  effective_from: string
  /** 空=长期有效（含端点） */
  effective_to: string
  /** 成本对象（母机名/员工/线路） */
  subject: string
  remark: string
  status: string
  operator_id: number
  created_at: string
  updated_at: string
  /** 该成本项对「当月」的计入金额（一次性项只在发生月计入） */
  monthly_amount: number
}

export interface CostItemListQuery {
  keyword?: string
  category?: string
  status?: string
  page?: number
  page_size?: number
}

export interface CostItemListResponse {
  items: CostItemInfo[]
  meta: ListMeta
  /** 筛选口径下的「按月计入合计」（不受分页影响） */
  monthly_total: number
}

export interface CostItemRequest {
  name: string
  category: string
  amount: number
  cycle?: string
  occurred_on?: string
  effective_from?: string
  effective_to?: string
  subject?: string
  remark?: string
  status?: string
}

/** 单渠道月度台账行：期初 + 期间充值 − 期末 = 期间消耗。 */
export interface UpstreamLedgerRow {
  provider_id: number
  provider_name: string
  provider_type: string
  /** 期初余额 = 该月首日之前最近一条快照（上月末口径）；无历史快照时为 null */
  opening_balance: number | null
  opening_date: string
  /** 期间充值合计（正=充值，负=退还） */
  topup_total: number
  closing_balance: number | null
  closing_date: string
  /** 期间消耗；缺期初或期末快照时为 null（页面提示补录） */
  consumption: number | null
  /** 期末快照不是本月数据时为 true（消耗是「截至该日」的估算值） */
  estimated: boolean
  /** 本月完全无快照（期初/期末取自历史数据，需尽快录入） */
  missing_snapshot: boolean
  /** 最新一条快照余额（不限月份） */
  latest_balance: number | null
  latest_date: string
  currency: string
  /** 本月流水口径消耗（净额）；null=该渠道账本不可作准 */
  ledger_consumption: number | null
  /** 本月账本条目数（0=账本已同步但本月无流水） */
  ledger_entries: number
  /** 本月取数来源：ledger（流水）/ snapshot（快照推算）/ none */
  cost_source: string
  /** 两种口径差额 = 流水 − 快照（都有值时给出；非 0 需人工核查） */
  ledger_diff: number | null
}

export interface UpstreamLedgerResponse {
  month: string
  rows: UpstreamLedgerRow[]
  /** 全部渠道期间消耗合计（流水优先、快照兜底，见每行 cost_source） */
  total_consumption: number
  /** 支持自动抓取余额的渠道 ID（适配器实现了账户余额读取） */
  snapshot_supported_providers: number[]
  /** 支持同步上游账本的渠道 ID（适配器实现了账本读取：消费/充值流水） */
  ledger_supported_providers: number[]
  /** 最近一次账本同步时间（RFC3339，空=未同步过） */
  ledger_synced_at: string
  /** 上游余额水位告警（余额 < 未来 30 天到期金额 / < 配置阈值） */
  alerts: BalanceAlert[]
}

/** 上游余额水位告警。level: ok | warning | critical | unknown（取数失败）。 */
export interface BalanceAlert {
  provider_id: number
  provider_name: string
  level: string
  balance: number
  currency: string
  /** 未来 30 天内到期（含已到期未付）的续费金额合计 */
  due_within_30d: number
  due_count: number
  /** 配置的低水位阈值（0=未配置） */
  threshold: number
  message: string
}

/** 账本同步请求：provider_id=0 表示全部支持的渠道；full=true 全量回填。 */
export interface CostLedgerSyncRequest {
  provider_id?: number
  full?: boolean
}

export interface CostLedgerSyncProviderResult {
  provider_id: number
  provider_name: string
  /** 本次写入的消费/充值条数（含更新的历史行） */
  consumption: number
  topup: number
  /** 上游侧总数（判断是否追平） */
  consumption_total: number
  topup_total: number
  /** 失败原因（空=成功） */
  failed: string
}

export interface CostLedgerSyncResponse {
  results: CostLedgerSyncProviderResult[]
}

/** 账本流水条目（消费/充值）。 */
export interface CostLedgerEntryInfo {
  id: number
  provider_id: number
  provider_name: string
  /** consume=消费 / topup=充值 */
  kind: string
  occurred_at: string
  amount: number
  refund_amount: number
  /** 净额 = amount − refund_amount */
  net_amount: number
  /** 上游类型原文（订购产品 / 续费 / 用户充值 / 人工入账） */
  category: string
  /** 上游账单号 / 交易号 */
  ref_no: string
  description: string
  currency: string
}

export interface CostLedgerEntryListQuery {
  provider_id?: number
  kind?: string
  month?: string
  page?: number
  page_size?: number
}

export interface CostLedgerEntryListResponse {
  items: CostLedgerEntryInfo[]
  meta: ListMeta
  /** 筛选口径（渠道+月份）的消费净额合计 */
  consumption_total: number
  topup_total: number
}

export interface CostSnapshotRequest {
  provider_id: number
  /** YYYY-MM-DD，空=今天；同一渠道同日重复录入会覆盖 */
  snapshot_date?: string
  balance?: number
  remark?: string
}

export interface CostTopupRequest {
  provider_id: number
  /** YYYY-MM-DD，空=今天 */
  occurred_on?: string
  /** 正=充值，负=渠道退款/冲正 */
  amount?: number
  remark?: string
}

export interface CostSnapshotInfo {
  id: number
  provider_id: number
  provider_name: string
  snapshot_date: string
  balance: number
  currency: string
  /** manual=人工录入，auto=抓取 */
  source: string
  remark: string
  created_at: string
}

export interface CostTopupInfo {
  id: number
  provider_id: number
  provider_name: string
  occurred_on: string
  amount: number
  remark: string
  operator_id: number
  created_at: string
}

export interface CostBalanceRecordQuery {
  /** 0/不传=全部渠道 */
  provider_id?: number
  /** YYYY-MM，空=不限月份 */
  month?: string
  page?: number
  page_size?: number
}

export interface CostSnapshotListResponse {
  items: CostSnapshotInfo[]
  meta: ListMeta
}

export interface CostTopupListResponse {
  items: CostTopupInfo[]
  meta: ListMeta
}

/** 成本构成行：auto=true 为系统按数据自动计算，false 来自成本项配置。 */
export interface CostLine {
  key: string
  label: string
  amount: number
  auto: boolean
  category: string
  /** 数据来源说明（页面直接展示） */
  usage: string
}

export interface CostTrendPoint {
  month: string
  service_revenue: number
  cost_total: number
  upstream_cost: number
  fixed_cost: number
  other_cost: number
  profit: number
  /** 百分比，保留 2 位 */
  profit_rate: number
}

export interface CostOverviewResponse {
  /** 统计月 YYYY-MM */
  month: string
  /** 数据截至日期（当月为今天，历史月为月末） */
  as_of: string
  /** 是否当月（月中视图：固定成本另给按天摊分口径） */
  current: boolean
  /** 收入（营业收入口径）：期间消费 − 期间退款 */
  service_revenue: number
  consume_total: number
  refund_total: number
  /** 资金口径收入（含充值等资金搬运），仅作参考 */
  fund_income: number
  cost_total: number
  /** 成本按「固定成本摊到截至日」重算后的合计（月中参考） */
  cost_to_date: number
  upstream_cost: number
  /** 成本项配置合计（整月口径） */
  fixed_cost: number
  /** 成本项按天摊到截至日（当月才有意义） */
  fixed_cost_to_date: number
  /** 用户佣金入账（资金流水 type=commission 收入合计） */
  commission_cost: number
  /** 推广返现计提 */
  referral_cost: number
  profit: number
  /** 利润率（百分比，2 位小数；无收入时为 0） */
  profit_rate: number
  profit_to_date: number
  profit_rate_to_date: number
  cost_lines: CostLine[]
  upstream_rows: UpstreamLedgerRow[]
  trend: CostTrendPoint[]
  /** 口径说明（页面页脚展示，与后端同文） */
  caliber: string
  /** 未配置成本项时的提示（避免「成本为 0」被误读） */
  unconfigured_hint: string
  /** 流水口径：上游账本消费净额合计（主口径） */
  upstream_cost_ledger: number
  /** 快照推算口径：期初 + 充值 − 期末（核对口径） */
  upstream_cost_snapshot: number
  /** 上游成本取数来源：ledger / snapshot / mixed / none */
  upstream_cost_source: string
  /** 差额 = 流水 − 快照（只对两种口径都有的渠道累计；非 0 需核查） */
  upstream_ledger_diff: number
  /** 最近一次账本同步时间（RFC3339，空=未同步过） */
  ledger_synced_at: string
  /** 上游余额水位告警 */
  balance_alerts: BalanceAlert[]
}
