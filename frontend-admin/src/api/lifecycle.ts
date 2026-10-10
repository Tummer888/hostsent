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
  // 到期阶段自动执行总开关（默认 false：只派生阶段、不动上游）。
  auto_enforce: boolean
  // 预演开关（默认 true）：总开关开启时，true 仍只计算不下发。
  enforce_dry_run: boolean
  updated_at: string
}

export interface PolicyUpdateRequest {
  remind_days?: string
  auto_renew_default?: boolean
  grace_days?: number
  destroy_keep_days?: number
  auto_enforce?: boolean
  enforce_dry_run?: boolean
}

// 到期处置预演单条结果
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

// 到期处置预演报告（dry-run，只读）
export function previewEnforcement(): Promise<EnforcementPreviewResponse> {
  return request.get<EnforcementPreviewResponse>({ url: '/lifecycle/enforcement/preview' })
}

// 手动对单实例执行一次阶段处置（真实下发上游，需二次验证票据）
export function enforceInstance(id: number, reason?: string, verifyTicket?: string): Promise<{ action: string }> {
  return request.post<{ action: string }>({
    url: `/lifecycle/enforcement/${id}/run`,
    data: { reason },
    headers: verifyTicket ? { 'X-Verify-Ticket': verifyTicket } : undefined,
  })
}
