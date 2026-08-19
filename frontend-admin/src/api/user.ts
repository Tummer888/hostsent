import { request } from '@/utils/request'

export interface UserListQuery {
  page?: number
  page_size?: number
  status?: string
  filter?: string
  region?: string
  keyword?: string
}

export interface UserInfo {
  id: number
  username: string
  real_name: string
  role: string
  roles: string[]
  email: string
  phone: string
  region: string
  balance: number
  status: string
  created_at: string
  last_login_at?: string
}

export interface UserPermissionItem {
  id: number
  name: string
  code: string
  type: string
  path?: string
}

export interface UserInstanceItem {
  id: number
  name: string
  region: string
  specs: string
  status: string
  expire_at: string
}

export interface UserOrderItem {
  id: number
  order_no: string
  product: string
  amount: number
  status: string
  created_at: string
}

export interface UserBillItem {
  id: number
  billing_month: string
  amount: number
  status: string
}

export interface UserTransactionItem {
  id: number
  txn_no: string
  type: string
  amount: number
  created_at: string
}

export interface UserTicketItem {
  id: number
  ticket_no: string
  title: string
  category: string
  priority: string
  status: string
  updated_at: string
}

export interface UserDetailAggregateResponse {
  profile: UserInfo
  permissions: UserPermissionItem[]
  instances: UserInstanceItem[]
  orders: UserOrderItem[]
  bills: UserBillItem[]
  transactions: UserTransactionItem[]
  tickets: UserTicketItem[]
}

export interface UserUpdateRequest {
  username: string
  email: string
  phone: string
  status: string
}

export interface UserListMeta {
  page: number
  page_size: number
  total: number
}

export interface UserListResponse {
  items: UserInfo[]
  meta: UserListMeta
}

export interface UserStatsResponse {
  total: number
  today_new: number
  active: number
  disabled: number
  pending_real_name: number
  pending_review: number
}

export interface UserGroupListQuery {
  page?: number
  page_size?: number
  status?: string
  keyword?: string
}

export interface UserGroupInfo {
  id: number
  name: string
  code: string
  description: string
  status: string
  sort_order: number
  created_at: string
  updated_at: string
}

export interface UserGroupRequest {
  name: string
  code: string
  description: string
  status: string
  sort_order: number
}

export interface UserGroupListResponse {
  items: UserGroupInfo[]
  meta: UserListMeta
}

export interface RegionStatItem {
  region: string
  count: number
}

export interface RegionStatsResponse {
  items: RegionStatItem[]
  total: number
}

export function getUserList(params: UserListQuery): Promise<UserListResponse> {
  return request.get<UserListResponse>({
    url: '/users',
    params: {
      page: params.page,
      page_size: params.page_size,
      status: params.status,
      filter: params.filter,
      region: params.region,
      keyword: params.keyword,
    },
  })
}

export function getUserDetail(id: string | number): Promise<UserInfo> {
  return request.get<UserInfo>({
    url: `/users/${id}`,
  })
}

export function getUserDetailAggregate(id: string | number): Promise<UserDetailAggregateResponse> {
  return request.get<UserDetailAggregateResponse>({
    url: `/users/${id}/detail-aggregate`,
  })
}

export function updateUserDetail(id: string | number, data: UserUpdateRequest): Promise<UserInfo> {
  return request.put<UserInfo>({
    url: `/users/${id}`,
    data,
  })
}

export function getUserStats(): Promise<UserStatsResponse> {
  return request.get<UserStatsResponse>({
    url: '/users/stats',
  })
}

export function getRegionStats(): Promise<RegionStatsResponse> {
  return request.get<RegionStatsResponse>({
    url: '/users/region-stats',
  })
}

export function getUserGroupList(params: UserGroupListQuery): Promise<UserGroupListResponse> {
  return request.get<UserGroupListResponse>({
    url: '/user-groups',
    params: {
      page: params.page,
      page_size: params.page_size,
      status: params.status,
      keyword: params.keyword,
    },
  })
}

export function getUserGroupDetail(id: string | number): Promise<UserGroupInfo> {
  return request.get<UserGroupInfo>({
    url: `/user-groups/${id}`,
  })
}

export function createUserGroup(data: UserGroupRequest): Promise<UserGroupInfo> {
  return request.post<UserGroupInfo>({
    url: '/user-groups',
    data,
  })
}

export function updateUserGroup(id: string | number, data: UserGroupRequest): Promise<UserGroupInfo> {
  return request.put<UserGroupInfo>({
    url: `/user-groups/${id}`,
    data,
  })
}

export function deleteUserGroup(id: string | number): Promise<string> {
  return request.delete<string>({
    url: `/user-groups/${id}`,
  })
}
