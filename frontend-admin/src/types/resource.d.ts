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

  /** 是否具备商品目录能力（可作「导入上游商品」的数据源）：自营算力平台为 false */
  catalog_supported?: boolean
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

/** 平台可配置项：下单时能下发给上游的规格参数（CPU/内存/磁盘/带宽…） */
export interface ConfigOptionSpec {
  key: string
  label: string
  group: string
  required: boolean
  default: string
  widget: string
  value_source: string
  options?: ProviderFieldOption[]
  min_value?: number
  max_value?: number
  step?: number
  unit?: string
  help?: string
  options_help?: string
  multi_value: boolean
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
  /** 上游是否真的能报告机器电源状态（false 时回源不覆盖本地开关机状态） */
  reports_power_state: boolean
  /** 平台支持的可配置项（驱动「平台配置项」与「配置档」） */
  option_catalog?: ConfigOptionSpec[] | null
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
  /** 池类别：capacity=容量型（宿主机/存储），location=位置型（区域/可用区）；不传=全部 */
  category?: string
  /** group=region 时按区域分组返回层级树（区域为父、宿主机/存储为子） */
  group?: string
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
  /** 池类别：capacity（有 CPU/内存/磁盘）/ location（只有位置，无容量） */
  category: string
  /** 上级池的上游 ID（宿主机/存储 → 所属区域）；空=顶层 */
  parent_upstream_id: string
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
  // ---- 层级视图（group=region）----
  /** 该区域下的宿主机/存储池（仅层级视图填充） */
  children?: PoolInfo[]
  /** 子池数量 */
  child_count?: number
  /** 容量字段是否为子池汇总（区域自身无容量） */
  aggregated?: boolean
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
  /** 容量型池数（有 CPU/内存/磁盘配额） */
  capacity_pools: number
  /** 位置型池数（区域/可用区，无容量） */
  location_pools: number
}

export interface PoolListResponse {
  items: PoolInfo[]
  meta: ListMeta
  summary: PoolCapacitySummary
  /** 当前筛选结果中出现的地域 */
  regions: string[]
}

/** 渠道关联数据的一行（删除确认弹窗展示）。 */
export interface ProviderAssociationItem {
  id: number
  name: string
  /** product 上游商品 / pool 资源池 */
  kind: string
  /** 商品：型号/编码；资源池：类型 + 地域 */
  subtitle: string
  /** 该上游商品被多少销售商品引用（>0 表示连带删除会断履约链路，会被拦住） */
  has_orders: number
}

/** 渠道关联数据详情（删除确认弹窗用）。 */
export interface ProviderAssociations {
  provider_id: number
  products: ProviderAssociationItem[]
  pools: ProviderAssociationItem[]
  /**
   * 该渠道下**未销毁**的实例。
   *
   * 它们挡删除：渠道删掉后实例无法同步/操作，而渠道没有恢复入口，
   * 实例会永久卡在列表里（实测：删渠道导致实例卡死）。
   */
  instances: ProviderAssociationItem[]
}

/** 连带删除结果。 */
export interface ProviderCascadeResult {
  deleted_products: number
  deleted_pools: number
}

export interface ProductListQuery {
  keyword?: string
  provider_id?: number
  /** 上游分类（gid）：左侧分类导航按分类筛选 */
  group_id?: number
  /** 状态：1=在售，0=未在售；不传=全部（0 也有效，后端用指针区分） */
  status?: number
  page?: number
  page_size?: number
}

/** 上游商品对客加价规则：设定后立即重算已导入商品的售价与周期价格矩阵 */
export interface ProductUpstreamMarkupRequest {
  resource_product_id: number
  /** percent=代理价×v%；fixed=代理价+v；list_percent=上游标价×v%；空=清除规则 */
  markup_type: '' | 'percent' | 'fixed' | 'list_percent'
  markup_value: number
  remark?: string
}

export interface ProductUpstreamMarkupResponse {
  affected: number
  message: string
  details: string[]
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
  /** 分类名不是上游公示名，而是按商品名前缀派生（上游隐藏分组），前端打「派生」标记 */
  group_name_derived?: boolean
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
  /** 上游可售库存：-1=不限量；0=售罄；>0=剩余可售数量（随目录同步刷新） */
  stock: number
  /** 上游是否开启库存控制：1=限量（stock 为余量），0=不限量 */
  stock_control: number
  /** 上游前台在售判定：visible/group_hidden/product_hidden/retired（status 只有 1/0，这里给出原因） */
  visibility?: string
  /** 价格来源：pricings=商品周期价；catalog=上游前台展示价回填；none=上游未定价 */
  price_source?: string
  /** 上游前台展示价（cart/index），用于核对周期价与前台展示价 */
  catalog_price?: number
  catalog_sale_price?: number
  catalog_cycle?: string
  /** 该上游商品是否已导入为本地商品（导入弹窗禁止重复勾选；上游商品页区分已导入/未导入） */
  imported?: boolean
  imported_product_id?: number
  /** 已导入商品的状态：0=草稿 1=上架 2=下架 */
  imported_status?: number
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

/**
 * 渠道链路（双链路重构）：upstream=上游转售，compute=自营平台。
 * 控制台顶部据此切换视图；空字符串表示不过滤（全部）。
 */
export type ProviderKind = 'upstream' | 'compute' | ''

export interface SyncTaskListQuery {
  provider_id?: number
  task_type?: string
  status?: string
  provider_kind?: string
  page?: number
  page_size?: number
}

export interface SyncLogListQuery {
  provider_id?: number
  task_id?: number
  status?: string
  provider_kind?: string
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
  /** 渠道名（服务端反查；缺失时前端回退显示「渠道 #id」）。 */
  provider_name: string
  /** 渠道链路（upstream/compute），用于渠道列标签。 */
  provider_kind: string
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
  provider_name: string
  provider_kind: string
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
  provider_kind?: string
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
  provider_kind: string
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
  provider_kind?: string
  page?: number
  page_size?: number
}

/** 上游调价事件 */
export interface PriceChangeInfo {
  id: number
  provider_id: number
  provider_kind: string
  scope: string
  resource_product_id: number
  /** 上游商品名（服务端反查；为空时前端回退显示商品 ID）。 */
  resource_product_name: string
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
  provider_kind?: string
  page?: number
  page_size?: number
}

/** 同步差异记录（真实数据源：sync_diffs） */
export interface SyncDiffInfo {
  id: number
  task_id: number
  provider_id: number
  provider_name: string
  provider_kind: string
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
  provider_kind?: string
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
  provider_kind: string
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
  provider_kind?: string
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
  provider_kind: string
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
  // 状态向（doc120）。
  // upstream_status 是上游**服务状态**原文（raw_data.domainstatus，
  // Active/Suspended/Deleted...），不是机器电源状态：财务型上游（zjmf）
  // 没有电源字段，客户关机后它仍然是 Active。
  upstream_status: string
  // upstream_status_name 上游服务状态中文名（与本地侧同一套用词）。
  upstream_status_name: string
  status_anomaly: string
  status_anomaly_name: string
  // local_service_state(_name) 本地**服务**维度，是与 upstream_status 比对时
  // 的本地侧口径（服务中/宽限期/开通中/已暂停/已终止/已取消）。
  // 展示"状态（本地 / 上游）"用它；status/status_name 是机器电源维度
  // （运行中/已停止），只在"这台机器开没开机"的场景使用。
  local_service_state: string
  local_service_state_name: string
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
  // 状态不同步计数（doc120）。
  status_mismatch: number
  status_stale: number
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

// 对账差异台账（doc120）：reconcile_findings 的行视图。
// 与 ReconcileItem 的区别：这是**持久化台账**（差异存在多久、谁处理了），
// ReconcileItem 是**当前时刻的比对快照**。
export interface ReconcileFindingItem {
  id: number
  provider_id: number
  instance_id: number
  kind: string
  kind_name: string
  severity: string
  local_value: string
  upstream_value: string
  delta?: number
  status: string
  note: string
  first_seen_at: string
  last_seen_at: string
  notified_at: string | null
  // days_persisted 已持续天数（前端也可自行按 first_seen_at 算，此项为后端口径）。
  days_persisted: number
}

export interface ReconcileFindingListResponse {
  total: number
  list: ReconcileFindingItem[]
  page: number
  page_size: number
}
