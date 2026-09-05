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
    path: '/resource',
    component: () => import('@/layouts/index.vue'),
    redirect: '/resource/dashboard',
    meta: { title: '资源管理' },
    children: [
      // —— 资源总览 ——
      {
        path: 'overview',
        name: 'ResourceOverview',
        redirect: '/resource/dashboard',
        meta: { title: '资源总览', role: 'admin' },
      },
      {
        path: 'dashboard',
        name: 'ResourceDashboard',
        component: () => import('@/pages/resource/dashboard/index.vue'),
        meta: { title: '资源总览', role: 'admin' },
      },
      {
        path: 'sync-monitor',
        name: 'ResourceSyncMonitor',
        component: () => import('@/pages/resource/sync-monitor/index.vue'),
        meta: { title: '同步监控', role: 'admin' },
      },
      // —— 上游对接管理 ——
      {
        path: 'connection',
        name: 'ResourceConnection',
        redirect: '/resource/providers',
        meta: { title: '上游对接管理', role: 'admin' },
      },
      {
        path: 'providers',
        name: 'ResourceProviders',
        component: () => import('@/pages/resource/providers/index.vue'),
        meta: { title: '上游提供商', role: 'admin' },
      },
      {
        path: 'providers/create',
        name: 'ResourceProvidersCreate',
        component: () => import('@/pages/resource/providers/create.vue'),
        meta: { title: '添加提供商', role: 'admin' },
      },
      {
        path: 'providers/:id',
        name: 'ResourceProvidersDetail',
        component: () => import('@/pages/resource/providers/detail.vue'),
        meta: { title: '提供商详情', role: 'admin' },
      },
      {
        path: 'pools',
        name: 'ResourcePools',
        component: () => import('@/pages/resource/pools/index.vue'),
        meta: { title: '资源池管理', role: 'admin' },
      },
      {
        path: 'connectivity',
        name: 'ResourceConnectivity',
        component: () => import('@/pages/resource/connectivity/index.vue'),
        meta: { title: '连接测试', role: 'admin' },
      },
      // —— 资源同步与对账 ——
      {
        path: 'sync-center',
        name: 'ResourceSyncCenter',
        redirect: '/resource/sync',
        meta: { title: '资源同步与对账', role: 'admin' },
      },
      {
        path: 'sync',
        name: 'ResourceSync',
        component: () => import('@/pages/resource/sync/index.vue'),
        meta: { title: '同步任务', role: 'admin' },
      },
      {
        path: 'logs',
        name: 'ResourceLogs',
        component: () => import('@/pages/resource/logs/index.vue'),
        meta: { title: '同步日志', role: 'admin' },
      },
      {
        path: 'reconciliation',
        name: 'ResourceReconciliation',
        component: () => import('@/pages/resource/reconciliation/index.vue'),
        meta: { title: '对账报告', role: 'admin' },
      },
      // —— 资源商品管理 ——
      {
        path: 'products-center',
        name: 'ResourceProductsCenter',
        redirect: '/resource/products',
        meta: { title: '资源商品管理', role: 'admin' },
      },
      {
        path: 'products',
        name: 'ResourceProducts',
        component: () => import('@/pages/resource/products/index.vue'),
        meta: { title: '商品列表', role: 'admin' },
      },
      {
        path: 'product-sync',
        name: 'ResourceProductSync',
        component: () => import('@/pages/resource/product-sync/index.vue'),
        meta: { title: '商品同步', role: 'admin' },
      },
      {
        path: 'pricing',
        name: 'ResourcePricing',
        component: () => import('@/pages/resource/pricing/index.vue'),
        meta: { title: '定价管理', role: 'admin' },
      },
      // —— 实例资源 ——
      {
        path: 'instance',
        name: 'ResourceInstance',
        redirect: '/resource/instances',
        meta: { title: '实例资源', role: 'admin' },
      },
      {
        path: 'instances',
        name: 'ResourceInstances',
        component: () => import('@/pages/resource/instances/index.vue'),
        meta: { title: '云主机实例', role: 'admin' },
      },
      // —— 运维工具 ——
      {
        path: 'ops',
        name: 'ResourceOps',
        redirect: '/resource/api-test',
        meta: { title: '运维工具', role: 'admin' },
      },
      {
        path: 'api-test',
        name: 'ResourceApiTest',
        component: () => import('@/pages/resource/api-test/index.vue'),
        meta: { title: 'API测试', role: 'admin' },
      },
      {
        path: 'anomalies',
        name: 'ResourceAnomalies',
        component: () => import('@/pages/resource/anomalies/index.vue'),
        meta: { title: '异常处理', role: 'admin' },
      },
      {
        path: 'settings',
        name: 'ResourceSettings',
        component: () => import('@/pages/resource/settings/index.vue'),
        meta: { title: '系统配置', role: 'admin' },
      },
    ],
  },
  {
    path: '/product',
    component: () => import('@/layouts/index.vue'),
    redirect: '/product/products',
    meta: { title: '产品管理' },
    children: [
      {
        path: 'products',
        name: 'ProductProducts',
        component: () => import('@/pages/product/products/index.vue'),
        meta: { title: '产品列表', role: 'admin' },
      },
      {
        path: 'products/create',
        name: 'ProductProductsCreate',
        component: () => import('@/pages/product/products/create.vue'),
        meta: { title: '新建产品', role: 'admin' },
      },
      {
        path: 'products/:id/edit',
        name: 'ProductProductsEdit',
        component: () => import('@/pages/product/products/edit.vue'),
        meta: { title: '编辑产品', role: 'admin' },
      },
      {
        path: 'products/:id',
        name: 'ProductProductsDetail',
        component: () => import('@/pages/product/products/detail.vue'),
        meta: { title: '产品详情', role: 'admin' },
      },
      {
        path: 'categories',
        name: 'ProductCategories',
        component: () => import('@/pages/product/categories/index.vue'),
        meta: { title: '分类管理', role: 'admin' },
      },
      {
        path: 'pricing',
        name: 'ProductPricing',
        component: () => import('@/pages/product/pricing/index.vue'),
        meta: { title: '价格与上下架', role: 'admin' },
      },
    ],
  },
  {
    path: '/orders',
    component: () => import('@/layouts/index.vue'),
    redirect: '/orders/list',
    meta: { title: '订单管理' },
    children: [
      {
        path: 'list',
        name: 'OrderList',
        component: () => import('@/pages/order/index.vue'),
        meta: { title: '订单列表', role: 'admin' },
      },
      {
        path: 'detail/:id',
        name: 'OrderDetail',
        component: () => import('@/pages/order/detail.vue'),
        meta: { title: '订单详情', role: 'admin' },
      },
      {
        path: 'refunds',
        name: 'OrderRefunds',
        component: () => import('@/pages/order/refunds/index.vue'),
        meta: { title: '退款管理', role: 'admin' },
      },
      {
        path: 'refunds/:id',
        name: 'OrderRefundDetail',
        component: () => import('@/pages/order/refunds/detail.vue'),
        meta: { title: '退款详情', role: 'admin' },
      },
      {
        path: 'stats',
        name: 'OrderStats',
        component: () => import('@/pages/order/stats/index.vue'),
        meta: { title: '订单统计', role: 'admin' },
      },
    ],
  },
  {
    path: '/tickets',
    component: () => import('@/layouts/index.vue'),
    redirect: '/tickets/list',
    meta: { title: '工单支持' },
    children: [
      {
        path: 'list',
        name: 'TicketList',
        component: () => import('@/pages/ticket/index.vue'),
        meta: { title: '工单列表', role: 'admin' },
      },
      {
        path: 'detail/:id',
        name: 'TicketDetail',
        component: () => import('@/pages/ticket/detail.vue'),
        meta: { title: '工单详情', role: 'admin' },
      },
      {
        path: 'categories',
        name: 'TicketCategories',
        component: () => import('@/pages/ticket/categories/index.vue'),
        meta: { title: '工单分类管理', role: 'admin' },
      },
      {
        path: 'stats',
        name: 'TicketStats',
        component: () => import('@/pages/ticket/stats/index.vue'),
        meta: { title: '工单统计', role: 'admin' },
      },
    ],
  },
  {
    path: '/finance',
    component: () => import('@/layouts/index.vue'),
    redirect: '/finance/wallets',
    meta: { title: '财务管理' },
    children: [
      {
        path: 'wallets',
        name: 'FinanceWallets',
        component: () => import('@/pages/finance/wallets/index.vue'),
        meta: { title: '用户钱包', role: 'admin' },
      },
      {
        path: 'transactions',
        name: 'FinanceTransactions',
        component: () => import('@/pages/finance/transactions/index.vue'),
        meta: { title: '资金流水', role: 'admin' },
      },
      {
        path: 'transactions/adjust',
        name: 'FinanceAdjust',
        component: () => import('@/pages/finance/transactions/adjust.vue'),
        meta: { title: '人工调账', role: 'admin' },
      },
      {
        path: 'recharges',
        name: 'FinanceRecharges',
        component: () => import('@/pages/finance/recharges/index.vue'),
        meta: { title: '充值管理', role: 'admin' },
      },
      {
        path: 'withdrawals',
        name: 'FinanceWithdrawals',
        component: () => import('@/pages/finance/withdrawals/index.vue'),
        meta: { title: '提现管理', role: 'admin' },
      },
      {
        path: 'bills',
        name: 'FinanceBills',
        component: () => import('@/pages/finance/bills/index.vue'),
        meta: { title: '账单管理', role: 'admin' },
      },
      {
        path: 'recon',
        name: 'FinanceReconciliation',
        component: () => import('@/pages/finance/bills/recon.vue'),
        meta: { title: '对账中心', role: 'admin' },
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
      {
        // 系统配置：键值型配置项的增删改查
        path: 'config',
        name: 'SystemConfig',
        component: () => import('@/pages/system/config/index.vue'),
        meta: { title: '系统配置', role: 'admin' },
      },
      {
        // 操作审计：管理员操作日志查询与 CSV 导出
        path: 'audit-logs',
        name: 'SystemAuditLogs',
        component: () => import('@/pages/system/audit-logs/index.vue'),
        meta: { title: '操作审计', role: 'admin' },
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
