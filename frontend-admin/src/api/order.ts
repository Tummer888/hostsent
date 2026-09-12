import { request } from '@/utils/request'

import type {
  OrderDetail,
  OrderInfo,
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
