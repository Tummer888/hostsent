import request from '@/utils/request'

// 用户中心 · 续费管理接口（baseURL=/api/v1，路径拼接为 /api/v1/uc/cloud/*）
// 注意：本项目 request 拦截器返回完整信封，因此各函数类型标注为 { data: T }。

export interface ListMeta {
  page: number
  page_size: number
  total: number
}

// 生命周期策略摘要
export interface LifecyclePolicySummary {
  remind_days: string
  auto_renew_default: boolean
  grace_days: number
  destroy_keep_days: number
  updated_at: string
}

// 我的实例续费条目（聚合视图）
export interface UserInstanceRenewalItem {
  id: number
  instance_mark: string
  name: string
  product_id: number
  product_name: string
  unit_price: number
  billing_mode: string
  status: string
  expire_at: string
  days_left: number
  stage: string
  auto_renew: boolean
  auto_period: number
}

// 续费管理聚合视图
export interface UserRenewalsViewResponse {
  items: UserInstanceRenewalItem[]
  policy: LifecyclePolicySummary
}

// 续费记录
export interface RenewalInfo {
  id: number
  renewal_no: string
  instance_id: number
  instance_mark: string
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
  meta: ListMeta
}

// 查询我的续费管理聚合视图（实例到期情况 + 策略摘要）
export function getRenewalsView() {
  return request.get<any, { data: UserRenewalsViewResponse }>('/uc/instances/renewals')
}

// 发起续费（余额支付）
export function renewMyInstance(instanceId: number, periodCount = 1) {
  return request.post<any, { data: { renewal: RenewalInfo } }>(`/uc/instances/${instanceId}/renew`, {
    period_count: periodCount,
  })
}

// 切换自动续费开关
export function toggleMyAutoRenew(instanceId: number, enabled: boolean, periodCount = 1) {
  return request.put<any, { data: string }>(`/uc/instances/${instanceId}/auto-renew`, {
    enabled,
    period_count: periodCount,
  })
}

// 查询我的续费记录
export function getMyRenewals(params: { status?: string; page?: number; page_size?: number } = {}) {
  return request.get<any, { data: RenewalListResponse }>('/uc/renewals', { params })
}

// 查询我的续费记录详情
export function getMyRenewalDetail(id: number) {
  return request.get<any, { data: RenewalInfo }>(`/uc/renewals/${id}`)
}
