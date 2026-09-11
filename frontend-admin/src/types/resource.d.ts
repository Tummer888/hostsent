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
  page?: number
  page_size?: number
}

export interface PoolInfo {
  id: number
  provider_id: number
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
}

export interface PoolListResponse {
  items: PoolInfo[]
  meta: ListMeta
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
  product_id: number
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
