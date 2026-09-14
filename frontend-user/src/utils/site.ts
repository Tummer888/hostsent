/**
 * 官网门户（frontend-site）地址工具。
 *
 * 用户中心与官网是两个独立站点：条款、隐私政策、帮助中心等公开内容只在门户维护，
 * 用户中心一律外链过去，不重复实现一份（否则同一份法律文本要在三个前端各维护一遍）。
 *
 * 未配置 `VITE_SITE_URL` 时**返回空串**，由调用方决定怎么降级 ——
 * 硬编码一个可能不存在或不是自家的地址，比"点不动"更糟。
 */
const SITE_URL = String(import.meta.env.VITE_SITE_URL || '').replace(/\/+$/, '')

/** 官网是否已配置。未配置时任何门户链接都不应渲染成可点链接。 */
export const siteUrlConfigured = SITE_URL !== ''

/** 拼接官网路径：`sitePath('/help')` → `http://site/help`；未配置时返回空串。 */
export function sitePath(path: string): string {
  if (!SITE_URL) return ''
  const normalized = path.startsWith('/') ? path : `/${path}`
  return `${SITE_URL}${normalized}`
}

/**
 * 在新窗口打开官网页面。
 *
 * 返回 false 表示未配置官网地址（调用方据此给出提示），true 表示已尝试打开。
 * 用 `noopener` 断开 opener 引用：门户页面无法通过 `window.opener` 反向操作控制台。
 */
export function openSite(path: string): boolean {
  const url = sitePath(path)
  if (!url) return false
  window.open(url, '_blank', 'noopener')
  return true
}
