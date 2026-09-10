import { extractStatusCode } from '#shared/schemas/api'
import { EMPTY_PRODUCT_LIST, type Product, type ProductList } from '#shared/schemas/product'

/**
 * 商品数据获取（规范 R2/R3：走 BFF，且必须有默认值兜底）。
 *
 * useAsyncData 以查询条件为 key 去重，同一请求内多处调用同一条件只取一次；
 * SSR 与客户端首屏复用同一份数据，避免重复请求。
 */

export interface UseProductsOptions {
  /** 仅取推荐位商品（官网首页「热门产品」）。 */
  featured?: boolean
  page?: number
  pageSize?: number
  categoryId?: number
  keyword?: string
}

export function useProducts(options: UseProductsOptions = {}) {
  const params = new URLSearchParams()
  if (options.featured !== undefined) params.set('featured', String(options.featured))
  if (options.page) params.set('page', String(options.page))
  if (options.pageSize) params.set('page_size', String(options.pageSize))
  if (options.categoryId) params.set('category_id', String(options.categoryId))
  if (options.keyword) params.set('keyword', options.keyword)

  const queryString = params.toString()
  const endpoint = `/api/products${queryString ? `?${queryString}` : ''}`

  const { data, pending, error } = useAsyncData<ProductList>(
    `products:${queryString || 'all'}`,
    () => $fetch<ProductList>(endpoint),
    { default: () => EMPTY_PRODUCT_LIST },
  )

  const list = computed<ProductList>(() => data.value ?? EMPTY_PRODUCT_LIST)

  return {
    list,
    items: computed<Product[]>(() => list.value.items),
    total: computed(() => list.value.total),
    pending,
    error,
  }
}

/** 详情页加载状态：区分「内容不存在」与「服务临时不可用」，避免 SEO 误判。 */
export type ProductLoadState = 'ok' | 'not-found' | 'unavailable'

/**
 * 商品详情。
 *
 * 返回值即 `useAsyncData` 的结果对象（本身可 await），并附带 `product` / `state`。
 * 详情页需 `await useProduct(id)` 才能在 setup 阶段拿到最终状态并抛出正确的 404/503；
 * 若在 setup 里同步读取，SSR 首屏时数据尚未就绪，会把正常商品误判为已下线。
 */
export function useProduct(id: MaybeRefOrGetter<number | string>) {
  const productId = computed(() => Number(toValue(id)))

  const asyncData = useAsyncData<Product | null>(
    `product:${productId.value}`,
    () => $fetch<Product | null>(`/api/products/${productId.value}`),
    { default: () => null },
  )

  const product = computed<Product | null>(() => asyncData.data.value ?? null)

  const state = computed<ProductLoadState>(() => {
    if (product.value) return 'ok'
    // 后端明确 404 = 内容不存在；无状态码或 5xx = 服务故障
    const status = extractStatusCode(asyncData.error.value)
    if (status === undefined || status >= 500) return 'unavailable'
    return 'not-found'
  })

  return Object.assign(asyncData, { product, state })
}
