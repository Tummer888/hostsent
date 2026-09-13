import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { getSiteContent } from '@/api/site'

/**
 * 站点品牌信息（官网名称 / Logo / 版权 / 备案号），来源是管理端「系统配置」写入、
 * 后端 `/api/v1/public/site-content` 按白名单公开的同一份 `system_configs` 数据。
 *
 * 为什么用户端也要读它：控制台的登录页、顶栏 Logo、页脚版权、浏览器标题都带品牌名，
 * 原先散落硬编码在各页面里（12 处 `宿派云控` 字面量）。改一处品牌要翻全仓库，
 * 而且与官网可能不一致 —— 这里收敛成一个数据源，运营改后台两边同时变。
 *
 * 容错：接口不可用（未登录场景下网络异常、后端未起）时逐字段回落 FALLBACK，
 * 页面永远拿得到一个可用的名字，不会出现「空白品牌」。
 */

/** 兜底值：与 frontend-site/shared/schemas/siteContent.ts 的 DEFAULT_SITE_CONTENT 对齐。 */
const FALLBACK = {
  name: '宿派云控',
  logo: '',
  copyright: '',
  icp: '',
  licence: '',
} as const

/** 管理端历史扁平键与 doc80 规范点号键都认，前者优先（运营实际维护入口写的是扁平键）。 */
function pick(items: Record<string, string>, flatKey: string, dottedKey: string): string {
  const flat = items[flatKey]
  if (typeof flat === 'string' && flat.trim()) return flat.trim()
  const dotted = items[dottedKey]
  if (typeof dotted === 'string' && dotted.trim()) return dotted.trim()
  return ''
}

export const useBrandStore = defineStore('brand', () => {
  const name = ref<string>(FALLBACK.name)
  const logo = ref<string>(FALLBACK.logo)
  const copyright = ref<string>(FALLBACK.copyright)
  const icp = ref<string>(FALLBACK.icp)

  /** Logo 未配置时用品牌名首字做文字标记，避免顶栏出现空方块。 */
  const logoMark = computed(() => (name.value || FALLBACK.name).slice(0, 1).toUpperCase())

  /** 页脚版权行：未配置时回落到「© 当前年 品牌名」，不编造公司实体。 */
  const copyrightText = computed(() => {
    if (copyright.value) return copyright.value
    return `© ${new Date().getFullYear()} ${name.value}`
  })

  let loaded = false

  /** 拉取品牌配置；失败保持兜底值，不抛错、不阻塞首屏。 */
  async function load() {
    if (loaded) return
    try {
      const { data } = await getSiteContent()
      const items = data?.items || {}
      name.value = pick(items, 'site_name', 'site.name') || FALLBACK.name
      logo.value = pick(items, 'site_logo', 'site.logo')
      copyright.value = pick(items, 'site_copyright', 'site.copyright')
      icp.value = pick(items, 'site_icp', 'site.icp')
      loaded = true
    } catch {
      // 品牌信息是装饰性数据：拿不到就用兜底值，不打断登录与页面渲染
    }
  }

  return { name, logo, copyright, icp, logoMark, copyrightText, load }
})
