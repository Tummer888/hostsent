import { asQueryInt, unwrapEnvelope } from '#shared/schemas/api'
import { EMPTY_ANNOUNCEMENT_LIST, parseAnnouncementList } from '#shared/schemas/announcement'

const DEFAULT_LIMIT = 5
const MAX_LIMIT = 20

/**
 * 公告列表 BFF 代理。
 *
 * 上游 `GET /api/v1/public/announcements`（公开无需登录，仅已发布且未下线公告）。
 * 后端未配置或异常时返回空列表，首页公告区块整体隐藏。
 */
export default defineEventHandler(async (event) => {
  setHeader(event, 'cache-control', 'public, s-maxage=300, stale-while-revalidate=900')

  const { public: publicConfig } = useRuntimeConfig(event)
  if (!publicConfig.apiBase) return EMPTY_ANNOUNCEMENT_LIST

  const limit = Math.min(asQueryInt(getQuery(event).limit) ?? DEFAULT_LIMIT, MAX_LIMIT)

  try {
    const raw = await $fetch(`${publicConfig.apiBase}/api/v1/public/announcements`, {
      query: { limit: String(limit) },
      timeout: 3000,
    })
    const data = unwrapEnvelope(raw)
    return data === null ? EMPTY_ANNOUNCEMENT_LIST : parseAnnouncementList(data)
  } catch (error) {
    console.warn('[announcements] 读取公告失败，返回空列表:', error)
    return EMPTY_ANNOUNCEMENT_LIST
  }
})
