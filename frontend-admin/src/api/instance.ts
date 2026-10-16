import { request } from '@/utils/request'

import type {
  InstanceAddIPRequest,
  InstanceAdjustContext,
  InstanceAdjustRequest,
  InstanceAttachDiskRequest,
  InstanceBandwidthRequest,
  InstanceBatchActionRequest,
  InstanceBatchActionResponse,
  InstanceBatchDestroyRequest,
  InstanceBatchDestroyResponse,
  InstanceBatchPurgeRequest,
  InstanceBatchPurgeResponse,
  InstanceCredentialReveal,
  InstanceDestroyRequest,
  InstanceDetail,
  InstanceOpsListQuery,
  InstanceOpsListResponse,
  InstancePowerRequest,
  InstancePurgeRequest,
  InstanceSetStateRequest,
  InstanceRebindProductRequest,
  InstanceRebindProductResult,
  InstanceRelatedResponse,
  InstanceReinstallRequest,
  InstanceReinstallResult,
  InstanceReinstallImages,
  InstanceRebindProductCandidates,
  InstanceRelinkRequest,
  InstanceRelinkResult,
  InstanceRemarkUpdateRequest,
  InstanceRescueRequest,
  InstanceResizeRequest,
  InstanceSnapshotCreateRequest,
  InstanceSnapshotInfo,
  InstanceSnapshotRestoreRequest,
  InstanceStatsResponse,
  InstanceTypeSummary,
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

/**
 * 产品线导航数据（类型清单 + 各自实例数）。
 * 类型登记在后端（pkg/model/product_kind.go）；当前没有实例的预埋类型也会返回（reserved=true），
 * 由前端决定是否显示 —— 这样接入新产品线时无需改前端菜单。
 */
export function getInstanceTypes(): Promise<InstanceTypeSummary> {
  return request.get<InstanceTypeSummary>({
    url: '/instances/types',
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

/** 重新开通待开通实例（重投来源订单的履约任务；不产生新订单/新账单）。 */
export function retryInstanceProvision(id: number): Promise<InstanceDetail> {
  return request.post<InstanceDetail>({
    url: `/instances/${id}/provision-retry`,
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

/**
 * 删除实例记录（物理删除已销毁的行，连同其运维流水）。
 *
 * 与 destroyInstance 分两个接口，是因为它们是两个语义不同的步骤：
 * 销毁会通知渠道终止机器，删除只清本地记录。合并会让人以为"删掉就完事"，
 * 而机器可能仍在渠道侧计费。
 */
export function purgeInstance(id: number, data: InstancePurgeRequest): Promise<string> {
  return request.post<string>({
    url: `/instances/${id}/purge`,
    data,
  })
}

export function suspendInstance(id: number, reason?: string, reasonClass?: string): Promise<string> {
  return request.post<string>({
    url: `/instances/${id}/suspend`,
    data: { reason, reason_class: reasonClass },
  })
}

/**
 * 修改实例服务状态（服务中 / 已暂停 / 已取消）。
 *
 * 与 suspend/unsuspend 的分工：那两个是"具体动作"，本接口是**状态入口** ——
 * 按目标状态分派到对应动作，并额外支持"取消"（占位行专用）。
 * 服务层只接受这三个可被人工设置的状态，避免直接写 status 造出自相矛盾的行。
 */
export function setInstanceState(
  id: number,
  data: InstanceSetStateRequest,
): Promise<string> {
  return request.post<string>({
    url: `/instances/${id}/state`,
    data,
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

// 批量销毁（不可逆）：逐台结果 + 逐台确认文本，与批量运维同一响应形态
export function batchDestroyInstances(
  data: InstanceBatchDestroyRequest,
  verifyTicket?: string,
): Promise<InstanceBatchDestroyResponse> {
  return request.post<InstanceBatchDestroyResponse>({
    url: '/instances/batch-destroy',
    data,
    headers: verifyTicket ? { 'X-Verify-Ticket': verifyTicket } : undefined,
  })
}

// 批量删除记录（物理删除已销毁的行）：与批量销毁同一逐台确认口径
export function batchPurgeInstances(
  data: InstanceBatchPurgeRequest,
  verifyTicket?: string,
): Promise<InstanceBatchPurgeResponse> {
  return request.post<InstanceBatchPurgeResponse>({
    url: '/instances/batch-purge',
    data,
    headers: verifyTicket ? { 'X-Verify-Ticket': verifyTicket } : undefined,
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

/**
 * 重装可选镜像列表。
 * 来源随链路不同：自营取渠道镜像目录，对接上游取**上游商品的操作系统配置子项**
 * （下发给上游的就是配置子项 ID）。两者都读不到时返回 source=none，运营按 hint 手填。
 */
export function getInstanceReinstallImages(id: number): Promise<InstanceReinstallImages> {
  return request.get<InstanceReinstallImages>({
    url: `/instances/${id}/reinstall-images`,
  })
}

/**
 * 更换对接商品的候选项（选框数据源）。
 * 本地商品给全量（附上下架标记与已配周期价）；上游镜像只给**同渠道**的。
 */
export function getInstanceRebindCandidates(id: number): Promise<InstanceRebindProductCandidates> {
  return request.get<InstanceRebindProductCandidates>({
    url: `/instances/${id}/rebind-candidates`,
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

// ---- 详情页第四轮：账期自定义与明文凭据 ----

/**
 * 调整实例到期时间与付款周期（本地口径，不产生账单、不推送上游）。
 * 返回调整后的最新详情，页面可直接用它刷新，无需再拉一次。
 */
export function adjustInstanceBilling(id: number, data: InstanceAdjustRequest): Promise<InstanceDetail> {
  return request.put<InstanceDetail>({
    url: `/instances/${id}/billing`,
    data,
  })
}

/** 查看登录明文密码（敏感操作，后端会写审计流水）。 */
export function revealInstanceCredential(id: number): Promise<InstanceCredentialReveal> {
  return request.post<InstanceCredentialReveal>({
    url: `/instances/${id}/credential`,
  })
}

/**
 * 账期调整上下文：可选周期以**上游能力**为准（避免选中上游不卖的周期），
 * 并带上该周期上游标价/采购价与我方售价/成本，供运营比价后定绑定价。
 */
export function getInstanceBillingContext(id: number): Promise<InstanceAdjustContext> {
  return request.get<InstanceAdjustContext>({
    url: `/instances/${id}/billing-context`,
  })
}

/** 按上游实例号重新关联（上游换机/换号后拉取新机器信息）。 */
export function relinkInstanceUpstream(
  id: number,
  data: InstanceRelinkRequest,
  verifyTicket?: string,
): Promise<InstanceRelinkResult> {
  return request.post<InstanceRelinkResult>({
    url: `/instances/${id}/relink`,
    data,
    headers: verifyTicket ? { 'X-Verify-Ticket': verifyTicket } : undefined,
  })
}

/** 更换实例绑定的本地商品 / 上游镜像（上游换产品后同步本地归属）。 */
export function rebindInstanceProduct(
  id: number,
  data: InstanceRebindProductRequest,
): Promise<InstanceRebindProductResult> {
  return request.post<InstanceRebindProductResult>({
    url: `/instances/${id}/rebind-product`,
    data,
  })
}
