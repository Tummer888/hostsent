import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/pages/login/index.vue'),
    meta: { title: '登录', requiresAuth: false },
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/pages/register/index.vue'),
    meta: { title: '注册', requiresAuth: false },
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
        path: 'cloud/images',
        name: 'ImageList',
        component: () => import('@/pages/cloud/images/index.vue'),
        meta: { title: '镜像管理', icon: 'layers', permission: 'instance:view' },
      },
      {
        path: 'cloud/renewals',
        name: 'UserRenewals',
        component: () => import('@/pages/cloud/renewals/index.vue'),
        meta: { title: '续费管理', icon: 'refresh', permission: 'order:create' },
      },
      {
        path: 'order',
        name: 'OrderList',
        component: () => import('@/pages/order/index.vue'),
        meta: { title: '我的订单', icon: 'order', permission: 'order:view' },
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
        path: 'agent',
        redirect: '/agent/overview',
        meta: { title: '代理中心', icon: 'usergroup', requiresAgent: true },
        children: [
          {
            path: 'overview',
            name: 'AgentOverview',
            component: () => import('@/pages/agent/index.vue'),
            meta: { title: '代理概览', icon: 'dashboard', requiresAgent: true },
          },
          {
            path: 'team',
            name: 'AgentTeam',
            component: () => import('@/pages/agent/team/index.vue'),
            meta: { title: '我的团队', icon: 'usergroup', requiresAgent: true },
          },
          {
            path: 'commissions',
            name: 'AgentCommissions',
            component: () => import('@/pages/agent/commissions/index.vue'),
            meta: { title: '佣金明细', icon: 'money', requiresAgent: true },
          },
          {
            path: 'settlements',
            name: 'AgentSettlements',
            component: () => import('@/pages/agent/settlements/index.vue'),
            meta: { title: '结算记录', icon: 'wallet', requiresAgent: true },
          },
          {
            path: 'materials',
            name: 'AgentMaterials',
            component: () => import('@/pages/agent/materials/index.vue'),
            meta: { title: '推广素材', icon: 'share', requiresAgent: true },
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
        ],
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/pages/error/404.vue'),
    meta: { title: '页面不存在' },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior() {
    return { top: 0 }
  },
})

export default router
