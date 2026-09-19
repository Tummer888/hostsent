import request from '@/utils/request'

/**
 * 实名认证（doc104 §5）。
 *
 * 判据统一走 `real_name_verified_at`（后端 status 字段），前端不用「real_name 是否非空」
 * 判断是否已实名 —— 那是被修掉的后门（改昵称即视为已实名）。
 */

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
  submitted_at: string
  reviewed_at?: string
  reviewer_name: string
  reject_reason_code?: string
  reject_reason?: string
  review_note?: string
  provider: string
  provider_result: string
  provider_message: string
  provider_checked_at?: string
  review_round: number
  created_at: string
  updated_at: string
}

export interface VerificationDetail extends VerificationInfo {
  documents: VerificationDocument[]
  enterprise?: VerificationEnterprise
  logs: VerificationReviewLog[]
}

/** 实名状态卡：none / pending / approved / rejected */
export interface VerificationStatus {
  status: string
  enabled: boolean
  allowed_types: string[]
  application_id: number
  verification_type: string
  real_name: string
  id_number_masked: string
  mobile_masked: string
  submitted_at?: string
  reviewed_at?: string
  reject_reason: string
  review_note: string
  provider_result: string
  review_round: number
  cooldown_hours: number
  cooldown_remain_seconds: number
}

export interface VerificationSubmitRequest {
  verification_type: string
  real_name: string
  subject_name?: string
  id_type?: string
  id_number: string
  mobile?: string
  country_code?: string
  company_name?: string
  credit_code?: string
  legal_person_name?: string
  contact_name?: string
  captcha_key?: string
  captcha_code?: string
  otp_code?: string
  verify_ticket?: string
}

export interface VerificationAuthorizeResult {
  auth_url: string
  txn_no: string
  provider: string
  expire_at: string
}

export function getVerificationStatus() {
  return request.get<any, { data: VerificationStatus }>('/uc/verification')
}

export function submitVerification(data: VerificationSubmitRequest) {
  return request.post<any, { data: VerificationInfo }>('/uc/verification', data)
}

export function getMyVerificationApplications(params?: { page?: number; page_size?: number }) {
  return request.get<any, { data: { items: VerificationInfo[]; meta: { page: number; page_size: number; total: number } } }>(
    '/uc/verification/applications',
    { params },
  )
}

export function getMyVerificationDetail(id: number | string) {
  return request.get<any, { data: VerificationDetail }>(`/uc/verification/applications/${id}`)
}

/** 跳转式核验（支付宝）：拿到认证入口 URL。 */
export function authorizeVerification(id: number | string) {
  return request.post<any, { data: VerificationAuthorizeResult }>(`/uc/verification/applications/${id}/authorize`)
}

/**
 * 上传实名材料。
 *
 * 同类型重传即覆盖（后端按 application_id + document_type 去重），
 * 因此重复选择证件正面不会在审核页堆出两张图。
 */
export function uploadVerificationDocument(applicationId: number | string, documentType: string, file: File) {
  const form = new FormData()
  form.append('document_type', documentType)
  form.append('file', file)
  return request.post<any, { data: VerificationDocument }>(`/uc/verification/applications/${applicationId}/documents`, form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

/** 下载材料（走鉴权端点，不暴露存储路径）。 */
export function downloadVerificationDocument(id: number | string, fileName = 'document') {
  return request
    .get<Blob>(`/uc/verification/documents/${id}/download`, { responseType: 'blob' })
    .then((response) => {
      const blob = (response as unknown as { data: Blob }).data
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = fileName
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      window.URL.revokeObjectURL(url)
    })
}
