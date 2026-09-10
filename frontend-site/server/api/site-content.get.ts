import {
  DEFAULT_SITE_CONTENT,
  fromFlatConfig,
  resolveSiteContent,
} from '#shared/schemas/siteContent'

/**
 * 站点配置 BFF 代理。
 *
 * 前端只访问本接口，不直连后端业务地址（规范 R2），由此统一缓存策略与错误处理。
 * Phase 3 接入后端后，这里会带上内部密钥并做白名单过滤；当前若未配置 apiBase
 * 或后端不可用，直接返回内置默认值，保证官网永远可用。
 */
export default defineEventHandler(async (event) => {
  setHeader(event, 'cache-control', 'public, s-maxage=60, stale-while-revalidate=300')

  const { public: publicConfig } = useRuntimeConfig(event)
  if (!publicConfig.apiBase) {
    return DEFAULT_SITE_CONTENT
  }

  try {
    const flat = await $fetch<Record<string, unknown>>(
      `${publicConfig.apiBase}/api/v1/public/site-content`,
      { timeout: 3000 },
    )
    return resolveSiteContent(fromFlatConfig(flat))
  } catch (error) {
    // 后端故障/接口未上线时回落默认值，避免首页白屏
    console.warn('[site-content] 读取后端配置失败，回落到默认值:', error)
    return DEFAULT_SITE_CONTENT
  }
})
