import { unwrapEnvelope } from '#shared/schemas/api'
import {
  DEFAULT_SITE_CONTENT,
  fromFlatConfig,
  resolveSiteContent,
} from '#shared/schemas/siteContent'

/**
 * 站点配置 BFF 代理。
 *
 * 前端只访问本接口，不直连后端业务地址（规范 R2），由此统一缓存策略与错误处理。
 * 后端可用时读取 `/api/v1/public/site-content`（品牌白名单），配置为空或损坏的字段
 * 由 resolveSiteContent 逐区块回落默认值；后端不可用时整体回落，保证官网永不白屏。
 */
export default defineEventHandler(async (event) => {
  setHeader(event, 'cache-control', 'public, s-maxage=60, stale-while-revalidate=300')

  const { public: publicConfig } = useRuntimeConfig(event)
  if (!publicConfig.apiBase) {
    return DEFAULT_SITE_CONTENT
  }

  try {
    const raw = await $fetch(`${publicConfig.apiBase}/api/v1/public/site-content`, {
      timeout: 3000,
    })
    // 必须先拆信封：后端业务失败时 HTTP 仍是 200 且 code !== 0，
    // 直接当成配置用会把错误响应当成「字段全部缺失」，静默丢弃后台已保存的品牌值。
    const flat = unwrapEnvelope<Record<string, unknown>>(raw)
    if (flat === null) {
      console.warn('[site-content] 后端返回业务失败，回落到默认值')
      return DEFAULT_SITE_CONTENT
    }
    // items 是白名单键值对（`{ "site.name": "...", "site_name": "..." }`），
    // 兼容外层直接就是键值对的旧契约。
    const items = (flat.items ?? flat) as Record<string, unknown>
    return resolveSiteContent(fromFlatConfig(items))
  } catch (error) {
    // 后端故障/接口未上线时回落默认值，避免首页白屏
    console.warn('[site-content] 读取后端配置失败，回落到默认值:', error)
    return DEFAULT_SITE_CONTENT
  }
})
