import {
  CONTENT_MAX_PAGE_SIZE,
  fetchArticleCategories,
  fetchArticleList,
} from '../utils/content'
import { CATALOG_MAX_PAGE_SIZE, fetchProductList } from '../utils/catalog'
import { unwrapEnvelope } from '#shared/schemas/api'
import { parseAnnouncementList } from '#shared/schemas/announcement'
import { ARTICLE_KINDS, LIST_KINDS } from '#shared/schemas/content'

/** sitemap 收录的公告条数上限（与后端公开接口的 50 条上限一致）。 */
const ANNOUNCEMENT_SITEMAP_LIMIT = 50

/**
 * sitemap.xml。
 *
 * 枚举可公开索引的营销页、上架商品详情与全部已发布内容页（doc100 Q9）；
 * 后端不可用时退化为仅静态页，不会输出 500 让搜索引擎判定整站异常。
 */
function escapeXml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&apos;')
}

interface Entry {
  loc: string
  changefreq: string
  priority: string
}

export default defineEventHandler(async (event) => {
  const { public: publicConfig } = useRuntimeConfig(event)
  const origin = (publicConfig.siteUrl || getRequestURL(event).origin).replace(/\/+$/, '')

  const staticEntries: Entry[] = [
    { loc: `${origin}/`, changefreq: 'daily', priority: '1.0' },
    { loc: `${origin}/products`, changefreq: 'daily', priority: '0.9' },
    { loc: `${origin}/news`, changefreq: 'daily', priority: '0.8' },
    { loc: `${origin}/help`, changefreq: 'weekly', priority: '0.8' },
    { loc: `${origin}/announcements`, changefreq: 'daily', priority: '0.7' },
    { loc: `${origin}/terms`, changefreq: 'monthly', priority: '0.3' },
    { loc: `${origin}/privacy`, changefreq: 'monthly', priority: '0.3' },
  ]

  const { items: products } = await fetchProductList(event, { pageSize: CATALOG_MAX_PAGE_SIZE })

  // 内容页：news / help 走列表接口拿 slug，逐类型取一页上限条数。
  // 内容量超过单页上限时这里只会收录前 N 条 —— 好过为了「收全」而在 sitemap 里
  // 串行发几十次请求（搜索引擎对 sitemap 有抓取预算，慢响应反而降低收录率）。
  const contentEntries: Entry[] = []
  for (const kind of ARTICLE_KINDS) {
    if (!LIST_KINDS.includes(kind)) continue
    const list = await fetchArticleList(event, { kind, pageSize: CONTENT_MAX_PAGE_SIZE })
    for (const article of list.items) {
      contentEntries.push({
        loc: `${origin}/${kind}/${encodeURIComponent(article.slug)}`,
        changefreq: 'weekly',
        priority: '0.6',
      })
    }
  }

  // 分类页（`/news?category=`、`/help?category=`）也收录：
  // 它们是内容聚合页，有独立检索价值。带 query 的地址收录效果弱于静态路径，
  // 但比不收录好，且无需为分类单独开页面路由。
  for (const kind of LIST_KINDS) {
    const categories = await fetchArticleCategories(event, kind)
    const walk = (nodes: typeof categories) => {
      for (const node of nodes) {
        if (node.slug) {
          contentEntries.push({
            loc: `${origin}/${kind}?category=${encodeURIComponent(node.slug)}`,
            changefreq: 'weekly',
            priority: '0.5',
          })
        }
        walk(node.children)
      }
    }
    walk(categories)
  }

  // 公告详情页
  const announcementEntries: Entry[] = []
  if (publicConfig.apiBase) {
    try {
      const raw = await $fetch(`${publicConfig.apiBase}/api/v1/public/announcements`, {
        query: { limit: String(ANNOUNCEMENT_SITEMAP_LIMIT) },
        timeout: 3000,
      })
      const data = unwrapEnvelope(raw)
      for (const item of data === null ? [] : parseAnnouncementList(data)) {
        announcementEntries.push({
          loc: `${origin}/announcements/${item.id}`,
          changefreq: 'monthly',
          priority: '0.4',
        })
      }
    } catch (error) {
      console.warn('[sitemap] 读取公告失败，跳过公告页:', error)
    }
  }

  const entries: Entry[] = [
    ...staticEntries,
    ...products.map((product) => ({
      loc: `${origin}/products/${product.id}`,
      changefreq: 'weekly',
      priority: '0.8',
    })),
    ...contentEntries,
    ...announcementEntries,
  ]

  const body = [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    ...entries.map((entry) =>
      [
        '  <url>',
        `    <loc>${escapeXml(entry.loc)}</loc>`,
        `    <changefreq>${entry.changefreq}</changefreq>`,
        `    <priority>${entry.priority}</priority>`,
        '  </url>',
      ].join('\n'),
    ),
    '</urlset>',
    '',
  ].join('\n')

  setHeader(event, 'content-type', 'application/xml; charset=utf-8')
  setHeader(event, 'cache-control', 'public, max-age=600, stale-while-revalidate=1800')
  return body
})
