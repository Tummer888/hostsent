// 类型定义（拆分自 interface.d.ts：system 域）
import type { ListMeta } from './common'

export interface TicketListQuery {
  keyword?: string
  user_keyword?: string
  user_id?: number
  category?: string
  priority?: string
  status?: string
  assigned_to?: number
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

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

export interface TicketListResponse {
  items: TicketInfo[]
  meta: ListMeta
}

export interface TicketReplyInfo {
  id: number
  ticket_id: number
  sender_type: string // user / admin
  sender_id: number
  sender_name: string
  content: string
  created_at: string
}

export interface TicketDetail extends TicketInfo {
  description: string
  first_reply_at: string
  resolved_at: string
  closed_at: string
  replies: TicketReplyInfo[]
}

export interface TicketReplyRequest {
  content: string
}

export interface TicketAssignRequest {
  assigned_to: number
}

export interface TicketStatusRequest {
  status: string
}

export interface TicketCategorySaveRequest {
  name: string
  code: string
  description?: string
  sort_order?: number
  status?: string
}

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

export interface TicketStatusStat {
  status: string
  count: number
}

export interface TicketTrendPoint {
  date: string
  count: number
}

export interface TicketCategoryStat {
  category: string
  count: number
}

export interface TicketStatsResponse {
  total: number
  open: number
  in_progress: number
  waiting_user: number
  resolved: number
  closed: number
  avg_first_reply_seconds: number
  status_distribution: TicketStatusStat[]
  trend: TicketTrendPoint[]
  category_distribution: TicketCategoryStat[]
}
