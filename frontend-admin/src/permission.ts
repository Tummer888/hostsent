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
  MailIcon,
  MenuIcon,
  MoneyIcon,
  OrderIcon,
  RefreshIcon,
  ServerIcon,
  ServiceIcon,
  SettingIcon,
  ShareIcon,
  SoundIcon,
  StopIcon,
  TagIcon,
  TimeIcon,
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
        title: '用户等级',
        path: '/users/levels',
        icon: iconWrapper(TagIcon),
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
        // P7 菜单归位（doc16 §9.2）：渠道与平台 / 容量与位置 / 同步与调度 / 实例 / 运维。
        // 本轮 S1：渠道与平台按链路拆两页（上游转售 / 自营平台对接），连接测试内联到列表行内。
        title: '渠道与平台',
        path: '/resource/channels',
        icon: iconWrapper(CloudIcon),
        children: [
          { title: '上游转售渠道', path: '/resource/providers', icon: iconWrapper(CloudIcon) },
          { title: '自营平台对接', path: '/resource/platforms', icon: iconWrapper(ServerIcon) },
        ],
      },
      {
        title: '容量与位置',
        path: '/resource/capacity',
        icon: iconWrapper(LayersIcon),
        children: [
          { title: '资源池与容量', path: '/resource/pools', icon: iconWrapper(LayersIcon) },
        ],
      },
      {
        title: '同步与调度',
        path: '/resource/sync-group',
        icon: iconWrapper(RefreshIcon),
        children: [
          { title: '同步与调度', path: '/resource/sync-center', icon: iconWrapper(RefreshIcon) },
        ],
      },
      {
        title: '实例',
        path: '/resource/instance',
        icon: iconWrapper(ServerIcon),
        children: [
          { title: '云主机实例', path: '/resource/instances', icon: iconWrapper(ServerIcon) },
        ],
      },
      {
        title: '运维',
        path: '/resource/ops',
        icon: iconWrapper(SettingIcon),
        children: [
          { title: '异常处理', path: '/resource/anomalies', icon: iconWrapper(ErrorCircleIcon) },
          { title: '任务队列', path: '/resource/task-queue', icon: iconWrapper(RefreshIcon) },
          { title: '实例对账', path: '/resource/reconcile', icon: iconWrapper(VerifyIcon) },
        ],
      },
    ],
  },
  {
    title: '实例管理',
    path: '/instances',
    icon: iconWrapper(ServerIcon),
    children: [
      { title: '实例运维台', path: '/instances/list', icon: iconWrapper(ServerIcon) },
    ],
  },
  {
    title: '产品管理',
    path: '/product',
    icon: iconWrapper(AppIcon),
    children: [
      {
        title: '商品管理',
        path: '/product/mgmt',
        icon: iconWrapper(AppIcon),
        children: [
          { title: '商品列表', path: '/product/products', icon: iconWrapper(AppIcon) },
        ],
      },
      {
        // T7.2 商品对接（doc16 §9.3）：选品/成本与加价等商品决策动作归产品管理。
        title: '商品对接',
        path: '/product/binding',
        icon: iconWrapper(CloudIcon),
        children: [
          { title: '上游商品目录', path: '/product/upstream', icon: iconWrapper(AppIcon) },
          { title: '成本与加价', path: '/product/cost-pricing', icon: iconWrapper(MoneyIcon) },
        ],
      },
      {
        title: '规格管理',
        path: '/product/spec',
        icon: iconWrapper(LayersIcon),
        children: [
          { title: '规格模板', path: '/product/spec/templates', icon: iconWrapper(LayersIcon) },
          { title: '自定义规格', path: '/product/spec/custom', icon: iconWrapper(FileIcon) },
          { title: '规格映射', path: '/product/spec/mappings', icon: iconWrapper(LinkIcon) },
        ],
      },
      {
        title: '定价与计费',
        path: '/product/pricing-center',
        icon: iconWrapper(MoneyIcon),
        children: [
          { title: '价格策略', path: '/product/pricing', icon: iconWrapper(MoneyIcon) },
          { title: '价格计算器', path: '/product/pricing/calculator', icon: iconWrapper(ChartBarIcon) },
          { title: '价格历史', path: '/product/pricing/history', icon: iconWrapper(HistoryIcon) },
          { title: '折扣策略', path: '/product/pricing/policies', icon: iconWrapper(MoneyIcon) },
        ],
      },
      {
        title: '促销管理',
        path: '/product/promotion',
        icon: iconWrapper(TagIcon),
        children: [
          { title: '优惠券管理', path: '/product/promotion/coupons', icon: iconWrapper(TagIcon) },
          { title: '折扣活动', path: '/product/promotion/activities', icon: iconWrapper(ChartBarIcon) },
          { title: '套餐组合', path: '/product/promotion/bundles', icon: iconWrapper(AppIcon) },
          { title: '推荐位管理', path: '/product/promotion/recommends', icon: iconWrapper(DataCheckedIcon) },
        ],
      },
      {
        title: '商品分类',
        path: '/product/category',
        icon: iconWrapper(TagIcon),
        children: [
          { title: '分类管理', path: '/product/categories', icon: iconWrapper(TagIcon) },
        ],
      },
      // T7.4：上游商品同步组已下线，任务/日志/差异由「资源管理 → 同步与调度」承接。
    ],
  },
  {
    title: '订单管理',
    path: '/orders',
    icon: iconWrapper(OrderIcon),
    children: [
      {
        title: '订单列表',
        path: '/orders/list',
        icon: iconWrapper(OrderIcon),
        children: [{ title: '订单列表', path: '/orders/list', icon: iconWrapper(OrderIcon) }],
      },
      {
        title: '退款管理',
        path: '/orders/refunds',
        icon: iconWrapper(MoneyIcon),
        children: [{ title: '退款管理', path: '/orders/refunds', icon: iconWrapper(MoneyIcon) }],
      },
      {
        title: '订单统计',
        path: '/orders/stats',
        icon: iconWrapper(ChartBarIcon),
        children: [{ title: '订单统计', path: '/orders/stats', icon: iconWrapper(ChartBarIcon) }],
      },
    ],
  },
  {
    title: '财务管理',
    path: '/finance',
    icon: iconWrapper(MoneyIcon),
    children: [
      {
        title: '财务总览',
        path: '/finance/overview',
        icon: iconWrapper(DashboardIcon),
        children: [
          { title: '财务总览', path: '/finance/overview', icon: iconWrapper(DashboardIcon) },
        ],
      },
      {
        title: '账户管理',
        path: '/finance/accounts',
        icon: iconWrapper(MoneyIcon),
        children: [
          { title: '用户钱包', path: '/finance/accounts/wallets', icon: iconWrapper(MoneyIcon) },
          { title: '人工调账', path: '/finance/accounts/adjust', icon: iconWrapper(FileIcon) },
        ],
      },
      {
        title: '交易流水',
        path: '/finance/transactions-center',
        icon: iconWrapper(MoneyIcon),
        children: [
          { title: '资金流水', path: '/finance/transactions', icon: iconWrapper(MoneyIcon) },
        ],
      },
      {
        title: '充值提现',
        path: '/finance/recharge-center',
        icon: iconWrapper(FileIcon),
        children: [
          { title: '充值管理', path: '/finance/recharges', icon: iconWrapper(MoneyIcon) },
          { title: '提现管理', path: '/finance/withdrawals', icon: iconWrapper(FileIcon) },
        ],
      },
      {
        title: '账单管理',
        path: '/finance/bill-center',
        icon: iconWrapper(FileIcon),
        children: [
          { title: '账单管理', path: '/finance/bills', icon: iconWrapper(FileIcon) },
          { title: '对账中心', path: '/finance/recon', icon: iconWrapper(CheckCircleIcon) },
        ],
      },
      {
        title: '财务报表',
        path: '/finance/report',
        icon: iconWrapper(ChartBarIcon),
        children: [
          { title: '财务报表', path: '/finance/report', icon: iconWrapper(ChartBarIcon) },
        ],
      },
      {
        title: '财务配置',
        path: '/finance/config',
        icon: iconWrapper(SettingIcon),
        children: [
          { title: '财务配置', path: '/finance/config', icon: iconWrapper(SettingIcon) },
        ],
      },
    ],
  },
  {
    title: '推广返现',
    path: '/referral',
    icon: iconWrapper(ShareIcon),
    children: [
      {
        title: '返现台账',
        path: '/referral/cashbacks',
        icon: iconWrapper(MoneyIcon),
        children: [{ title: '返现台账', path: '/referral/cashbacks', icon: iconWrapper(MoneyIcon) }],
      },
      {
        title: '提现审核',
        path: '/referral/withdrawals',
        icon: iconWrapper(FileIcon),
        children: [{ title: '提现审核', path: '/referral/withdrawals', icon: iconWrapper(FileIcon) }],
      },
      {
        title: '邀请关系',
        path: '/referral/invitees',
        icon: iconWrapper(UsergroupIcon),
        children: [{ title: '邀请关系', path: '/referral/invitees', icon: iconWrapper(UsergroupIcon) }],
      },
    ],
  },
  {
    title: '系统管理',
    path: '/system',
    icon: iconWrapper(SettingIcon),
    children: [
      {
        title: '权限管理',
        path: '/system/permission-center',
        icon: iconWrapper(LockOnIcon),
        children: [
          { title: '菜单管理', path: '/system/menus', icon: iconWrapper(MenuIcon) },
          { title: '角色列表', path: '/system/roles', icon: iconWrapper(UsergroupIcon) },
          { title: '权限分配', path: '/system/permissions', icon: iconWrapper(SettingIcon) },
          { title: '管理员列表', path: '/system/admins', icon: iconWrapper(UserListIcon) },
        ],
      },
      {
        title: '系统配置',
        path: '/system/config-center',
        icon: iconWrapper(SettingIcon),
        children: [{ title: '系统配置', path: '/system/config', icon: iconWrapper(SettingIcon) }],
      },
      {
        title: '安全审计',
        path: '/system/audit-center',
        icon: iconWrapper(HistoryIcon),
        children: [
          { title: '操作审计', path: '/system/audit-logs', icon: iconWrapper(HistoryIcon) },
          { title: '公告管理', path: '/system/announcements', icon: iconWrapper(SoundIcon) },
        ],
      },
    ],
  },
  {
    title: '工单支持',
    path: '/tickets',
    icon: iconWrapper(ServiceIcon),
    children: [
      {
        title: '工单列表',
        path: '/tickets/list',
        icon: iconWrapper(ServiceIcon),
        children: [{ title: '工单列表', path: '/tickets/list', icon: iconWrapper(ServiceIcon) }],
      },
      {
        title: '工单分类管理',
        path: '/tickets/categories',
        icon: iconWrapper(FileIcon),
        children: [{ title: '工单分类管理', path: '/tickets/categories', icon: iconWrapper(FileIcon) }],
      },
      {
        title: '工单统计',
        path: '/tickets/stats',
        icon: iconWrapper(ChartBarIcon),
        children: [{ title: '工单统计', path: '/tickets/stats', icon: iconWrapper(ChartBarIcon) }],
      },
    ],
  },
  {
    title: '生命周期管理',
    path: '/lifecycle',
    icon: iconWrapper(TimeIcon),
    children: [
      {
        title: '到期管理',
        path: '/lifecycle/expiring',
        icon: iconWrapper(TimeIcon),
        children: [{ title: '到期管理', path: '/lifecycle/expiring', icon: iconWrapper(TimeIcon) }],
      },
      {
        title: '续费记录',
        path: '/lifecycle/renewals',
        icon: iconWrapper(RefreshIcon),
        children: [{ title: '续费记录', path: '/lifecycle/renewals', icon: iconWrapper(RefreshIcon) }],
      },
      {
        title: '生命周期策略',
        path: '/lifecycle/policy',
        icon: iconWrapper(SettingIcon),
        children: [{ title: '生命周期策略', path: '/lifecycle/policy', icon: iconWrapper(SettingIcon) }],
      },
    ],
  },
  {
    title: '消息中心',
    path: '/notification',
    icon: iconWrapper(MailIcon),
    children: [
      {
        title: '通知记录',
        path: '/notification/records',
        icon: iconWrapper(MailIcon),
        children: [{ title: '通知记录', path: '/notification/records', icon: iconWrapper(MailIcon) }],
      },
      {
        title: '通知模板',
        path: '/notification/templates',
        icon: iconWrapper(FileIcon),
        children: [{ title: '通知模板', path: '/notification/templates', icon: iconWrapper(FileIcon) }],
      },
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

        // 路由级权限校验：meta.permission 由后端权限码对齐（P1-11）。
        // 前端仅做体验拦截，真正的越权防护由后端 RequirePermission 承担。
        const required = to.meta?.permission as string | string[] | undefined
        if (required) {
          const codes = Array.isArray(required) ? required : [required]
          if (!userStore.hasPermission(...codes)) {
            MessagePlugin.warning('无权访问该页面')
            next({ path: '/dashboard/base', replace: true })
            return
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
