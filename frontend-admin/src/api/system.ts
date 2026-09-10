import { request } from '@/utils/request'

import type { ListResponse } from './security'

/** 系统配置信息（字段与后端 snake_case 保持一致） */
export interface SystemConfigInfo {
  id: number
  config_key: string
  config_value: string
  value_type: 'string' | 'bool' | 'int' | 'decimal' | 'json'
  config_group: string
  description: string
  sort_order: number
  status: 'active' | 'disabled'
  created_at: string
  updated_at: string
}

/** 配置列表查询参数 */
export interface SystemConfigListQuery extends Record<string, unknown> {
  page?: number
  page_size?: number
  group?: string
  keyword?: string
}

/** 新增配置请求体 */
export interface SystemConfigCreateRequest {
  config_key: string
  config_value: string
  value_type: SystemConfigInfo['value_type']
  config_group: string
  description?: string
  sort_order?: number
  status: SystemConfigInfo['status']
}

/** 编辑配置请求体（不含 config_key，配置键创建后不可修改） */
export interface SystemConfigUpdateRequest
  extends Omit<SystemConfigCreateRequest, 'config_key'> { }

/** 分组批量保存的单项配置 */
export interface SystemConfigBatchItem {
  config_key: string
  config_value: string
  value_type: SystemConfigInfo['value_type']
  config_group?: string
  description?: string
  sort_order?: number
  status?: SystemConfigInfo['status']
}

/** 分页元信息（与 security.ts 中 ListMeta 结构一致） */
export interface SystemConfigListMeta {
  page: number
  page_size: number
  total: number
}

/** 配置列表响应 */
export type SystemConfigListResponse = ListResponse<SystemConfigInfo>

/** 获取配置列表 */
export function getConfigList(
  params: SystemConfigListQuery,
): Promise<SystemConfigListResponse> {
  return request.get<SystemConfigListResponse>({ url: '/system/configs', params })
}

/** 按配置键查询单条配置 */
export function getConfigByKey(key: string): Promise<SystemConfigInfo> {
  return request.get<SystemConfigInfo>({ url: `/system/configs/${key}` })
}

/** 新增配置 */
export function createConfig(data: SystemConfigCreateRequest): Promise<SystemConfigInfo> {
  return request.post<SystemConfigInfo>({ url: '/system/configs', data })
}

/** 更新配置（按 id） */
export function updateConfig(
  id: number,
  data: SystemConfigUpdateRequest,
): Promise<SystemConfigInfo> {
  return request.put<SystemConfigInfo>({ url: `/system/configs/${id}`, data })
}

/** 删除配置 */
export function deleteConfig(id: number): Promise<string> {
  return request.delete<string>({ url: `/system/configs/${id}` })
}

/** 按分组查询全部配置项（分组化配置页面加载原值用，按 sort_order 排序） */
export function getConfigListByGroup(group: string): Promise<SystemConfigInfo[]> {
  return request.get<SystemConfigInfo[]>({ url: `/system/configs/group/${group}` })
}

/** 按分组批量保存配置项（按 config_key 幂等 upsert，返回该分组最新配置列表） */
export function batchSaveConfigs(
  group: string,
  items: SystemConfigBatchItem[],
): Promise<SystemConfigInfo[]> {
  return request.post<SystemConfigInfo[]>({
    url: '/system/configs/batch',
    data: { config_group: group, items },
  })
}

// —— 审计日志便捷导出：复用 @/api/security 中的定义，勿在此重复实现 ——
export { getAuditLogList } from './security'
export type { AuditLogInfo, AuditLogListQuery } from './security'
