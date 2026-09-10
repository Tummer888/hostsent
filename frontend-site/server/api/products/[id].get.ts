import { fetchProductDetail } from '../../utils/catalog'

/**
 * 商品详情 BFF 代理。
 *
 * 上游 `GET /api/v1/uc/products/:id`（公开无需登录，仅上架商品可读）。
 *
 * 状态码语义刻意区分，避免影响 SEO：
 *  - 后端明确返回业务失败（商品不存在/已下架）→ 404，允许搜索引擎移除该页；
 *  - 后端网络异常/超时 → 503，属于临时故障，不能误判成内容消失。
 */
export default defineEventHandler(async (event) => {
  const id = Number(getRouterParam(event, 'id'))
  if (!Number.isInteger(id) || id <= 0) {
    throw createError({ statusCode: 404, statusMessage: 'Product not found' })
  }

  let product: Awaited<ReturnType<typeof fetchProductDetail>>
  try {
    product = await fetchProductDetail(event, id)
  } catch (error) {
    console.warn(`[products] 读取商品详情失败 id=${id}:`, error)
    throw createError({ statusCode: 503, statusMessage: 'Product service unavailable' })
  }

  if (!product) {
    // 后端未配置 apiBase 时也落到这里：官网可脱离后端独立开发，此时视为无内容
    throw createError({ statusCode: 404, statusMessage: 'Product not found' })
  }

  setHeader(event, 'cache-control', 'public, s-maxage=600, stale-while-revalidate=3600')
  return product
})
