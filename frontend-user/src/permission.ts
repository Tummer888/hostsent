import type { App } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import type { RouteRecordNormalized } from 'vue-router'

import router from './router'
import { useUserStore } from './store'
import { useBrandStore } from './store/modules/brand'
import { useMemberStore } from './store/modules/member'
import { useMenuStore } from './store/modules/menu'

// 免登录路径白名单：登录页本身 + 第三方登录回调页。
//
// /oauth/callback 必须在这里：用户被三方重定向回来时浏览器里还没有平台令牌
// （令牌要由这一页调 /uc/oauth/exchange 才能换到），若走 requiresAuth 会被守卫
// 弹回登录页，一次性票据随之作废，用户表现为「授权成功却回到登录页」。
const whiteList = ['/login', '/oauth/callback']

/**
 * 预热目标路由的懒加载组件。
 *
 * Vue Router 是「先跑完全局守卫、再去加载目标路由的异步组件」，而下面的守卫又在等
 * 用户信息/菜单两个接口，首屏就变成了「接口往返 → 再下 chunk」的串行瀑布。
 * 这里提前触发动态 import：chunk 下载与接口请求并行，模块本身会被路由复用，不会重复下载。
 */
function warmRouteChunks(matched: RouteRecordNormalized[]) {
  matched.forEach((record) => {
    Object.values(record.components || {}).forEach((component) => {
      if (typeof component === 'function') void (component as () => Promise<unknown>)()
    })
  })
}

export function setupPermission(app: App) {
  router.beforeEach(async (to, from, next) => {
    const userStore = useUserStore()
    const memberStore = useMemberStore()
    const menuStore = useMenuStore()
    const brandStore = useBrandStore()

    if (to.meta?.title) {
      // 品牌名来自管理端站点配置，不再写死；未拉到配置时 store 内部有兜底值。
      document.title = `${to.meta.title} - ${brandStore.name}`
    }

    if (userStore.isLoggedIn && whiteList.includes(to.path)) {
      next('/')
      return
    }

    if (whiteList.includes(to.path)) {
      next()
      return
    }

    if (!userStore.isLoggedIn) {
      next(`/login?redirect=${encodeURIComponent(to.fullPath)}`)
      return
    }

    // 用户信息与菜单是权限判定与页面渲染的前置数据，未加载时先补齐。
    // 两者互不依赖：并行拉取（并同时预热目标页面 chunk）比串行少一到两个往返，
    // 首屏关键路径上的这点等待直接体现在「白屏时长」上。
    warmRouteChunks(to.matched)
    await Promise.all([
      userStore.loaded
        ? Promise.resolve()
        : userStore.fetchUserInfo().catch((e) => {
            console.error('Failed to load user info:', e)
          }),
      menuStore.loaded
        ? Promise.resolve()
        : menuStore.loadMenus('user').catch((e) => {
            console.error('Failed to load menus:', e)
          }),
    ])

    // 子账号无权进入主账号专属页面（如成员管理）。前端仅拦入口，后端仍会拒绝。
    if (to.meta?.ownerOnly && userStore.isSubAccount) {
      MessagePlugin.warning('该页面仅主账号可访问')
      next('/dashboard')
      return
    }

    // meta.permission 为「需持有的客户侧权限码」，子账号按授予集合判定，主账号天然通过。
    const required = to.meta?.permission
    if (required) {
      const codes = Array.isArray(required) ? (required as string[]) : [required as string]
      if (codes.length && !codes.some((code) => memberStore.has(code))) {
        MessagePlugin.warning('没有访问该页面的权限')
        next('/dashboard')
        return
      }
    }

    next()
  })
}
