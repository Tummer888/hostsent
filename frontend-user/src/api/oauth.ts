import request from '@/utils/request'

/**
 * 第三方登录（doc104 §6）。
 *
 * 三条链路共用一个 `ticket` 概念：三方回调 → 后端签一次性票据 → 前端拿票据换正式令牌。
 * 令牌绝不出现在 URL 里（会被浏览器历史、Referer、日志记录），因此这里没有
 * 「回调直接带 token」的接口，只有 exchange。
 */

/** 已启用的渠道元数据；不含任何凭证。 */
export interface PublicOAuthProvider {
  provider: string
  name: string
  icon: string
  mode: string
}

/** 我的绑定条目（后端刻意不回显 openid）。 */
export interface MyOAuthBinding {
  provider: string
  name: string
  icon: string
  nickname: string
  avatar: string
  status: string
  bound_at: string
  last_login_at?: string
  is_primary: boolean
  /** 服务端预判：解绑后是否仍保留其他登录方式 */
  can_unbind: boolean
}

export interface OAuthExchangeResult {
  token: string
  need_bind: boolean
  /**
   * 绑定流程完成（非登录）。
   *
   * 后端把「绑定」也走同一条 ticket → exchange 链路（回调是免登录入口，
   * 只有票据能安全带回结果）。此时 token 为空，前端必须回个人中心刷新绑定列表，
   * 而不是当登录成功处理 —— 否则用户会看到一个空会话的「已登录」状态。
   */
  bind_done?: boolean
  provider: string
  user?: {
    id: number
    username: string
    name: string
    email: string
    avatar: string
    status: string
  }
}

/** 登录页渲染第三方图标用：只返回已启用且适配器已注册的渠道。 */
export function getOAuthProviders() {
  return request.get<any, { data: PublicOAuthProvider[] }>('/uc/oauth/providers')
}

/** 取登录授权跳转地址（后端签发 state 后返回完整 URL）。 */
export function getOAuthAuthorizeUrl(provider: string, inviteCode?: string) {
  return request.get<any, { data: { authorize_url: string } }>(`/uc/oauth/${provider}/authorize`, {
    params: inviteCode ? { invite_code: inviteCode } : undefined,
  })
}

/** ticket → 正式令牌。ticket 60 秒有效且一次性。 */
export function exchangeOAuthTicket(ticket: string) {
  return request.post<any, { data: OAuthExchangeResult }>('/uc/oauth/exchange', { ticket })
}

/** 我的绑定列表。 */
export function getMyOAuthBindings() {
  return request.get<any, { data: MyOAuthBinding[] }>('/uc/oauth/bindings')
}

/** 取绑定用的授权地址（state 里带当前用户 ID，回调只能绑到发起时的账号）。 */
export function getOAuthBindAuthorizeUrl(provider: string) {
  return request.get<any, { data: { authorize_url: string } }>(`/uc/oauth/${provider}/bind-authorize`)
}

export function unbindOAuth(provider: string) {
  return request.delete<any, { data: string }>(`/uc/oauth/bindings/${provider}`)
}
