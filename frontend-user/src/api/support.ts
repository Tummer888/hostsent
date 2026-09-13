import request from '@/utils/request'

// 用户中心 · 工单支持接口（baseURL=/api/v1，路径拼接为 /api/v1/uc/support/*）
// 注意：本项目 request 拦截器返回的是完整信封 { code, message, data }，因此各函数返回 .data。
// 该拦截器对 responseType=blob 直接透传 axios response，附件下载据此拿到原始 Blob。

export interface ListMeta {
  page: number
  page_size: number
  total: number
}

// 工单列表项
export interface TicketInfo {
  id: number
  ticket_no: string
  user_id: number
  username: string
  title: string
  category: string
  category_name: string
  priority: string
  status: string
  assigned_to: number
  assigned_name: string
  order_id: number
  instance_id: number
  reply_count: number
  created_at: string
  updated_at: string
}

// 工单列表响应
export interface TicketListResponse {
  items: TicketInfo[]
  meta: ListMeta
}

// 工单附件（S2）：用户端只会拿到非内部附件
export interface TicketAttachmentInfo {
  id: number
  ticket_id: number
  reply_id: number
  file_name: string
  file_url: string
  file_size: number
  file_type: string
  is_internal: boolean
  created_at: string
}

// 工单回复信息
export interface TicketReplyInfo {
  id: number
  ticket_id: number
  sender_type: string // user / admin
  sender_id: number
  sender_name: string
  content: string
  attachments: TicketAttachmentInfo[]
  created_at: string
}

// 工单详情（含对话回复记录）
export interface TicketDetail extends TicketInfo {
  description: string
  first_reply_at: string
  resolved_at: string
  closed_at: string
  replies: TicketReplyInfo[]
  /** 工单主附件（未挂到具体回复的，S2） */
  attachments: TicketAttachmentInfo[]
}

// 工单分类信息（S2：含提交前置条件）
export interface TicketCategoryInfo {
  id: number
  name: string
  code: string
  description: string
  /** 提交需已完成实名认证 */
  require_realname: boolean
  /** 提交需关联本人订单或实例 */
  require_binding: boolean
  /** 管理员回复需双人复核（用户端仅提示，不阻断提交） */
  need_review: boolean
}

// 分类列表响应：附带当前账号实名状态，省一次额外请求
export interface TicketCategoryListResponse {
  items: TicketCategoryInfo[]
  realname_ok: boolean
}

// 提交工单请求
export interface TicketCreateRequest {
  title: string
  description: string
  category: string
  priority?: string
  order_id?: number
  instance_id?: number
  /** 提交时一并携带的附件 ID（先调上传接口拿 ID） */
  attachment_ids?: number[]
}

// 查询我的工单列表
export function getMyTickets(params: { page?: number; page_size?: number } = {}) {
  return request.get<any, { data: TicketListResponse }>('/uc/support/tickets', { params })
}

// 查询我的工单详情（含回复记录）
export function getMyTicketDetail(id: string | number) {
  return request.get<any, { data: TicketDetail }>(`/uc/support/tickets/${id}`)
}

// 提交工单
export function createMyTicket(data: TicketCreateRequest) {
  return request.post<any, { data: TicketDetail }>('/uc/support/tickets', data)
}

// 追加工单回复（可携带已上传的附件 ID）
export function replyMyTicket(id: string | number, content: string, attachmentIds: number[] = []) {
  return request.post<any, { data: TicketDetail }>(`/uc/support/tickets/${id}/replies`, {
    content,
    attachment_ids: attachmentIds,
  })
}

// 取消工单（仅待处理 open 状态可取消）
export function cancelMyTicket(id: string | number) {
  return request.post<any, { data: TicketDetail }>(`/uc/support/tickets/${id}/cancel`)
}

// 获取可用工单分类列表（含前置条件与当前账号实名状态）
export function getTicketCategories() {
  return request.get<any, { data: TicketCategoryListResponse }>('/uc/support/ticket-categories')
}

// 上传我的工单附件：新建工单前 ticket 尚未存在，传 0 表示待挂载
export function uploadMyTicketAttachment(id: string | number, file: File) {
  const form = new FormData()
  form.append('file', file)
  return request.post<any, { data: TicketAttachmentInfo }>(`/uc/support/tickets/${id}/attachments`, form)
}

// 下载我的工单附件（带鉴权，浏览器保存）
//
// 后端错误也走 HTTP 200 + JSON 信封，因此这里需要先辨别响应体是文件还是错误包，
// 否则会把 {"code":20002,...} 当成附件存成文件。
export async function downloadMyTicketAttachment(id: number, fileName = 'attachment') {
  const response = await request.get<any, any>(`/uc/support/attachments/${id}/download`, { responseType: 'blob' })
  const blob = (response?.data ?? response) as Blob
  if (blob.type && blob.type.includes('application/json')) {
    const text = await blob.text()
    let message = '附件下载失败'
    try {
      message = JSON.parse(text)?.message || message
    } catch {
      /* 保留默认文案 */
    }
    throw new Error(message)
  }
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = fileName
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(url)
}
