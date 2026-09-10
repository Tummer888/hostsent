import { CATALOG_MAX_PAGE_SIZE, fetchProductList } from '../utils/catalog'

/**
 * sitemap.xml。
 *
 * 只枚举可公开索引的营销页与上架商品详情；后端不可用时退化为仅静态页，
 * 不会输出 500 让搜索引擎判定整站异常。
 */
function escapeXml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&apos;')
}

export default defineEventHandler(async (event) => {
  const { public: publicConfig } = useRuntimeConfig(event)
  const origin = (publicConfig.siteUrl || getRequestURL(event).origin).replace(/\/+$/, '')

  const { items } = await fetchProductList(event, { pageSize: CATALOG_MAX_PAGE_SIZE })

  const entries = [
    { loc: `${origin}/`, changefreq: 'daily', priority: '1.0' },
    { loc: `${origin}/products`, changefreq: 'daily', priority: '0.9' },
    ...items.map((product) => ({
      loc: `${origin}/products/${product.id}`,
      changefreq: 'weekly',
      priority: '0.8',
    })),
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
