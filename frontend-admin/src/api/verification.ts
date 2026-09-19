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

export interface VerificationInfo {
  id: number
  user_id: number
  username: string
  verification_type: string
  status: string
  real_name: string
  subject_name: string
  id_type: string
  id_number_masked: string
  mobile_masked: string
  risk_flags: string
  submitted_at: string
  reviewed_at?: string
  reviewed_by?: number
  reviewer_name: string
  reject_reason_code?: string
  reject_reason?: string
  review_note?: string
  // —— 迁移 051 新增（doc104 §5.2）——
  provider: string
  provider_result: string
  provider_message: string
  provider_checked_at?: string
  review_round: number
  created_at: string
  updated_at: string
}

export interface VerificationDocument {
  id: number
  document_type: string
  file_url: string
  sort: number
}

export interface VerificationEnterprise {
  company_name: string
  credit_code_masked: string
  legal_person_name: string
  contact_name: string
  business_license: string
}

export interface VerificationReviewLog {
  id: number
  from_status: string
  to_status: string
  action: string
  operator_id: number
  operator_name: string
  note: string
  reject_reason_code: string
  reject_reason: string
  created_at: string
}

/** 详情 = 主记录 + 附件 + 企业信息 + 审核轨迹（doc104 §5.7）。 */
export interface VerificationDetail extends VerificationInfo {
  documents: VerificationDocument[]
  enterprise?: VerificationEnterprise
  logs: VerificationReviewLog[]
  certify_modes: Array<{ label: string; value: string }>
}

export interface VerificationListQuery extends Record<string, unknown> {
  page?: number
  page_size?: number
  user_id?: number
  username?: string
  verification_type?: string
  reviewer_name?: string
  keyword?: string
  start_time?: string
  end_time?: string
}

export function getPendingVerificationList(params: VerificationListQuery): Promise<ListResponse<VerificationInfo>> {
  return request.get<ListResponse<VerificationInfo>>({ url: '/verifications/pending', params })
}

export function getApprovedVerificationList(params: VerificationListQuery): Promise<ListResponse<VerificationInfo>> {
  return request.get<ListResponse<VerificationInfo>>({ url: '/verifications/approved', params })
}

export function getRejectedVerificationList(params: VerificationListQuery): Promise<ListResponse<VerificationInfo>> {
  return request.get<ListResponse<VerificationInfo>>({ url: '/verifications/rejected', params })
}

export function getVerificationDetail(id: number): Promise<VerificationDetail> {
  return request.get<VerificationDetail>({ url: `/verifications/${id}` })
}

export function getVerificationLogs(id: number): Promise<VerificationReviewLog[]> {
  return request.get<VerificationReviewLog[]>({ url: `/verifications/${id}/logs` })
}

export interface VerificationReviewRequest {
  reject_reason_code?: string
  reject_reason?: string
  note?: string
}

// 整单通过 / 驳回 / 撤销（doc104 §5.1）：审核是整单粒度，不逐条审附件。
export function approveVerification(id: number, data: VerificationReviewRequest = {}): Promise<VerificationInfo> {
  return request.post<VerificationInfo>({ url: `/verifications/${id}/approve`, data })
}

export function rejectVerification(id: number, data: VerificationReviewRequest): Promise<VerificationInfo> {
  return request.post<VerificationInfo>({ url: `/verifications/${id}/reject`, data })
}

export function revokeVerification(id: number, data: VerificationReviewRequest = {}): Promise<VerificationInfo> {
  return request.post<VerificationInfo>({ url: `/verifications/${id}/revoke`, data })
}

export interface ProviderCheckResult {
  ok: boolean
  provider: string
  passed: boolean
  biz_code: string
  message: string
  txn_no: string
  auto_approved: boolean
  auto_rejected: boolean
}

// 手动触发三方核验：跳转式服务商（支付宝）会返回 ok=false + 业务说明，HTTP 仍是 200。
export function providerCheckVerification(id: number): Promise<ProviderCheckResult> {
  return request.post<ProviderCheckResult>({ url: `/verifications/${id}/provider-check` })
}

// ---------- 实名配置 ----------

export interface VerificationConfigInfo {
  id: number
  config_key: string
  config_group: string
  config_value: string
  value_type: string
  status: string
  description: string
  updated_by: number
  created_at: string
  updated_at: string
}

export interface VerificationConfigQuery extends Record<string, unknown> {
  config_group?: string
  keyword?: string
  status?: string
}

export interface VerificationConfigUpsertRequest {
  config_key: string
  config_value?: string
  value_type?: string
  status?: string
  description?: string
}

export function getVerificationConfigs(params?: VerificationConfigQuery): Promise<VerificationConfigInfo[]> {
  return request.get<VerificationConfigInfo[]>({ url: '/verifications/configs', params })
}

export function upsertVerificationConfig(data: VerificationConfigUpsertRequest): Promise<VerificationConfigInfo> {
  return request.post<VerificationConfigInfo>({ url: '/verifications/configs', data })
}

export function deleteVerificationConfig(id: number): Promise<void> {
  return request.delete<void>({ url: `/verifications/configs/${id}` })
}

// ---------- 核验服务商 ----------

export interface VerificationCredentialField {
  key: string
  label: string
  type: 'string' | 'password' | 'number' | 'select' | 'bool' | 'textarea'
  required: boolean
  secret: boolean
  placeholder?: string
  help?: string
  default?: string
  options?: Array<{ label: string; value: string }>
}

export interface VerificationProviderType {
  type: string
  name: string
  mode: string
  icon: string
  doc_url: string
  adapter_version: string
  implemented: boolean
  builtin: boolean
  credential_schema: VerificationCredentialField[] | null
  certify_modes: Array<{ label: string; value: string }> | null
}

export interface VerificationProviderInfo {
  id: number
  provider_type: string
  name: string
  mode: string
  endpoint: string
  priority: number
  health_status: string
  last_error: string
  last_check_at: string
  status: number
  is_default: boolean
  remark: string
  supported: boolean
  implemented: boolean
  builtin: boolean
  certify_modes: Array<{ label: string; value: string }> | null
  credentials: Record<string, string>
  credential_keys: string[]
}

export interface VerificationProviderUpsertRequest {
  id?: number
  provider_type?: string
  name?: string
  endpoint?: string
  priority?: number
  status?: number
  is_default?: boolean
  remark?: string
  credentials?: Record<string, string>
}

export function getVerificationProviderTypes(): Promise<VerificationProviderType[]> {
  return request.get<VerificationProviderType[]>({ url: '/verifications/provider-types' })
}

export function getVerificationProviders(): Promise<VerificationProviderInfo[]> {
  return request.get<VerificationProviderInfo[]>({ url: '/verifications/providers' })
}

export function upsertVerificationProvider(
  data: VerificationProviderUpsertRequest,
): Promise<VerificationProviderInfo> {
  return request.post<VerificationProviderInfo>({ url: '/verifications/providers', data })
}

export function deleteVerificationProvider(id: number): Promise<void> {
  return request.delete<void>({ url: `/verifications/providers/${id}` })
}

// 测试失败也返回 200 + {ok:false,message}（对齐支付/验证码契约）。
export function testVerificationProvider(id: number): Promise<{ ok: boolean; message: string }> {
  return request.post<{ ok: boolean; message: string }>({ url: `/verifications/providers/${id}/test` })
}
