import request from '@/utils/request'

// 用户中心 · 工单支持接口（baseURL=/api/v1，路径拼接为 /api/v1/uc/support/*）
// 注意：本项目 request 拦截器返回的是完整信封 { code, message, data }，因此各函数返回 .data。

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

// 工单回复信息
export interface TicketReplyInfo {
  id: number
  ticket_id: number
  sender_type: string // user / admin
  sender_id: number
  sender_name: string
  content: string
  created_at: string
}

// 工单详情（含对话回复记录）
export interface TicketDetail extends TicketInfo {
  description: string
  first_reply_at: string
  resolved_at: string
  closed_at: string
  replies: TicketReplyInfo[]
}

// 工单分类信息
export interface TicketCategoryInfo {
  id: number
  name: string
  code: string
  description: string
  sort_order: number
  status: string
  created_at: string
  updated_at: string
}

// 提交工单请求
export interface TicketCreateRequest {
  title: string
  description: string
  category: string
  priority?: string
  order_id?: number
  instance_id?: number
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

// 追加工单回复
export function replyMyTicket(id: string | number, content: string) {
  return request.post<any, { data: TicketDetail }>(`/uc/support/tickets/${id}/replies`, { content })
}

// 取消工单（仅待处理 open 状态可取消）
export function cancelMyTicket(id: string | number) {
  return request.post<any, { data: TicketDetail }>(`/uc/support/tickets/${id}/cancel`)
}

// 获取可用工单分类列表
export function getTicketCategories() {
  return request.get<any, { data: TicketCategoryInfo[] }>('/uc/support/ticket-categories')
}
