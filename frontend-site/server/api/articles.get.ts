import { asQueryInt } from '#shared/schemas/api'
import { isArticleKind, type ArticleKind } from '#shared/schemas/content'
import { fetchArticleList } from '../utils/content'

/** 门户列表页默认每页条数。 */
const DEFAULT_PAGE_SIZE = 10

/**
 * 内容列表 BFF 代理（新闻 / 帮助共用）。
 *
 * 上游 `GET /api/v1/public/articles?kind=&category_id=&page=&page_size=`
 * （公开无需登录，仅返回已发布且未下线内容）。
 * `kind` 不在白名单内直接返回空列表：门户只有这四种形态，透传给后端只是徒增一次请求。
 */
export default defineEventHandler(async (event) => {
  setHeader(event, 'cache-control', 'public, s-maxage=300, stale-while-revalidate=900')

  const query = getQuery(event)
  const rawKind = typeof query.kind === 'string' ? query.kind.trim() : ''
  const kind: ArticleKind = isArticleKind(rawKind) ? rawKind : 'news'

  return fetchArticleList(event, {
    kind,
    categoryId: asQueryInt(query.category_id),
    page: asQueryInt(query.page),
    pageSize: asQueryInt(query.page_size) ?? DEFAULT_PAGE_SIZE,
  })
})
