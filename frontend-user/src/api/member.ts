import request from '@/utils/request'

export interface MemberInfo {
  id: number
  username: string
  name: string
  email: string
  phone: string
  remark: string
  status: string
  permissions: string[]
  created_at: string
  last_login_at?: string
}

export interface PermissionOption {
  code: string
  label: string
}

export interface MemberListQuery {
  page?: number
  page_size?: number
  status?: string
  keyword?: string
}

export interface MemberListResponse {
  items: MemberInfo[]
  meta: { page: number; page_size: number; total: number }
  max_sub_accounts: number
  permission_options: PermissionOption[]
}

export interface MemberCreateParams {
  username: string
  email: string
  phone?: string
  password?: string
  remark?: string
  permissions?: string[]
}

export interface MemberCreateResponse {
  member: MemberInfo
  /** 一次性密码：仅创建成功时返回一次，请主账号及时转达 */
  password: string
}

export interface MemberUpdateParams {
  remark?: string
  status?: string
}

export interface MemberOperationLog {
  id: number
  actor_user_id: number
  actor_name: string
  module: string
  action: string
  target: string
  detail: string
  ip: string
  created_at: string
}

export interface MemberOperationLogResponse {
  items: MemberOperationLog[]
  meta: { page: number; page_size: number; total: number }
}

export function getMemberList(params: MemberListQuery) {
  return request.get<any, { data: MemberListResponse }>('/uc/members', { params })
}

export function createMember(data: MemberCreateParams) {
  return request.post<any, { data: MemberCreateResponse }>('/uc/members', data)
}

export function updateMember(id: number, data: MemberUpdateParams) {
  return request.put<any, { data: MemberInfo }>(`/uc/members/${id}`, data)
}

export function setMemberPermissions(id: number, permissions: string[]) {
  return request.put<any, { data: MemberInfo }>(`/uc/members/${id}/permissions`, { permissions })
}

export function deleteMember(id: number) {
  return request.delete<any, { data: string }>(`/uc/members/${id}`)
}

export function getMemberLogs(id: number, params?: { page?: number; page_size?: number }) {
  return request.get<any, { data: MemberOperationLogResponse }>(`/uc/members/${id}/logs`, { params })
}
