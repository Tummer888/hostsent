import type { App, Component } from 'vue'
import { h, shallowRef } from 'vue'

import {
  AppIcon,
  ChartBarIcon,
  ChartBubbleIcon,
  CloudIcon,
  DashboardIcon,
  DownloadIcon,
  FileIcon,
  FolderIcon,
  GiftIcon,
  HistoryIcon,
  KeyIcon,
  LayersIcon,
  LinkIcon,
  LockOnIcon,
  MailIcon,
  MoneyIcon,
  OrderIcon,
  RefreshIcon,
  RootListIcon,
  SecuredIcon,
  SendIcon,
  ServerIcon,
  ServiceIcon,
  SettingIcon,
  ShareIcon,
  SoundIcon,
  TagIcon,
  TicketIcon,
  UploadIcon,
  UserIcon,
  UsergroupIcon,
  VerifyIcon,
  WalletIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import router from '@/router'
import { useBrandStore, useMenuStore, useUserStore } from '@/store'

function iconWrapper(icon: Component) {
  return shallowRef({
    render() {
      return h(icon)
    },
  })
}

// navMenu 仅用于「后端不可达」时的降级兜底（layouts/index.vue 的 fallbackMenus）。
// 权威定义在 backend/internal/pkg/db/db.go 的 SeedMenus()，四处一致性
// （seed / router.meta.permission / permission_map.go / 本文件）
// 由 backend/internal/pkg/db/menu_align_test.go 锁定。
//
// 因此这里只保留「一级域 + 直接子项」两层：后端菜单树的二级/三级结构由 seed 下发，
// 兜底菜单不复制它 —— 少写一份就少一处漂移。
// 兜底菜单的图标按名字取（与后端 seed 的 icon 字段同名），避免为每个入口重复写组件名。
const navIconMap: Record<string, Component> = {
  dashboard: DashboardIcon,
  user: UserIcon,
  usergroup: UsergroupIcon,
  key: KeyIcon,
  verify: VerifyIcon,
  tag: TagIcon,
  layers: LayersIcon,
  cloud: CloudIcon,
  refresh: RefreshIcon,
  setting: SettingIcon,
  server: ServerIcon,
  product: AppIcon,
  resource: LayersIcon,
  money: MoneyIcon,
  order: OrderIcon,
  'chart-bar': ChartBarIcon,
  wallet: WalletIcon,
  file: FileIcon,
  download: DownloadIcon,
  'lock-on': LockOnIcon,
  history: HistoryIcon,
  safety: SecuredIcon,
  service: ServiceIcon,
  ticket: TicketIcon,
  folder: FolderIcon,
  share: ShareIcon,
  mail: MailIcon,
  'root-list': RootListIcon,
  send: SendIcon,
  gift: GiftIcon,
  link: LinkIcon,
  sound: SoundIcon,
  upload: UploadIcon,
}

function navIcon(name: string) {
  return iconWrapper(navIconMap[name] || AppIcon)
}

export const navMenu = [
  {
    title: '仪表盘',
    path: '/dashboard',
    icon: navIcon('dashboard'),
    children: [
      { title: '概览', path: '/dashboard/base', icon: navIcon('dashboard') },
    ],
  },
  {
    title: '用户管理',
    path: '/users',
    icon: navIcon('user'),
    children: [
      { title: '用户总览', path: '/users/overview', icon: navIcon('dashboard') },
      { title: '账户管理', path: '/users/accounts', icon: navIcon('usergroup') },
      { title: '用户等级', path: '/users/levels', icon: navIcon('tag') },
      { title: '安全与风控', path: '/users/security', icon: navIcon('key') },
      { title: '实名认证', path: '/users/verification', icon: navIcon('verify') },
    ],
  },
  {
    title: '资源管理',
    path: '/resource',
    icon: navIcon('resource'),
    children: [
      { title: '渠道与平台', path: '/resource/channels', icon: navIcon('cloud') },
      { title: '资源池与容量', path: '/resource/pools', icon: navIcon('layers') },
      { title: '同步与调度', path: '/resource/sync-center', icon: navIcon('refresh') },
      { title: '运维', path: '/resource/ops', icon: navIcon('setting') },
    ],
  },
  {
    title: '实例管理',
    path: '/instances',
    icon: navIcon('server'),
    children: [
      { title: '实例运维台', path: '/instances/list', icon: navIcon('server') },
      { title: '云主机实例', path: '/instances/inventory', icon: navIcon('server') },
    ],
  },
  {
    title: '产品管理',
    path: '/product',
    icon: navIcon('product'),
    children: [
      { title: '商品列表', path: '/product/products', icon: navIcon('product') },
      { title: '商品对接', path: '/product/binding', icon: navIcon('cloud') },
      { title: '分类管理', path: '/product/categories', icon: navIcon('tag') },
      { title: '规格管理', path: '/product/spec', icon: navIcon('layers') },
      { title: '定价与计费', path: '/product/pricing-center', icon: navIcon('money') },
      { title: '促销管理', path: '/product/promotion', icon: navIcon('tag') },
    ],
  },
  {
    title: '订单管理',
    path: '/orders',
    icon: navIcon('order'),
    children: [
      { title: '订单列表', path: '/orders/list', icon: navIcon('order') },
      { title: '退款管理', path: '/orders/refunds', icon: navIcon('money') },
      { title: '订单统计', path: '/orders/stats', icon: navIcon('chart-bar') },
    ],
  },
  {
    title: '财务管理',
    path: '/finance',
    icon: navIcon('wallet'),
    children: [
      { title: '财务总览', path: '/finance/overview', icon: navIcon('dashboard') },
      { title: '钱包与调账', path: '/finance/accounts', icon: navIcon('usergroup') },
      { title: '资金流水', path: '/finance/transactions', icon: navIcon('money') },
      { title: '充值提现', path: '/finance/recharge-center', icon: navIcon('download') },
      { title: '账单管理', path: '/finance/bill-center', icon: navIcon('file') },
      { title: '财务报表', path: '/finance/report', icon: navIcon('chart-bar') },
      { title: '财务配置', path: '/finance/config', icon: navIcon('setting') },
    ],
  },
  {
    title: '系统管理',
    path: '/system',
    icon: navIcon('setting'),
    children: [
      { title: '权限管理', path: '/system/permission-center', icon: navIcon('lock-on') },
      { title: '系统配置', path: '/system/config', icon: navIcon('setting') },
      { title: '验证码配置', path: '/system/captcha', icon: navIcon('safety') },
      { title: '操作审计', path: '/system/audit-logs', icon: navIcon('history') },
      { title: '日志中心', path: '/system/log-center', icon: navIcon('file') },
    ],
  },
  {
    title: '工单支持',
    path: '/tickets',
    icon: navIcon('service'),
    children: [
      { title: '工单列表', path: '/tickets/list', icon: navIcon('ticket') },
      { title: '复核中心', path: '/tickets/reviews', icon: navIcon('verify') },
      { title: '工单分类管理', path: '/tickets/categories', icon: navIcon('folder') },
      { title: '工单统计', path: '/tickets/stats', icon: navIcon('chart-bar') },
    ],
  },
  {
    title: '生命周期管理',
    path: '/lifecycle',
    icon: navIcon('history'),
    children: [
      { title: '到期管理', path: '/lifecycle/expiring', icon: navIcon('history') },
      { title: '续费记录', path: '/lifecycle/renewals', icon: navIcon('order') },
      { title: '生命周期策略', path: '/lifecycle/policy', icon: navIcon('setting') },
    ],
  },
  {
    title: '消息中心',
    path: '/notification',
    icon: navIcon('mail'),
    children: [
      { title: '通知模板', path: '/notification/templates', icon: navIcon('root-list') },
      { title: '短信模板', path: '/notification/sms-templates', icon: navIcon('file') },
      { title: '渠道配置', path: '/notification/channels', icon: navIcon('setting') },
      { title: '通知记录', path: '/notification/records', icon: navIcon('mail') },
      { title: '发送日志', path: '/notification/deliveries', icon: navIcon('root-list') },
      { title: '消息群发', path: '/notification/broadcast', icon: navIcon('send') },
    ],
  },
  {
    title: '推广返现',
    path: '/referral',
    icon: navIcon('share'),
    children: [
      { title: '返现台账', path: '/referral/cashbacks', icon: navIcon('money') },
      { title: '提现审核', path: '/referral/withdrawals', icon: navIcon('upload') },
      { title: '邀请关系', path: '/referral/invitees', icon: navIcon('usergroup') },
    ],
  },
  {
    title: '支付中心',
    path: '/payment',
    icon: navIcon('money'),
    children: [
      { title: '支付概览', path: '/payment/overview', icon: navIcon('dashboard') },
      { title: '渠道管理', path: '/payment/channel-center', icon: navIcon('link') },
      { title: '交易管理', path: '/payment/trade-center', icon: navIcon('order') },
      { title: '出款与对账', path: '/payment/payout-center', icon: navIcon('verify') },
    ],
  },
  {
    title: '积分中心',
    path: '/points',
    icon: navIcon('gift'),
    children: [
      { title: '积分概览', path: '/points/overview', icon: navIcon('dashboard') },
      { title: '积分规则', path: '/points/rules', icon: navIcon('setting') },
      { title: '积分账户', path: '/points/accounts', icon: navIcon('usergroup') },
      { title: '积分流水', path: '/points/transactions', icon: navIcon('history') },
    ],
  },
  {
    title: '销售中心',
    path: '/sales',
    icon: navIcon('share'),
    children: [
      { title: '客户归属', path: '/sales/customers', icon: navIcon('usergroup') },
      { title: '提成台账', path: '/sales/commissions', icon: navIcon('money') },
      { title: '提成审核', path: '/sales/withdrawals', icon: navIcon('upload') },
      { title: '业绩与排行', path: '/sales/performance', icon: navIcon('chart-bar') },
    ],
  },
  {
    title: '内容管理',
    path: '/content',
    icon: navIcon('file'),
    children: [
      { title: '内容文章', path: '/content/articles', icon: navIcon('file') },
      { title: '内容分类', path: '/content/categories', icon: navIcon('folder') },
      { title: '友情链接', path: '/content/links', icon: navIcon('link') },
      { title: '公告管理', path: '/content/announcements', icon: navIcon('sound') },
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
    // 品牌名（浏览器标题后缀）：一次会话内只拉一次，失败保持兜底值，不阻断路由。
    void useBrandStore().load()

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
      document.title = `${to.meta.title} - ${useBrandStore().titleSuffix}`
    }
  })
}
