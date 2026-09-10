// 官网门户 Nuxt 配置
// 架构基线见 docs/实施计划/80-官网门户与品牌配置架构设计.md
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  ssr: true,
  devtools: { enabled: false },

  // 与后端一致的容器部署方式（SWR 依赖 Node 进程，不能用纯静态托管）
  nitro: { preset: 'node-server' },

  css: ['~/assets/css/main.css'],

  runtimeConfig: {
    // 仅服务端可见：Phase 3 调用内部 revalidate 接口用
    internalToken: process.env.NUXT_INTERNAL_TOKEN || '',
    public: {
      // 后端地址；为空时 BFF 直接返回内置默认值（便于脱离后端开发官网）
      apiBase: process.env.NUXT_PUBLIC_API_BASE || '',
      siteTimezone: process.env.NUXT_PUBLIC_SITE_TIMEZONE || 'Asia/Shanghai',
      // 用户控制台地址（frontend-user）；为空时详情页不展示购买入口，避免死链
      consoleUrl: process.env.NUXT_PUBLIC_CONSOLE_URL || '',
      // 站点对外域名；为空时 robots/sitemap 回落到请求头里的 origin
      siteUrl: process.env.NUXT_PUBLIC_SITE_URL || '',
    },
  },

  // 开发端口固定为 3003，与 scripts/start-frontends.sh 保持一致
  devServer: { port: 3003, host: '0.0.0.0' },

  // 缓存策略：只对公开营销页启用 SWR；含用户态路径一律 no-store
  routeRules: {
    '/': {
      swr: 300,
      headers: { 'cache-control': 'public, s-maxage=300, stale-while-revalidate=1800' },
    },
    '/products/**': {
      swr: 900,
      headers: { 'cache-control': 'public, s-maxage=900, stale-while-revalidate=3600' },
    },
    '/campaign/**': {
      // 活动页时间敏感：短 TTL + 活动起止时刻主动 purge
      swr: 60,
      headers: { 'cache-control': 'public, s-maxage=60, stale-while-revalidate=300' },
    },
    '/console/**': {
      ssr: false,
      headers: { 'cache-control': 'no-store' },
    },
  },

  app: {
    head: {
      htmlAttrs: { lang: 'zh-CN' },
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      ],
    },
  },

  typescript: { strict: true },
})
