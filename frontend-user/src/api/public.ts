// 公开接口（登录前使用，无需鉴权）：/api/v1/public/*（doc91 §3.3/§10.2）。
//
// 为什么要单独建一份而不是复用 '@/utils/request'：那个实例是统一的业务 request，
// 响应拦截器在 401/业务错误时会调 userStore.logout() 并跳 /login。登录页调这些
// 接口时本来就没有令牌，401 分支会把「验证码过期」这类正常业务错误变成
// 「登录已过期」的跳转。这里只做信封解包，不做任何鉴权副作用。
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
      const err = new Error(body.message || `请求失败，错误码：${body.code}`) as Error & { code?: number }
      err.code = body.code
      return Promise.reject(err)
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
  /** data:image/png;base64,... 可直接放进 <img src> */
  image_base64: string
  expires_in: number
  provider: string
  params?: Record<string, string>
}

/** 场景生效策略（前端据此决定是否渲染图形码/验证码行）。 */
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

/** 取图形码挑战。 */
export function getImageChallenge(scene: string, level?: string): Promise<PublicImageChallenge> {
  return instance.get('/captcha/image', { params: { scene, level } }) as Promise<PublicImageChallenge>
}

/** 拉取验证码总闸与各场景策略（前端「要不要渲染」的唯一依据）。 */
export function getAuthConfig(): Promise<PublicAuthConfig> {
  return instance.get('/auth-config') as Promise<PublicAuthConfig>
}

/** 公开下发 OTP（服务端有场景白名单：仅登录前场景）。 */
export function sendVerifyCode(data: {
  scene: string
  channel: 'email' | 'sms'
  target?: string
  captcha_key?: string
  captcha_code?: string
}): Promise<PublicSendCodeResult> {
  return instance.post('/verify-code/send', data) as Promise<PublicSendCodeResult>
}
