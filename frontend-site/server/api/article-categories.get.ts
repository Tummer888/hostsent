import { fetchArticleCategories } from '../utils/content'
import { isArticleKind, type ArticleKind } from '#shared/schemas/content'

/**
 * 内容分类树 BFF 代理（新闻分栏 / 帮助目录）。
 *
 * 上游 `GET /api/v1/public/article-categories?kind=`（公开无需登录，仅启用分类）。
 * 后端异常时返回空数组：侧栏退化为只有「全部」一项，页面其余部分不受影响。
 */
export default defineEventHandler(async (event) => {
  setHeader(event, 'cache-control', 'public, s-maxage=600, stale-while-revalidate=1800')

  const rawKind = String(getQuery(event).kind ?? '').trim()
  const kind: ArticleKind = isArticleKind(rawKind) ? rawKind : 'help'

  return fetchArticleCategories(event, kind)
})
