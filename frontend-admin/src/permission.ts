import type { App, Component } from 'vue'
import { h, shallowRef } from 'vue'

import {
  ChartBubbleIcon,
  ChevronRightIcon,
  DashboardIcon,
  LayersIcon,
  ServiceIcon,
  SettingIcon,
  UserIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import router from '@/router'
import { useMenuStore, useUserStore } from '@/store'

function iconWrapper(icon: Component) {
  return shallowRef({
    render() {
      return h(icon)
    },
  })
}

export const navMenu = [
  {
    title: '仪表盘',
    path: '/dashboard',
    icon: iconWrapper(DashboardIcon),
    children: [{ title: '概览', path: '/dashboard/base' }],
  },
  {
    title: '用户管理',
    path: '/users',
    icon: iconWrapper(UserIcon),
    children: [{ title: '用户列表', path: '/users/list' }],
  },
  {
    title: '资源管理',
    path: '/resources',
    icon: iconWrapper(ChevronRightIcon),
    children: [{ title: '云主机', path: '/resources/instances' }],
  },
  {
    title: '系统管理',
    path: '/system',
    icon: iconWrapper(SettingIcon),
    children: [{ title: '菜单管理', path: '/system/menus' }],
  },
]

export const featureIcons = {
  security: iconWrapper(ServiceIcon),
  stable: iconWrapper(ChartBubbleIcon),
  manage: iconWrapper(LayersIcon),
  service: iconWrapper(ServiceIcon),
}

export function setupPermission(app: App<Element>) {
  app

  router.beforeEach(async (to, _from, next) => {
    const userStore = useUserStore()

    if (userStore.token) {
      if (to.path === '/login') {
        next({ path: '/dashboard/base', replace: true })
        return
      }
      try {
        if (!userStore.userInfo.id) {
          await userStore.getUserInfo()
        }
        // 拉取后端菜单树，驱动侧边栏动态渲染
        const menuStore = useMenuStore()
        if (!menuStore.loaded) {
          try {
            await menuStore.loadMenus('admin')
          } catch {
            // 后端不可达时降级为静态 navMenu，不阻断登录
          }
        }
        next()
      } catch (error) {
        const message = (error as Error)?.message || '认证失败，请重新登录'
        try {
          MessagePlugin.error(message)
        } catch {
          // ignore when message plugin not yet available
        }
        userStore.logout()
        next({
          path: '/login',
          query: { redirect: encodeURIComponent(to.fullPath) },
          replace: true,
        })
      }
      return
    }

    const isPublic = to.path === '/login' || (to.meta?.public as boolean)
    if (isPublic) {
      next()
      return
    }

    next({
      path: '/login',
      query: { redirect: encodeURIComponent(to.fullPath) },
      replace: true,
    })
  })

  router.afterEach((to) => {
    if (to.meta?.title) {
      const suffix = '宿派云控 管理平台'
      document.title = `${to.meta.title} - ${suffix}`
    }
  })
}
