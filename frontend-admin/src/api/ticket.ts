import { request } from '@/utils/request'

import type {
  ReviewQueueResponse,
  TicketAssignRequest,
  TicketAttachmentInfo,
  TicketCategoryInfo,
  TicketCategorySaveRequest,
  TicketDetail,
  TicketInfo,
  TicketListQuery,
  TicketListResponse,
  TicketReplyRequest,
  TicketReviewRequest,
  TicketStatsResponse,
  TicketStatusRequest,
  TicketTransferRequest,
} from '@/types/interface'

// ===== 工单管理（doc50；S2/S3 部门范围与双人复核） =====

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
      review_status: params.review_status,
      department_id: params.department_id,
      assigned_to: params.assigned_to,
      start_time: params.start_time,
      end_time: params.end_time,
      view: params.view,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

/** 查询待复核回复队列（S3 复核中心） */
export function getTicketReviews(params: { page?: number; page_size?: number } = {}): Promise<ReviewQueueResponse> {
  return request.get<ReviewQueueResponse>({ url: '/tickets/reviews', params })
}

/** 复核待复核回复（S3）：通过 / 驳回，驳回必填 note */
export function reviewTicketReply(replyId: number, data: TicketReviewRequest): Promise<TicketDetail> {
  return request.post<TicketDetail>({ url: `/tickets/replies/${replyId}/review`, data })
}

/** 查询工单详情（含回复记录） */
export function getTicketDetail(id: number): Promise<TicketDetail> {
  return request.get<TicketDetail>({ url: `/tickets/${id}` })
}

/** 管理员回复工单（支持内部备注与附件） */
export function replyTicket(id: number, data: TicketReplyRequest): Promise<TicketDetail> {
  return request.post<TicketDetail>({ url: `/tickets/${id}/reply`, data })
}

/** 上传工单附件（先上传拿 ID，再随建单/回复提交） */
export function uploadTicketAttachment(id: number, file: File, isInternal = false): Promise<TicketAttachmentInfo> {
  const form = new FormData()
  form.append('file', file)
  if (isInternal) {
    form.append('is_internal', 'true')
  }
  return request.post<TicketAttachmentInfo>({ url: `/tickets/${id}/attachments`, data: form })
}

/** 下载工单附件（带鉴权，走 blob 保存） */
export function downloadTicketAttachment(id: number, fileName = 'attachment'): Promise<void> {
  return request
    .get<Blob>({ url: `/tickets/attachments/${id}/download`, responseType: 'blob', _skipResultUnwrap: true } as never)
    .then(async (response) => {
      const blob = (response as unknown as { data: Blob }).data
      // 后端错误也返回 HTTP 200 + JSON 信封，先辨别再落盘，避免把错误包存成附件。
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
    })
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

// ===== 工单分类（doc50；S2/S3 前置条件与复核开关） =====

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
