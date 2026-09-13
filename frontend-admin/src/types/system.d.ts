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
  /** 复核状态（S3）：pending / approved / rejected */
  review_status?: string
  /** 部门筛选（S2）：与登录员工的数据范围取交集 */
  department_id?: number
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
  /** 归属部门快照（S2） */
  department_id: number
  department_name: string
  order_id: number
  instance_id: number
  reply_count: number
  /** 复核状态（S3）：pending 时列表高亮提示 */
  review_status: string
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

export interface TicketAttachmentInfo {
  id: number
  ticket_id: number
  reply_id: number
  file_name: string
  file_url: string
  file_size: number
  file_type: string
  uploader_id: number
  /** 上传者展示名（仅管理端返回） */
  uploader_name?: string
  /** 内部附件（S2）：用户端不可见 */
  is_internal: boolean
  created_at: string
}

export interface TicketReplyInfo {
  id: number
  ticket_id: number
  sender_type: string // user / admin
  sender_id: number
  sender_name: string
  content: string
  /** 内部备注（S2）：仅管理端可见 */
  is_internal: boolean
  /** 复核状态（S3）：pending / approved / rejected / 空 */
  review_status: string
  reviewer_id?: number
  reviewer_name?: string
  reviewed_at?: string
  review_note?: string
  attachments: TicketAttachmentInfo[]
  created_at: string
}

export interface TicketLogInfo {
  id: number
  ticket_id: number
  operator_id: number
  operator_name: string
  action: string // create/assign/claim/transfer/reply/internal_note/review_request/review/status/close/cancel
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
  /** 工单主附件（未挂到具体回复的，S2） */
  attachments: TicketAttachmentInfo[]
  /** 工单级最近复核意见（S3） */
  review_note: string
  reviewer_name?: string
  reviewed_at?: string
}

export interface TicketReplyRequest {
  content: string
  /** 内部备注（S2）：需 ticket:internal_note 权限 */
  is_internal?: boolean
  /** 随回复携带的附件 ID（先上传拿 ID） */
  attachment_ids?: number[]
}

export interface TicketReviewRequest {
  action: 'approve' | 'reject'
  /** 复核意见；驳回必填 */
  note?: string
}

/** 待复核回复队列项（S3 复核中心） */
export interface ReviewQueueItem {
  reply_id: number
  ticket_id: number
  ticket_no: string
  title: string
  category: string
  category_name: string
  department_id: number
  department_name: string
  sender_id: number
  sender_name: string
  content: string
  /** 已等待复核秒数 */
  waiting_seconds: number
  created_at: string
}

export interface ReviewQueueResponse {
  items: ReviewQueueItem[]
  meta: ListMeta
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
  /** 自动派单目标客服组 ID（deprecated，S2 起按 department_id 派单） */
  default_group_id?: number
  /** 首次响应时限（小时），0 表示不启用 */
  sla_hours?: number
  /** 归属部门（S2）：决定派单候选与数据范围 */
  department_id?: number
  /** 提交前置条件：要求已实名 */
  require_realname?: boolean
  /** 提交前置条件：要求关联本人订单或实例 */
  require_binding?: boolean
  /** 管理员回复需双人复核（S3） */
  need_review?: boolean
  /** 仅这些用户角色可提交（空=不限） */
  visible_role_codes?: string[]
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
  /** 归属部门（S2） */
  department_id: number
  department_name: string
  require_realname: boolean
  require_binding: boolean
  need_review: boolean
  visible_role_codes: string[]
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
