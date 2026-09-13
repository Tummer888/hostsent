import request from '@/utils/request'

// 用户中心 · 安全设置与二次验证接口（doc91 §4.4/§4.5）。
// baseURL=/api/v1，路径拼为 /api/v1/uc/security/*。

/** 单场景生效策略（前端只渲染它，不渲染平台基线）。 */
export interface EffectiveScene {
  scene: string
  name: string
  image_required: boolean
  otp_required: boolean
  otp_channel: string
  min_channel_level: number
  /** 平台强制项：开关置灰 + 锁图标，用户无法关闭 */
  platform_forced: boolean
  user_can_tighten: boolean
  /** 被平台提回基线时的说明文案（例如「已按平台安全要求使用 短信 验证」） */
  notice: string
  /** platform / user_tightened */
  source: string
}

export interface SecuritySettings {
  mfa_enabled: boolean
  mfa_channel: string
  trust_window_minutes: number
  phone: string
  phone_masked: string
  phone_bound: boolean
  email: string
  email_masked: string
  email_bound: boolean
  scenes: EffectiveScene[]
  notice: string
}

/** 可选通道（低等级通道会被服务端过滤或回显为不可用）。 */
export interface ChannelOption {
  channel: string
  label: string
  level: number
  available: boolean
  reason?: string
}

/** 某场景的验证要求（关键操作前端据此弹框）。 */
export interface VerificationRequirement {
  scene: string
  image_required: boolean
  channel: string
  target_masked: string
  need_verification: boolean
  ticket_exists: boolean
  verify_ticket_ttl: number
  channels: ChannelOption[]
}

/** 只表达「加严」：没有关闭语义，平台强制项会被服务端忽略并回显真实值。 */
export interface SecuritySettingsUpdateRequest {
  mfa_enabled?: boolean
  mfa_channel?: string
  scene_overrides?: Record<string, { otp_required?: boolean; otp_channel?: string; image_required?: boolean }>
  trust_window_minutes?: number
}

export function getSecuritySettings() {
  return request.get<any, { data: SecuritySettings }>('/uc/security/settings')
}

export function updateSecuritySettings(data: SecuritySettingsUpdateRequest) {
  return request.put<any, { data: SecuritySettings }>('/uc/security/settings', data)
}

/** 已登录用户下发验证码：目标由服务端按账号绑定解析，前端传空即可。 */
export function sendSecurityVerification(data: {
  scene: string
  channel: 'email' | 'sms'
  captcha_key?: string
  captcha_code?: string
}) {
  return request.post<
    any,
    { data: { sent: boolean; channel: string; target_masked: string; expire_in: number; cooldown: number } }
  >('/uc/security/verification/send', data)
}

/** 校验验证码并换取关键操作票据（X-Verify-Ticket）。 */
export function verifySecurityCode(data: { scene: string; code: string }) {
  return request.post<any, { data: { verify_ticket: string; expire_in: number } }>(
    '/uc/security/verification/verify',
    data,
  )
}

/** 查询某场景的验证要求（无票据时弹框用）。 */
export function getVerificationRequirement(scene: string) {
  return request.get<any, { data: VerificationRequirement }>('/uc/security/verification/requirement', {
    params: { scene },
  })
}
