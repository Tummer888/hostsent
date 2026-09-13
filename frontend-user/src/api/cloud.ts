import request from '@/utils/request'

// 用户中心 · 云主机管理接口（baseURL=/api/v1，路径为 /api/v1/uc/instances/*）

export interface InstanceInfo {
  id: number
  instance_id: string
  provider_type: string
  provider_id: number
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
  expire_at: string
  created_at: string
  updated_at: string
}

export interface InstanceListResponse {
  items: InstanceInfo[]
  total: number
}

export interface VNCResult {
  url: string
  external: boolean
}

export function listInstances(params: { live?: boolean } = {}) {
  return request.get<any, { data: InstanceListResponse }>('/uc/instances', { params })
}

export function getInstance(id: number, live = false) {
  return request.get<any, { data: InstanceInfo }>(`/uc/instances/${id}`, { params: { live } })
}

export function powerInstance(id: number, action: 'on' | 'off' | 'reboot' | 'hard_off' | 'hard_reboot') {
  return request.post<any, { data: unknown }>(`/uc/instances/${id}/power`, { action })
}

export function vncInstance(id: number) {
  return request.post<any, { data: VNCResult }>(`/uc/instances/${id}/vnc`)
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
  return request.delete<any, { data: unknown }>(`/uc/instances/${id}`, {
    data: { confirm_mark: confirmMark, reason },
    headers: verifyTicket ? { 'X-Verify-Ticket': verifyTicket } : undefined,
  })
}
