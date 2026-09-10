import { DEFAULT_SITE_CONTENT, type SiteContent } from '#shared/schemas/siteContent'

/**
 * 站点品牌与文案的统一入口。
 *
 * 规范 R1：组件内禁止硬编码品牌名、联系方式、备案号，一律经此读取。
 * 规范 R3：数据获取必须有默认值兜底，接口异常时页面回落到内置默认值。
 *
 * useAsyncData 以固定 key 去重，同一次请求内 Header / Footer / 页面多次调用只取一次。
 */
export function useSiteContent() {
  const { data } = useAsyncData<SiteContent>(
    'site-content',
    () => $fetch<SiteContent>('/api/site-content'),
    { default: () => DEFAULT_SITE_CONTENT },
  )

  const content = computed<SiteContent>(() => data.value ?? DEFAULT_SITE_CONTENT)
  const themeVars = computed(() => buildThemeVars(content.value.theme))

  return { content, themeVars }
}
