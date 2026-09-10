import type { H3Event } from 'h3'
import { unwrapEnvelope } from '#shared/schemas/api'
import {
  EMPTY_PRODUCT_LIST,
  parseProduct,
  parseProductList,
  type Product,
  type ProductList,
} from '#shared/schemas/product'

/**
 * 商品数据读取的服务端共用逻辑。
 *
 * BFF 路由（server/api/products.*）与 sitemap 生成都需要读商品，
 * 收敛在这里避免两处各写一遍参数白名单、信封拆解与校验兜底。
 */

export interface CatalogQuery {
  keyword?: string
  categoryId?: number
  featured?: boolean
  page?: number
  pageSize?: number
}

/** sitemap 一次最多枚举的商品数（与后端 repo 的 page_size 上限一致）。 */
export const CATALOG_MAX_PAGE_SIZE = 100

function toUpstreamQuery(query: CatalogQuery): Record<string, string> {
  const upstream: Record<string, string> = {}
  if (query.keyword) upstream.keyword = query.keyword
  if (query.categoryId) upstream.category_id = String(query.categoryId)
  if (query.featured !== undefined) upstream.featured = String(query.featured)
  if (query.page) upstream.page = String(query.page)
  if (query.pageSize) upstream.page_size = String(query.pageSize)
  return upstream
}

/** 读取商品列表；后端未配置或异常时返回空列表。 */
export async function fetchProductList(event: H3Event, query: CatalogQuery = {}): Promise<ProductList> {
  const { public: publicConfig } = useRuntimeConfig(event)
  if (!publicConfig.apiBase) return EMPTY_PRODUCT_LIST

  try {
    const raw = await $fetch(`${publicConfig.apiBase}/api/v1/uc/products`, {
      query: toUpstreamQuery(query),
      timeout: 3000,
    })
    const data = unwrapEnvelope(raw)
    return data === null ? EMPTY_PRODUCT_LIST : parseProductList(data)
  } catch (error) {
    console.warn('[catalog] 读取商品列表失败，返回空列表:', error)
    return EMPTY_PRODUCT_LIST
  }
}

/** 读取商品详情；未配置/业务失败/校验不通过返回 null。网络异常同样返回 null。 */
export async function fetchProductDetail(event: H3Event, id: number): Promise<Product | null> {
  const { public: publicConfig } = useRuntimeConfig(event)
  if (!publicConfig.apiBase) return null

  const raw = await $fetch(`${publicConfig.apiBase}/api/v1/uc/products/${id}`, { timeout: 3000 })
  const data = unwrapEnvelope(raw)
  return data === null ? null : parseProduct(data)
}
