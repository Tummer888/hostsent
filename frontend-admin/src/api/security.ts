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
  ip_region: string
  user_agent: string
  device_fingerprint: string
  platform: string
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
  handled_at?: string
  handle_note?: string
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
  updated_by: number
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
  ip_region: string
  user_agent: string
  device_fingerprint: string
  login_at: string
  last_active_at: string
  expired_at: string
  status: string
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
  risk_type?: string
  risk_level?: string
  status?: string
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
  start_time?: string
  end_time?: string
}

export interface RiskHandleRequest {
  note?: string
}

export interface BlacklistCreateRequest {
  type: string
  target_value: string
  status?: string
  source?: string
  reason?: string
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

export function getAuditLogList(params: AuditLogListQuery): Promise<ListResponse<AuditLogInfo>> {
  return request.get<ListResponse<AuditLogInfo>>({ url: '/security/audit-logs', params })
}

export function getRiskEventList(params: RiskEventListQuery): Promise<ListResponse<RiskEventInfo>> {
  return request.get<ListResponse<RiskEventInfo>>({ url: '/security/risk-events', params })
}

export function ignoreRiskEvent(id: number, data: RiskHandleRequest = {}): Promise<RiskEventInfo> {
  return request.post<RiskEventInfo>({ url: `/security/risk-events/${id}/ignore`, data })
}

export function handleRiskEvent(id: number, data: RiskHandleRequest = {}): Promise<RiskEventInfo> {
  return request.post<RiskEventInfo>({ url: `/security/risk-events/${id}/handle`, data })
}

export function blacklistRiskEvent(id: number, data: RiskHandleRequest = {}): Promise<BlacklistInfo> {
  return request.post<BlacklistInfo>({ url: `/security/risk-events/${id}/blacklist`, data })
}

export function revokeRiskEventSessions(id: number, data: RiskHandleRequest = {}): Promise<ListResponse<SessionInfo>> {
  return request.post<ListResponse<SessionInfo>>({ url: `/security/risk-events/${id}/revoke-sessions`, data })
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
  return request.post<BlacklistInfo>({ url: `/security/blacklists/${id}/release`, data: {} })
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
