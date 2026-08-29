import { request } from '@/utils/request'

import type {
  CreateSyncTaskRequest,
  InstanceInfo,
  InstanceListQuery,
  InstanceListResponse,
  PoolInfo,
  PoolListQuery,
  PoolListResponse,
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
  SyncLogInfo,
  SyncLogListQuery,
  SyncLogListResponse,
  SyncTaskInfo,
  SyncTaskListQuery,
  SyncTaskListResponse,
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

// ===== 上游提供商 =====

export function getProviderList(params: ProviderListQuery): Promise<ProviderListResponse> {
  return request.get<ProviderListResponse>({
    url: '/upstream/providers',
    params: {
      page: params.page,
      page_size: params.page_size,
      keyword: params.keyword,
      provider_type: params.provider_type,
      status: params.status,
    },
  })
}

export function getProviderDetail(id: number): Promise<ProviderInfo> {
  return request.get<ProviderInfo>({
    url: `/upstream/providers/${id}`,
  })
}

export function createProvider(data: ProviderCreateRequest): Promise<ProviderInfo> {
  return request.post<ProviderInfo>({
    url: '/upstream/providers',
    data,
  })
}

export function updateProvider(id: number, data: ProviderUpdateRequest): Promise<ProviderInfo> {
  return request.put<ProviderInfo>({
    url: `/upstream/providers/${id}`,
    data,
  })
}

export function deleteProvider(id: number): Promise<string> {
  return request.delete<string>({
    url: `/upstream/providers/${id}`,
  })
}

export function testConnection(id: number): Promise<TestConnectionResult> {
  return request.post<TestConnectionResult>({
    url: `/upstream/providers/${id}/test`,
  })
}

export function getProviderTypes(): Promise<ProviderTypeItem[]> {
  return request.get<ProviderTypeItem[]>({
    url: '/upstream/providers/types',
  })
}

// ===== 资源池 =====

export function getPoolList(params: PoolListQuery): Promise<PoolListResponse> {
  return request.get<PoolListResponse>({
    url: '/upstream/pools',
    params: {
      provider_id: params.provider_id,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getPoolDetail(id: number): Promise<PoolInfo> {
  return request.get<PoolInfo>({
    url: `/upstream/pools/${id}`,
  })
}

// ===== 上游商品 =====

export function getProductList(params: ProductListQuery): Promise<ProductListResponse> {
  return request.get<ProductListResponse>({
    url: '/upstream/products',
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
    url: `/upstream/products/${id}`,
  })
}

export function updateProductPrice(id: number, data: ProductPriceRequest): Promise<ProductInfo> {
  return request.put<ProductInfo>({
    url: `/upstream/products/${id}/price`,
    data,
  })
}

// ===== 同步任务 / 日志 / 实例 =====

export function getSyncTaskList(params: SyncTaskListQuery): Promise<SyncTaskListResponse> {
  return request.get<SyncTaskListResponse>({
    url: '/upstream/sync/tasks',
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
    url: `/upstream/sync/tasks/${id}`,
  })
}

export function createSyncTask(data: CreateSyncTaskRequest): Promise<SyncTaskInfo> {
  return request.post<SyncTaskInfo>({
    url: '/upstream/sync',
    data,
  })
}

export function getSyncLogList(params: SyncLogListQuery): Promise<SyncLogListResponse> {
  return request.get<SyncLogListResponse>({
    url: '/upstream/sync/logs',
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
    url: '/upstream/instances',
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
    url: `/upstream/instances/${id}`,
  })
}
