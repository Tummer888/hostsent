import { defineStore } from 'pinia'

import { getSiteContent } from '@/api/public'

/**
 * 站点品牌信息（管理端标题/登录页品牌）。
 *
 * 数据源与官网、用户控制台是同一份：管理端「系统配置 → 站点品牌」写入
 * `system_configs`，后端 `/api/v1/public/site-content` 按白名单公开。
 * 此前管理端把品牌名硬编码在登录页（`宿派云控`）与路由标题后缀里，运营改名后
 * 管理端还是旧名 —— 同一个平台在三端显示三个品牌，是最容易被截图对比的一类问题。
 *
 * 走公开接口（`@/api/public`）而不是管理端 request：登录页在拿到令牌之前就要用，
 * 而管理端 request 带 401 跳登录的拦截器。
 */

/** 兜底值：与 frontend-site/shared/schemas/siteContent.ts 的默认名对齐，接口不可用时不留空白。 */
const FALLBACK_NAME = '宿派云控'

export const useBrandStore = defineStore('brand', {
  state: () => ({
    name: FALLBACK_NAME,
    logo: '',
    /** 本次会话是否已成功拉取过（避免每次路由切换都打接口） */
    loaded: false,
  }),

  getters: {
    /** 品牌名首字，用于 Logo 缺图时的文字标记。 */
    logoMark: (state): string => (state.name || FALLBACK_NAME).slice(0, 1).toUpperCase(),
    /** 浏览器标题后缀。 */
    titleSuffix: (state): string => `${state.name || FALLBACK_NAME} 管理平台`,
  },

  actions: {
    /**
     * 拉取品牌配置。失败保持兜底值：品牌名是装饰性数据，
     * 不该因为它把登录页或整个后台卡住。
     *
     * 并发去重：路由守卫与登录页 onMounted 都可能触发首次加载，
     * 没有这层保护会同时发两个请求、并且都认为自己该写值。
     */
    async load() {
      if (this.loaded) return
      if (pending) return pending
      pending = this.fetchBrand()
      try {
        await pending
      } finally {
        pending = null
      }
    },

    async fetchBrand() {
      try {
        const items = await getSiteContent()
        // 规范点号键优先，历史扁平键兜底（与迁移 045 的处置一致：扁平键已置 disabled，
        // 但尚未执行迁移的环境仍只有扁平键有值）。
        this.name = pick(items, 'site.name', 'site_name') || FALLBACK_NAME
        this.logo = pick(items, 'site.logo', 'site_logo')
        this.loaded = true
      } catch {
        /* 保持兜底值 */
      }
    },
  },
})

/** 首次加载的 in-flight 去重（模块级，不参与持久化）。 */
let pending: Promise<void> | null = null

/** 点号键优先、扁平键兜底，都为空时返回空串。 */
function pick(items: Record<string, string>, dottedKey: string, flatKey: string): string {
  const dotted = items?.[dottedKey]
  if (typeof dotted === 'string' && dotted.trim()) return dotted.trim()
  const flat = items?.[flatKey]
  if (typeof flat === 'string' && flat.trim()) return flat.trim()
  return ''
}
