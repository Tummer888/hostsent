import request from '@/utils/request'

export interface LoginParams {
  username: string
  password: string
}

export interface RegisterParams {
  username: string
  password: string
  email: string
  phone?: string
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
}

export function login(data: LoginParams) {
  return request.post<any, { data: LoginResponse }>('/auth/login', data, {
    _skipResultUnwrap: false,
  } as any)
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
