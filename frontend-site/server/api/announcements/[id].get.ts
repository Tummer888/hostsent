import { asQueryInt, unwrapEnvelope } from '#shared/schemas/api'
import { parseAnnouncement } from '#shared/schemas/announcement'

/**
 * 公告详情 BFF 代理（`/announcements/:id`）。
 *
 * 状态码语义与内容详情一致：
 *  - 后端业务失败（公告不存在 / 已下线）→ 404；
 *  - 后端网络异常 → 503。
 */
export default defineEventHandler(async (event) => {
  const id = asQueryInt(getRouterParam(event, 'id') ?? '')
  if (id === undefined) {
    throw createError({ statusCode: 404, statusMessage: 'Announcement not found' })
  }

  const { public: publicConfig } = useRuntimeConfig(event)
  if (!publicConfig.apiBase) {
    throw createError({ statusCode: 404, statusMessage: 'Announcement not found' })
  }

  let raw: unknown
  try {
    raw = await $fetch(`${publicConfig.apiBase}/api/v1/public/announcements/${id}`, {
      timeout: 3000,
    })
  } catch (error) {
    console.warn(`[announcements] 读取公告详情失败 id=${id}:`, error)
    throw createError({ statusCode: 503, statusMessage: 'Announcement service unavailable' })
  }

  const announcement = parseAnnouncement(unwrapEnvelope(raw))
  if (!announcement) {
    throw createError({ statusCode: 404, statusMessage: 'Announcement not found' })
  }

  setHeader(event, 'cache-control', 'public, s-maxage=300, stale-while-revalidate=900')
  return announcement
})
