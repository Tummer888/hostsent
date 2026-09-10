import type { App } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'

import router from './router'
import { useUserStore } from './store'
import { useMemberStore } from './store/modules/member'
import { useMenuStore } from './store/modules/menu'

const whiteList = ['/login', '/register']

export function setupPermission(app: App) {
  router.beforeEach(async (to, from, next) => {
    const userStore = useUserStore()
    const memberStore = useMemberStore()
    const menuStore = useMenuStore()

    if (to.meta?.title) {
      document.title = `${to.meta.title} - 宿派云控用户控制台`
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
    if (!userStore.loaded) {
      try {
        await userStore.fetchUserInfo()
      } catch (e) {
        console.error('Failed to load user info:', e)
      }
    }
    if (!menuStore.loaded) {
      try {
        await menuStore.loadMenus('user')
      } catch (e) {
        console.error('Failed to load menus:', e)
      }
    }

    // 子账号无权进入主账号专属页面（如成员管理）。前端仅拦入口，后端仍会拒绝。
    if (to.meta?.ownerOnly && userStore.isSubAccount) {
      MessagePlugin.warning('该页面仅主账号可访问')
      next('/dashboard')
      return
    }

    // 代理专区门禁（P6-03）：后端按 distribution_agents 存在与否放行，前端仅拦入口。
    if (to.meta?.requiresAgent && !memberStore.isAgent) {
      MessagePlugin.warning('该页面仅代理可访问')
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
