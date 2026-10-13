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
  /** 上游实例号；为空表示尚未开通（待开通占位行）。 */
  provider_instance_id: string
  /** 待开通（未在上游开通）：前端显示「待开通」并只开放「重新开通」。 */
  pending: boolean
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
  /** 管理员内部备注（仅管理端可见，客户看不到）。 */
  remark: string
  /** 客户备注（客户下单填写或自助修改，客户在自己界面也看得到）。 */
  customer_remark: string
  expire_at: string
  days_left: number
  expire_state: string
  last_synced_at: string
  // 到期处置退避状态（doc61 §8.4）
  enforce_attempts: number
  enforce_next_at: string
  last_enforce_error: string
  /** 绑定的续费单价（元/周期）；null = 未绑定，按商品实时算价。 */
  renewal_price: number | null
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
  /** 维护类（平台接口实测存在）：重置密码 / 救援系统 / 快照 / 硬件变更 */
  reset_password: boolean
  rescue: boolean
  snapshot: boolean
  bandwidth: boolean
  add_ip: boolean
  attach_disk: boolean
  /** NAT 访问地址可读（渠道声明了 NatReader 能力）。 */
  nat_access: boolean
}

/**
 * NAT 机型访问地址。
 *
 * NAT 机型没有独立公网 IP，此处是客户**唯一**能连上机器的方式；
 * 独立 IP 机型该字段为 undefined（后端不下发，前端不渲染）。
 */
export interface InstanceNatAccess {
  /** 是否 NAT 机型。依据是机型而非规则数量：刚开通还没建规则也是 NAT 机型。 */
  is_nat: boolean
  /** 端口转发地址，已拼成 "IP:端口"，可直接复制使用。 */
  forwards: string[]
  /** 共享建站域名。 */
  domains: string[]
  /** 读取失败原因；与「没有转发规则」区分，非空时提示可重试而非显示"无 NAT 配置"。 */
  error?: string
}

/** 来源订单财务口径：首付是历史成交，续费价是下次应付（商品当前周期价）。 */
/** 开通履约状态（待开通实例）：manual=重试无效需人工（如上游余额不足）。 */
export interface InstanceProvisionState {
  provision_status?: string
  provision_error?: string
}

export interface InstanceSourceOrder {
  id: number
  order_no: string
  status: string
  product_id: number
  product_name: string
  cycle: string
  cycle_name: string
  quantity: number
  total_amount: number
  paid_amount: number
  discount_amount: number
  first_paid_amount: number
  pay_method: string
  pay_time: string
  ordered_at: string
  renewal_id: number
  renew_unit_cost: number
  renew_cycle: string
  renew_amount: number
  renew_cost: number
  /** renew_amount 是否来自实例绑定价（instances.renewal_price），而非商品周期价。 */
  renew_bound: boolean
  /** 上游续费金额（对接商品；来自上游商品各周期价，未取到为 0）。 */
  upstream_renew_amount: number
  upstream_renew_cycle: string
  /** 管理员在订单上写的备注（内部可见）。 */
  remark: string
  /** 客户下单时填写的备注。 */
  customer_remark: string
}

/** 售出商品与我方对接关系。 */
export interface InstanceProduct {
  id: number
  code: string
  name: string
  source_mode: string
  source_provider_id: number
  source_provider_name: string
  provider_type: string
  source_product_id: number
  price_model: string
  price: number
  cost_price: number
  upstream_markup_type: string
  upstream_markup_value: number
  status: number
  /** 商品行已不存在（重新导入/清理后常见）：名称取订单快照，前端标「商品已删除」。 */
  deleted: boolean
}

/** 对接商品的上游信息（source_mode=upstream 时有值）。 */
export interface InstanceUpstream {
  resource_product_id: number
  /** 该上游资源商品所属渠道；与实例渠道不一致时 provider_mismatch=true。 */
  provider_id: number
  provider_name: string
  provider_mismatch: boolean
  upstream_id: string
  name: string
  group_id: number
  group_name: string
  list_price: number
  cost_price: number
  discount_desc: string
  pay_type: string
  renewable: boolean
  /** 上游标价（各周期）。 */
  list_cycles: Record<string, number>
  /** 我方采购价（已含代理折扣，各周期）。 */
  cost_cycles: Record<string, number>
}

/** 实例生效配置项；source 说明取值来源，locked 表示上游锁定改不动。 */
export interface InstanceConfigItem {
  key: string
  label: string
  value: string
  source: string
  locked: boolean
  unit: string
}

/** 登录连接信息（不含明文密码；明文走 reveal 接口）。 */
export interface InstanceCredential {
  username: string
  port: string
  has_password: boolean
  password_hint: string
  has_panel_password: boolean
}

export interface InstanceCredentialReveal {
  username: string
  password: string
  port: string
}

export interface InstanceDetail extends InstanceItem {
  capabilities: InstanceCapabilities
  capability_error: string
  order?: InstanceSourceOrder
  product?: InstanceProduct
  upstream?: InstanceUpstream
  configs?: InstanceConfigItem[]
  credential?: InstanceCredential
  /** 待开通实例的履约状态与最近失败原因（manual=需人工处理，如上游余额不足）。 */
  provision_status?: string
  provision_error?: string
  /** NAT 机型访问地址（doc122）；独立 IP 机型不存在该字段。 */
  nat_access?: InstanceNatAccess
}

/** 账期/付款周期调整请求（本地口径，不产生账单；对接商品需 force）。 */
export interface InstanceAdjustRequest {
  expire_at?: string
  clear_expire?: boolean
  billing_mode?: string
  /**
   * 绑定续费单价（元/周期）。不传 = 本次不改；>0 = 绑定此价；-1 = 清除绑定
   * （回到按商品实时算价）。
   */
  renewal_price?: number
  force?: boolean
  reason?: string
}

/** 账期调整对话框里的一个可选周期（周期取自上游能力，价格取自上游镜像 + 我方价格矩阵）。 */
export interface InstanceBillingCycleOption {
  cycle: string
  name: string
  upstream: boolean
  upstream_list: number
  upstream_cost: number
  sale: number
  cost: number
  has_product_price: boolean
}

/** 账期调整上下文（可选周期 + 当前绑定续费价）。 */
export interface InstanceAdjustContext {
  cycles: InstanceBillingCycleOption[]
  current_cycle: string
  current_renewal_price: number | null
  renewal_price_hint: string
  /** 未能读到上游支持周期：cycles 回落到平台全集，界面需提示"未经上游核对"。 */
  upstream_cycles_unavailable: boolean
}

/** 按上游实例号重新关联（上游换机/换号）。 */
export interface InstanceRelinkRequest {
  upstream_instance_id: string
  confirm_mark: string
  /** 是否把本地展示号（instance_id）也改成新上游号（仅原两者一致时才会生效）。 */
  migrate_instance_id?: boolean
  reason?: string
}

export interface InstanceRelinkResult {
  old_provider_instance_id: string
  new_provider_instance_id: string
  instance_id: string
  status: string
  public_ip: string
  private_ip: string
  name: string
  os: string
  region: string
  instance_id_migrated: boolean
}

/** 更换实例绑定的本地商品 / 上游镜像（上游换产品）。 */
export interface InstanceRebindProductRequest {
  sell_product_id?: number
  /** 不传 = 不改；0 = 清空（不再关联任何上游镜像）。 */
  upstream_product_id?: number
  /** 顺带更新绑定续费价：不传 = 不改；-1 = 清除绑定。 */
  renewal_price?: number
  reason?: string
}

export interface InstanceRebindProductResult {
  old_sell_product_id: number
  new_sell_product_id: number
  old_upstream_product_id: number
  new_upstream_product_id: number
  sell_product_name: string
  upstream_product_name: string
  upstream_changed: boolean
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

// 批量运维（doc61 P1）：逐台结果三态 —— success / skipped（无需执行）/ failed（真实失败）
export interface InstanceBatchActionRequest {
  action: string
  ids: number[]
  reason?: string
}

export interface InstanceBatchActionItem {
  id: number
  instance_mark: string
  name: string
  status: string
  message: string
  before_status: string
  after_status: string
}

export interface InstanceBatchActionResponse {
  total: number
  succeeded: number
  failed: number
  skipped: number
  items: InstanceBatchActionItem[]
}

// 批量销毁（不可逆）：confirm_marks 是「实例记录 ID（字符串）→ 该台确认文本」，
// 服务端逐台校验；缺哪台就跳过哪台（勾选全选 ≠ 一键删机器）。
export interface InstanceBatchDestroyRequest {
  ids: number[]
  confirm_marks: Record<string, string>
  reason?: string
}

// 与批量运维同一响应形态，便于前端复用同一套逐台结果回显。
export interface InstanceBatchDestroyResponse {
  total: number
  succeeded: number
  failed: number
  skipped: number
  items: InstanceBatchActionItem[]
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

// ---- 维护类操作请求/响应（与后端 admin/instance/dto 同形）----

export interface InstanceReinstallRequest {
  os: string
  port?: number
  /** 是否同时格式化数据盘（数据将丢失） */
  format_data_disk?: boolean
  system_disk_size?: number
  reason?: string
}

/** 平台重装后签发的初始凭据；只返回一次，不落库。 */
export interface InstanceReinstallResult {
  username?: string
  password?: string
}

export interface InstanceRescueRequest {
  /** 救援系统类型（平台口径：魔方云 1/2） */
  system: number
  temp_password: string
}

export interface InstanceSnapshotCreateRequest {
  /** snap（快照）| backup（备份）；空视为 snap */
  type?: string
  name?: string
  /** 目标磁盘 ID；空则后端取系统盘 */
  disk_id?: string
}

export interface InstanceSnapshotRestoreRequest {
  snapshot_id: string
  confirm_mark: string
}

export interface InstanceSnapshotInfo {
  id: string
  name: string
  type: string
  size: string
  status: number
  disk_id: string
  disk_name: string
  create_time: string
  remarks: string
}

export interface InstanceBandwidthRequest {
  in_bw: number
  out_bw: number
  reason?: string
}

export interface InstanceAddIPRequest {
  /** 4 或 6；空视为 4 */
  version?: number
  num: number
  ip_group?: string
}

export interface InstanceAttachDiskRequest {
  size_gb: number
  store?: string
  reason?: string
}
