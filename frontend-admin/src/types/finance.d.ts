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
