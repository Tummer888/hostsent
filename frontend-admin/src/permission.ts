import type { App, Component } from 'vue'
import { h, shallowRef } from 'vue'

import {
  AiToolIcon,
  AppIcon,
  ChartBarIcon,
  ChartBubbleIcon,
  CheckCircleIcon,
  CloudDownloadIcon,
  CloudIcon,
  ControlPlatformIcon,
  DashboardIcon,
  DataCheckedIcon,
  ErrorCircleIcon,
  FileIcon,
  HistoryIcon,
  KeyIcon,
  LayersIcon,
  LinkIcon,
  LockOnIcon,
  MenuIcon,
  MoneyIcon,
  RefreshIcon,
  ServerIcon,
  ServiceIcon,
  SettingIcon,
  StopIcon,
  TagIcon,
  UserCircleIcon,
  UserIcon,
  UserListIcon,
  UsergroupIcon,
  VerifyIcon,
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
    children: [{ title: '概览', path: '/dashboard/base', icon: iconWrapper(DashboardIcon) }],
  },
  {
    title: '用户管理',
    path: '/users',
    icon: iconWrapper(UserIcon),
    children: [
      { title: '用户总览', path: '/users/overview', icon: iconWrapper(DashboardIcon) },
      {
        title: '账户管理',
        path: '/users/accounts',
        icon: iconWrapper(UsergroupIcon),
        children: [
          { title: '用户列表', path: '/users/accounts/list', icon: iconWrapper(UserListIcon) },
          { title: '用户组/组织管理', path: '/users/accounts/groups', icon: iconWrapper(ControlPlatformIcon) },
        ],
      },
      {
        title: '分销与代理商管理',
        path: '/users/partners',
        icon: iconWrapper(LinkIcon),
        children: [
          { title: '代理商列表', path: '/users/partners/agents', icon: iconWrapper(UsergroupIcon) },
          { title: '代理商等级配置', path: '/users/partners/levels', icon: iconWrapper(TagIcon) },
          { title: '下级用户管理', path: '/users/partners/subordinates', icon: iconWrapper(UserListIcon) },
          { title: '返利/佣金记录', path: '/users/partners/commissions', icon: iconWrapper(MoneyIcon) },
        ],
      },
      { title: '用户详情', path: '/users/accounts/detail', icon: iconWrapper(UserCircleIcon) },
      {
        title: '权限与角色',
        path: '/users/rbac',
        icon: iconWrapper(LockOnIcon),
        children: [
          { title: '角色列表', path: '/users/rbac/roles', icon: iconWrapper(UsergroupIcon) },
          { title: '权限分配', path: '/users/rbac/permissions', icon: iconWrapper(SettingIcon) },
          { title: '管理员列表', path: '/users/rbac/admins', icon: iconWrapper(UserListIcon) },
        ],
      },
      {
        title: '安全与风控',
        path: '/users/security',
        icon: iconWrapper(KeyIcon),
        children: [
          { title: '登录日志', path: '/users/security/login-logs', icon: iconWrapper(HistoryIcon) },
          { title: '操作审计日志', path: '/users/security/audit-logs', icon: iconWrapper(FileIcon) },
          { title: '异常行为监控', path: '/users/security/risk', icon: iconWrapper(ChartBarIcon) },
          { title: '黑名单管理', path: '/users/security/blacklist', icon: iconWrapper(StopIcon) },
          { title: '会话管理', path: '/users/security/sessions', icon: iconWrapper(RefreshIcon) },
        ],
      },
      {
        title: '资源配额与等级',
        path: '/users/quota',
        icon: iconWrapper(LayersIcon),
        children: [
          { title: '配额模板管理', path: '/users/quota/templates', icon: iconWrapper(LayersIcon) },
          { title: '用户等级管理', path: '/users/quota/tiers', icon: iconWrapper(TagIcon) },
          { title: '配额调整记录', path: '/users/quota/changes', icon: iconWrapper(HistoryIcon) },
        ],
      },
      {
        title: '实名认证',
        path: '/users/verification',
        icon: iconWrapper(VerifyIcon),
        children: [
          { title: '待审核列表', path: '/users/verification/pending', icon: iconWrapper(HistoryIcon) },
          { title: '审核通过列表', path: '/users/verification/approved', icon: iconWrapper(CheckCircleIcon) },
          { title: '审核拒绝列表', path: '/users/verification/rejected', icon: iconWrapper(ErrorCircleIcon) },
          { title: '认证配置', path: '/users/verification/config', icon: iconWrapper(SettingIcon) },
        ],
      },
    ],
  },
  {
    title: '资源管理',
    path: '/resource',
    icon: iconWrapper(LayersIcon),
    children: [
      {
        title: '资源总览',
        path: '/resource/overview',
        icon: iconWrapper(DashboardIcon),
        children: [
          { title: '资源总览', path: '/resource/dashboard', icon: iconWrapper(DashboardIcon) },
          { title: '同步监控', path: '/resource/sync-monitor', icon: iconWrapper(DataCheckedIcon) },
        ],
      },
      {
        title: '上游对接管理',
        path: '/resource/connection',
        icon: iconWrapper(CloudIcon),
        children: [
          { title: '上游提供商', path: '/resource/providers', icon: iconWrapper(CloudIcon) },
          { title: '资源池管理', path: '/resource/pools', icon: iconWrapper(LayersIcon) },
          { title: '连接测试', path: '/resource/connectivity', icon: iconWrapper(LinkIcon) },
        ],
      },
      {
        title: '资源同步与对账',
        path: '/resource/sync-center',
        icon: iconWrapper(RefreshIcon),
        children: [
          { title: '同步任务', path: '/resource/sync', icon: iconWrapper(RefreshIcon) },
          { title: '同步日志', path: '/resource/logs', icon: iconWrapper(HistoryIcon) },
          { title: '对账报告', path: '/resource/reconciliation', icon: iconWrapper(VerifyIcon) },
        ],
      },
      {
        title: '资源商品管理',
        path: '/resource/products-center',
        icon: iconWrapper(AppIcon),
        children: [
          { title: '商品列表', path: '/resource/products', icon: iconWrapper(AppIcon) },
          { title: '商品同步', path: '/resource/product-sync', icon: iconWrapper(CloudDownloadIcon) },
          { title: '定价管理', path: '/resource/pricing', icon: iconWrapper(MoneyIcon) },
        ],
      },
      {
        title: '实例资源',
        path: '/resource/instance',
        icon: iconWrapper(ServerIcon),
        children: [{ title: '云主机实例', path: '/resource/instances', icon: iconWrapper(ServerIcon) }],
      },
      {
        title: '运维工具',
        path: '/resource/ops',
        icon: iconWrapper(SettingIcon),
        children: [
          { title: 'API测试', path: '/resource/api-test', icon: iconWrapper(AiToolIcon) },
          { title: '异常处理', path: '/resource/anomalies', icon: iconWrapper(ErrorCircleIcon) },
          { title: '系统配置', path: '/resource/settings', icon: iconWrapper(SettingIcon) },
        ],
      },
    ],
  },
  {
    title: '产品管理',
    path: '/product',
    icon: iconWrapper(AppIcon),
    children: [
      { title: '产品列表', path: '/product/products', icon: iconWrapper(AppIcon) },
      { title: '分类管理', path: '/product/categories', icon: iconWrapper(TagIcon) },
      { title: '价格与上下架', path: '/product/pricing', icon: iconWrapper(MoneyIcon) },
    ],
  },
  {
    title: '系统管理',
    path: '/system',
    icon: iconWrapper(SettingIcon),
    children: [
      { title: '菜单管理', path: '/system/menus', icon: iconWrapper(MenuIcon) },
      { title: '角色列表', path: '/system/roles', icon: iconWrapper(UsergroupIcon) },
      { title: '权限分配', path: '/system/permissions', icon: iconWrapper(SettingIcon) },
      { title: '管理员列表', path: '/system/admins', icon: iconWrapper(UserListIcon) },
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
