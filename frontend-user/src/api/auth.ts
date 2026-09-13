import request from '@/utils/request'

/** 登录方式（doc91 §5.3）。 */
export type LoginType = 'password' | 'sms' | 'email'

export interface LoginParams {
  username?: string
  password?: string
  login_type?: LoginType
  phone?: string
  email?: string
  /** 短信/邮箱登录的一次性验证码 */
  code?: string
  captcha_key?: string
  captcha_code?: string
}

export interface RegisterParams {
  username: string
  password: string
  email: string
  phone?: string
  invite_code?: string
  captcha_key?: string
  captcha_code?: string
  /** 注册邮箱验证码（user_register 场景 otp_required 时必填） */
  email_code?: string
}

export interface LoginResponse {
  token: string
  user: {
    id: number
    username: string
    name: string
    email: string
    phone: string
    avatar: string
    role: string
  }
  // —— 登录二次验证（doc91 §4.6）——
  // need_otp=true 时不返回 token，须用 otp_token 调 verifyLoginOTP 换正式令牌。
  need_otp?: boolean
  otp_token?: string
  otp_channel?: string
  otp_target_masked?: string
  otp_expire_in?: number
}

export function login(data: LoginParams) {
  return request.post<any, { data: LoginResponse }>('/auth/login', data, {
    _skipResultUnwrap: false,
  } as any)
}

/** 登录二次验证：凭 otp_token + 验证码换正式令牌（doc91 §4.6）。 */
export function verifyLoginOTP(data: { otp_token: string; code: string }) {
  return request.post<any, { data: LoginResponse }>('/auth/login/verify-otp', data)
}

export function register(data: RegisterParams) {
  return request.post<any, { data: { id: number } }>('/auth/register', data)
}

export function getUserInfo() {
  return request.get<any, { data: any }>('/auth/userinfo')
}

export interface UpdateProfileParams {
  name?: string
  email?: string
  phone?: string
  avatar?: string
}

export function updateProfile(data: UpdateProfileParams) {
  return request.put<any, { data: any }>('/auth/profile', data)
}

export interface ChangePasswordParams {
  old_password: string
  new_password: string
}

export function changePassword(data: ChangePasswordParams) {
  return request.put<any, void>('/auth/password', data)
}

export function logout() {
  return request.post<any, void>('/auth/logout')
}

// ---------- 忘记密码（doc91 §5.5）：两步 —— 申请下发 OTP → 校验并重置 ----------

export interface ForgotPasswordParams {
  account: string
  captcha_key?: string
  captcha_code?: string
}

/** 申请重置：向账号绑定目标下发 OTP；无论账号是否存在都返回 sent=true（防枚举）。 */
export function forgotPassword(data: ForgotPasswordParams) {
  return request.post<any, { data: { sent: boolean } }>('/auth/forgot-password', data)
}

export interface ResetPasswordParams {
  account: string
  code: string
  new_password: string
}

/** 校验 OTP 并写入新密码（成功会撤销历史会话）。 */
export function resetPassword(data: ResetPasswordParams) {
  return request.post<any, void>('/auth/reset-password', data)
}
