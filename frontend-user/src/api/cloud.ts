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
