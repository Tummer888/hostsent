import { fetchFriendlyLinks } from '../utils/content'

/**
 * 友情链接 BFF 代理（页脚用）。
 *
 * 上游 `GET /api/v1/public/friendly-links`（公开无需登录，仅启用链接）。
 * 后端异常时返回空数组，页脚该栏目整体不渲染。
 */
export default defineEventHandler(async (event) => {
  setHeader(event, 'cache-control', 'public, s-maxage=300, stale-while-revalidate=900')
  return fetchFriendlyLinks(event)
})
