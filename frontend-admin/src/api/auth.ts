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
  roles: string[]
  email: string
  phone: string
  status: string
}

export interface LoginResponse {
  token: string
  user_info: AuthUserInfo
  permissions: string[]
  menus: string[]
}

export function login(data: LoginRequest): Promise<LoginResponse> {
  return request.post<LoginResponse>({
    url: '/auth/login',
    data,
  })
}

export function getCurrentUser(): Promise<AuthUserInfo> {
  return request.get<AuthUserInfo>({
    url: '/auth/me',
  })
}
