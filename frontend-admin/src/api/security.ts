import type { AxiosResponse } from 'axios'

import { request } from '@/utils/request'

export interface ListMeta {
  page: number
  page_size: number
  total: number
}

export interface ListResponse<T> {
  items: T[]
  meta: ListMeta
}

export interface LoginLogInfo {
  id: number
  user_id: number
  username: string
  login_type: string
  result: string
  failure_reason?: string
  ip: string
  user_agent: string
  device_fingerprint: string
  platform: string
  /** 登录主体域：user（客户）/ admin（员工后台）。两个 ID 空间共用 user_id 且会撞号 */
  subject_type: string
  risk_flag: string
  created_at: string
}

export interface AuditLogInfo {
  id: number
  operator_id: number
  operator_name: string
  module: string
  resource_type: string
  resource_id: string
  action: string
  request_method: string
  request_path: string
  request_payload: string
  response_code: number
  response_message: string
  ip: string
  user_agent: string
  trace_id: string
  created_at: string
}

export interface RiskEventInfo {
  id: number
  risk_type: string
  risk_level: string
  user_id: number
  username: string
  /**
   * 事件主体域：user（客户）/ admin（员工后台）。
   *
   * user_id 承载 users.id 与 admins.id 两个 ID 空间且会撞号，
   * 不带上它就无法区分「客户 7 号」与「员工 7 号」。
   */
  subject_type: string
  ip: string
  device_fingerprint: string
  rule_code: string
  summary: string
  detail_payload: string
  occur_count: number
  first_occurred_at: string
  last_occurred_at: string
  status: string
  handled_by: number
  /** 处置人账号名（后端批量补全，避免页面只显示一个数字 ID）。 */
  handled_by_name: string
  handled_at?: string
  handle_note?: string
  /**
   * 已执行过的处置动作码（去重）：level / handle / ignore / blacklist / revoke_sessions。
   *
   * 与 status 是两个维度：status 只说「待办关掉没有」，而拉黑、失效会话这类管控
   * 动作不改变待办状态，却必须让运营一眼看到「这条已经管控过了」。
   */
  actions?: string[]
  /** 处置动作的中文摘要（如「拉黑 · 失效会话」），列表页直接展示。 */
  action_summary?: string
  created_at: string
  updated_at: string
}

export interface BlacklistInfo {
  id: number
  type: string
  target_value: string
  status: string
  source: string
  reason: string
  effective_at: string
  expired_at?: string
  hit_count: number
  created_by: number
  created_by_name: string
  updated_by: number
  updated_by_name: string
  /**
   * 运行态：active / inactive / expired / pending。
   *
   * 与 status 分开：status 只是运营开关，运行态还叠加了生效/失效时间。
   * 一条限时黑名单到期后 status 仍是 active 但**已经不再拦截**，
   * 只看 status 会以为还封着。
   */
  runtime_status: string
  created_at: string
  updated_at: string
}

export interface SessionInfo {
  id: number
  session_id: string
  user_id: number
  username: string
  platform: string
  ip: string
  user_agent: string
  device_fingerprint: string
  login_at: string
  last_active_at: string
  expired_at: string
  status: string
  /** 会话主体域：user（客户）/ admin（员工后台），语义同 LoginLogInfo.subject_type */
  subject_type: string
  risk_flag: string
  revoked_reason?: string
  revoked_by: number
  revoked_at?: string
  created_at: string
  updated_at: string
}

export interface LoginLogListQuery extends Record<string, unknown> {
  page?: number
  page_size?: number
  user_id?: number
  username?: string
  result?: string
  login_type?: string
  ip?: string
  risk_flag?: string
  /** 主体域过滤：客户详情面板必须传 user，否则同 ID 的员工后台登录会混进来 */
  subject_type?: 'user' | 'admin'
  start_time?: string
  end_time?: string
}

export interface AuditLogListQuery extends Record<string, unknown> {
  page?: number
  page_size?: number
  operator?: string
  module?: string
  action?: string
  result?: string
  resource_type?: string
  resource_id?: string
  start_time?: string
  end_time?: string
}

export interface RiskEventListQuery extends Record<string, unknown> {
  page?: number
  page_size?: number
  /** 按用户过滤（后端 applyRiskEventFilters 支持；缺失时该参数会被静默忽略） */
  user_id?: number
  risk_type?: string
  risk_level?: string
  status?: string
  /**
   * 按「已执行过的处置动作」筛选：blacklist / revoke_sessions / handle / ignore / level。
   *
   * 与 status 互补：status 筛「待办关掉没有」，action 筛「做没做过某类管控」。
   */
  action?: string
  keyword?: string
  start_time?: string
  end_time?: string
}

export interface BlacklistListQuery extends Record<string, unknown> {
  page?: number
  page_size?: number
  type?: string
  status?: string
  source?: string
  keyword?: string
  start_time?: string
  end_time?: string
}

export interface SessionListQuery extends Record<string, unknown> {
  page?: number
  page_size?: number
  user_id?: number
  username?: string
  status?: string
  platform?: string
  ip?: string
  risk_flag?: string
  /** 主体域过滤，语义同 LoginLogListQuery.subject_type */
  subject_type?: 'user' | 'admin'
  start_time?: string
  end_time?: string
}

/**
 * 关闭待办（处置 / 忽略）的请求体。
 *
 * 处置与忽略只回答「这条待办关掉没有」，与「要不要同时做管控」是两件事：
 * 大部分事件看一眼就知道没事（忽略即可），但确认有问题的那些，运营希望
 * 一次点完「处置 + 拉黑 + 踢会话」，而不是关完单再去列表里重新找到它。
 */
export interface RiskHandleRequest {
  note?: string
  /** 同时把该事件来源加入黑名单。 */
  blacklist?: boolean
  /** 拉黑维度：ip / user / device / phone / email；留空由后端按事件推断。 */
  blacklist_type?: string
  /** 同时强制下线该主体（仅客户域支持）。 */
  revoke_sessions?: boolean
}

/** 从风险事件拉黑（管控动作，不改变待办状态，除非 close_event）。 */
export interface RiskBlacklistRequest {
  /** 拉黑维度；留空按事件可用字段推断（IP → 设备 → 账号）。 */
  type?: string
  note?: string
  /** 同时把这条待办关成「已处置」。 */
  close_event?: boolean
}

/** 从风险事件失效会话（管控动作，不改变待办状态，除非 close_event）。 */
export interface RiskRevokeRequest {
  note?: string
  /** 同时把这条待办关成「已处置」。 */
  close_event?: boolean
}

/** 一条处置流水（详情抽屉的处置时间线）。 */
export interface RiskEventActionInfo {
  id: number
  /** level / handle / ignore / blacklist / revoke_sessions。 */
  action: string
  operator_id: number
  operator_name: string
  note: string
  /** 动作结果 JSON 快照（拉黑维度与值、失效会话数、等级变更前后）。 */
  detail: string
  created_at: string
}

/** 手动调整风险等级（doc06 §4.3「手动升级风险等级」）。 */
export interface RiskLevelRequest {
  /** low / medium / high / critical。 */
  risk_level: string
  /** 调整原因，写进处置说明。 */
  note?: string
  /** 调级的同时关掉待办（默认 false，仅标记不结案）。 */
  close_event?: boolean
  /** 配合 close_event：handled（已处置）/ ignored（已忽略）。 */
  close_as?: string
}

export interface BlacklistCreateRequest {
  type: string
  target_value: string
  status?: string
  source?: string
  reason?: string
  /**
   * 失效时间（"YYYY-MM-DD HH:mm:ss"）。**留空表示永久生效** ——
   * doc06 §4.4 关键规则 2 要求同时支持永久与限时两种模式。
   */
  expired_at?: string
}

export interface BlacklistUpdateRequest {
  status?: string
  reason?: string
  expired_at?: string
}

export interface BlacklistStatusRequest {
  status: string
}

export interface SessionRevokeRequest {
  reason?: string
}

export interface SessionBatchRevokeRequest {
  ids: number[]
  reason?: string
}

export interface SessionRevokeUserAllRequest {
  user_id: number
  reason?: string
}

export function getLoginLogList(params: LoginLogListQuery): Promise<ListResponse<LoginLogInfo>> {
  return request.get<ListResponse<LoginLogInfo>>({ url: '/security/login-logs', params })
}

/**
 * 导出登录日志 CSV。
 *
 * `_skipResultUnwrap` 是必须的：这个接口返回的是文件流而不是 `{code,data,message}`
 * 信封，不跳过解包会把 CSV 文本当成业务响应解析。
 */
export function exportLoginLogs(params: LoginLogListQuery) {
  return request.get<AxiosResponse<Blob>>({
    url: '/security/login-logs/export',
    params,
    responseType: 'blob',
    _skipResultUnwrap: true,
  })
}

/** 黑名单命中记录（按黑名单类型关联登录日志）。 */
export function getBlacklistHits(id: number, params: { page?: number; page_size?: number } = {}) {
  return request.get<ListResponse<LoginLogInfo>>({ url: `/security/blacklists/${id}/hits`, params })
}

export function getAuditLogList(params: AuditLogListQuery): Promise<ListResponse<AuditLogInfo>> {
  return request.get<ListResponse<AuditLogInfo>>({ url: '/security/audit-logs', params })
}

export function getRiskEventList(params: RiskEventListQuery): Promise<ListResponse<RiskEventInfo>> {
  return request.get<ListResponse<RiskEventInfo>>({ url: '/security/risk-events', params })
}

/** 风险事件汇总（待处理 / 已处置 / 已忽略 / 总数），复用列表的筛选条件。 */
export function getRiskEventStats(
  params: Omit<RiskEventListQuery, 'page' | 'page_size' | 'status'> = {},
): Promise<Record<string, number>> {
  return request.get<Record<string, number>>({ url: '/security/risk-events/stats', params })
}

/** 手动调整风险等级（默认只改等级，不改处置状态）。 */
export function updateRiskEventLevel(id: number, data: RiskLevelRequest): Promise<RiskEventInfo> {
  return request.post<RiskEventInfo>({ url: `/security/risk-events/${id}/level`, data })
}

/** 关闭待办：处置 / 忽略，可同时附带管控动作（拉黑、失效会话）。 */
export function ignoreRiskEvent(id: number, data: RiskHandleRequest = {}): Promise<RiskEventInfo> {
  return request.post<RiskEventInfo>({ url: `/security/risk-events/${id}/ignore`, data })
}

export function handleRiskEvent(id: number, data: RiskHandleRequest = {}): Promise<RiskEventInfo> {
  return request.post<RiskEventInfo>({ url: `/security/risk-events/${id}/handle`, data })
}

/**
 * 从风险事件拉黑来源（管控动作，不改变待办状态，除非传 close_event）。
 *
 * type 留空时后端按事件可用字段推断（IP → 设备 → 账号）。
 */
export function blacklistRiskEvent(id: number, data: RiskBlacklistRequest = {}): Promise<BlacklistInfo> {
  return request.post<BlacklistInfo>({ url: `/security/risk-events/${id}/blacklist`, data })
}

/** 失效该事件主体的全部有效会话（同样不改待办状态，除非传 close_event）。 */
export function revokeRiskEventSessions(id: number, data: RiskRevokeRequest = {}): Promise<ListResponse<SessionInfo>> {
  return request.post<ListResponse<SessionInfo>>({ url: `/security/risk-events/${id}/revoke-sessions`, data })
}

/** 风险事件处置时间线（谁在什么时候做了什么）。 */
export function getRiskEventActions(id: number): Promise<ListResponse<RiskEventActionInfo>> {
  return request.get<ListResponse<RiskEventActionInfo>>({ url: `/security/risk-events/${id}/actions` })
}

export function getBlacklistList(params: BlacklistListQuery): Promise<ListResponse<BlacklistInfo>> {
  return request.get<ListResponse<BlacklistInfo>>({ url: '/security/blacklists', params })
}

export function createBlacklist(data: BlacklistCreateRequest): Promise<BlacklistInfo> {
  return request.post<BlacklistInfo>({ url: '/security/blacklists', data })
}

export function updateBlacklist(id: number, data: BlacklistUpdateRequest): Promise<BlacklistInfo> {
  return request.put<BlacklistInfo>({ url: `/security/blacklists/${id}`, data })
}

export function updateBlacklistStatus(id: number, data: BlacklistStatusRequest): Promise<BlacklistInfo> {
  return request.patch<BlacklistInfo>({ url: `/security/blacklists/${id}/status`, data })
}

export function releaseBlacklist(id: number): Promise<BlacklistInfo> {
  return request.delete<BlacklistInfo>({ url: `/security/blacklists/${id}` })
}

export function getSessionList(params: SessionListQuery): Promise<ListResponse<SessionInfo>> {
  return request.get<ListResponse<SessionInfo>>({ url: '/security/sessions', params })
}

export function revokeSession(id: number, data: SessionRevokeRequest = {}): Promise<SessionInfo> {
  return request.post<SessionInfo>({ url: `/security/sessions/${id}/revoke`, data })
}

export function batchRevokeSessions(data: SessionBatchRevokeRequest): Promise<ListResponse<SessionInfo>> {
  return request.post<ListResponse<SessionInfo>>({ url: '/security/sessions/batch-revoke', data })
}

export function revokeUserAllSessions(data: SessionRevokeUserAllRequest): Promise<ListResponse<SessionInfo>> {
  return request.post<ListResponse<SessionInfo>>({ url: '/security/sessions/revoke-user-all', data })
}
