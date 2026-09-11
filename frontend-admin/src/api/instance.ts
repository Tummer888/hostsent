import { request } from '@/utils/request'

import type {
  InstanceDestroyRequest,
  InstanceDetail,
  InstanceOpsListQuery,
  InstanceOpsListResponse,
  InstancePowerRequest,
  InstanceRelatedResponse,
  InstanceRemarkUpdateRequest,
  InstanceResizeRequest,
  InstanceStatsResponse,
  InstanceVNCResponse,
  OperationListQuery,
  OperationListResponse,
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
