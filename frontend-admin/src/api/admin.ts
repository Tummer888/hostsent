import { request } from '@/utils/request'

import type {
  CreateSyncTaskRequest,
  InstanceInfo,
  InstanceListQuery,
  InstanceListResponse,
  PoolInfo,
  PoolListQuery,
  PoolListResponse,
  PriceChangeHandleRequest,
  PriceChangeInfo,
  PriceChangeListQuery,
  PriceChangeListResponse,
  ProductInfo,
  ProductListQuery,
  ProductListResponse,
  ProductPriceRequest,
  ProviderCreateRequest,
  ProviderInfo,
  ProviderListQuery,
  ProviderListResponse,
  ProviderTypeItem,
  ProviderUpdateRequest,
  SyncDiffInfo,
  SyncDiffListQuery,
  SyncDiffListResponse,
  SyncDiffSummaryResponse,
  SyncLogInfo,
  SyncLogListQuery,
  SyncLogListResponse,
  SyncScheduleInfo,
  SyncScheduleListQuery,
  SyncScheduleListResponse,
  SyncScheduleUpdateRequest,
  SyncScopeMeta,
  SyncTaskInfo,
  SyncTaskListQuery,
  SyncTaskListResponse,
  TaskQueueCategoryCount,
  TaskQueueListQuery,
  TaskQueueListResponse,
  TestConnectionResult,
} from '@/types/interface'

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
  roles?: string[]
  department?: string
  position?: string
  service_group_id?: number
  must_change_password?: boolean
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
  role_ids?: number[]
  department?: string
  position?: string
  status?: string
}

export interface AdminUpdateRequest {
  email: string
  role: string
  role_ids?: number[]
  department?: string
  position?: string
  status: string
}

export interface AdminStatusRequest {
  status: string
}

export interface AdminResetPasswordRequest {
  password: string
}

// ===== 员工管理（P2-01，新路径 /staff，超管独占） =====

export function getAdminList(params: AdminListQuery): Promise<AdminListResponse> {
  return request.get<AdminListResponse>({
    url: '/staff',
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
    url: `/staff/${id}`,
  })
}

export function createAdmin(data: AdminCreateRequest): Promise<AdminInfo> {
  return request.post<AdminInfo>({
    url: '/staff',
    data,
  })
}

export function updateAdmin(id: string | number, data: AdminUpdateRequest): Promise<AdminInfo> {
  return request.put<AdminInfo>({
    url: `/staff/${id}`,
    data,
  })
}

/** 覆盖式设置员工角色（写 admin_roles） */
export function setAdminRoles(id: string | number, roleIDs: number[]): Promise<string> {
  return request.put<string>({
    url: `/staff/${id}/roles`,
    data: { role_ids: roleIDs },
  })
}

export function updateAdminStatus(id: string | number, data: AdminStatusRequest): Promise<string> {
  return request.patch<string>({
    url: `/staff/${id}/status`,
    data,
  })
}

export function resetAdminPassword(id: string | number, data: AdminResetPasswordRequest): Promise<string> {
  return request.post<string>({
    url: `/staff/${id}/reset-password`,
    data,
  })
}

export function deleteAdmin(id: string | number): Promise<string> {
  return request.delete<string>({
    url: `/staff/${id}`,
  })
}

// ===== 管理端操作审计（P2-06） =====

export interface AdminAuditLogQuery {
  page?: number
  page_size?: number
  admin_id?: number
  resource_type?: string
  action?: string
  keyword?: string
  start_time?: string
  end_time?: string
}

export interface AdminAuditLogInfo {
  id: number
  admin_id: number
  admin_name: string
  module: string
  resource_type: string
  resource_id: string
  action: string
  request_method: string
  request_path: string
  response_code: number
  ip: string
  user_agent: string
  detail: string
  created_at: string
}

export interface AdminAuditLogResponse {
  items: AdminAuditLogInfo[]
  meta: AdminListMeta
}

/** 查询管理端操作审计日志 */
export function getAdminAuditLogs(params: AdminAuditLogQuery): Promise<AdminAuditLogResponse> {
  return request.get<AdminAuditLogResponse>({
    url: '/audit-logs',
    params: {
      page: params.page,
      page_size: params.page_size,
      admin_id: params.admin_id,
      resource_type: params.resource_type,
      action: params.action,
      keyword: params.keyword,
      start_time: params.start_time,
      end_time: params.end_time,
    },
  })
}

// ===== 上游提供商 =====

export function getProviderList(params: ProviderListQuery): Promise<ProviderListResponse> {
  return request.get<ProviderListResponse>({
    url: '/resource/providers',
    params: {
      page: params.page,
      page_size: params.page_size,
      keyword: params.keyword,
      provider_type: params.provider_type,
      kind: params.kind,
      status: params.status,
    },
  })
}

export function getProviderDetail(id: number): Promise<ProviderInfo> {
  return request.get<ProviderInfo>({
    url: `/resource/providers/${id}`,
  })
}

export function createProvider(data: ProviderCreateRequest): Promise<ProviderInfo> {
  return request.post<ProviderInfo>({
    url: '/resource/providers',
    data,
  })
}

export function updateProvider(id: number, data: ProviderUpdateRequest): Promise<ProviderInfo> {
  return request.put<ProviderInfo>({
    url: `/resource/providers/${id}`,
    data,
  })
}

export function deleteProvider(id: number): Promise<string> {
  return request.delete<string>({
    url: `/resource/providers/${id}`,
  })
}

export function testConnection(id: number): Promise<TestConnectionResult> {
  return request.post<TestConnectionResult>({
    url: `/resource/providers/${id}/test`,
  })
}

// 恢复渠道同步：解除熔断（P0/T0.3 后台一键恢复）
export function resumeProviderSync(id: number): Promise<ProviderInfo> {
  return request.post<ProviderInfo>({
    url: `/resource/providers/${id}/sync/resume`,
  })
}

export function getProviderTypes(): Promise<ProviderTypeItem[]> {
  return request.get<ProviderTypeItem[]>({
    url: '/resource/providers/types',
  })
}

// ===== 资源池 =====

export function getPoolList(params: PoolListQuery): Promise<PoolListResponse> {
  return request.get<PoolListResponse>({
    url: '/resource/pools',
    params: {
      provider_id: params.provider_id,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getPoolDetail(id: number): Promise<PoolInfo> {
  return request.get<PoolInfo>({
    url: `/resource/pools/${id}`,
  })
}

// ===== 上游商品 =====

export function getProductList(params: ProductListQuery): Promise<ProductListResponse> {
  return request.get<ProductListResponse>({
    url: '/resource/products',
    params: {
      keyword: params.keyword,
      provider_id: params.provider_id,
      status: params.status,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getProductDetail(id: number): Promise<ProductInfo> {
  return request.get<ProductInfo>({
    url: `/resource/products/${id}`,
  })
}

export function updateProductPrice(id: number, data: ProductPriceRequest): Promise<ProductInfo> {
  return request.put<ProductInfo>({
    url: `/resource/products/${id}/price`,
    data,
  })
}

// ===== 同步任务 / 日志 / 实例 =====

export function getSyncTaskList(params: SyncTaskListQuery): Promise<SyncTaskListResponse> {
  return request.get<SyncTaskListResponse>({
    url: '/resource/sync/tasks',
    params: {
      provider_id: params.provider_id,
      task_type: params.task_type,
      status: params.status,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getSyncTaskDetail(id: number): Promise<SyncTaskInfo> {
  return request.get<SyncTaskInfo>({
    url: `/resource/sync/tasks/${id}`,
  })
}

export function createSyncTask(data: CreateSyncTaskRequest): Promise<SyncTaskInfo> {
  return request.post<SyncTaskInfo>({
    url: '/resource/sync',
    data,
  })
}

export function getSyncLogList(params: SyncLogListQuery): Promise<SyncLogListResponse> {
  return request.get<SyncLogListResponse>({
    url: '/resource/sync/logs',
    params: {
      provider_id: params.provider_id,
      task_id: params.task_id,
      status: params.status,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getInstanceList(params: InstanceListQuery): Promise<InstanceListResponse> {
  return request.get<InstanceListResponse>({
    url: '/resource/instances',
    params: {
      provider_id: params.provider_id,
      user_id: params.user_id,
      status: params.status,
      keyword: params.keyword,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getInstanceDetail(id: number): Promise<InstanceInfo> {
  return request.get<InstanceInfo>({
    url: `/resource/instances/${id}`,
  })
}

// ===== 同步框架（P3/T3.6）：调度 / 调价事件 / 差异 =====

export function getSyncScopeMeta(): Promise<SyncScopeMeta[]> {
  return request.get<SyncScopeMeta[]>({
    url: '/resource/sync/scopes',
  })
}

export function getSyncScheduleList(params: SyncScheduleListQuery): Promise<SyncScheduleListResponse> {
  return request.get<SyncScheduleListResponse>({
    url: '/resource/sync/schedules',
    params: {
      provider_id: params.provider_id,
      enabled: params.enabled,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function updateSyncSchedule(id: number, data: SyncScheduleUpdateRequest): Promise<SyncScheduleInfo> {
  return request.put<SyncScheduleInfo>({
    url: `/resource/sync/schedules/${id}`,
    data,
  })
}

export function getPriceChangeList(params: PriceChangeListQuery): Promise<PriceChangeListResponse> {
  return request.get<PriceChangeListResponse>({
    url: '/resource/sync/price-changes',
    params: {
      provider_id: params.provider_id,
      status: params.status,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function handlePriceChanges(data: PriceChangeHandleRequest): Promise<{ handled: number }> {
  return request.post<{ handled: number }>({
    url: '/resource/sync/price-changes/handle',
    data,
  })
}

export function getSyncDiffList(params: SyncDiffListQuery): Promise<SyncDiffListResponse> {
  return request.get<SyncDiffListResponse>({
    url: '/resource/sync/diffs',
    params: {
      provider_id: params.provider_id,
      task_id: params.task_id,
      scope: params.scope,
      action: params.action,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getSyncDiffSummary(params?: { provider_id?: number; days?: number }): Promise<SyncDiffSummaryResponse> {
  return request.get<SyncDiffSummaryResponse>({
    url: '/resource/sync/diffs/summary',
    params: {
      provider_id: params?.provider_id,
      days: params?.days,
    },
  })
}

// ===== 任务队列（运维 · 平台动作是否到达上游） =====

export function getTaskQueueList(params: TaskQueueListQuery): Promise<TaskQueueListResponse> {
  return request.get<TaskQueueListResponse>({
    url: '/resource/task-queue',
    params: {
      category: params.category,
      status: params.status,
      upstream_state: params.upstream_state,
      provider_id: params.provider_id,
      keyword: params.keyword,
      created_from: params.created_from,
      created_to: params.created_to,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getTaskQueueCategories(): Promise<TaskQueueCategoryCount[]> {
  return request.get<TaskQueueCategoryCount[]>({
    url: '/resource/task-queue/categories',
  })
}
