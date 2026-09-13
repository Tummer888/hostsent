import { request } from '@/utils/request'

export interface LoginRequest {
  username: string
  password: string
  captcha_key?: string
  captcha_code?: string
}

export interface AuthUserInfo {
  id: number
  username: string
  role: string
  roles?: string[]
  email: string
  phone?: string
  status: string
  avatar?: string
  department?: string
  position?: string
  last_login_ip?: string
  service_group_id?: number
  must_change_password?: boolean
  permissions?: string[]
}

export interface LoginResponse {
  token: string
  user_info: AuthUserInfo
  permissions: string[]
  roles: string[]
  menus: string[]
  must_change_password: boolean
  // —— 登录二次验证（doc91 §5.2）——
  // need_otp=true 时本次不返回 token，前端须引导用户完成 OTP 验证。
  need_otp?: boolean
  otp_token?: string
  otp_channel?: string
  otp_target_masked?: string
  otp_expire_in?: number
}

export interface ChangePasswordRequest {
  old_password: string
  new_password: string
}

export function login(data: LoginRequest): Promise<LoginResponse> {
  return request.post<LoginResponse>({
    url: '/auth/login',
    data,
  })
}

/** 登录二次验证：凭 otp_token + 验证码换正式令牌（doc91 §5.2）。 */
export function verifyLoginOTP(data: { otp_token: string; code: string }): Promise<LoginResponse> {
  return request.post<LoginResponse>({
    url: '/auth/login/verify-otp',
    data,
  })
}

export function getCurrentUser(): Promise<AuthUserInfo> {
  return request.get<AuthUserInfo>({
    url: '/auth/me',
  })
}

// 管理员自助改密（首次登录强制改密也走此接口）。
export function changePassword(data: ChangePasswordRequest): Promise<void> {
  return request.post<void>({
    url: '/auth/change-password',
    data,
  })
}
