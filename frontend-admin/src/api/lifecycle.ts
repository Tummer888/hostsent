import { request } from '@/utils/request'

// ===== 生命周期与续费管理（doc60）=====

// 到期实例列表项
export interface ExpiringInstanceItem {
  id: number
  instance_mark: string
  name: string
  user_id: number
  username: string
  product_id: number
  product_name: string
  unit_price: number
  billing_mode: string
  status: string
  expire_at: string
  days_left: number
  stage: string
  auto_renew: boolean
}

export interface ExpiringListResponse {
  items: ExpiringInstanceItem[]
  meta: { page: number; page_size: number; total: number }
}

// 续费记录
export interface RenewalInfo {
  id: number
  renewal_no: string
  instance_id: number
  instance_mark: string
  user_id: number
  username: string
  product_id: number
  product_name: string
  billing_mode: string
  period_count: number
  amount: number
  source: string
  status: string
  order_id: number
  order_no: string
  pay_time: string
  expire_before: string
  expire_after: string
  fail_reason: string
  created_at: string
}

export interface RenewalListResponse {
  items: RenewalInfo[]
  meta: { page: number; page_size: number; total: number }
}

// 生命周期策略
export interface LifecyclePolicy {
  remind_days: string
  auto_renew_default: boolean
  grace_days: number
  destroy_keep_days: number
  updated_at: string
}

export interface PolicyUpdateRequest {
  remind_days?: string
  auto_renew_default?: boolean
  grace_days?: number
  destroy_keep_days?: number
}

export interface ExpiringListQuery {
  stage?: string
  keyword?: string
  page?: number
  page_size?: number
}

export interface RenewalListQuery {
  keyword?: string
  user_id?: number
  status?: string
  source?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

// 查询到期实例列表（按剩余天数升序）
export function getExpiringInstances(params: ExpiringListQuery): Promise<ExpiringListResponse> {
  return request.get<ExpiringListResponse>({ url: '/instances/expiring', params: { ...params } })
}

// 查询续费记录列表
export function getRenewalList(params: RenewalListQuery): Promise<RenewalListResponse> {
  return request.get<RenewalListResponse>({ url: '/renewals', params: { ...params } })
}

// 查询续费记录详情
export function getRenewalDetail(id: number): Promise<RenewalInfo> {
  return request.get<RenewalInfo>({ url: `/renewals/${id}` })
}

// 管理员代续费
export function renewInstance(
  id: number,
  data: { period_count?: number; amount?: number; remark?: string }
): Promise<RenewalInfo> {
  return request.post<RenewalInfo>({ url: `/instances/${id}/renew`, data })
}

// 查询生命周期策略
export function getLifecyclePolicy(): Promise<LifecyclePolicy> {
  return request.get<LifecyclePolicy>({ url: '/lifecycle/policy' })
}

// 更新生命周期策略
export function updateLifecyclePolicy(data: PolicyUpdateRequest): Promise<string> {
  return request.put<string>({ url: '/lifecycle/policy', data })
}

// 手动触发生命周期扫描
export function triggerLifecycleScan(): Promise<string> {
  return request.post<string>({ url: '/lifecycle/scan' })
}
