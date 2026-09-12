import { request } from '@/utils/request'

import type {
  CallbackListQuery,
  CallbackListResponse,
  ChannelCreateRequest,
  ChannelInfo,
  ChannelListQuery,
  ChannelListResponse,
  ChannelTestResponse,
  ChannelTypeItem,
  ChannelUpdateRequest,
  MethodOptionsResponse,
  OrderConfirmRequest,
  PaymentOrderInfo,
  PaymentOrderListQuery,
  PaymentOrderListResponse,
  PayoutInfo,
  PayoutListQuery,
  PayoutListResponse,
  PayoutMarkPaidRequest,
  ReconRecordListResponse,
  ReconRequest,
  ReconResponse,
  PaymentRefundListQuery,
  PaymentRefundListResponse,
} from '@/types/interface'

// ===== 渠道类型（能力矩阵 / 动态凭证表单） =====

export function getChannelTypes(): Promise<{ items: ChannelTypeItem[] }> {
  return request.get<{ items: ChannelTypeItem[] }>({ url: '/payment/channel-types' })
}

// ===== 渠道实例 =====

export function getChannelList(params: ChannelListQuery): Promise<ChannelListResponse> {
  return request.get<ChannelListResponse>({
    url: '/payment/channels',
    params: {
      type: params.type,
      status: params.status,
      keyword: params.keyword,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getChannel(id: number): Promise<ChannelInfo> {
  return request.get<ChannelInfo>({ url: `/payment/channels/${id}` })
}

export function createChannel(data: ChannelCreateRequest): Promise<ChannelInfo> {
  return request.post<ChannelInfo>({ url: '/payment/channels', data })
}

export function updateChannel(id: number, data: ChannelUpdateRequest): Promise<ChannelInfo> {
  return request.put<ChannelInfo>({ url: `/payment/channels/${id}`, data })
}

export function updateChannelStatus(id: number, status: number): Promise<void> {
  return request.patch<void>({ url: `/payment/channels/${id}/status`, data: { status } })
}

export function testChannel(id: number): Promise<ChannelTestResponse> {
  return request.post<ChannelTestResponse>({ url: `/payment/channels/${id}/test` })
}

// ===== 支付方式路由 =====

export function getMethodOptions(scene: string): Promise<MethodOptionsResponse> {
  return request.get<MethodOptionsResponse>({ url: '/payment/methods', params: { scene } })
}

// ===== 支付单 =====

export function getPaymentOrders(params: PaymentOrderListQuery): Promise<PaymentOrderListResponse> {
  return request.get<PaymentOrderListResponse>({
    url: '/payment/orders',
    params: {
      user_id: params.user_id,
      payment_no: params.payment_no,
      biz_type: params.biz_type,
      channel_code: params.channel_code,
      status: params.status,
      start_time: params.start_time,
      end_time: params.end_time,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function getPaymentOrder(id: number): Promise<PaymentOrderInfo> {
  return request.get<PaymentOrderInfo>({ url: `/payment/orders/${id}` })
}

export function confirmPaymentOrder(id: number, data: OrderConfirmRequest): Promise<PaymentOrderInfo> {
  return request.post<PaymentOrderInfo>({ url: `/payment/orders/${id}/confirm`, data })
}

export function closePaymentOrder(id: number): Promise<PaymentOrderInfo> {
  return request.post<PaymentOrderInfo>({ url: `/payment/orders/${id}/close` })
}

export function syncPaymentOrder(id: number): Promise<PaymentOrderInfo> {
  return request.post<PaymentOrderInfo>({ url: `/payment/orders/${id}/sync` })
}

// ===== 回调日志 =====

export function getCallbacks(params: CallbackListQuery): Promise<CallbackListResponse> {
  return request.get<CallbackListResponse>({
    url: '/payment/callbacks',
    params: {
      channel_code: params.channel_code,
      payment_no: params.payment_no,
      verify_ok: params.verify_ok,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

// ===== 渠道退款单 =====

export function getRefunds(params: PaymentRefundListQuery): Promise<PaymentRefundListResponse> {
  return request.get<PaymentRefundListResponse>({
    url: '/payment/refunds',
    params: {
      payment_no: params.payment_no,
      user_id: params.user_id,
      status: params.status,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

// ===== 打款单 =====

export function getPayouts(params: PayoutListQuery): Promise<PayoutListResponse> {
  return request.get<PayoutListResponse>({
    url: '/payment/payouts',
    params: {
      user_id: params.user_id,
      status: params.status,
      mode: params.mode,
      start_time: params.start_time,
      end_time: params.end_time,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function markPayoutPaid(id: number, data: PayoutMarkPaidRequest): Promise<PayoutInfo> {
  return request.post<PayoutInfo>({ url: `/payment/payouts/${id}/mark-paid`, data })
}

export function retryPayout(id: number): Promise<PayoutInfo> {
  return request.post<PayoutInfo>({ url: `/payment/payouts/${id}/retry` })
}

// ===== 渠道对账 =====

export function getReconRecords(page = 1, pageSize = 20): Promise<ReconRecordListResponse> {
  return request.get<ReconRecordListResponse>({
    url: '/payment/recon',
    params: { page, page_size: pageSize },
  })
}

export function runReconcile(data: ReconRequest): Promise<ReconResponse> {
  return request.post<ReconResponse>({ url: '/payment/recon', data })
}
