import request from '@/utils/request'

// 用户中心 · 云主机管理接口（baseURL=/api/v1，路径为 /api/v1/uc/instances/*）

/** 电源操作可用动作。 */
export type PowerAction = 'on' | 'off' | 'reboot' | 'hard_off' | 'hard_reboot'

/** 实例可用操作能力位（由上游能力推导，前端据此置灰按钮）。 */
export interface InstanceCapabilities {
  power: boolean
  console: boolean
  reinstall: boolean
  reset_password: boolean
  rescue: boolean
  snapshot: boolean
  destroy: boolean
  /** 是否提供性能监控图表（CPU/内存/磁盘/网卡）。 */
  metrics: boolean
}

export interface InstanceInfo {
  id: number
  instance_id: string
  // 说明：服务端刻意不下发 provider_type/provider_id（对接渠道的类型码与配置行 ID）。
  name: string
  status: string // running / stopped / creating / ...
  power_status: string // on / off / unknown
  os: string
  cpu: number
  memory: number
  disk: number
  disk_type: string
  bandwidth: number
  region: string
  zone: string
  public_ip: string
  private_ip: string
  billing_mode: string
  /** 开通该实例的真实操作人（子账号下单时有值，P4-09） */
  actor_user_id: number
  actor_name: string
  order_id: number
  pending: boolean
  /** 待开通实例的履约状态与最近失败原因（manual=需人工处理，如上游余额不足）。 */
  provision_status?: string
  provision_error?: string
  expire_at: string
  /** 生命周期阶段：active/grace/suspended/destroyed（与状态是两个维度）。 */
  lifecycle_stage: string
  /** 到期状态：none/expiring/expired/normal（读时计算）。 */
  expire_state: string
  /** 距到期天数（负数表示已过期天数）。 */
  days_left: number
  created_at: string
  updated_at: string
  capabilities?: InstanceCapabilities
  /**
   * NAT 机型访问地址。NAT 机型没有独立公网 IP，这里是**唯一**能连上机器的方式；
   * 独立 IP 机型该字段为 undefined。
   */
  nat_access?: NatAccessInfo
}

/** NAT 机型访问地址（doc122）。 */
export interface NatAccessInfo {
  /** 是否 NAT 机型（依据机型而非规则数量：刚开通还没建规则也是 NAT 机型）。 */
  is_nat: boolean
  /** 端口转发地址，已拼成 "IP:端口"，可直接复制使用。 */
  forwards: string[]
  /** 共享建站域名。 */
  domains: string[]
  /** 读取失败原因；与「没有转发规则」区分，非空时提示刷新重试。 */
  error?: string
}

/** 列表汇总（服务端与 items 同口径统计，前端不自己数）。 */
export interface InstanceListSummary {
  total: number
  running: number
  stopped: number
  creating: number
  expiring: number
  expired: number
  destroyed: number
}

export interface InstanceListResponse {
  items: InstanceInfo[]
  total: number
  summary: InstanceListSummary
}

export interface VNCResult {
  url: string
  external: boolean
}

export function listInstances(params: { live?: boolean } = {}) {
  return request.get<unknown, { data: InstanceListResponse }>('/uc/instances', { params })
}

export function getInstance(id: number, live = false) {
  return request.get<unknown, { data: InstanceInfo }>(`/uc/instances/${id}`, { params: { live } })
}

export function powerInstance(id: number, action: PowerAction) {
  return request.post<unknown, { data: unknown }>(`/uc/instances/${id}/power`, { action })
}

/** 重新开通待开通实例（重投订单履约任务；不产生新订单/新账单）。 */
export function retryInstanceProvision(id: number) {
  return request.post<unknown, { data: InstanceInfo }>(`/uc/instances/${id}/provision-retry`)
}

export function vncInstance(id: number) {
  return request.post<unknown, { data: VNCResult }>(`/uc/instances/${id}/vnc`)
}

/**
 * 自助销毁实例（doc91 §6.4）。
 *
 * 注意本项目的 request 是 axios 实例本身（不是 admin 那种 {url} 对象式封装），
 * 所以 DELETE 带 body 要写成 request.delete(url, { data })。confirmMark 必须
 * 等于 instance_id，服务端会逐字比对。
 *
 * 二次验证：场景策略要求时后端返回 403 + 20017，此时需先用
 * verifySecurityCode('instance_destroy', code) 拿 verify_ticket，再带
 * verifyTicket 重放本次请求。
 *
 * 销毁不可逆且不退还剩余费用：服务端只终止上游实例并把本地状态置 deleted，
 * 不触发任何退款/余额返还。
 */
export function destroyInstance(id: number, confirmMark: string, reason?: string, verifyTicket?: string) {
  return request.delete<unknown, { data: unknown }>(`/uc/instances/${id}`, {
    data: { confirm_mark: confirmMark, reason },
    headers: verifyTicket ? { 'X-Verify-Ticket': verifyTicket } : undefined,
  })
}

// ---- 维护类自助操作（与后端 /uc/instances/* 一一对应）----

/** 重装系统；返回平台新签发的初始凭据（只返回一次，前端必须提示立即保存）。 */
export interface ReinstallResult {
  username?: string
  password?: string
}

export function reinstallInstance(
  id: number,
  data: {
    os: string
    port?: number
    format_data_disk?: boolean
    system_disk_size?: number
    reason?: string
  },
  verifyTicket?: string,
) {
  return request.post<unknown, { data: ReinstallResult }>(`/uc/instances/${id}/reinstall`, data, {
    headers: verifyTicket ? { 'X-Verify-Ticket': verifyTicket } : undefined,
  })
}

export function resetInstancePassword(id: number, password: string) {
  return request.post<unknown, { data: unknown }>(`/uc/instances/${id}/reset-password`, { password })
}

export function rescueInstance(id: number, system: number, tempPassword: string) {
  return request.post<unknown, { data: unknown }>(`/uc/instances/${id}/rescue`, {
    system,
    temp_password: tempPassword,
  })
}

export function exitRescueInstance(id: number) {
  return request.post<unknown, { data: unknown }>(`/uc/instances/${id}/exit-rescue`)
}

export interface SnapshotInfo {
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

export function listInstanceSnapshots(id: number, type?: string) {
  return request.get<unknown, { data: SnapshotInfo[] }>(`/uc/instances/${id}/snapshots`, { params: { type } })
}

export function createInstanceSnapshot(id: number, data: { type?: string; name?: string; disk_id?: string }) {
  return request.post<unknown, { data: unknown }>(`/uc/instances/${id}/snapshots`, data)
}

export function deleteInstanceSnapshot(id: number, snapshotId: string) {
  return request.delete<unknown, { data: unknown }>(`/uc/instances/${id}/snapshots/${snapshotId}`)
}

export function restoreInstanceSnapshot(id: number, snapshotId: string, confirmMark: string, verifyTicket?: string) {
  return request.post<unknown, { data: unknown }>(
    `/uc/instances/${id}/snapshots/restore`,
    { snapshot_id: snapshotId, confirm_mark: confirmMark },
    { headers: verifyTicket ? { 'X-Verify-Ticket': verifyTicket } : undefined },
  )
}

// ---- 只读运维可见性：操作流水 + 性能监控 ----

/** 实例操作流水项（用户可见的只读审计，不含内部参数）。 */
export interface InstanceOperation {
  id: number
  action: string
  operator_type: string // admin/user/system
  operator_name: string
  result: string // success/failed/skipped
  error_message?: string
  created_at: string
}

export function listInstanceOperations(id: number, limit = 20) {
  return request.get<unknown, { data: InstanceOperation[] }>(`/uc/instances/${id}/operations`, {
    params: { limit },
  })
}

/** 监控图表目录；supported=false 时 reason 说明原因（前端显示说明区块）。 */
export interface MetricsCatalog {
  supported: boolean
  reason?: string
  charts: MetricsChartInfo[]
}

export interface MetricsChartInfo {
  /** 平台原生图表标识：请求 series 时必须原样回传（财务型上游会透传，如 flow）。 */
  type: string
  /** 归一化语义：cpu/memory/disk/network（仅用于配色/图标，可能等于 type）。 */
  kind: string
  title: string
  select?: MetricsChartOption[]
}

export interface MetricsChartOption {
  label: string
  value: string
}

export interface MetricsPoint {
  time: string
  value: number
}

export interface MetricsSeries {
  type: string
  unit: string
  /** area / line / bar */
  chart_type: string
  /** 与 list 一一对应的曲线名。 */
  label: string[]
  /** list[i] 是 label[i] 的采样点。 */
  list: MetricsPoint[][]
}

export function getInstanceMetricsCatalog(id: number) {
  return request.get<unknown, { data: MetricsCatalog }>(`/uc/instances/${id}/metrics`)
}

/** 时间窗预设：1h/6h/24h/7d（后端默认 6h）。 */
export type MetricsRange = '1h' | '6h' | '24h' | '7d'

export function getInstanceMetricsSeries(id: number, params: { type: string; select?: string; range?: MetricsRange }) {
  return request.get<unknown, { data: MetricsSeries }>(`/uc/instances/${id}/metrics/series`, { params })
}
