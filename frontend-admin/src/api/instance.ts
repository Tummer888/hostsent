import { request } from '@/utils/request'

import type {
  InstanceAddIPRequest,
  InstanceAttachDiskRequest,
  InstanceBandwidthRequest,
  InstanceBatchActionRequest,
  InstanceBatchActionResponse,
  InstanceDestroyRequest,
  InstanceDetail,
  InstanceOpsListQuery,
  InstanceOpsListResponse,
  InstancePowerRequest,
  InstanceRelatedResponse,
  InstanceReinstallRequest,
  InstanceReinstallResult,
  InstanceRemarkUpdateRequest,
  InstanceRescueRequest,
  InstanceResizeRequest,
  InstanceSnapshotCreateRequest,
  InstanceSnapshotInfo,
  InstanceSnapshotRestoreRequest,
  InstanceStatsResponse,
  InstanceVNCResponse,
  OperationListQuery,
  OperationListResponse,
  OperationLogListResponse,
  OperationLogQuery,
} from '@/types/interface'

// ===== 实例运维台（跨用户实例运维） =====

export function getInstanceList(params: InstanceOpsListQuery): Promise<InstanceOpsListResponse> {
  return request.get<InstanceOpsListResponse>({
    url: '/instances',
    params: {
      keyword: params.keyword,
      user_keyword: params.user_keyword,
      user_id: params.user_id,
      provider_id: params.provider_id,
      status: params.status,
      source_mode: params.source_mode,
      expire_state: params.expire_state,
      expire_within_days: params.expire_within_days,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getInstanceStats(): Promise<InstanceStatsResponse> {
  return request.get<InstanceStatsResponse>({
    url: '/instances/stats',
  })
}

export function getInstanceDetail(id: number, live = false): Promise<InstanceDetail> {
  return request.get<InstanceDetail>({
    url: `/instances/${id}`,
    params: { live },
  })
}

export function getInstanceOperations(id: number, params: OperationListQuery): Promise<OperationListResponse> {
  return request.get<OperationListResponse>({
    url: `/instances/${id}/operations`,
    params: {
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getInstanceRelated(id: number): Promise<InstanceRelatedResponse> {
  return request.get<InstanceRelatedResponse>({
    url: `/instances/${id}/related`,
  })
}

export function syncInstance(id: number): Promise<InstanceDetail> {
  return request.post<InstanceDetail>({
    url: `/instances/${id}/sync`,
  })
}

export function powerInstance(id: number, action: string): Promise<string> {
  const data: InstancePowerRequest = { action }
  return request.post<string>({
    url: `/instances/${id}/power`,
    data,
  })
}

export function updateInstanceRemark(id: number, data: InstanceRemarkUpdateRequest): Promise<string> {
  return request.put<string>({
    url: `/instances/${id}/remark`,
    data,
  })
}

export function getInstanceVNC(id: number): Promise<InstanceVNCResponse> {
  return request.post<InstanceVNCResponse>({
    url: `/instances/${id}/vnc`,
  })
}

export function resizeInstance(id: number, data: InstanceResizeRequest): Promise<string> {
  return request.post<string>({
    url: `/instances/${id}/resize`,
    data,
  })
}

export function destroyInstance(id: number, data: InstanceDestroyRequest): Promise<string> {
  return request.delete<string>({
    url: `/instances/${id}`,
    data,
  })
}

export function suspendInstance(id: number, reason?: string): Promise<string> {
  return request.post<string>({
    url: `/instances/${id}/suspend`,
    data: { reason },
  })
}

export function unsuspendInstance(id: number): Promise<string> {
  return request.post<string>({
    url: `/instances/${id}/unsuspend`,
  })
}

// 全局操作流水（跨实例审计视图）
export function getInstanceOperationLogs(params: OperationLogQuery): Promise<OperationLogListResponse> {
  return request.get<OperationLogListResponse>({
    url: '/instances/operations',
    params: { ...params },
  })
}

// 批量运维（开机/关机/重启/同步/暂停/恢复），返回逐台结果
export function batchInstanceAction(data: InstanceBatchActionRequest): Promise<InstanceBatchActionResponse> {
  return request.post<InstanceBatchActionResponse>({
    url: '/instances/batch',
    data,
  })
}

// ---- 维护类操作（平台接口实测存在：重装/重置密码/救援/快照/硬件）----

/** 重装系统；返回平台新签发的初始凭据（只此一次，页面必须提示立即保存）。 */
export function reinstallInstance(id: number, data: InstanceReinstallRequest): Promise<InstanceReinstallResult> {
  return request.post<InstanceReinstallResult>({
    url: `/instances/${id}/reinstall`,
    data,
  })
}

export function resetInstancePassword(id: number, password: string): Promise<string> {
  return request.post<string>({
    url: `/instances/${id}/reset-password`,
    data: { password },
  })
}

export function rescueInstance(id: number, data: InstanceRescueRequest): Promise<string> {
  return request.post<string>({
    url: `/instances/${id}/rescue`,
    data,
  })
}

export function exitRescueInstance(id: number): Promise<string> {
  return request.post<string>({
    url: `/instances/${id}/exit-rescue`,
  })
}

export function getInstanceSnapshots(id: number, type?: string): Promise<InstanceSnapshotInfo[]> {
  return request.get<InstanceSnapshotInfo[]>({
    url: `/instances/${id}/snapshots`,
    params: { type },
  })
}

export function createInstanceSnapshot(id: number, data: InstanceSnapshotCreateRequest): Promise<string> {
  return request.post<string>({
    url: `/instances/${id}/snapshots`,
    data,
  })
}

export function deleteInstanceSnapshot(id: number, snapshotId: string): Promise<string> {
  return request.delete<string>({
    url: `/instances/${id}/snapshots/${snapshotId}`,
  })
}

/** 用快照恢复（覆盖系统盘，需二次确认）。 */
export function restoreInstanceSnapshot(id: number, data: InstanceSnapshotRestoreRequest): Promise<string> {
  return request.post<string>({
    url: `/instances/${id}/snapshots/restore`,
    data,
  })
}

export function updateInstanceBandwidth(id: number, data: InstanceBandwidthRequest): Promise<string> {
  return request.put<string>({
    url: `/instances/${id}/bandwidth`,
    data,
  })
}

export function addInstanceIP(id: number, data: InstanceAddIPRequest): Promise<string> {
  return request.post<string>({
    url: `/instances/${id}/ips`,
    data,
  })
}

export function attachInstanceDisk(id: number, data: InstanceAttachDiskRequest): Promise<string> {
  return request.post<string>({
    url: `/instances/${id}/disks`,
    data,
  })
}
