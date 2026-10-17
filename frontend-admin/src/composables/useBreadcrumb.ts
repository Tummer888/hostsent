import type { RouteLocationMatched, RouteLocationNormalizedLoaded } from 'vue-router'
import type { FlatMenu } from '@/store/modules/menu'

/**
 * 「当前页在导航树里的位置」解析，供布局层面包屑使用。
 *
 * 为什么放在布局层统一算，而不是各页自己写一段面包屑标记：
 * 一是页面有 148 个，逐个补一份「一级域 / 二级 / 当前页」既写不完也改不动；
 * 二是面包屑的内容本就等于菜单树的信息，重复写第二份必然漂移。
 *
 * 层级取「菜单树」而不是「路由 matched」：路由表里二级目录（如 /users/accounts）
 * 多数没有独立 component，matched 里不会出现；而菜单树里「账户管理」是实打实的一层，
 * 面包屑少了它就会从「用户管理」直接跳到「用户列表」，看不出当前在哪个分组下。
 */

export interface Crumb {
  title: string
  /** 有 path 的中间层可点击回跳；当前页与分组节点不给 path。 */
  path?: string
}

export interface BreadcrumbResult {
  /** 完整层级：一级域 / （二级分组） / 当前页 */
  items: Crumb[]
  /** 一级域名称（用于菜单栏标题兜底），如「用户管理」 */
  groupTitle?: string
}

/** 在最外层菜单树里按路径找到从一级域到该路径的完整链条。 */
function findChain(menus: FlatMenu[], targetPath: string, trail: FlatMenu[] = []): FlatMenu[] | undefined {
  for (const menu of menus) {
    const next = [...trail, menu]
    // 命中本身，或命中的是某个子节点的路径
    if (menu.path === targetPath) return next
    if (menu.children?.length) {
      const hit = findChain(menu.children, targetPath, next)
      if (hit) return hit
    }
  }
  return undefined
}

/** 叶子路径可能带查询串而菜单里没有，这里退一步按路径前缀找最贴近的父级链。 */
function findChainByPrefix(menus: FlatMenu[], targetPath: string): FlatMenu[] | undefined {
  const exact = findChain(menus, targetPath)
  if (exact) return exact
  // 下钻页（详情/新建）不在菜单里，取「同前缀的兄弟列表页」那条链作为归属
  const segments = targetPath.split('/').filter(Boolean)
  for (let i = segments.length - 1; i > 0; i -= 1) {
    const probe = '/' + segments.slice(0, i).join('/')
    const hit = findChain(menus, probe)
    if (hit) return hit
  }
  return undefined
}

/**
 * 由路由 + 菜单树算出面包屑。
 * 菜单尚未加载时（首屏、后端不可达降级）退回路由 matched 的一级标题 + 当前页标题，
 * 至少保证「一级域 / 当前页」两层不断。
 */
export function resolveBreadcrumb(route: RouteLocationNormalizedLoaded, menus: FlatMenu[]): BreadcrumbResult {
  const currentTitle = (route.meta?.title as string) || '当前页'
  // 层级从一级菜单起算：「首页」不进面包屑 —— 它本身就是一个与其他域并列的
  // 一级菜单（仪表盘），把「首页 / 仪表盘 / 概览」写成三段既重复又让人以为
  // 首页是整站的根节点。返回入口由顶栏的返回按钮承担，不需要这里再放一条。
  const items: Crumb[] = []

  const chain = findChainByPrefix(menus, route.path)
  if (chain?.length) {
    const groupTitle = chain[0]?.name
    for (let i = 0; i < chain.length; i += 1) {
      const node = chain[i]
      const isLast = i === chain.length - 1
      // 命中当前页自身时不再重复列一遍（当前页统一在末尾收口）
      if (node.path === route.path && isLast) break
      // 一级域只作层级展示（点它没有独立落地页），二级及其后可点击回跳
      if (i === 0) {
        items.push({ title: node.name })
      } else if (node.path) {
        items.push({ title: node.name, path: node.path })
      } else {
        items.push({ title: node.name })
      }
    }
    items.push({ title: currentTitle })
    return { items, groupTitle }
  }

  // 兜底：菜单不可用，用路由 matched 里的首个父级标题
  const parents = (route.matched as RouteLocationMatched[]).filter((m) => m.meta?.title)
  const groupTitle = parents.length > 1 ? (parents[0].meta.title as string) : undefined
  if (groupTitle && groupTitle !== currentTitle) items.push({ title: groupTitle })
  items.push({ title: currentTitle })
  return { items, groupTitle }
}
