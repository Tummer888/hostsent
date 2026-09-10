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
  /** 工作台视图：my_todo / unassigned / involved / sla_breached */
  view?: string
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
  /** SLA 首次响应时限（小时），0 表示未启用 */
  sla_hours: number
  /** 是否已超时未首次响应 */
  sla_breached: boolean
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

export interface TicketLogInfo {
  id: number
  ticket_id: number
  operator_id: number
  operator_name: string
  action: string // create/assign/claim/transfer/reply/status/close/cancel
  from_value: string
  to_value: string
  note: string
  created_at: string
}

export interface TicketDetail extends TicketInfo {
  description: string
  first_reply_at: string
  resolved_at: string
  closed_at: string
  replies: TicketReplyInfo[]
  logs: TicketLogInfo[]
}

export interface TicketReplyRequest {
  content: string
}

export interface TicketAssignRequest {
  assigned_to: number
}

export interface TicketTransferRequest {
  to_id: number
  note?: string
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
  /** 自动派单目标角色 code */
  default_role_code?: string
  /** 自动派单目标客服组 ID */
  default_group_id?: number
  /** 首次响应时限（小时），0 表示不启用 */
  sla_hours?: number
}

export interface TicketCategoryInfo {
  id: number
  name: string
  code: string
  description: string
  sort_order: number
  status: string
  default_role_code: string
  default_group_id: number
  sla_hours: number
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
