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
  real_name?: string
  role?: string
  roles?: string[]
  phone?: string
  email?: string
  region?: string
  balance?: number
  status: string
  created_at: string
  last_login_at?: string
  updated_at?: string
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
}

export interface UserStatsResponse {
  total: number
  today_new: number
  active: number
  disabled: number
  pending_real_name: number
  pending_review: number
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
  status: string
  sort_order: number
  description?: string
  created_at: string
  updated_at: string
}

export interface UserGroupRequest {
  name: string
  code: string
  status: string
  sort_order: number
  description?: string
}

export interface UserGroupListResponse {
  items: UserGroupInfo[]
  meta: UserListMeta
}

export interface AgentLevelListQuery {
  page?: number
  page_size?: number
  status?: string
  keyword?: string
}

export interface AgentLevelInfo {
  id: number
  name: string
  code: string
  weight: number
  direct_commission_rate: number
  indirect_commission_rate: number
  renewal_commission_rate: number
  upgrade_reward_amount: number
  self_purchase_rebate_rate: number
  allow_manual_price: boolean
  allow_sub_agent: boolean
  max_sub_agent_depth: number
  status: string
  description?: string
  created_at: string
  updated_at: string
}

export interface AgentLevelRequest {
  name: string
  code: string
  weight: number
  direct_commission_rate: number
  indirect_commission_rate: number
  renewal_commission_rate: number
  upgrade_reward_amount: number
  self_purchase_rebate_rate: number
  allow_manual_price: boolean
  allow_sub_agent: boolean
  max_sub_agent_depth: number
  status: string
  description: string
}

export interface AgentLevelListResponse {
  items: AgentLevelInfo[]
  meta: UserListMeta
}

export interface AgentListQuery {
  page?: number
  page_size?: number
  status?: string
  agent_level_id?: number
  keyword?: string
}

export interface AgentInfo {
  id: number
  user_id: number
  username: string
  real_name?: string
  phone?: string
  email?: string
  region?: string
  agent_level_id: number
  agent_level_name: string
  inviter_agent_id?: number
  invite_code: string
  direct_user_count: number
  team_user_count: number
  total_commission_amount: number
  withdrawable_commission_amount: number
  balance: number
  status: string
  created_at: string
  updated_at: string
}

export interface AgentRequest {
  user_id: number
  agent_level_id: number
  inviter_agent_id?: number
  invite_code: string
  status: string
  remark?: string
}

export interface AgentListResponse {
  items: AgentInfo[]
  meta: UserListMeta
}

export interface SubordinateListQuery {
  page?: number
  page_size?: number
  agent_id?: number
  status?: string
  level_depth?: number
  keyword?: string
}

export interface SubordinateInfo {
  id: number
  agent_id: number
  agent_name: string
  user_id: number
  username: string
  real_name?: string
  phone?: string
  parent_agent_id?: number
  parent_agent_name?: string
  level_depth: number
  relation_path: string
  contribution_amount: number
  commission_amount: number
  status: string
  created_at: string
  updated_at: string
}

export interface SubordinateRequest {
  agent_id: number
  user_id: number
  level_depth: number
  relation_path: string
  contribution_amount: number
  commission_amount: number
  status: string
}

export interface SubordinateListResponse {
  items: SubordinateInfo[]
  meta: UserListMeta
}

export interface CommissionListQuery {
  page?: number
  page_size?: number
  agent_id?: number
  status?: string
  commission_type?: string
  keyword?: string
}

export interface CommissionInfo {
  id: number
  agent_id: number
  agent_name: string
  subordinate_id?: number
  subordinate_name?: string
  order_no: string
  source_type: string
  commission_type: string
  base_amount: number
  rate: number
  amount: number
  status: string
  freeze_until?: string
  settled_at?: string
  remark?: string
  created_at: string
  updated_at: string
}

export interface CommissionRequest {
  agent_id: number
  subordinate_id?: number
  order_no: string
  source_type: string
  commission_type: string
  base_amount: number
  rate: number
  amount: number
  status: string
  freeze_until?: string
  settled_at?: string
  remark?: string
}

export interface CommissionStatusActionRequest {
  freeze_until?: string
  remark?: string
}

export interface CommissionListResponse {
  items: CommissionInfo[]
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

export function getAgentLevelList(params: AgentLevelListQuery): Promise<AgentLevelListResponse> {
  return request.get<AgentLevelListResponse>({
    url: '/distribution/agent-levels',
    params: {
      page: params.page,
      page_size: params.page_size,
      status: params.status,
      keyword: params.keyword,
    },
  })
}

export function getAgentLevelDetail(id: string | number): Promise<AgentLevelInfo> {
  return request.get<AgentLevelInfo>({
    url: `/distribution/agent-levels/${id}`,
  })
}

export function createAgentLevel(data: AgentLevelRequest): Promise<AgentLevelInfo> {
  return request.post<AgentLevelInfo>({
    url: '/distribution/agent-levels',
    data,
  })
}

export function updateAgentLevel(id: string | number, data: AgentLevelRequest): Promise<AgentLevelInfo> {
  return request.put<AgentLevelInfo>({
    url: `/distribution/agent-levels/${id}`,
    data,
  })
}

export function deleteAgentLevel(id: string | number): Promise<string> {
  return request.delete<string>({
    url: `/distribution/agent-levels/${id}`,
  })
}

export function getAgentList(params: AgentListQuery): Promise<AgentListResponse> {
  return request.get<AgentListResponse>({
    url: '/distribution/agents',
    params: {
      page: params.page,
      page_size: params.page_size,
      status: params.status,
      agent_level_id: params.agent_level_id,
      keyword: params.keyword,
    },
  })
}

export function getAgentDetail(id: string | number): Promise<AgentInfo> {
  return request.get<AgentInfo>({
    url: `/distribution/agents/${id}`,
  })
}

export function createAgent(data: AgentRequest): Promise<AgentInfo> {
  return request.post<AgentInfo>({
    url: '/distribution/agents',
    data,
  })
}

export function updateAgent(id: string | number, data: AgentRequest): Promise<AgentInfo> {
  return request.put<AgentInfo>({
    url: `/distribution/agents/${id}`,
    data,
  })
}

export function deleteAgent(id: string | number): Promise<string> {
  return request.delete<string>({
    url: `/distribution/agents/${id}`,
  })
}

export function getSubordinateList(params: SubordinateListQuery): Promise<SubordinateListResponse> {
  return request.get<SubordinateListResponse>({
    url: '/distribution/subordinates',
    params: {
      page: params.page,
      page_size: params.page_size,
      agent_id: params.agent_id,
      status: params.status,
      level_depth: params.level_depth,
      keyword: params.keyword,
    },
  })
}

export function getSubordinateDetail(id: string | number): Promise<SubordinateInfo> {
  return request.get<SubordinateInfo>({
    url: `/distribution/subordinates/${id}`,
  })
}

export function createSubordinate(data: SubordinateRequest): Promise<SubordinateInfo> {
  return request.post<SubordinateInfo>({
    url: '/distribution/subordinates',
    data,
  })
}

export function updateSubordinate(id: string | number, data: SubordinateRequest): Promise<SubordinateInfo> {
  return request.put<SubordinateInfo>({
    url: `/distribution/subordinates/${id}`,
    data,
  })
}

export function deleteSubordinate(id: string | number): Promise<string> {
  return request.delete<string>({
    url: `/distribution/subordinates/${id}`,
  })
}

export function getCommissionList(params: CommissionListQuery): Promise<CommissionListResponse> {
  return request.get<CommissionListResponse>({
    url: '/distribution/commissions',
    params: {
      page: params.page,
      page_size: params.page_size,
      agent_id: params.agent_id,
      status: params.status,
      commission_type: params.commission_type,
      keyword: params.keyword,
    },
  })
}

export function getCommissionDetail(id: string | number): Promise<CommissionInfo> {
  return request.get<CommissionInfo>({
    url: `/distribution/commissions/${id}`,
  })
}

export function createCommission(data: CommissionRequest): Promise<CommissionInfo> {
  return request.post<CommissionInfo>({
    url: '/distribution/commissions',
    data,
  })
}

export function updateCommission(id: string | number, data: CommissionRequest): Promise<CommissionInfo> {
  return request.put<CommissionInfo>({
    url: `/distribution/commissions/${id}`,
    data,
  })
}

export function freezeCommission(id: string | number, data: CommissionStatusActionRequest): Promise<CommissionInfo> {
  return request.post<CommissionInfo>({
    url: `/distribution/commissions/${id}/freeze`,
    data,
  })
}

export function unfreezeCommission(id: string | number, data: CommissionStatusActionRequest = {}): Promise<CommissionInfo> {
  return request.post<CommissionInfo>({
    url: `/distribution/commissions/${id}/unfreeze`,
    data,
  })
}

export function cancelCommission(id: string | number, data: CommissionStatusActionRequest = {}): Promise<CommissionInfo> {
  return request.post<CommissionInfo>({
    url: `/distribution/commissions/${id}/cancel`,
    data,
  })
}

export function deleteCommission(id: string | number): Promise<string> {
  return request.delete<string>({
    url: `/distribution/commissions/${id}`,
  })
}
