/**
 * 控制台「最近访问」记录。
 *
 * 存在 localStorage 而不是后端：`users` 域没有「用户偏好/浏览历史」模型，
 * 为一条控制台装饰数据建表并不划算；换设备不同步是可以接受的代价。
 *
 * 记录点在 `layouts/index.vue` 的路由 watch —— 放在布局而不是各页面里，
 * 页面作者不需要记得调它，新页面自动被记录。
 */

const STORAGE_KEY = 'user_recent_pages'
const MAX_ITEMS = 6

/** 不记录：首页（进来就在这）、详情页（标题带 id 不适合做快捷入口）。 */
const SKIP_PREFIXES = ['/dashboard']

export interface RecentPage {
  title: string
  path: string
}

export function recordRecentPage(path: string, title: string): void {
  if (!path || !title) return
  if (SKIP_PREFIXES.some((prefix) => path.startsWith(prefix))) return
  // 带 query 的路径（如 /shop?product=1）按路径主体归并，避免同名条目刷屏
  const normalized = path.split('?')[0]
  if (!normalized) return

  try {
    const rest = read().filter((item) => item.path !== normalized)
    const next = [{ title, path: normalized }, ...rest].slice(0, MAX_ITEMS)
    localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
  } catch {
    // 隐私模式下 localStorage 不可写：最近访问是装饰性数据，失败静默
  }
}

export function getRecentPages(): RecentPage[] {
  return read()
}

function read(): RecentPage[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter(
      (item): item is RecentPage =>
        !!item &&
        typeof item === 'object' &&
        typeof (item as RecentPage).title === 'string' &&
        typeof (item as RecentPage).path === 'string',
    )
  } catch {
    return []
  }
}
