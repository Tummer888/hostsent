import type { H3Event } from 'h3'
import { unwrapEnvelope } from '#shared/schemas/api'
import {
  EMPTY_ARTICLE_CATEGORIES,
  EMPTY_ARTICLE_LIST,
  EMPTY_FRIENDLY_LINKS,
  parseArticleCategories,
  parseArticleDetail,
  parseArticleList,
  parseFriendlyLinks,
  type ArticleCategory,
  type ArticleDetail,
  type ArticleKind,
  type ArticleList,
  type FriendlyLink,
} from '#shared/schemas/content'

/**
 * 内容中心读取的服务端共用逻辑。
 *
 * 与 `catalog.ts` 同构：BFF 路由与 sitemap 生成都需要读内容页，
 * 参数白名单、信封拆解与 schema 校验收敛在这里，避免多份实现漂移。
 *
 * 错误语义（doc100 §9.1 / doc80 R12）：
 *  - 列表类：后端异常或业务失败 → 空列表（页面上是「暂无内容」而不是 500）；
 *  - 详情类：业务失败（40400/40004）→ null（门户 404）；网络异常 → 抛出（门户 503）。
 *    两者必须区分，否则后端一抖动搜索引擎会把整个内容库判定为已删除。
 */

/** sitemap 与列表页一次最多枚举的条数（与后端 page_size 上限一致）。 */
export const CONTENT_MAX_PAGE_SIZE = 50

/** 后端业务错误码：内容不存在 / 尚未发布。 */
const NOT_FOUND_CODES = new Set([40004, 40400])

/**
 * 判断一次 ofetch 异常是否代表「后端明确说这条内容不存在」。
 *
 * 后端业务失败时 HTTP 仍是 200（信封 code !== 0），所以拿不到 status，
 * 只能从响应体里读 code。读不到就按网络异常处理——宁可 503 也不能把
 * 服务故障误判成内容消失。
 */
function isBusinessFailure(error: unknown): boolean {
  if (typeof error !== 'object' || error === null) return false
  const data = (error as { data?: unknown }).data
  if (typeof data !== 'object' || data === null) return false
  const code = (data as { code?: unknown }).code
  return typeof code === 'number' && NOT_FOUND_CODES.has(code)
}

export interface ArticleListQuery {
  kind: ArticleKind
  categoryId?: number
  page?: number
  pageSize?: number
}

/** 读取已发布文章列表；后端未配置或异常时返回空列表。 */
export async function fetchArticleList(
  event: H3Event,
  query: ArticleListQuery,
): Promise<ArticleList> {
  const { public: publicConfig } = useRuntimeConfig(event)
  if (!publicConfig.apiBase) return EMPTY_ARTICLE_LIST

  const upstream: Record<string, string> = { kind: query.kind }
  if (query.categoryId) upstream.category_id = String(query.categoryId)
  if (query.page) upstream.page = String(query.page)
  if (query.pageSize) upstream.page_size = String(query.pageSize)

  try {
    const raw = await $fetch(`${publicConfig.apiBase}/api/v1/public/articles`, {
      query: upstream,
      timeout: 3000,
    })
    const data = unwrapEnvelope(raw)
    return data === null ? EMPTY_ARTICLE_LIST : parseArticleList(data)
  } catch (error) {
    console.warn(`[content] 读取文章列表失败 kind=${query.kind}，返回空列表:`, error)
    return EMPTY_ARTICLE_LIST
  }
}

/**
 * 读取文章详情。
 *
 * 返回 null 表示「确认不存在」（业务失败或后端未配置），调用方应响应 404；
 * 网络异常直接抛出，由调用方转成 503。
 */
export async function fetchArticleDetail(
  event: H3Event,
  kind: ArticleKind,
  slug: string,
): Promise<ArticleDetail | null> {
  const { public: publicConfig } = useRuntimeConfig(event)
  if (!publicConfig.apiBase) return null

  try {
    const raw = await $fetch(
      `${publicConfig.apiBase}/api/v1/public/articles/${kind}/${encodeURIComponent(slug)}`,
      { timeout: 3000 },
    )
    const data = unwrapEnvelope(raw)
    return data === null ? null : parseArticleDetail(data)
  } catch (error) {
    if (isBusinessFailure(error)) return null
    throw error
  }
}

/**
 * 读取「每类型一篇」的文档（条款 / 隐私政策）。
 *
 * 与详情接口的差别：这类内容尚未配置时后端返回业务失败，门户应展示
 * 「内容尚未准备好」的占位页而不是 404 —— 页面本身是存在的。
 * 因此这里把业务失败也收敛成 null，由页面决定文案；只有网络异常才抛 503。
 */
export async function fetchSingletonArticle(
  event: H3Event,
  kind: ArticleKind,
): Promise<ArticleDetail | null> {
  const { public: publicConfig } = useRuntimeConfig(event)
  if (!publicConfig.apiBase) return null

  try {
    const raw = await $fetch(
      `${publicConfig.apiBase}/api/v1/public/article-singletons/${kind}`,
      { timeout: 3000 },
    )
    const data = unwrapEnvelope(raw)
    return data === null ? null : parseArticleDetail(data)
  } catch (error) {
    if (isBusinessFailure(error)) return null
    throw error
  }
}

/** 读取分类树；后端未配置或异常时返回空数组（侧栏只显示「全部」）。 */
export async function fetchArticleCategories(
  event: H3Event,
  kind: ArticleKind,
): Promise<ArticleCategory[]> {
  const { public: publicConfig } = useRuntimeConfig(event)
  if (!publicConfig.apiBase) return EMPTY_ARTICLE_CATEGORIES

  try {
    const raw = await $fetch(`${publicConfig.apiBase}/api/v1/public/article-categories`, {
      query: { kind },
      timeout: 3000,
    })
    const data = unwrapEnvelope(raw)
    return data === null ? EMPTY_ARTICLE_CATEGORIES : parseArticleCategories(data)
  } catch (error) {
    console.warn(`[content] 读取内容分类失败 kind=${kind}，返回空分类:`, error)
    return EMPTY_ARTICLE_CATEGORIES
  }
}

/** 读取友情链接；后端未配置或异常时返回空数组（页脚该栏目整体隐藏）。 */
export async function fetchFriendlyLinks(event: H3Event): Promise<FriendlyLink[]> {
  const { public: publicConfig } = useRuntimeConfig(event)
  if (!publicConfig.apiBase) return EMPTY_FRIENDLY_LINKS

  try {
    const raw = await $fetch(`${publicConfig.apiBase}/api/v1/public/friendly-links`, {
      timeout: 3000,
    })
    const data = unwrapEnvelope(raw)
    return data === null ? EMPTY_FRIENDLY_LINKS : parseFriendlyLinks(data)
  } catch (error) {
    console.warn('[content] 读取友情链接失败，返回空列表:', error)
    return EMPTY_FRIENDLY_LINKS
  }
}
