import type { Router } from 'vue-router'

/**
 * 路由 chunk 预热。
 *
 * 菜单 hover 是「即将访问」最早的可靠信号：距离真正点击通常还有几百毫秒，
 * 足够把该页面的懒加载模块提前拉下来。
 * - 开发态：Vite 按需 transform，首次进入一个页面要现场编译整条模块图；
 * - 生产态：虽然已是静态 chunk，首次点击仍要等一次网络往返。
 * 这段等待都能被 hover 的时间覆盖，点击时组件通常已就绪。
 *
 * 同路径只预热一次。vue-router 解析过的路由，其 `components.default`
 * 已被替换成组件对象，此时不需要也无法再调用。
 */
const warmed = new Set<string>()

export function prefetchRoute(router: Router, path?: string): void {
  if (!path || warmed.has(path)) return
  warmed.add(path)
  try {
    const matched = router.resolve(path).matched
    const loader = matched[matched.length - 1]?.components?.default
    if (typeof loader === 'function') {
      void (loader as () => Promise<unknown>)().catch(() => {})
    }
  } catch {
    // 路径未注册（如后端菜单下发但前端路由尚未同步）时忽略：预热失败不影响正常导航。
  }
}
