import { asQueryBool, asQueryInt, asQueryString } from '#shared/schemas/api'
import { fetchProductList } from '../utils/catalog'

/**
 * 商品列表 BFF 代理（规范 R2：组件不直连后端）。
 *
 * 上游 `GET /api/v1/uc/products`（公开无需登录，仅返回上架商品）。
 * 查询参数白名单透传，避免把公开接口变成后端任意参数的透传通道。
 * 后端未配置或异常时返回空列表，页面降级为空态而不是白屏。
 */
export default defineEventHandler(async (event) => {
  setHeader(event, 'cache-control', 'public, s-maxage=300, stale-while-revalidate=1800')

  const query = getQuery(event)
  return fetchProductList(event, {
    keyword: asQueryString(query.keyword),
    categoryId: asQueryInt(query.category_id),
    featured: asQueryBool(query.featured),
    page: asQueryInt(query.page),
    pageSize: asQueryInt(query.page_size),
  })
})
