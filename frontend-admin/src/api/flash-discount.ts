// 限时活动折扣 API（doc108 §8J）。
//
// 与代理折扣（agent-level.ts）的边界：
//   - agent-level.ts：面向**代理**的拿货折扣，长期有效，落在 agent_level_discounts 逐格矩阵；
//   - 本文件：面向**普通用户**的营销活动，按生效窗口自动开始/结束。
//
// 两者互不叠加：代理用户不参与活动折扣（后端在算价时按身份判断）。
import { request } from '@/utils/request'

/** 折扣类型：折扣率（0.85 = 八五折）/ 直减固定金额。 */
export type FlashDiscountType = 'rate' | 'amount'

/** 无定向条目时的作用范围。 */
export type FlashDiscountScope = 'all' | 'category' | 'product'

/** 运行态：按当前时间窗口实时推导（不是存库字段）。 */
export type FlashRuntimeStatus = 'pending' | 'running' | 'expired' | 'disabled'

export interface FlashDiscountItem {
  target_type: 'category' | 'product'
  target_id: number
  target_name?: string
}

export interface FlashDiscountInfo {
  id: number
  name: string
  code: string
  description?: string
  discount_type: FlashDiscountType
  discount_value: number
  scope: FlashDiscountScope
  /** 生效开始；空串 = 立即开始。 */
  start_at: string
  /** 生效结束；空串 = 不限结束。 */
  end_at: string
  /** 启停意图：active / disabled。 */
  status: string
  /** 运行态（后端按当前时间推导）：pending / running / expired / disabled。 */
  runtime_status: FlashRuntimeStatus
  remark?: string
  item_count: number
  items: FlashDiscountItem[]
  created_at: string
  updated_at: string
}

export interface FlashDiscountRequest {
  name: string
  code: string
  description?: string
  discount_type: FlashDiscountType
  discount_value: number
  scope: FlashDiscountScope
  start_at?: string
  end_at?: string
  status?: string
  remark?: string
  /** 不传 = 不改条目；传（含空数组）= 整体覆盖。 */
  items?: Array<{ target_type: 'category' | 'product'; target_id: number }>
}

export interface FlashDiscountListQuery {
  page?: number
  page_size?: number
  keyword?: string
  status?: string
  /** 只看此刻进行中的活动（按时间窗实时判断）。 */
  running?: boolean
}

export function getFlashDiscountList(
  params: FlashDiscountListQuery = {},
): Promise<{ items: FlashDiscountInfo[]; meta: { page: number; page_size: number; total: number } }> {
  return request.get({
    url: '/product/promotion/flash-discounts',
    params: {
      page: params.page,
      page_size: params.page_size,
      keyword: params.keyword,
      status: params.status,
      running: params.running ? 'true' : undefined,
    },
  })
}

export function getFlashDiscountDetail(id: number): Promise<FlashDiscountInfo> {
  return request.get({ url: `/product/promotion/flash-discounts/${id}` })
}

export function createFlashDiscount(data: FlashDiscountRequest): Promise<FlashDiscountInfo> {
  return request.post<FlashDiscountInfo>({ url: '/product/promotion/flash-discounts', data })
}

export function updateFlashDiscount(id: number, data: FlashDiscountRequest): Promise<FlashDiscountInfo> {
  return request.put<FlashDiscountInfo>({ url: `/product/promotion/flash-discounts/${id}`, data })
}

export function deleteFlashDiscount(id: number): Promise<string> {
  return request.delete<string>({ url: `/product/promotion/flash-discounts/${id}` })
}
