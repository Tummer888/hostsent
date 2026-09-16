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
  avatar?: string
  /** 用户分层：free / pro … */
  tier?: string
  last_login_ip?: string
  last_login_ip_region?: string
  oauth_provider?: string
  oauth_providers?: string[]
  oauth_openid?: string
  balance?: number
  total_consume_amount?: number
  status: string
  /** 手机/邮箱验证时间；空表示未验证（迁移 042 落列，此前未接出） */
  phone_verified_at?: string | null
  email_verified_at?: string | null
  /** 推广邀请：邀请码、邀请人 ID 与用户名、绑定时间 */
  invite_code?: string | null
  inviter_user_id?: number | null
  inviter_name?: string
  invited_at?: string | null
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

/**
 * 用户资料部分更新请求：字段全部可选，缺省 = 不修改，显式传空串 = 清空。
 * 手机/邮箱非必填（库中 14/40 个用户无手机号，标必填会让这些账号无法保存）。
 */
export interface UserUpdateRequest {
  username?: string
  real_name?: string
  phone?: string
  email?: string
  region?: string
  sub_account_remark?: string
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

/** 实例摘要（聚合返回，字段对齐 instances 权威表） */
export interface UserInstanceBrief {
  id: number
  instance_id: string
  name: string
  region: string
  zone: string
  cpu: number
  memory: number
  disk: number
  os: string
  public_ip: string
  status: string
  billing_mode: string
  lifecycle_stage: string
  order_id: number | null
  source_mode: string
  /** null 表示未设置到期时间，前端显示「未设置」而非 1970-01-01 */
  expire_at: string | null
  created_at: string
}

/** 订单摘要（权威表 orders） */
export interface UserOrderBrief {
  id: number
  order_no: string
  product_name: string
  final_amount: number
  status: string
  pay_method: string
  renewal_id: number
  created_at: string
  paid_at: string | null
}

/** 账单摘要（权威表 bills） */
export interface UserBillBrief {
  id: number
  bill_no: string
  billing_month: string
  amount: number
  bill_type: string
  status: string
  created_at: string
}

/** 资金流水摘要（权威表 wallet_transactions） */
export interface UserTransactionBrief {
  id: number
  txn_no: string
  type: string
  direction: number
  amount: number
  balance_after: number
  remark: string
  created_at: string
}

/** 工单摘要（权威表 tickets） */
export interface UserTicketBrief {
  id: number
  ticket_no: string
  title: string
  category: string
  priority: string
  status: string
  updated_at: string
  created_at: string
}

/** 用户绑定的后台角色（roles.scope='admin'），与客户侧权限语义不同 */
export interface UserRoleBrief {
  id: number
  code: string
  name: string
  scope: string
}

/** 详情页各域计数，供 Tab 徽标与统计卡使用 */
export interface UserDetailSummary {
  instance_count: number
  running_instance_count: number
  order_count: number
  order_total_amount: number
  bill_count: number
  unpaid_bill_count: number
  transaction_count: number
  ticket_count: number
  open_ticket_count: number
  login_count: number
  active_session_count: number
  risk_event_count: number
  operation_log_count: number
  verification_count: number
}

/**
 * 用户详情聚合响应。
 *
 * 各业务域的完整列表由既有分页接口承担（/orders、/tickets、/instances、
 * /finance/bills …），聚合只给「资料 + 计数 + 近期若干条」。
 * degraded 记录采集失败的段名，前端据此在对应 Tab 显示「数据暂不可用」。
 */
export interface UserDetailAggregateResponse {
  profile: UserInfo
  rbac_roles: UserRoleBrief[]
  /** 客户侧权限码（sub_account_permissions，固定枚举） */
  permissions: string[]
  summary: UserDetailSummary
  recent_instances: UserInstanceBrief[]
  recent_orders: UserOrderBrief[]
  recent_bills: UserBillBrief[]
  recent_transactions: UserTransactionBrief[]
  recent_tickets: UserTicketBrief[]
  degraded: string[]
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
      sales_admin_id: params.sales_admin_id,
      unassigned_sales: params.unassigned_sales,
    },
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
