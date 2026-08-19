import type { App } from 'vue'
import router from './router'
import { useUserStore } from './store'
import { useMenuStore } from './store/modules/menu'

const whiteList = ['/login']

export function setupPermission(app: App) {
  router.beforeEach(async (to, from, next) => {
    const userStore = useUserStore()
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

    if (userStore.isLoggedIn && !menuStore.loaded) {
      try {
        await userStore.fetchUserInfo()
        await menuStore.loadMenus('user')
      } catch (e) {
        console.error('Failed to load data:', e)
      }
    }

    next()
  })
}
