// 验证码与二次验证体系（doc91）· 管理端 API
// 对应后端 internal/modules/uc/captcha/{dto,handler}，路由前缀 /api/v1/admin/captcha。
import { request } from '@/utils/request'

/** 凭证字段描述（与后端 integration.Field 同构，驱动动态表单 + 字段级加密） */
export interface CaptchaCredentialField {
  key: string
  label: string
  type: 'string' | 'password' | 'number' | 'select' | 'bool' | 'textarea'
  required: boolean
  secret: boolean
  placeholder?: string
  help?: string
  default?: string
  options?: Array<{ label: string; value: string }>
}

/** 服务商类型（描述符驱动前端动态表单） */
export interface CaptchaProviderType {
  type: string
  name: string
  mode: string
  icon: string
  doc_url: string
  adapter_version: string
  /** 适配器是否已注册真实实现 */
  implemented: boolean
  /** 内建兜底（native），不可删除、不可停用 */
  builtin: boolean
  credential_schema: CaptchaCredentialField[] | null
}

export interface CaptchaProviderInfo {
  id: number
  provider_type: string
  name: string
  mode: string
  endpoint: string
  priority: number
  health_status: string
  last_error: string
  last_check_at: string
  status: number
  is_default: boolean
  remark: string
  /** 脱敏回显：secret 字段为掩码，原样回传表示不修改 */
  credentials: Record<string, string>
  credential_keys: string[]
  implemented: boolean
  builtin: boolean
  supported: boolean
}

export interface CaptchaProviderSaveRequest {
  provider_type: string
  name?: string
  endpoint?: string
  credentials?: Record<string, string>
  scenes?: string[]
  priority?: number
  status?: number
  is_default?: boolean
  remark?: string
}

export function getCaptchaProviderTypes(): Promise<CaptchaProviderType[]> {
  return request.get<CaptchaProviderType[]>({ url: '/captcha/providers/types' })
}

// 注意：管理端验证码接口直接返回数组（不经 items 包装），与消息中心渠道接口的 {items} 不同。
export function getCaptchaProviders(params?: {
  provider_type?: string
  status?: number
}): Promise<CaptchaProviderInfo[]> {
  return request.get<CaptchaProviderInfo[]>({ url: '/captcha/providers', params })
}

export function createCaptchaProvider(data: CaptchaProviderSaveRequest): Promise<CaptchaProviderInfo> {
  return request.post<CaptchaProviderInfo>({ url: '/captcha/providers', data })
}

export function updateCaptchaProvider(
  id: number,
  data: CaptchaProviderSaveRequest,
): Promise<CaptchaProviderInfo> {
  return request.put<CaptchaProviderInfo>({ url: `/captcha/providers/${id}`, data })
}

export function deleteCaptchaProvider(id: number): Promise<void> {
  return request.delete<void>({ url: `/captcha/providers/${id}` })
}

export interface CaptchaProviderTestResponse {
  ok: boolean
  /** true = 适配器待接入（不是失败） */
  pending?: boolean
  message: string
}

// 测试接口只回 message（成功/失败靠 code 区分），失败时统一抛错由调用方捕获。
export function testCaptchaProvider(id: number, target?: string): Promise<string> {
  return request.post<string>({ url: `/captcha/providers/${id}/test`, data: { target: target || '' } })
}

// ---------- 场景策略 ----------

export interface CaptchaPolicyInfo {
  scene: string
  name: string
  image_required: boolean
  image_provider_id: number
  image_level: string
  otp_required: boolean
  otp_channel: string
  min_channel_level: number
  user_can_tighten: boolean
  user_can_choose_channel: boolean
  max_attempts: number
  ttl_seconds: number
  send_interval_seconds: number
  daily_limit_per_target: number
  status: string
  remark: string
  /** 图形码或 OTP 开启时置位：开启后用户无法关闭 */
  platform_forced: boolean
}

export interface CaptchaPolicyUpdateRequest {
  image_required?: boolean
  image_provider_id?: number
  image_level?: string
  otp_required?: boolean
  otp_channel?: string
  min_channel_level?: number
  user_can_tighten?: boolean
  user_can_choose_channel?: boolean
  max_attempts?: number
  ttl_seconds?: number
  send_interval_seconds?: number
  daily_limit_per_target?: number
  status?: string
  remark?: string
}

export function getCaptchaPolicies(): Promise<CaptchaPolicyInfo[]> {
  return request.get<CaptchaPolicyInfo[]>({ url: '/captcha/policies' })
}

export function updateCaptchaPolicy(
  scene: string,
  data: CaptchaPolicyUpdateRequest,
): Promise<CaptchaPolicyInfo> {
  return request.put<CaptchaPolicyInfo>({ url: `/captcha/policies/${scene}`, data })
}

// ---------- 统计 ----------

export interface CaptchaSceneStat {
  scene: string
  total: number
  used: number
  failed: number
  expired: number
  /** 通过率（0-1） */
  pass_rate: number
  cost_fen: number
  cost_yuan: number
}

export interface CaptchaStatsResponse {
  from: string
  to: string
  items: CaptchaSceneStat[]
}

// 统计时间区间：后端读 from / to（RFC3339 或 YYYY-MM-DD HH:mm:ss），与列表页的 start_time 命名不同。
export function getCaptchaStats(params: { from?: string; to?: string }): Promise<CaptchaStatsResponse> {
  return request.get<CaptchaStatsResponse>({ url: '/captcha/stats', params })
}
