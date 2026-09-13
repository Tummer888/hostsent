import type { LoginResponse } from '@/api/auth'
import { request } from '@/utils/request'

export interface UserListQuery {
  page?: number
  page_size?: number
  status?: string
  filter?: string
  last_login_ip_region?: string
  keyword?: string
  user_level_id?: number
  /** 用户组筛选，0 或空表示不筛选 */
  user_group_id?: number
  /** 主账号/子账号筛选（P4-10）：'true' 仅子账号，'false' 仅主账号，空为全部 */
  is_sub_account?: string
  /** 归属销售筛选（doc86 §4.1.10），0 或空表示不筛选 */
  sales_admin_id?: number
  /** 仅看未归属销售的用户（doc86 §4.1.10），'true' 生效 */
  unassigned_sales?: string
}

export interface UserInfo {
  id: number
  username: string
  real_name?: string
  role?: string
  roles?: string[]
  email?: string
  phone?: string
  user_group_id?: number
  user_group_name?: string
  user_level_id?: number
  user_level_name?: string
  user_level_code?: string
  region?: string
  last_login_ip?: string
  last_login_ip_region?: string
  oauth_provider?: string
  oauth_providers?: string[]
  balance?: number
  total_consume_amount?: number
  status: string
  /** 子账号标识（P4-10） */
  is_sub_account?: boolean
  owner_user_id?: number
  owner_name?: string
  sub_account_remark?: string
  /** 归属销售（doc86 §4.1.10） */
  sales_admin_id?: number
  sales_admin_name?: string
  created_at: string
  last_login_at?: string
  updated_at?: string
}

/** 管理端成员（子账号）项（P4-10） */
export interface SubAccountMemberInfo {
  id: number
  username: string
  name?: string
  email?: string
  phone?: string
  remark?: string
  status: string
  permissions: string[]
  created_at: string
  last_login_at?: string
}

export interface SubAccountMemberListResponse {
  items: SubAccountMemberInfo[]
  total: number
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

export interface UserUpdateRequest {
  username?: string
  real_name?: string
  phone?: string
  email?: string
  region?: string
  status?: string
  /** 所属用户组：不传表示不修改，0 表示移出分组，其余为组 ID */
  user_group_id?: number
}

export interface UserCreateRequest {
  id?: number
  username: string
  email: string
  phone: string
  password: string
  status: string
  role_ids?: number[]
  user_group_id?: number
}

export interface UserStatusRequest {
  status: string
}

export interface ResetPasswordRequest {
  password: string
}

export interface AssignRolesRequest {
  role_ids: number[]
}

export interface AdminImpersonateRequest {
  user_id: number
}

export interface UserRechargeRequest {
  amount: number
  remark?: string
}

/** 用户总览统计响应 */
export interface UserStatsResponse {
  /** 总用户数 */
  total: number
  /** 今日新增 */
  today_new: number
  /** 活跃用户 */
  active: number
  /** 冻结用户 */
  disabled: number
  /** 待实名 */
  pending_real_name: number
  /** 待审核 */
  pending_review: number
  /** 用户总余额 */
  total_balance: number
  /** 已购用户数 */
  purchased_count: number
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

export interface RoleInfo {
  id: number
  name: string
  code: string
  status: string
  /** 角色域：admin（后台员工）/ user（客户），工单分类可提交角色据此过滤 */
  scope?: string
  description?: string
  created_at?: string
  updated_at?: string
}

export interface RoleRequest {
  name: string
  code: string
  status: string
  description?: string
}

export interface AssignPermissionsRequest {
  permission_ids: number[]
}

export interface PermissionNode {
  id: number
  parent_id: number
  name: string
  code: string
  type: string
  path?: string
  component?: string
  icon?: string
  sort_order?: number
  status?: string
  children?: PermissionNode[]
}

export interface PermissionRequest {
  parent_id?: number
  name: string
  code: string
  type: string
  path?: string
  component?: string
  icon?: string
  sort_order?: number
  status: string
}

export interface UserGroupListQuery {
  page?: number
  page_size?: number
  status?: string
  keyword?: string
  /** 组类型三态：'' 全部，'true' 仅代理组，'false' 仅普通组 */
  is_agent_group?: string
}

export interface UserGroupInfo {
  id: number
  name: string
  code: string
  status: string
  sort_order: number
  price_policy_id?: number
  is_default: boolean
  is_agent_group: boolean
  description?: string
  created_at: string
  updated_at: string
}

export interface UserGroupRequest {
  name: string
  code: string
  status: string
  sort_order: number
  price_policy_id?: number
  is_default?: boolean
  is_agent_group?: boolean
  description?: string
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

export interface UserLevelListQuery {
  page?: number
  page_size?: number
  status?: string
  keyword?: string
}

export interface UserLevelInfo {
  id: number
  name: string
  code: string
  weight: number
  status: string
  feature_flags: string
  upgrade_condition: string
  upgrade_threshold: number
  max_sub_accounts: number
  benefits?: string
  description?: string
  created_at: string
  updated_at: string
}

export interface UserLevelRequest {
  name: string
  code: string
  weight: number
  status: string
  feature_flags?: string
  upgrade_condition?: string
  upgrade_threshold?: number
  max_sub_accounts?: number
  benefits?: string
  description?: string
}

export interface UserLevelListResponse {
  items: UserLevelInfo[]
  meta: UserListMeta
}

export function getUserList(params: UserListQuery): Promise<UserListResponse> {
  return request.get<UserListResponse>({
    url: '/users',
    params: {
      page: params.page,
      page_size: params.page_size,
      status: params.status,
      filter: params.filter,
      last_login_ip_region: params.last_login_ip_region,
      keyword: params.keyword,
      user_level_id: params.user_level_id,
      user_group_id: params.user_group_id,
      is_sub_account: params.is_sub_account,
    },
  })
}

export function getUserDetail(id: string | number): Promise<UserInfo> {
  return request.get<UserInfo>({
    url: `/users/${id}`,
  })
}

/** 查询指定主账号名下的成员（子账号）及权限（P4-10） */
export function getUserMembers(id: string | number): Promise<SubAccountMemberListResponse> {
  return request.get<SubAccountMemberListResponse>({
    url: `/users/${id}/members`,
  })
}

export function createUser(data: UserCreateRequest): Promise<UserInfo> {
  return request.post<UserInfo>({
    url: '/users',
    data,
  })
}

export function updateUser(id: string | number, data: UserUpdateRequest): Promise<UserInfo> {
  return request.put<UserInfo>({
    url: `/users/${id}`,
    data,
  })
}

export function updateUserStatus(id: string | number, data: UserStatusRequest): Promise<string> {
  return request.patch<string>({
    url: `/users/${id}/status`,
    data,
  })
}

export function impersonateUser(data: AdminImpersonateRequest): Promise<LoginResponse> {
  return request.post<LoginResponse>({
    url: `/users/${data.user_id}/impersonate`,
    data: {},
  })
}

export function rechargeUser(id: string | number, data: UserRechargeRequest): Promise<string> {
  return request.post<string>({
    url: `/users/${id}/recharge`,
    data,
  })
}

export function resetUserPassword(id: string | number, data: ResetPasswordRequest): Promise<string> {
  return request.post<string>({
    url: `/users/${id}/reset-password`,
    data,
  })
}

export function assignUserRoles(id: string | number, data: AssignRolesRequest): Promise<string> {
  return request.post<string>({
    url: `/users/${id}/roles`,
    data,
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

export function getUserLevelList(params: UserLevelListQuery): Promise<UserLevelListResponse> {
  return request.get<UserLevelListResponse>({
    url: '/user-levels',
    params: {
      page: params.page,
      page_size: params.page_size,
      status: params.status,
      keyword: params.keyword,
    },
  })
}

export function createUserLevel(data: UserLevelRequest): Promise<UserLevelInfo> {
  return request.post<UserLevelInfo>({
    url: '/user-levels',
    data,
  })
}

export function updateUserLevel(id: string | number, data: UserLevelRequest): Promise<UserLevelInfo> {
  return request.put<UserLevelInfo>({
    url: `/user-levels/${id}`,
    data,
  })
}

export function deleteUserLevel(id: string | number): Promise<string> {
  return request.delete<string>({
    url: `/user-levels/${id}`,
  })
}

export function getRoleList(): Promise<RoleInfo[]> {
  return request.get<RoleInfo[]>({
    url: '/roles',
  })
}

export function getRoleDetail(id: string | number): Promise<RoleInfo> {
  return request.get<RoleInfo>({
    url: `/roles/${id}`,
  })
}

export function createRole(data: RoleRequest): Promise<RoleInfo> {
  return request.post<RoleInfo>({
    url: '/roles',
    data,
  })
}

export function updateRole(id: string | number, data: RoleRequest): Promise<RoleInfo> {
  return request.put<RoleInfo>({
    url: `/roles/${id}`,
    data,
  })
}

export function deleteRole(id: string | number): Promise<string> {
  return request.delete<string>({
    url: `/roles/${id}`,
  })
}

export function getRolePermissionIds(id: string | number): Promise<number[]> {
  return request.get<number[]>({
    url: `/roles/${id}/permissions`,
  })
}

export function assignRolePermissions(id: string | number, data: AssignPermissionsRequest): Promise<string> {
  return request.post<string>({
    url: `/roles/${id}/permissions`,
    data,
  })
}

export function getPermissionTree(): Promise<PermissionNode[]> {
  return request.get<PermissionNode[]>({
    url: '/permissions/tree',
  })
}

export function createPermission(data: PermissionRequest): Promise<PermissionNode> {
  return request.post<PermissionNode>({
    url: '/permissions',
    data,
  })
}

export function updatePermission(id: string | number, data: PermissionRequest): Promise<PermissionNode> {
  return request.put<PermissionNode>({
    url: `/permissions/${id}`,
    data,
  })
}

export function deletePermission(id: string | number): Promise<string> {
  return request.delete<string>({
    url: `/permissions/${id}`,
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
      is_agent_group: params.is_agent_group,
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

export interface AdminOrderCreateRequest {
  product_id: number
  billing_cycle?: string
  price?: number
  pay_mode?: 'balance' | 'create'
}

export interface AdminOrderBrief {
  id: number
  order_no: string
  product_name: string
  billing_cycle: string
  total_amount: number
  status: string
  pay_method: string
}

export function createUserOrder(id: string | number, data: AdminOrderCreateRequest): Promise<AdminOrderBrief> {
  return request.post<AdminOrderBrief>({
    url: `/users/${id}/orders`,
    data,
  })
}
