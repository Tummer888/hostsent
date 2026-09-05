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
        meta: { title: '我的云主机', icon: 'cloud' },
      },
      {
        path: 'cloud/images',
        name: 'ImageList',
        component: () => import('@/pages/cloud/images/index.vue'),
        meta: { title: '镜像管理', icon: 'layers' },
      },
      {
        path: 'order',
        name: 'OrderList',
        component: () => import('@/pages/order/index.vue'),
        meta: { title: '我的订单', icon: 'order' },
      },
      {
        path: 'billing',
        name: 'Billing',
        component: () => import('@/pages/billing/index.vue'),
        meta: { title: '费用中心', icon: 'wallet' },
      },
      {
        path: 'billing/balance',
        name: 'BillingBalance',
        component: () => import('@/pages/billing/balance.vue'),
        meta: { title: '余额与充值', icon: 'wallet' },
      },
      {
        path: 'billing/transactions',
        name: 'BillingTransactions',
        component: () => import('@/pages/billing/transactions.vue'),
        meta: { title: '资金流水', icon: 'money' },
      },
      {
        path: 'support',
        meta: { title: '工单中心' },
        children: [
          {
            path: 'tickets',
            name: 'TicketList',
            component: () => import('@/pages/support/tickets.vue'),
            meta: { title: '我的工单', icon: 'service' },
          },
          {
            path: 'tickets/create',
            name: 'UserTicketCreate',
            component: () => import('@/pages/support/ticket-create.vue'),
            meta: { title: '提交工单' },
          },
          {
            path: 'tickets/:id',
            name: 'UserTicketDetail',
            component: () => import('@/pages/support/ticket-detail.vue'),
            meta: { title: '工单详情' },
          },
        ],
      },
      {
        path: 'profile',
        name: 'Profile',
        component: () => import('@/pages/profile/index.vue'),
        meta: { title: '个人中心', icon: 'user' },
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
