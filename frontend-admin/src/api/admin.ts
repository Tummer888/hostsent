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
  ProductUpstreamMarkupRequest,
  ProductUpstreamMarkupResponse,
  ProviderAssociations,
  ProviderCascadeResult,
  ProviderCreateRequest,
  ProviderInfo,
  ProviderListQuery,
  ProviderListResponse,
  ProviderTypeItem,
  PlatformResources,
  ProviderUpdateRequest,
  ReconcileFindingListResponse,
  ReconcileListQuery,
  ReconcileListResponse,
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
  // S1 员工体系：组织维度筛选
  department_id?: number
  staff_type?: string
  sales_enabled?: string
  is_resigned?: string
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
  // S1 员工体系
  real_name?: string
  phone?: string
  department_id?: number
  department_name?: string
  staff_type?: string
  sales_enabled?: boolean
  joined_at?: string
  resigned_at?: string
  is_resigned?: boolean
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
  // S1 员工体系
  real_name?: string
  phone?: string
  department_id?: number
  staff_type?: string
  sales_enabled?: boolean
}

export interface AdminUpdateRequest {
  email: string
  role: string
  role_ids?: number[]
  department?: string
  position?: string
  status: string
  // S1 员工体系
  real_name?: string
  phone?: string
  department_id?: number
  staff_type?: string
  sales_enabled?: boolean
}

// ===== 组织部门（S1 员工体系，doc86 §2.1） =====

export interface DepartmentListQuery {
  kind?: string
  status?: string
  /** flat=1 返回平铺列表，否则返回两级树 */
  flat?: number
  keyword?: string
}

export interface DepartmentInfo {
  id: number
  name: string
  code: string
  kind: string
  parent_id: number
  leader_admin_id: number
  leader_name?: string
  remark?: string
  sort_order: number
  status: string
  admin_count: number
  category_count: number
  children?: DepartmentInfo[]
  created_at: string
}

export interface DepartmentListResponse {
  items: DepartmentInfo[]
  tree: boolean
}

export interface DepartmentSaveRequest {
  name: string
  code: string
  kind?: string
  parent_id?: number
  leader_admin_id?: number
  remark?: string
  sort_order?: number
  status?: string
}

export interface AdminStatusRequest {
  status: string
}

export interface AdminResetPasswordRequest {
  password: string
}

// ===== 员工管理（P2-01，新路径 /staff） =====

export function getAdminList(params: AdminListQuery): Promise<AdminListResponse> {
  return request.get<AdminListResponse>({
    url: '/staff',
    params: {
      page: params.page,
      page_size: params.page_size,
      role: params.role,
      status: params.status,
      keyword: params.keyword,
      department_id: params.department_id,
      staff_type: params.staff_type,
      sales_enabled: params.sales_enabled,
      is_resigned: params.is_resigned,
    },
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

/**
 * 员工离职（S1）：置离职标记 + 禁用账号，并交待在途客户。
 * 已有提成/工单不受影响，历史记录保留可追溯。
 */
export function resignAdmin(id: string | number, data?: { reason?: string; transfer_to_admin_id?: number }): Promise<string> {
  return request.post<string>({
    url: `/staff/${id}/resign`,
    data: data ?? {},
  })
}

// ===== 组织部门（S1 员工体系） =====

/** 查询部门（默认两级树；flat=1 平铺） */
export function getDepartmentList(params?: DepartmentListQuery): Promise<DepartmentListResponse> {
  return request.get<DepartmentListResponse>({
    url: '/departments',
    params: {
      kind: params?.kind,
      status: params?.status,
      flat: params?.flat,
      keyword: params?.keyword,
    },
  })
}

export function createDepartment(data: DepartmentSaveRequest): Promise<DepartmentInfo> {
  return request.post<DepartmentInfo>({ url: '/departments', data })
}

export function updateDepartment(id: number, data: DepartmentSaveRequest): Promise<DepartmentInfo> {
  return request.put<DepartmentInfo>({ url: `/departments/${id}`, data })
}

/** 删除部门：有在职员工或工单分类引用时后端返回 409 拒绝 */
export function deleteDepartment(id: number): Promise<string> {
  return request.delete<string>({ url: `/departments/${id}` })
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

/** 平台可售资源目录（区域/节点/存储/镜像）：自营规格模板配置平台参数用 */
export function getProviderPlatformResources(id: number): Promise<PlatformResources> {
  return request.get<PlatformResources>({
    url: `/resource/providers/${id}/resources`,
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

/**
 * 删除资源池。
 *
 * 注意：池由平台同步 upsert，在线渠道的池会在下次同步重新出现；
 * 本接口用于清理演示/历史脏数据（界面上已就此提示）。
 */
export function deletePool(id: number): Promise<string> {
  return request.delete<string>({ url: `/resource/pools/${id}` })
}

/** 渠道关联数据（删除确认弹窗：列出该渠道下的商品与资源池）。 */
export function getProviderAssociations(id: number): Promise<ProviderAssociations> {
  return request.get<ProviderAssociations>({ url: `/resource/providers/${id}/associations` })
}

/** 连带删除渠道及其未被引用的商品与资源池。 */
export function deleteProviderCascade(id: number): Promise<ProviderCascadeResult> {
  return request.delete<ProviderCascadeResult>({ url: `/resource/providers/${id}/cascade` })
}

// ===== 上游商品 =====

export function getProductList(params: ProductListQuery): Promise<ProductListResponse> {
  return request.get<ProductListResponse>({
    url: '/resource/products',
    params: {
      keyword: params.keyword,
      provider_id: params.provider_id,
      group_id: params.group_id,
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

/**
 * 设定上游商品加价规则并重算已导入商品售价（「上游商品」页调价）。
 * 走商品域接口：规则落在已导入的销售商品上（upstream_markup_type/value），
 * 由商品服务按规则重算售价与周期价格矩阵，并写商品变更历史。
 */
export function applyUpstreamMarkup(
  data: ProductUpstreamMarkupRequest,
): Promise<ProductUpstreamMarkupResponse> {
  return request.post<ProductUpstreamMarkupResponse>({
    url: '/product/products/upstream-markup',
    data,
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

// ===== 实例对账（运维 · 本地实例 vs 上游） =====

export function getReconcileList(params: ReconcileListQuery): Promise<ReconcileListResponse> {
  return request.get<ReconcileListResponse>({
    url: '/resource/reconcile',
    params: {
      provider_id: params.provider_id,
      keyword: params.keyword,
      anomaly: params.anomaly,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

// 对账差异台账（doc120）：持久化的差异记录，可处置（确认/忽略）。
export function getReconcileFindings(
  params: { provider_id?: number; page?: number; page_size?: number },
): Promise<ReconcileFindingListResponse> {
  return request.get<ReconcileFindingListResponse>({
    url: '/resource/reconcile/findings',
    params: {
      provider_id: params.provider_id,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function updateReconcileFinding(id: number, body: { status: string; note?: string }): Promise<void> {
  return request.put<void>({
    url: `/resource/reconcile/findings/${id}`,
    data: body,
  })
}
