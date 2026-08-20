import type { RouteRecordRaw } from 'vue-router'
import { createRouter, createWebHistory } from 'vue-router'

const routes: Array<RouteRecordRaw> = [
  {
    path: '/',
    redirect: '/login',
  },
  {
    path: '/login',
    name: 'login',
    component: () => import('@/pages/login/index.vue'),
    meta: { title: '管理员登录', public: true },
  },
  {
    path: '/dashboard',
    component: () => import('@/layouts/index.vue'),
    redirect: '/dashboard/base',
    meta: { title: '仪表盘' },
    children: [
      {
        path: 'base',
        name: 'DashboardBase',
        component: () => import('@/pages/dashboard/base/index.vue'),
        meta: { title: '概览', role: 'admin' },
      },
    ],
  },
  {
    path: '/users',
    component: () => import('@/layouts/index.vue'),
    redirect: '/users/overview',
    meta: { title: '用户管理' },
    children: [
      {
        path: 'overview',
        name: 'UserOverview',
        component: () => import('@/pages/users/overview/index.vue'),
        meta: { title: '用户总览', role: 'admin' },
      },
      {
        path: 'accounts/list',
        name: 'UserAccountsList',
        component: () => import('@/pages/users/accounts/list/index.vue'),
        meta: { title: '用户列表', role: 'admin' },
      },
      {
        path: 'accounts/detail',
        name: 'UserAccountsDetail',
        component: () => import('@/pages/users/accounts/detail/index.vue'),
        meta: { title: '用户详情', role: 'admin' },
      },
      {
        path: 'accounts/groups',
        name: 'UserAccountsGroups',
        component: () => import('@/pages/users/accounts/groups/index.vue'),
        meta: { title: '用户组管理', role: 'admin' },
      },
      {
        path: 'rbac/roles',
        name: 'UserRbacRoles',
        component: () => import('@/pages/system/roles/index.vue'),
        meta: { title: '角色列表', role: 'admin' },
      },
      {
        path: 'rbac/permissions',
        name: 'UserRbacPermissions',
        component: () => import('@/pages/system/permissions/index.vue'),
        meta: { title: '权限分配', role: 'admin' },
      },
      {
        path: 'rbac/admins',
        name: 'UserRbacAdmins',
        component: () => import('@/pages/system/admins/index.vue'),
        meta: { title: '管理员列表', role: 'admin' },
      },
      {
        path: 'partners/levels',
        name: 'UserPartnersLevels',
        component: () => import('@/pages/users/partners/levels/index.vue'),
        meta: { title: '代理商等级配置', role: 'admin' },
      },
      {
        path: 'partners/agents',
        name: 'UserPartnersAgents',
        component: () => import('@/pages/users/partners/agents/index.vue'),
        meta: { title: '代理商列表', role: 'admin' },
      },
      {
        path: 'partners/subordinates',
        name: 'UserPartnersSubordinates',
        component: () => import('@/pages/users/partners/subordinates/index.vue'),
        meta: { title: '下级用户管理', role: 'admin' },
      },
      {
        path: 'partners/commissions',
        name: 'UserPartnersCommissions',
        component: () => import('@/pages/users/partners/commissions/index.vue'),
        meta: { title: '返利佣金记录', role: 'admin' },
      },
      {
        path: 'partners/settlements',
        name: 'UserPartnersSettlements',
        component: () => import('@/pages/users/partners/settlements/index.vue'),
        meta: { title: '代理结算单', role: 'admin' },
      },
      {
        path: 'security/login-logs',
        name: 'UserSecurityLoginLogs',
        component: () => import('@/pages/users/security/login-logs/index.vue'),
        meta: { title: '登录日志', role: 'admin' },
      },
      {
        path: 'security/audit-logs',
        name: 'UserSecurityAuditLogs',
        component: () => import('@/pages/users/security/audit-logs/index.vue'),
        meta: { title: '操作审计日志', role: 'admin' },
      },
      {
        path: 'security/risk',
        name: 'UserSecurityRisk',
        component: () => import('@/pages/users/security/risk/index.vue'),
        meta: { title: '异常行为监控', role: 'admin' },
      },
      {
        path: 'security/blacklist',
        name: 'UserSecurityBlacklist',
        component: () => import('@/pages/users/security/blacklist/index.vue'),
        meta: { title: '黑名单管理', role: 'admin' },
      },
      {
        path: 'security/sessions',
        name: 'UserSecuritySessions',
        component: () => import('@/pages/users/security/sessions/index.vue'),
        meta: { title: '会话管理', role: 'admin' },
      },
    ],
  },
  {
    path: '/system',
    component: () => import('@/layouts/index.vue'),
    redirect: '/system/menus',
    meta: { title: '系统管理' },
    children: [
      {
        path: 'menus',
        name: 'SystemMenus',
        component: () => import('@/pages/system/menus/index.vue'),
        meta: { title: '菜单管理', role: 'admin' },
      },
      {
        path: 'roles',
        name: 'SystemRoles',
        component: () => import('@/pages/system/roles/index.vue'),
        meta: { title: '角色列表', role: 'admin' },
      },
      {
        path: 'permissions',
        name: 'SystemPermissions',
        component: () => import('@/pages/system/permissions/index.vue'),
        meta: { title: '权限分配', role: 'admin' },
      },
      {
        path: 'admins',
        name: 'SystemAdmins',
        component: () => import('@/pages/system/admins/index.vue'),
        meta: { title: '管理员列表', role: 'admin' },
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory('/'),
  routes,
  scrollBehavior() {
    return { top: 0 }
  },
})

export default router
