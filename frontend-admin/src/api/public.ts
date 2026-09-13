// 公开接口（登录前使用，无需鉴权）：/api/v1/public/*（doc91 §3.3/§10.2）。
//
// 为什么单开一个 axios 实例：管理端 request 的 baseURL 是 /api/v1/admin 且带
// 401 跳登录逻辑。登录页调这些接口时还没有令牌，走那套拦截器会把 429/业务错误
// 当成会话过期末处理。这里只做「信封解包 + 抛错」，不做任何鉴权跳转。
import axios, { type AxiosInstance } from 'axios'

const instance: AxiosInstance = axios.create({
  baseURL: '/api/v1/public',
  timeout: 15_000,
  headers: { 'Content-Type': 'application/json; charset=utf-8' },
})

instance.interceptors.response.use(
  (response) => {
    const body = response.data as { code?: number; message?: string; data?: unknown }
    if (body && typeof body === 'object' && 'code' in body) {
      if (body.code === 0) return body.data as never
      return Promise.reject(new Error(body.message || `请求失败，错误码：${body.code}`))
    }
    return response.data as never
  },
  (error) => {
    const status = error?.response?.status
    const message = error?.response?.data?.message
    if (status === 429) {
      return Promise.reject(new Error(message || '操作过于频繁，请稍后再试'))
    }
    if (message) return Promise.reject(new Error(message))
    if (error?.code === 'ECONNABORTED') return Promise.reject(new Error('请求超时，请稍后重试'))
    return Promise.reject(new Error(error?.message || '网络异常，请稍后重试'))
  },
)

/** 图形码挑战响应（不含答案）。 */
export interface PublicImageChallenge {
  captcha_key: string
  /** data:image/png;base64,... 已是可直接放进 <img src> 的 Data URI */
  image_base64: string
  expires_in: number
  provider: string
  params?: Record<string, string>
}

/** 场景生效策略（前端据此决定是否渲染图形码）。 */
export interface PublicScenePolicy {
  image_required: boolean
  otp_required: boolean
  otp_channel?: string
}

export interface PublicAuthConfig {
  captcha_enabled: boolean
  scenes: Record<string, PublicScenePolicy>
  image_provider: string
  third_party: { provider: string; captcha_id: string }
}

/** 公开 OTP 下发结果（target 只回打码值）。 */
export interface PublicSendCodeResult {
  sent: boolean
  channel: string
  target_masked: string
  expire_in: number
  cooldown: number
}

/** 取图形码挑战；scene 决定用哪个场景的策略与服务商。 */
export function getImageChallenge(scene: string, level?: string): Promise<PublicImageChallenge> {
  return instance.get('/captcha/image', { params: { scene, level } }) as Promise<PublicImageChallenge>
}

/**
 * 拉取验证码总闸与各场景策略。
 *
 * 这是「后端要、前端没显示」死锁的唯一解法：前端不写死，完全按它渲染。
 */
export function getAuthConfig(): Promise<PublicAuthConfig> {
  return instance.get('/auth-config') as Promise<PublicAuthConfig>
}

/** 公开下发 OTP（登录前场景，服务端有场景白名单）。 */
export function sendVerifyCode(data: {
  scene: string
  channel: 'email' | 'sms'
  target?: string
  captcha_key?: string
  captcha_code?: string
}): Promise<PublicSendCodeResult> {
  return instance.post('/verify-code/send', data) as Promise<PublicSendCodeResult>
}
