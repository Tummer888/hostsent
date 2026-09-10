import { request } from '@/utils/request'

import type {
  TicketAssignRequest,
  TicketCategoryInfo,
  TicketCategorySaveRequest,
  TicketDetail,
  TicketInfo,
  TicketListQuery,
  TicketListResponse,
  TicketReplyRequest,
  TicketStatsResponse,
  TicketStatusRequest,
  TicketTransferRequest,
} from '@/types/interface'

// ===== 工单管理（doc50） =====

/** 查询工单分页列表（view 支持工作台视图：my_todo/unassigned/involved/sla_breached） */
export function getTickets(params: TicketListQuery): Promise<TicketListResponse> {
  return request.get<TicketListResponse>({
    url: '/tickets',
    params: {
      keyword: params.keyword,
      user_keyword: params.user_keyword,
      user_id: params.user_id,
      category: params.category,
      priority: params.priority,
      status: params.status,
      assigned_to: params.assigned_to,
      start_time: params.start_time,
      end_time: params.end_time,
      view: params.view,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

/** 查询工单详情（含回复记录） */
export function getTicketDetail(id: number): Promise<TicketDetail> {
  return request.get<TicketDetail>({ url: `/tickets/${id}` })
}

/** 管理员回复工单 */
export function replyTicket(id: number, data: TicketReplyRequest): Promise<TicketDetail> {
  return request.post<TicketDetail>({ url: `/tickets/${id}/reply`, data })
}

/** 分配工单给指定管理员 */
export function assignTicket(id: number, data: TicketAssignRequest): Promise<TicketDetail> {
  return request.put<TicketDetail>({ url: `/tickets/${id}/assign`, data })
}

/** 认领未分配工单（P2-03） */
export function claimTicket(id: number): Promise<TicketDetail> {
  return request.post<TicketDetail>({ url: `/tickets/${id}/claim` })
}

/** 转派工单给其他员工（P2-03） */
export function transferTicket(id: number, data: TicketTransferRequest): Promise<TicketDetail> {
  return request.put<TicketDetail>({ url: `/tickets/${id}/transfer`, data })
}

/** 更新工单状态（状态机校验） */
export function updateTicketStatus(id: number, data: TicketStatusRequest): Promise<TicketDetail> {
  return request.put<TicketDetail>({ url: `/tickets/${id}/status`, data })
}

/** 关闭工单 */
export function closeTicket(id: number): Promise<TicketDetail> {
  return request.post<TicketDetail>({ url: `/tickets/${id}/close` })
}

/** 查询工单统计概览 */
export function getTicketStats(): Promise<TicketStatsResponse> {
  return request.get<TicketStatsResponse>({ url: '/tickets/stats' })
}

// ===== 工单分类（doc50） =====

/** 查询工单分类列表（管理端含禁用分类） */
export function getTicketCategories(): Promise<TicketCategoryInfo[]> {
  return request.get<TicketCategoryInfo[]>({ url: '/ticket-categories' })
}

/** 创建工单分类 */
export function createTicketCategory(data: TicketCategorySaveRequest): Promise<TicketCategoryInfo> {
  return request.post<TicketCategoryInfo>({ url: '/ticket-categories', data })
}

/** 更新工单分类 */
export function updateTicketCategory(id: number, data: TicketCategorySaveRequest): Promise<TicketCategoryInfo> {
  return request.put<TicketCategoryInfo>({ url: `/ticket-categories/${id}`, data })
}

/** 删除工单分类 */
export function deleteTicketCategory(id: number): Promise<string> {
  return request.delete<string>({ url: `/ticket-categories/${id}` })
}
