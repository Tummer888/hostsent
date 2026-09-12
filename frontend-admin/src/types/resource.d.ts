// 类型定义（拆分自 interface.d.ts：resource 域）
import type { ListMeta } from './common'

export interface ProviderListQuery {
  page?: number
  page_size?: number
  keyword?: string
  provider_type?: string
  kind?: string
  status?: number
}

export interface ProviderCreateRequest {
  name: string
  provider_type: string
  api_endpoint: string
  api_key?: string
  api_secret?: string
  region?: string
  status?: number
  sync_enabled?: boolean
  sync_interval?: number
  contact_way?: string
  des?: string
  upstream_type?: string
  port?: string
  secure?: boolean
  disabled?: boolean
  user_prefix?: string
  account_type?: string
  // ---- 契约层（P2/T2.3~T2.4）----
  /** 动态凭证：字段由渠道 capabilities.credential_schema 声明，secret 字段加密落库 */
  credentials?: Record<string, string>
  timeout_seconds?: number
  retry_max?: number
  rate_limit_qps?: number
  /** 上游/平台侧运维控制台地址，后台一键跳转 */
  ops_console_url?: string
}

export interface ProviderUpdateRequest {
  name?: string
  api_endpoint?: string
  api_key?: string
  api_secret?: string
  region?: string
  status?: number
  sync_enabled?: boolean
  sync_interval?: number
  contact_way?: string
  des?: string
  upstream_type?: string
  port?: string
  secure?: boolean
  disabled?: boolean
  user_prefix?: string
  account_type?: string
  /** 动态凭证；值为空或等于脱敏回显时不覆盖已存值 */
  credentials?: Record<string, string>
  timeout_seconds?: number
  retry_max?: number
  rate_limit_qps?: number
  /** 上游/平台侧运维控制台地址，后台一键跳转 */
  ops_console_url?: string
}

export interface ProviderInfo {
  id: number
  name: string
  provider_type: string
  api_endpoint: string
  api_key: string
  api_secret: string
  region: string
  status: number
  sync_enabled: boolean
  sync_interval: number
  contact_way: string
  des: string
  upstream_type: string
  zjmf_finance_api_id: number
  port: string
  secure: boolean
  disabled: boolean
  user_prefix: string
  account_type: string
  last_sync_at: string | null
  kind: string
  sync_paused: boolean
  consecutive_failures: number
  last_sync_error: string
  last_success_at: string | null
  total_cpu: number
  total_memory: number
  total_disk: number
  used_cpu: number
  used_memory: number
  used_disk: number
  // ---- 契约层（P2/T2.3~T2.4）----
  /** 脱敏凭证快照（secret 字段仅首尾各 2 位），供动态表单回显 */
  credentials: Record<string, string>
  credential_keys: string[]
  /** 非空表示凭证解密失败（L8：需重新录入，不回退明文） */
  credential_error: string
  timeout_seconds: number
  retry_max: number
  rate_limit_qps: number
  /** 上游/平台侧运维控制台地址，后台一键跳转 */
  ops_console_url: string
  /** 渠道能力描述符，供后台能力矩阵展示 */
  capabilities: CapabilityDescriptor
  created_at: string
  updated_at: string
}

export interface ProviderListResponse {
  items: ProviderInfo[]
  meta: ListMeta
}

/** 凭证/端点字段控件类型 */
export type ProviderFieldType = 'string' | 'password' | 'number' | 'select' | 'bool' | 'textarea'

export interface ProviderFieldOption {
  label: string
  value: string
}

/** 凭证/端点字段描述：驱动后台动态表单 + 字段级加密 */
export interface ProviderField {
  key: string
  label: string
  type: ProviderFieldType
  required: boolean
  secret: boolean
  placeholder?: string
  help?: string
  default?: string
  options?: ProviderFieldOption[]
}

export interface ProviderRateLimitSpec {
  qps: number
  burst: number
}

/** 渠道路径能力描述符（契约②，与后端 upstream.CapabilityDescriptor 对应） */
export interface CapabilityDescriptor {
  kind: string
  sync_scopes: string[] | null
  spec_atoms: string[] | null
  billing_cycles: string[] | null
  operations: string[] | null
  renew_mode: string
  destroy_mode: string
  credential_schema: ProviderField[] | null
  endpoint_schema: ProviderField[] | null
  signer_type: string
  rate_limit: ProviderRateLimitSpec
  supports_paging: boolean
  field_dictionary?: Record<string, unknown> | null
  /** 适配器是否已注册（运行时计算，非落库） */
  implemented: boolean
}

export interface ProviderTypeItem {
  type: string
  name: string
  kind: string
  /** 是否已注册适配器（false 时可展示但连接测试会明确失败） */
  implemented: boolean
  adapter_version: string
  doc_url: string
  icon: string
  capabilities: CapabilityDescriptor
}

export interface TestConnectionResult {
  success: boolean
  message: string
}

export interface PoolListQuery {
  provider_id?: number
  /** 按池名 / 上游 ID 模糊搜索 */
  keyword?: string
  /** 按地域筛选（位置检测） */
  region?: string
  status?: number
  /** 仅看容量告警池：warn=用量≥60%，danger=用量≥80% */
  alert?: string
  page?: number
  page_size?: number
}

export interface PoolInfo {
  id: number
  provider_id: number
  /** 所属渠道名（列表展示用） */
  provider_name: string
  /** 所属渠道的运维平台地址（为空则不展示一键跳转） */
  provider_ops_url: string
  upstream_id: string
  name: string
  pool_type: string
  total_cpu: number
  total_memory: number
  total_disk: number
  used_cpu: number
  used_memory: number
  used_disk: number
  status: number
  last_sync_at: string | null
  /** 位置：池所属地域 / 可用区 */
  region: string
  zone: string
  /** 探针流（预留）：probe_status 为空表示探针未接入 */
  probe_status: string
  probe_at: string | null
  probe_message: string
  /** 用量百分比（后端算好，0~100） */
  cpu_usage_percent: number
  memory_usage_percent: number
  disk_usage_percent: number
}

/** 资源池容量汇总（按当前筛选条件的全量统计，不受分页影响） */
export interface PoolCapacitySummary {
  total_pools: number
  online_pools: number
  total_cpu: number
  used_cpu: number
  total_memory: number
  used_memory: number
  total_disk: number
  used_disk: number
  /** 用量≥60% 的池数（含 danger） */
  warning_pools: number
  /** 用量≥80% 的池数 */
  danger_pools: number
  /** 未标注位置的池数 */
  unlocated_pools: number
}

export interface PoolListResponse {
  items: PoolInfo[]
  meta: ListMeta
  summary: PoolCapacitySummary
  /** 当前筛选结果中出现的地域 */
  regions: string[]
}

export interface ProductListQuery {
  keyword?: string
  provider_id?: number
  status?: number
  page?: number
  page_size?: number
}

export interface ProductPriceRequest {
  cost_price: number
  sale_price: number
}

export interface ProductInfo {
  id: number
  provider_id: number
  upstream_id: string
  name: string
  group_id: number
  group_name: string
  cpu: number
  memory: number
  disk: number
  disk_type: string
  bandwidth: number
  os: string
  region: string
  zone: string
  specs: string
  raw_specs: string
  cost_price: number
  sale_price: number
  status: number
  created_at: string
  updated_at: string
}

export interface ProductListResponse {
  items: ProductInfo[]
  meta: ListMeta
}

export interface SyncResult {
  task_id: number
  status: string
  message: string
}

export interface SyncTaskListQuery {
  provider_id?: number
  task_type?: string
  status?: string
  page?: number
  page_size?: number
}

export interface SyncLogListQuery {
  provider_id?: number
  task_id?: number
  status?: string
  page?: number
  page_size?: number
}

export interface InstanceListQuery {
  provider_id?: number
  user_id?: number
  status?: string
  keyword?: string
  page?: number
  page_size?: number
}

export interface CreateSyncTaskRequest {
  provider_id: number
  task_type: string
}

export interface SyncTaskInfo {
  id: number
  provider_id: number
  task_type: string
  status: string
  total_count: number
  success_count: number
  error_message: string
  started_at: string | null
  completed_at: string | null
  created_at: string
}

export interface SyncTaskListResponse {
  items: SyncTaskInfo[]
  meta: ListMeta
}

export interface SyncLogInfo {
  id: number
  task_id: number
  provider_id: number
  sync_type: string
  status: string
  total_count: number
  success_count: number
  error_message: string
  details: string
  created_at: string
}

export interface SyncLogListResponse {
  items: SyncLogInfo[]
  meta: ListMeta
}

export interface InstanceInfo {
  id: number
  instance_id: string
  provider_id: number
  user_id: number
  sell_product_id: number
  upstream_product_id: number
  source_mode: string
  name: string
  cpu: number
  memory: number
  disk: number
  disk_type: string
  bandwidth: number
  os: string
  region: string
  zone: string
  status: string
  private_ip: string
  public_ip: string
  billing_mode: string
  expire_at: string | null
  created_at: string
}

export interface InstanceListResponse {
  items: InstanceInfo[]
  meta: ListMeta
}

// ============================================================================
// P3 同步框架（调度 / 调价事件 / 差异记录）
// ============================================================================

/** 调度配置列表查询 */
export interface SyncScheduleListQuery {
  provider_id?: number
  enabled?: boolean
  page?: number
  page_size?: number
}

/** 调度配置更新请求：节奏 / 启停 / 优先级 / 时间窗 */
export interface SyncScheduleUpdateRequest {
  interval_seconds?: number
  full_sync_interval_seconds?: number
  enabled?: boolean
  priority?: number
  /** 允许执行的小时区间 0-23 */
  window_start?: number | null
  window_end?: number | null
  /** 传 true 清空时间窗（与「不修改」区分） */
  window_clear?: boolean
  /** 传 true 使调度立即到期 */
  reset_next_run?: boolean
}

/** 渠道 × scope 调度配置 */
export interface SyncScheduleInfo {
  id: number
  provider_id: number
  provider_name: string
  scope: string
  scope_name: string
  interval_seconds: number
  full_sync_interval_seconds: number
  enabled: boolean
  priority: number
  window_start: number | null
  window_end: number | null
  last_run_at: string | null
  next_run_at: string | null
  last_status: string
  last_error: string
}

export interface SyncScheduleListResponse {
  items: SyncScheduleInfo[]
  meta: ListMeta
}

/** 已注册 scope 元数据 */
export interface SyncScopeMeta {
  scope: string
  name: string
  default_interval_seconds: number
}

/** 调价事件列表查询 */
export interface PriceChangeListQuery {
  provider_id?: number
  status?: string
  page?: number
  page_size?: number
}

/** 上游调价事件 */
export interface PriceChangeInfo {
  id: number
  provider_id: number
  scope: string
  resource_product_id: number
  upstream_id: string
  product_id: number
  field: string
  old_value: number | null
  new_value: number | null
  change_ratio: number | null
  threshold: number | null
  status: string
  applied: boolean
  remark: string
  created_at: string
  handled_at: string | null
}

export interface PriceChangeListResponse {
  items: PriceChangeInfo[]
  meta: ListMeta
}

/** 批量确认 / 驳回上游调价 */
export interface PriceChangeHandleRequest {
  ids: number[]
  action: 'confirm' | 'reject'
  remark?: string
}

/** 差异记录列表查询 */
export interface SyncDiffListQuery {
  provider_id?: number
  task_id?: number
  scope?: string
  action?: string
  page?: number
  page_size?: number
}

/** 同步差异记录（真实数据源：sync_diffs） */
export interface SyncDiffInfo {
  id: number
  task_id: number
  provider_id: number
  scope: string
  action: string
  local_id: number
  external_id: string
  field: string
  old_value: string
  new_value: string
  disposition: string
  remark: string
  created_at: string
}

export interface SyncDiffListResponse {
  items: SyncDiffInfo[]
  meta: ListMeta
}

/** 按渠道汇总的差异统计（对账页数据源） */
export interface SyncDiffSummaryProvider {
  provider_id: number
  total: number
  by_action: Record<string, number>
}

export interface SyncDiffSummaryResponse {
  items: SyncDiffSummaryProvider[]
}

// ===== 任务队列（运维 · 平台动作是否到达上游） =====

export interface TaskQueueListQuery {
  category?: string
  status?: string
  upstream_state?: string
  provider_id?: number
  keyword?: string
  created_from?: string
  created_to?: string
  page?: number
  page_size?: number
}

export interface TaskQueueItem {
  id: string
  category: string
  category_name: string
  action: string
  action_name: string
  subject: string
  ref_no: string
  instance_ref: string
  status: string
  status_name: string
  upstream_state: string
  upstream_state_name: string
  upstream_detail: string
  provider_id: number
  provider_name: string
  user_id: number
  username: string
  attempts: number
  max_attempts: number
  amount: number
  created_at: string
  finished_at: string | null
  duration_ms: number
}

export interface TaskQueueCategoryCount {
  category: string
  category_name: string
  total: number
  not_reached: number
}

export interface TaskQueueSummary {
  total: number
  pending: number
  running: number
  success: number
  failed: number
  manual: number
  reached: number
  not_reached: number
  reached_rate: number
  categories: TaskQueueCategoryCount[]
}

export interface TaskQueueListResponse {
  items: TaskQueueItem[]
  meta: ListMeta
  summary: TaskQueueSummary
}

// ===== 实例对账（运维 · 本地实例 vs 上游） =====

export interface ReconcileListQuery {
  provider_id?: number
  keyword?: string
  anomaly?: string
  page?: number
  page_size?: number
}

export interface ReconcileAnomalyItem {
  code: string
  name: string
  severity: string
  severity_name: string
}

export interface ReconcileItem {
  id: number
  instance_id: string
  name: string
  status: string
  status_name: string
  lifecycle_stage: string
  provider_id: number
  provider_name: string
  user_id: number
  username: string
  sell_product_id: number
  sell_product_name: string
  local_price: number | null
  upstream_product_id: number
  upstream_product_name: string
  upstream_sku: string
  upstream_cost: number | null
  margin_amount: number | null
  margin_rate: number | null
  price_anomaly: string
  price_anomaly_name: string
  local_expire_at: string | null
  upstream_expire_at: string | null
  expire_diff_days: number | null
  expire_anomaly: string
  expire_anomaly_name: string
  anomalies: ReconcileAnomalyItem[]
  severity: string
  severity_name: string
  detail: string
}

export interface ReconcileSummary {
  total: number
  danger: number
  warning: number
  ok: number
  below_cost: number
  thin_margin: number
  missing_price: number
  expire_early: number
  expire_late: number
  missing_expire: number
  negative_margin: number
  total_margin: number
  avg_margin_rate: number
  tolerance_days: number
  thin_margin_rate: number
}

export interface ReconcileListResponse {
  items: ReconcileItem[]
  meta: ListMeta
  summary: ReconcileSummary
}
