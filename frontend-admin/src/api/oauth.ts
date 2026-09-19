// 第三方登录（doc104 §6）· 管理端 API
// 对应后端 internal/modules/uc/oauth/{dto,handler}，路由前缀 /api/v1/admin/oauth。
import { request } from '@/utils/request'

/** 凭证字段描述（与后端 integration.Field 同构，驱动动态表单 + 字段级加密） */
export interface OAuthCredentialField {
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

/** 渠道类型元数据（描述符驱动前端动态表单） */
export interface OAuthProviderType {
  type: string
  name: string
  mode: string
  icon: string
  doc_url: string
  adapter_version: string
  default_scopes: string
  /** 适配器是否已注册真实实现 */
  implemented: boolean
  credential_schema: OAuthCredentialField[] | null
}

export interface OAuthProviderInfo {
  id: number
  provider: string
  name: string
  enabled: boolean
  mode: string
  scopes: string
  icon: string
  sort_order: number
  health_status: string
  last_error: string
  last_check_at: string
  remark: string
  /** 该渠道是否已注册真实适配器 */
  supported: boolean
  implemented: boolean
  /** 脱敏回显：secret 字段为掩码，原样回传表示不修改 */
  credentials: Record<string, string>
  credential_keys: string[]
  /** 回调地址：由后端按当前域名拼装，需复制到三方开放平台 */
  callback_url: string
  created_at: string
  updated_at: string
}

export interface OAuthProviderUpdateRequest {
  enabled?: boolean
  scopes?: string
  sort_order?: number
  remark?: string
  credentials?: Record<string, string>
}

export interface OAuthProviderTestResponse {
  ok: boolean
  message: string
}

export interface OAuthBindingInfo {
  id: number
  user_id: number
  username: string
  provider: string
  openid: string
  unionid?: string
  nickname: string
  avatar: string
  status: string
  bound_at: string
  last_login_at?: string
}

export interface OAuthBindingListQuery extends Record<string, unknown> {
  page?: number
  page_size?: number
  provider?: string
  user_id?: number
  status?: string
  keyword?: string
}

export interface OAuthBindingListResponse {
  items: OAuthBindingInfo[]
  meta: { page: number; page_size: number; total: number }
}

export function getOAuthProviderTypes(): Promise<OAuthProviderType[]> {
  return request.get<OAuthProviderType[]>({ url: '/oauth/provider-types' })
}

export function getOAuthProviders(): Promise<OAuthProviderInfo[]> {
  return request.get<OAuthProviderInfo[]>({ url: '/oauth/providers' })
}

// 按渠道名 upsert（三家固定）：没有渠道行时后端会按描述符创建，无需先「新建」。
export function updateOAuthProvider(
  provider: string,
  data: OAuthProviderUpdateRequest,
): Promise<OAuthProviderInfo> {
  return request.put<OAuthProviderInfo>({ url: `/oauth/providers/${provider}`, data })
}

// 测试接口失败时 HTTP 仍为 200，结果由 ok/message 表达（对齐支付/验证码契约）。
export function testOAuthProvider(provider: string): Promise<OAuthProviderTestResponse> {
  return request.post<OAuthProviderTestResponse>({ url: `/oauth/providers/${provider}/test` })
}

export function getOAuthBindings(params: OAuthBindingListQuery): Promise<OAuthBindingListResponse> {
  return request.get<OAuthBindingListResponse>({ url: '/oauth/bindings', params })
}
