// 类型定义（拆分自 interface.d.ts：order 域）
import type { ListMeta } from './common'

export interface OrderListQuery {
  keyword?: string
  user_keyword?: string
  user_id?: number
  product_id?: number
  status?: string
  pay_method?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface OrderInfo {
  id: number
  order_no: string
  user_id: number
  product_id: number
  product_name: string
  specs: string
  quantity: number
  price_model: string
  total_amount: number
  paid_amount: number
  status: string
  pay_method: string
  pay_time: string
  expire_time: string
  remark: string
  created_at: string
  updated_at: string
}

export interface OrderListResponse {
  items: OrderInfo[]
  meta: ListMeta
}

export interface OrderItemInfo {
  id: number
  order_id: number
  product_id: number
  product_name: string
  spec_code: string
  specs: string
  price: number
  quantity: number
  amount: number
  created_at: string
}

export interface OrderDetail extends OrderInfo {
  items: OrderItemInfo[]
  refunds: RefundInfo[]
}

export interface OrderRemarkUpdateRequest {
  remark: string
}

export interface RefundCreateRequest {
  amount: number
  reason?: string
}

export interface OrderStatusStat {
  status: string
  count: number
}

export interface OrderTrendPoint {
  date: string
  sales: number
  orders: number
}

export interface OrderStatsResponse {
  today_sales: number
  today_orders: number
  avg_order_value: number
  refund_rate: number
  status_distribution: OrderStatusStat[]
  trend: OrderTrendPoint[]
}

export interface RefundListQuery {
  keyword?: string
  order_id?: number
  status?: string
  page?: number
  page_size?: number
}

export interface RefundInfo {
  id: number
  refund_no: string
  order_id: number
  order_no: string
  user_id: number
  amount: number
  reason: string
  status: string
  audit_by: number
  audit_by_name: string
  audited_at: string
  created_at: string
  updated_at: string
}

export interface RefundListResponse {
  items: RefundInfo[]
  meta: ListMeta
}

export interface RefundApproveRequest {
  remark?: string
}
