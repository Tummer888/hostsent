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
  created_at: string
  updated_at: string
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
