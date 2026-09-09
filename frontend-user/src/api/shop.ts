import request from '@/utils/request'

export interface ProductInfo {
  id: number
  name: string
  description: string
  category_id: number
  product_type: string
  price: number
  price_model: string
  specs: string
  provision_mode: string
  config_options: string
  featured: boolean
  created_at: string
}

export interface ProductListResponse {
  items: ProductInfo[]
  page: number
  page_size: number
  total: number
}

export interface OrderInfo {
  id: number
  order_no: string
  product_id: number
  product_name: string
  specs: string
  price_model: string
  total_amount: number
  paid_amount: number
  status: string
  pay_method: string
  created_at: string
}

export interface OrderListResponse {
  items: OrderInfo[]
  page: number
  page_size: number
  total: number
}

export function getProducts(params: { keyword?: string; page?: number; page_size?: number } = {}) {
  return request.get<any, { data: ProductListResponse }>('/uc/products', { params })
}

export function getProductDetail(id: number) {
  return request.get<any, { data: ProductInfo }>(`/uc/products/${id}`)
}

export function createOrder(productId: number) {
  return request.post<any, { data: OrderInfo }>('/uc/orders', { product_id: productId })
}

export function getMyOrders(params: { status?: string; page?: number; page_size?: number } = {}) {
  return request.get<any, { data: OrderListResponse }>('/uc/orders', { params })
}
