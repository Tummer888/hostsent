// 类型定义（instance 域）：实例运维台（跨用户实例运维）
// 注意：resource.d.ts 已存在同名 InstanceListQuery / InstanceListResponse（资源实例列表），
// 为避免 interface.d.ts barrel 中 `export *` 重名被丢弃，此处使用 InstanceOps* 前缀。
import type { ListMeta } from './common'

export interface InstanceOpsListQuery {
  keyword?: string
  user_keyword?: string
  user_id?: number
  provider_id?: number
  status?: string
  source_mode?: string
  expire_state?: string
  expire_within_days?: number
  page?: number
  page_size?: number
}

export interface InstanceItem {
  id: number
  instance_id: string
  provider_id: number
  provider_name: string
  provider_type: string
  user_id: number
  username: string
  user_email: string
  user_phone: string
  order_id: number
  source_mode: string
  sell_product_id: number
  upstream_product_id: number
  provider_instance_id: string
  lifecycle_stage: string
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
  power_status: string
  private_ip: string
  public_ip: string
  billing_mode: string
  actor_user_id: number
  actor_name: string
  remark: string
  expire_at: string
  days_left: number
  expire_state: string
  last_synced_at: string
  // 到期处置退避状态（doc61 §8.4）
  enforce_attempts: number
  enforce_next_at: string
  last_enforce_error: string
  created_at: string
  updated_at: string
}

export interface InstanceOpsListResponse {
  items: InstanceItem[]
  meta: ListMeta
}

export interface InstanceStatsResponse {
  total: number
  running: number
  stopped: number
  creating: number
  error: number
  expiring: number
  expired: number
}

export interface InstanceCapabilities {
  power: boolean
  console: boolean
  resize: boolean
  destroy: boolean
  reinstall: boolean
  suspend: boolean
}

export interface InstanceDetail extends InstanceItem {
  capabilities: InstanceCapabilities
  capability_error: string
}

export interface OperationItem {
  id: number
  instance_id: number
  instance_mark: string
  user_id: number
  operator_type: string
  operator_id: number
  operator_name: string
  action: string
  params?: unknown
  before_status: string
  after_status: string
  result: string
  error_message: string
  created_at: string
}

export interface OperationListQuery {
  page?: number
  page_size?: number
}

// 全局操作流水（跨实例审计视图）
export interface OperationLogQuery {
  keyword?: string
  action?: string
  operator_type?: string
  result?: string
  user_id?: number
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface OperationLogItem extends OperationItem {
  instance_name: string
  username: string
}

export interface OperationLogListResponse {
  items: OperationLogItem[]
  meta: ListMeta
}

// 到期处置（doc61 §8.4）：预演报告与策略闸门
export interface LifecyclePolicyEnforce {
  auto_enforce: boolean
  enforce_dry_run: boolean
}

export interface EnforcementPreviewItem {
  instance_id: number
  instance_mark: string
  name: string
  user_id: number
  username: string
  provider_id: number
  stage: string
  target_stage: string
  action: string
  reason: string
  expire_at: string
  days_left: number
  capability_missing: boolean
}

export interface EnforcementPreviewResponse {
  enabled: boolean
  dry_run: boolean
  total: number
  stage_counts: Record<string, number>
  items: EnforcementPreviewItem[]
  generated_at: string
}

export interface OperationListResponse {
  items: OperationItem[]
  meta: ListMeta
}

export interface InstancePowerRequest {
  action: string
}

export interface InstanceRemarkUpdateRequest {
  remark: string
}

export interface InstanceVNCResponse {
  url: string
  password: string
  external: boolean
}

export interface InstanceResizeRequest {
  cpu: number
  memory: number
  disk: number
  disk_type: string
  reason?: string
}

export interface InstanceDestroyRequest {
  confirm_mark: string
  reason?: string
}

export interface RelatedOrder {
  id: number
  order_no: string
  status: string
  product_name: string
  total_amount: number
  paid_amount: number
  pay_method: string
  created_at: string
}

export interface RelatedRenewal {
  id: number
  renewal_no: string
  source: string
  status: string
  period_count: number
  amount: number
  expire_before: string
  expire_after: string
  created_at: string
}

export interface RelatedTicket {
  id: number
  ticket_no: string
  title: string
  priority: string
  status: string
  assigned_name: string
  created_at: string
}

export interface InstanceRelatedResponse {
  orders: RelatedOrder[]
  renewals: RelatedRenewal[]
  tickets: RelatedTicket[]
}
