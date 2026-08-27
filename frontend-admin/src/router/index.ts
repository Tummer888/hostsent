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
      {
        path: 'quota/resources',
        name: 'UserQuotaResources',
        component: () => import('@/pages/users/quota/resources/index.vue'),
        meta: { title: '用户资源配额', role: 'admin' },
      },
      {
        path: 'quota/templates',
        name: 'UserQuotaTemplates',
        component: () => import('@/pages/users/quota/templates/index.vue'),
        meta: { title: '配额模板管理', role: 'admin' },
      },
      {
        path: 'quota/tiers',
        name: 'UserQuotaTiers',
        component: () => import('@/pages/users/quota/tiers/index.vue'),
        meta: { title: '用户等级管理', role: 'admin' },
      },
      {
        path: 'quota/changes',
        name: 'UserQuotaChanges',
        component: () => import('@/pages/users/quota/changes/index.vue'),
        meta: { title: '配额调整记录', role: 'admin' },
      },
      {
        path: 'verification/pending',
        name: 'UserVerificationPending',
        component: () => import('@/pages/users/verification/pending/index.vue'),
        meta: { title: '实名认证待审核', role: 'admin' },
      },
      {
        path: 'verification/approved',
        name: 'UserVerificationApproved',
        component: () => import('@/pages/users/verification/approved/index.vue'),
        meta: { title: '实名认证审核通过', role: 'admin' },
      },
      {
        path: 'verification/rejected',
        name: 'UserVerificationRejected',
        component: () => import('@/pages/users/verification/rejected/index.vue'),
        meta: { title: '实名认证审核拒绝', role: 'admin' },
      },
      {
        path: 'verification/config',
        name: 'UserVerificationConfig',
        component: () => import('@/pages/users/verification/config/index.vue'),
        meta: { title: '认证配置', role: 'admin' },
      },
    ],
  },
  {
    path: '/upstream',
    component: () => import('@/layouts/index.vue'),
    redirect: '/upstream/dashboard',
    meta: { title: '资源管理' },
    children: [
      // —— 资源总览 ——
      {
        path: 'dashboard',
        name: 'UpstreamDashboard',
        component: () => import('@/pages/upstream/dashboard/index.vue'),
        meta: { title: '资源大盘', role: 'admin' },
      },
      {
        path: 'sync-monitor',
        name: 'UpstreamSyncMonitor',
        component: () => import('@/pages/upstream/sync-monitor/index.vue'),
        meta: { title: '同步监控', role: 'admin' },
      },
      // —— 上游对接管理 ——
      {
        path: 'providers',
        name: 'UpstreamProviders',
        component: () => import('@/pages/upstream/providers/index.vue'),
        meta: { title: '上游提供商', role: 'admin' },
      },
      {
        path: 'pools',
        name: 'UpstreamPools',
        component: () => import('@/pages/upstream/pools/index.vue'),
        meta: { title: '资源池管理', role: 'admin' },
      },
      {
        path: 'connectivity',
        name: 'UpstreamConnectivity',
        component: () => import('@/pages/upstream/connectivity/index.vue'),
        meta: { title: '连接测试', role: 'admin' },
      },
      // —— 资源同步与对账 ——
      {
        path: 'sync',
        name: 'UpstreamSync',
        component: () => import('@/pages/upstream/sync/index.vue'),
        meta: { title: '同步任务', role: 'admin' },
      },
      {
        path: 'logs',
        name: 'UpstreamLogs',
        component: () => import('@/pages/upstream/logs/index.vue'),
        meta: { title: '同步日志', role: 'admin' },
      },
      {
        path: 'reconciliation',
        name: 'UpstreamReconciliation',
        component: () => import('@/pages/upstream/reconciliation/index.vue'),
        meta: { title: '对账报告', role: 'admin' },
      },
      // —— 资源商品管理 ——
      {
        path: 'products',
        name: 'UpstreamProducts',
        component: () => import('@/pages/upstream/products/index.vue'),
        meta: { title: '商品列表', role: 'admin' },
      },
      {
        path: 'product-sync',
        name: 'UpstreamProductSync',
        component: () => import('@/pages/upstream/product-sync/index.vue'),
        meta: { title: '商品同步', role: 'admin' },
      },
      {
        path: 'pricing',
        name: 'UpstreamPricing',
        component: () => import('@/pages/upstream/pricing/index.vue'),
        meta: { title: '定价管理', role: 'admin' },
      },
      // —— 实例资源 ——
      {
        path: 'instances',
        name: 'UpstreamInstances',
        component: () => import('@/pages/upstream/instances/index.vue'),
        meta: { title: '云主机实例', role: 'admin' },
      },
      // —— 运维工具 ——
      {
        path: 'api-test',
        name: 'UpstreamApiTest',
        component: () => import('@/pages/upstream/api-test/index.vue'),
        meta: { title: 'API测试', role: 'admin' },
      },
      {
        path: 'anomalies',
        name: 'UpstreamAnomalies',
        component: () => import('@/pages/upstream/anomalies/index.vue'),
        meta: { title: '异常处理', role: 'admin' },
      },
      {
        path: 'settings',
        name: 'UpstreamSettings',
        component: () => import('@/pages/upstream/settings/index.vue'),
        meta: { title: '系统配置', role: 'admin' },
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
