import { request } from '@/utils/request'

export interface AdminListQuery {
  page?: number
  page_size?: number
  role?: string
  status?: string
  keyword?: string
}

export interface AdminInfo {
  id: number
  username: string
  email: string
  avatar?: string
  role: string
  department?: string
  status: string
  last_login_ip?: string
  last_login_at?: string
  created_at: string
}

export interface AdminListMeta {
  page: number
  page_size: number
  total: number
}

export interface AdminListResponse {
  items: AdminInfo[]
  meta: AdminListMeta
}

export interface AdminCreateRequest {
  username: string
  email: string
  password: string
  role?: string
  department?: string
  status?: string
}

export interface AdminUpdateRequest {
  email: string
  role: string
  department?: string
  status: string
}

export interface AdminStatusRequest {
  status: string
}

export interface AdminResetPasswordRequest {
  password: string
}

export function getAdminList(params: AdminListQuery): Promise<AdminListResponse> {
  return request.get<AdminListResponse>({
    url: '/auth/admins',
    params: {
      page: params.page,
      page_size: params.page_size,
      role: params.role,
      status: params.status,
      keyword: params.keyword,
    },
  })
}

export function getAdminDetail(id: string | number): Promise<AdminInfo> {
  return request.get<AdminInfo>({
    url: `/auth/admins/${id}`,
  })
}

export function createAdmin(data: AdminCreateRequest): Promise<AdminInfo> {
  return request.post<AdminInfo>({
    url: '/auth/admins',
    data,
  })
}

export function updateAdmin(id: string | number, data: AdminUpdateRequest): Promise<AdminInfo> {
  return request.put<AdminInfo>({
    url: `/auth/admins/${id}`,
    data,
  })
}

export function updateAdminStatus(id: string | number, data: AdminStatusRequest): Promise<string> {
  return request.patch<string>({
    url: `/auth/admins/${id}/status`,
    data,
  })
}

export function resetAdminPassword(id: string | number, data: AdminResetPasswordRequest): Promise<string> {
  return request.post<string>({
    url: `/auth/admins/${id}/reset-password`,
    data,
  })
}

export function deleteAdmin(id: string | number): Promise<string> {
  return request.delete<string>({
    url: `/auth/admins/${id}`,
  })
}
