import { request } from '@/utils/request'

import type {
  BatchActionResponse,
  RefundAttachmentInfo,
  RefundPolicyInfo,
  RefundPolicyListResponse,
  RefundPolicyRequest,
  RefundReviewRequest,
  BatchRefundAuditRequest,
  BatchVerifyRequest,
  OrderDetail,
  OrderListQuery,
  OrderListResponse,
  OrderRemarkUpdateRequest,
  OrderStatsResponse,
  RefundApproveRequest,
  RefundCreateRequest,
  RefundInfo,
  RefundListQuery,
  RefundListResponse,
} from '@/types/interface'

// ===== 订单管理 =====

export function getOrderList(params: OrderListQuery): Promise<OrderListResponse> {
  return request.get<OrderListResponse>({
    url: '/orders',
    params: {
      keyword: params.keyword,
      user_keyword: params.user_keyword,
      user_id: params.user_id,
      product_id: params.product_id,
      status: params.status,
      pay_method: params.pay_method,
      payment_no: params.payment_no,
      channel_tx: params.channel_tx,
      order_type: params.order_type,
      verify_state: params.verify_state,
      start_time: params.start_time,
      end_time: params.end_time,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getOrderDetail(id: number): Promise<OrderDetail> {
  return request.get<OrderDetail>({
    url: `/orders/${id}`,
  })
}

export function cancelOrder(id: number): Promise<string> {
  return request.post<string>({
    url: `/orders/${id}/cancel`,
  })
}

export function updateOrderRemark(id: number, data: OrderRemarkUpdateRequest): Promise<string> {
  return request.put<string>({
    url: `/orders/${id}/remark`,
    data,
  })
}

export function createOrderRefund(id: number, data: RefundCreateRequest): Promise<RefundInfo> {
  return request.post<RefundInfo>({
    url: `/orders/${id}/refund`,
    data,
  })
}

export function activateOrder(id: number): Promise<string> {
  return request.post<string>({
    url: `/orders/${id}/activate`,
  })
}

export function verifyOrder(id: number, remark = ''): Promise<string> {
  return request.post<string>({
    url: `/orders/${id}/verify`,
    data: { remark },
  })
}

// ===== 批量操作（逐条复用单条逻辑，返回成功/跳过/失败明细） =====

export function batchCancelOrders(ids: number[]): Promise<BatchActionResponse> {
  return request.post<BatchActionResponse>({ url: '/orders/batch-cancel', data: { ids } })
}

export function batchActivateOrders(ids: number[]): Promise<BatchActionResponse> {
  return request.post<BatchActionResponse>({ url: '/orders/batch-activate', data: { ids } })
}

export function batchVerifyOrders(data: BatchVerifyRequest): Promise<BatchActionResponse> {
  return request.post<BatchActionResponse>({ url: '/orders/batch-verify', data })
}

export function batchDeleteOrders(ids: number[]): Promise<BatchActionResponse> {
  return request.post<BatchActionResponse>({ url: '/orders/batch-delete', data: { ids } })
}

export function batchApproveRefunds(data: BatchRefundAuditRequest): Promise<BatchActionResponse> {
  return request.post<BatchActionResponse>({ url: '/refunds/batch-approve', data })
}

export function batchRejectRefunds(data: BatchRefundAuditRequest): Promise<BatchActionResponse> {
  return request.post<BatchActionResponse>({ url: '/refunds/batch-reject', data })
}

export function getOrderStats(): Promise<OrderStatsResponse> {
  return request.get<OrderStatsResponse>({
    url: '/orders/stats',
  })
}

// ===== 退款管理 =====

export function getRefundList(params: RefundListQuery): Promise<RefundListResponse> {
  return request.get<RefundListResponse>({
    url: '/refunds',
    params: {
      keyword: params.keyword,
      order_id: params.order_id,
      status: params.status,
      refund_mode: params.refund_mode,
      origin: params.origin,
      review_state: params.review_state,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getRefundDetail(id: number): Promise<RefundInfo> {
  return request.get<RefundInfo>({
    url: `/refunds/${id}`,
  })
}

export function approveRefund(id: number, data: RefundApproveRequest): Promise<RefundInfo> {
  return request.post<RefundInfo>({
    url: `/refunds/${id}/approve`,
    data,
  })
}

export function rejectRefund(id: number, data: RefundApproveRequest): Promise<RefundInfo> {
  return request.post<RefundInfo>({
    url: `/refunds/${id}/reject`,
    data,
  })
}

// ===== 退款策略（按商品来源 + 分类的可退时限/免审配置） =====

export function getRefundPolicies(): Promise<RefundPolicyListResponse> {
  return request.get<RefundPolicyListResponse>({ url: '/orders/refund-policies' })
}

export function createRefundPolicy(data: RefundPolicyRequest): Promise<RefundPolicyInfo> {
  return request.post<RefundPolicyInfo>({ url: '/orders/refund-policies', data })
}

export function updateRefundPolicy(id: number, data: RefundPolicyRequest): Promise<RefundPolicyInfo> {
  return request.put<RefundPolicyInfo>({ url: `/orders/refund-policies/${id}`, data })
}

export function deleteRefundPolicy(id: number): Promise<string> {
  return request.delete<string>({ url: `/orders/refund-policies/${id}` })
}

// ===== 退款凭据与事后复核 =====

export function uploadRefundAttachment(id: number, file: File): Promise<RefundAttachmentInfo> {
  const form = new FormData()
  form.append('file', file)
  return request.post<RefundAttachmentInfo>({ url: `/refunds/${id}/attachments`, data: form })
}

export function reviewRefund(id: number, data: RefundReviewRequest): Promise<RefundInfo> {
  return request.post<RefundInfo>({ url: `/refunds/${id}/review`, data })
}

export function downloadRefundAttachment(id: number, fileName = 'attachment'): Promise<void> {
  return request
    .get<Blob>({ url: `/refunds/attachments/${id}/download`, responseType: 'blob', _skipResultUnwrap: true } as never)
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
