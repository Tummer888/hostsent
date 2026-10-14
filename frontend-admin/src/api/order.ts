import { request } from '@/utils/request'

import type {
  BatchActionResponse,
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
