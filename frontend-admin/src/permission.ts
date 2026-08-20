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
    children: [
      { title: '用户列表', path: '/users/accounts/list' },
      { title: '用户组管理', path: '/users/accounts/groups' },
      { title: '代理商等级配置', path: '/users/partners/levels' },
      { title: '代理商列表', path: '/users/partners/agents' },
      { title: '下级用户管理', path: '/users/partners/subordinates' },
      { title: '返利佣金记录', path: '/users/partners/commissions' },
      { title: '代理结算单', path: '/users/partners/settlements' },
      { title: '登录日志', path: '/users/security/login-logs' },
      { title: '操作审计日志', path: '/users/security/audit-logs' },
      { title: '异常行为监控', path: '/users/security/risk' },
      { title: '黑名单管理', path: '/users/security/blacklist' },
      { title: '会话管理', path: '/users/security/sessions' },
      { title: '用户资源配额', path: '/users/quota/resources' },
      { title: '配额模板管理', path: '/users/quota/templates' },
      { title: '用户等级管理', path: '/users/quota/tiers' },
      { title: '配额调整记录', path: '/users/quota/changes' },
    ],
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
    children: [
      { title: '菜单管理', path: '/system/menus' },
      { title: '角色列表', path: '/system/roles' },
      { title: '权限分配', path: '/system/permissions' },
      { title: '管理员列表', path: '/system/admins' },
    ],
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
