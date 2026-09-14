import { isArticleKind } from '#shared/schemas/content'
import { fetchArticleDetail } from '../../../utils/content'

/**
 * 内容详情 BFF 代理（`/news/:slug`、`/help/:slug`）。
 *
 * 状态码语义刻意区分（doc80 R12，与 `products/[id].get.ts` 同一范本）：
 *  - 后端明确返回业务失败（内容不存在 / 已下线）→ 404，允许搜索引擎移除该页；
 *  - 后端网络异常/超时 → 503，属于临时故障，不能误判成内容消失。
 */
export default defineEventHandler(async (event) => {
  const kind = String(getRouterParam(event, 'kind') ?? '').trim()
  const slug = String(getRouterParam(event, 'slug') ?? '').trim()

  if (!isArticleKind(kind) || kind === 'terms' || kind === 'privacy' || !slug) {
    // 条款 / 隐私是「每类型一篇」，没有 slug 语义，走 /api/article-singletons。
    throw createError({ statusCode: 404, statusMessage: 'Article not found' })
  }

  let article: Awaited<ReturnType<typeof fetchArticleDetail>>
  try {
    article = await fetchArticleDetail(event, kind, slug)
  } catch (error) {
    console.warn(`[content] 读取内容详情失败 kind=${kind} slug=${slug}:`, error)
    throw createError({ statusCode: 503, statusMessage: 'Content service unavailable' })
  }

  if (!article) {
    throw createError({ statusCode: 404, statusMessage: 'Article not found' })
  }

  setHeader(event, 'cache-control', 'public, s-maxage=600, stale-while-revalidate=1800')
  return article
})
