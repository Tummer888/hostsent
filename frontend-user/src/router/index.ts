import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/pages/login/index.vue'),
    meta: { title: '登录', requiresAuth: false },
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
        path: 'support/tickets',
        name: 'TicketList',
        component: () => import('@/pages/support/tickets.vue'),
        meta: { title: '我的工单', icon: 'service' },
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
