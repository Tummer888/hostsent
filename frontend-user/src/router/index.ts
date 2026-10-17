import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

// 错误页必须静态引入，不能走懒加载。
//
// chunk 失效时「所有动态 import 都会失败」—— 错误页自己也是动态 import 的话，
// 跳过去只会再抛一次同样的错，形成 onError → replace → onError 的死循环。
// 静态引入让它们进首屏主包，与故障的 chunk 不在同一条加载路径上。
import ChunkLoadErrorPage from '@/pages/error/chunk.vue'
import NotFoundPage from '@/pages/error/404.vue'
import ServerErrorPage from '@/pages/error/500.vue'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/pages/login/index.vue'),
    meta: { title: '登录', requiresAuth: false },
  },
  {
    // 独立注册页已并入登录卡片：原地切换表单，不再单独跳页。
    // 保留 /register 路径重定向；函数式把原查询参数（如推广 ?invite_code=）一并带过去。
    path: '/register',
    redirect: (to) => ({ path: '/login', query: { ...to.query, mode: 'register' } }),
  },
  {
    path: '/forgot-password',
    name: 'ForgotPassword',
    component: () => import('@/pages/forgot-password/index.vue'),
    meta: { title: '找回密码', requiresAuth: false },
  },
  {
    // 第三方登录回调（doc104 §6.4）：后端 302 到这里并带一次性 ticket，
    // 本页调 /uc/oauth/exchange 换成正式令牌后再跳首页。
    // requiresAuth: false —— 此刻浏览器里还没有令牌（见 permission.ts 白名单）。
    path: '/oauth/callback',
    name: 'OAuthCallback',
    component: () => import('@/pages/oauth/callback.vue'),
    meta: { title: '登录中', requiresAuth: false },
  },
  {
    path: '/',
    component: () => import('@/layouts/index.vue'),
    redirect: '/dashboard',
    meta: { requiresAuth: true },
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/pages/dashboard/index.vue'),
        meta: { title: '控制台', icon: 'dashboard' },
      },
      {
        path: 'cloud/instances',
        name: 'InstanceList',
        component: () => import('@/pages/cloud/instances/index.vue'),
        meta: { title: '我的云主机', icon: 'cloud', permission: 'instance:view' },
      },
      {
        path: 'cloud/instances/:id',
        name: 'InstanceDetail',
        component: () => import('@/pages/cloud/instances/detail.vue'),
        meta: { title: '云主机详情', hidden: true, permission: 'instance:view' },
      },
      {
        path: 'shop',
        name: 'Shop',
        component: () => import('@/pages/shop/index.vue'),
        meta: { title: '购买云主机', icon: 'cart', permission: 'order:create' },
      },
      {
        // 配置选购页：下单唯一入口是购物车，本页选好配置后加入购物车。
        path: 'shop/:id',
        name: 'ShopConfig',
        component: () => import('@/pages/shop/config.vue'),
        meta: { title: '配置选购', hidden: true, permission: 'order:create' },
      },
      {
        path: 'cart',
        name: 'Cart',
        component: () => import('@/pages/cart/index.vue'),
        meta: { title: '购物车', icon: 'cart', permission: 'order:create' },
      },
      {
        path: 'cloud/images',
        name: 'ImageList',
        component: () => import('@/pages/cloud/images/index.vue'),
        meta: { title: '镜像管理', icon: 'layers', permission: 'instance:view' },
      },
      {
        path: 'cloud/renewals',
        name: 'UserRenewals',
        component: () => import('@/pages/cloud/renewals/index.vue'),
        // 进入页面只需 order:view（后端 GET /instances/renewals 同口径），
        // 页内的「立即续费」「自动续费」再单独要 order:create。
        meta: { title: '续费管理', icon: 'refresh', permission: 'order:view' },
      },
      {
        path: 'order',
        name: 'OrderList',
        component: () => import('@/pages/order/index.vue'),
        meta: { title: '我的订单', icon: 'order', permission: 'order:view' },
      },
      {
        path: 'order/:id',
        name: 'OrderDetail',
        component: () => import('@/pages/order/detail.vue'),
        meta: { title: '订单详情', hidden: true, permission: 'order:view' },
      },
      {
        path: 'billing',
        name: 'Billing',
        component: () => import('@/pages/billing/index.vue'),
        meta: { title: '费用中心', icon: 'wallet', permission: 'billing:view' },
      },
      {
        path: 'member',
        name: 'MemberManage',
        component: () => import('@/pages/member/index.vue'),
        meta: { title: '成员管理', icon: 'usergroup', ownerOnly: true },
      },
      {
        path: 'billing/balance',
        name: 'BillingBalance',
        component: () => import('@/pages/billing/balance.vue'),
        meta: { title: '余额与充值', icon: 'wallet', permission: 'billing:recharge' },
      },
      {
        path: 'billing/transactions',
        name: 'BillingTransactions',
        component: () => import('@/pages/billing/transactions.vue'),
        meta: { title: '资金流水', icon: 'money', permission: 'billing:view' },
      },
      {
        path: 'billing/payment-methods',
        name: 'BillingPaymentMethods',
        component: () => import('@/pages/billing/payment-methods.vue'),
        meta: { title: '支付方式', icon: 'wallet', permission: 'billing:view' },
      },
      {
        path: 'billing/invoices',
        name: 'BillingInvoices',
        component: () => import('@/pages/billing/invoices.vue'),
        meta: { title: '发票管理', icon: 'file-paste', permission: 'billing:view' },
      },
      {
        path: 'points',
        name: 'MyPoints',
        component: () => import('@/pages/points/index.vue'),
        meta: { title: '我的积分', icon: 'gift', permission: 'billing:view' },
      },
      {
        path: 'points/transactions',
        name: 'MyPointTransactions',
        component: () => import('@/pages/points/transactions.vue'),
        meta: { title: '积分明细', icon: 'money', permission: 'billing:view' },
      },
      {
        path: 'support',
        redirect: '/support/tickets',
        meta: { title: '工单中心' },
        children: [
          {
            path: 'tickets',
            name: 'TicketList',
            component: () => import('@/pages/support/tickets.vue'),
            meta: { title: '我的工单', icon: 'service', permission: 'ticket:view' },
          },
          {
            path: 'tickets/create',
            name: 'UserTicketCreate',
            component: () => import('@/pages/support/ticket-create.vue'),
            meta: { title: '提交工单', permission: 'ticket:submit' },
          },
          {
            path: 'tickets/:id',
            name: 'UserTicketDetail',
            component: () => import('@/pages/support/ticket-detail.vue'),
            meta: { title: '工单详情', permission: 'ticket:view' },
          },
        ],
      },
      {
        path: 'referral',
        redirect: '/referral/overview',
        // 推广数据是主账号资产（含返现与提现），子账号一律不可见；
        // 与 OWNER_ONLY_MENU_PATHS 是同一批路径，改一处要同时改两处。
        meta: { title: '推广邀请', icon: 'share', ownerOnly: true },
        children: [
          {
            path: 'overview',
            name: 'ReferralOverview',
            component: () => import('@/pages/referral/index.vue'),
            meta: { title: '推广概览', icon: 'dashboard' },
          },
          {
            path: 'invitees',
            name: 'ReferralInvitees',
            component: () => import('@/pages/referral/invitees/index.vue'),
            meta: { title: '我的邀请', icon: 'usergroup' },
          },
          {
            path: 'cashbacks',
            name: 'ReferralCashbacks',
            component: () => import('@/pages/referral/cashbacks/index.vue'),
            meta: { title: '返现明细', icon: 'money' },
          },
          {
            path: 'withdrawals',
            name: 'ReferralWithdrawals',
            component: () => import('@/pages/referral/withdrawals/index.vue'),
            meta: { title: '提现与转出', icon: 'wallet' },
          },
          {
            path: 'materials',
            name: 'ReferralMaterials',
            component: () => import('@/pages/referral/materials/index.vue'),
            meta: { title: '推广素材', icon: 'share' },
          },
        ],
      },
      {
        path: 'profile',
        meta: { title: '个人中心', icon: 'user' },
        children: [
          {
            path: '',
            name: 'Profile',
            component: () => import('@/pages/profile/index.vue'),
            meta: { title: '个人中心' },
          },
          {
            path: 'messages',
            name: 'UserMessages',
            component: () => import('@/pages/profile/messages.vue'),
            meta: { title: '我的消息' },
          },
          {
            path: 'preferences',
            name: 'UserNotifyPrefs',
            component: () => import('@/pages/profile/preferences.vue'),
            meta: { title: '通知偏好' },
          },
          {
            path: 'security',
            name: 'UserSecuritySettings',
            component: () => import('@/pages/profile/security/index.vue'),
            meta: { title: '安全设置' },
          },
        ],
      },
      // 错误页挂在 layout 内：保留侧边栏与顶栏，用户出错后仍有导航能力。
      {
        path: 'error/404',
        name: 'NotFound',
        component: NotFoundPage,
        meta: { title: '页面不存在', hidden: true },
      },
      {
        path: 'error/500',
        name: 'ServerError',
        component: ServerErrorPage,
        meta: { title: '页面加载失败', hidden: true },
      },
      {
        path: 'error/chunk',
        name: 'ChunkLoadError',
        component: ChunkLoadErrorPage,
        meta: { title: '检测到新版本', hidden: true },
      },
    ],
  },
  {
    // 未知路径兜底：保留 layout，直接重定向到 /error/404。
    // 不用「catch-all 自己渲染 404 页」的写法，是因为 layout 的 children 里
    // 已经有一个具名 NotFound，同名路由会让 vue-router 报警并产生歧义。
    path: '/:pathMatch(.*)*',
    redirect: '/error/404',
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior() {
    return { top: 0 }
  },
})

// 懒加载 chunk 失败（发版后旧哈希失效）单独识别。
// 这类错误发生在「守卫已放行、开始下载目标组件」之后，beforeEach 插不上手，
// 只能在这里捕获；不处理的话 router 会停在半路，页面保持白屏。
let chunkErrorRedirected = false
router.onError((error) => {
  const message = String((error as Error)?.message || '')
  const isChunkError =
    /Failed to fetch dynamically imported module/i.test(message) ||
    /Importing a module script failed/i.test(message) ||
    /error loading dynamically imported module/i.test(message)
  if (!isChunkError) return
  if (router.currentRoute.value.name === 'ChunkLoadError') return

  // 只重定向一次。若错误页自身的依赖也加载失败（整站资源都不可用），
  // 再 replace 会再次触发 onError，形成无限重试 —— 保留当前画面比空转更有用。
  if (chunkErrorRedirected) return
  chunkErrorRedirected = true

  void router.replace({ name: 'ChunkLoadError', query: { from: router.currentRoute.value.fullPath } }).finally(() => {
    window.setTimeout(() => {
      chunkErrorRedirected = false
    }, 5000)
  })
})

export default router
