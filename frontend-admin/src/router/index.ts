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
        meta: { title: '用户总览', role: 'admin', permission: 'system:user:list' },
      },
      {
        path: 'accounts/list',
        name: 'UserAccountsList',
        component: () => import('@/pages/users/accounts/list/index.vue'),
        meta: { title: '用户列表', role: 'admin', permission: 'system:user:list' },
      },
      {
        // 下钻页：由用户列表点击进入，不是导航目的地，因此不入侧边栏也不入标签栏。
        path: 'accounts/detail',
        name: 'UserAccountsDetail',
        component: () => import('@/pages/users/accounts/detail/index.vue'),
        meta: { title: '用户详情', role: 'admin', permission: 'user:detail', hideInTabs: true },
      },
      {
        path: 'accounts/groups',
        name: 'UserAccountsGroups',
        component: () => import('@/pages/users/accounts/groups/index.vue'),
        meta: { title: '用户组管理', role: 'admin', permission: 'user:group:list' },
      },
      {
        path: 'security/login-logs',
        name: 'UserSecurityLoginLogs',
        component: () => import('@/pages/users/security/login-logs/index.vue'),
        meta: { title: '登录日志', role: 'admin', permission: 'security:login-log:list' },
      },
      {
        path: 'security/audit-logs',
        name: 'UserSecurityAuditLogs',
        component: () => import('@/pages/users/security/audit-logs/index.vue'),
        meta: { title: '操作审计日志', role: 'admin', permission: 'security:audit:list' },
      },
      {
        path: 'security/risk',
        name: 'UserSecurityRisk',
        component: () => import('@/pages/users/security/risk/index.vue'),
        meta: { title: '异常行为监控', role: 'admin', permission: 'security:risk:list' },
      },
      {
        path: 'security/blacklist',
        name: 'UserSecurityBlacklist',
        component: () => import('@/pages/users/security/blacklist/index.vue'),
        meta: { title: '黑名单管理', role: 'admin', permission: 'security:blacklist:manage' },
      },
      {
        path: 'security/sessions',
        name: 'UserSecuritySessions',
        component: () => import('@/pages/users/security/sessions/index.vue'),
        meta: { title: '会话管理', role: 'admin', permission: 'security:session:manage' },
      },
      {
        path: 'levels',
        name: 'UserLevels',
        component: () => import('@/pages/users/levels/index.vue'),
        meta: { title: '用户等级管理', role: 'admin', permission: 'level:list' },
      },
      {
        path: 'verification/pending',
        name: 'UserVerificationPending',
        component: () => import('@/pages/users/verification/pending/index.vue'),
        meta: { title: '实名认证待审核', role: 'admin', permission: 'verification:list' },
      },
      {
        path: 'verification/approved',
        name: 'UserVerificationApproved',
        component: () => import('@/pages/users/verification/approved/index.vue'),
        meta: { title: '实名认证审核通过', role: 'admin', permission: 'verification:list' },
      },
      {
        path: 'verification/rejected',
        name: 'UserVerificationRejected',
        component: () => import('@/pages/users/verification/rejected/index.vue'),
        meta: { title: '实名认证审核拒绝', role: 'admin', permission: 'verification:list' },
      },
      {
        path: 'verification/config',
        name: 'UserVerificationConfig',
        component: () => import('@/pages/users/verification/config/index.vue'),
        meta: { title: '认证配置', role: 'admin', permission: 'verification:list' },
      },
    ],
  },
  {
    // P7 菜单归位（doc16 §9.2）：资源管理只做资源 —— 渠道与平台 / 容量与位置 / 同步与调度 / 实例 / 运维。
    path: '/resource',
    component: () => import('@/layouts/index.vue'),
    redirect: '/resource/providers',
    meta: { title: '资源管理' },
    children: [
      {
        path: 'overview',
        name: 'ResourceOverview',
        redirect: '/resource/providers',
        meta: { title: '资源总览', role: 'admin' },
      },
      // 资源总览/看板并入「容量与位置」（doc16 §9.4），旧路径 redirect 保持书签可用。
      {
        path: 'dashboard',
        name: 'ResourceDashboard',
        redirect: '/resource/pools',
        meta: { title: '资源总览', role: 'admin' },
      },
      // 同步监控并入同步与调度合并页（T3.6 sync-center）。
      {
        path: 'sync-monitor',
        name: 'ResourceSyncMonitor',
        redirect: '/resource/sync-center',
        meta: { title: '同步监控', role: 'admin' },
      },
      // —— 渠道与平台 ——
      // 本轮 S1：按链路拆两页 —— 上游转售渠道（kind=upstream）与自营平台对接（kind=compute），
      // 两个页面复用同一组件，通过路由 meta.channelKind 固定过滤链路。
      {
        path: 'channels',
        name: 'ResourceChannels',
        redirect: '/resource/providers',
        meta: { title: '渠道与平台', role: 'admin' },
      },
      {
        path: 'providers',
        name: 'ResourceProviders',
        component: () => import('@/pages/resource/providers/index.vue'),
        meta: { title: '上游转售渠道', role: 'admin', permission: 'resource:provider', channelKind: 'upstream' },
      },
      {
        path: 'platforms',
        name: 'ResourcePlatforms',
        component: () => import('@/pages/resource/providers/index.vue'),
        meta: { title: '自营平台对接', role: 'admin', permission: 'resource:provider', channelKind: 'compute' },
      },
      {
        path: 'providers/create',
        name: 'ResourceProvidersCreate',
        component: () => import('@/pages/resource/providers/create.vue'),
        meta: { title: '添加提供商', role: 'admin', permission: 'provider:create' },
      },
      {
        path: 'providers/:id',
        name: 'ResourceProvidersDetail',
        component: () => import('@/pages/resource/providers/detail.vue'),
        meta: { title: '提供商详情', role: 'admin', permission: 'resource:provider' },
      },
      // 连接测试页已下线（本轮 S1）：连通性内联到两个渠道列表的行内「测试连接」，旧路径 redirect。
      {
        path: 'connectivity',
        name: 'ResourceConnectivity',
        redirect: '/resource/providers',
        meta: { title: '连接测试', role: 'admin' },
      },
      // —— 容量与位置 ——
      {
        path: 'capacity',
        name: 'ResourceCapacity',
        redirect: '/resource/pools',
        meta: { title: '容量与位置', role: 'admin' },
      },
      {
        path: 'pools',
        name: 'ResourcePools',
        component: () => import('@/pages/resource/pools/index.vue'),
        meta: { title: '资源池与容量', role: 'admin', permission: 'resource:provider' },
      },
      // —— 同步与调度（T3.6 合并页：调度 / 任务 / 日志 / 差异 / 待确认调价）——
      {
        path: 'sync-center',
        name: 'ResourceSyncCenter',
        component: () => import('@/pages/resource/sync-center/index.vue'),
        meta: { title: '同步与调度', role: 'admin', permission: 'resource:sync' },
      },
      // 旧重复页面路径保留书签兼容：统一 redirect 到合并页（菜单已隐藏，文件待清理）。
      {
        path: 'sync',
        name: 'ResourceSync',
        redirect: '/resource/sync-center',
        meta: { title: '同步与调度', role: 'admin' },
      },
      {
        path: 'logs',
        name: 'ResourceLogs',
        redirect: '/resource/sync-center',
        meta: { title: '同步与调度', role: 'admin' },
      },
      {
        path: 'reconciliation',
        name: 'ResourceReconciliation',
        redirect: '/resource/sync-center',
        meta: { title: '同步与调度', role: 'admin' },
      },
      // —— 资源商品管理（T7.2 移入产品管理「商品对接」，旧路径 redirect）——
      {
        path: 'products-center',
        name: 'ResourceProductsCenter',
        redirect: '/product/upstream',
        meta: { title: '资源商品管理', role: 'admin' },
      },
      {
        path: 'products',
        name: 'ResourceProducts',
        redirect: '/product/upstream',
        meta: { title: '商品列表', role: 'admin' },
      },
      {
        // 商品同步并入「同步与调度」合并页（doc16 §9.1）。
        path: 'product-sync',
        name: 'ResourceProductSync',
        redirect: '/resource/sync-center',
        meta: { title: '商品同步', role: 'admin' },
      },
      {
        path: 'pricing',
        name: 'ResourcePricing',
        redirect: '/product/cost-pricing',
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
        meta: { title: '云主机实例', role: 'admin', permission: 'resource:instance' },
      },
      // —— 运维 ——
      {
        path: 'ops',
        name: 'ResourceOps',
        redirect: '/resource/anomalies',
        meta: { title: '运维', role: 'admin' },
      },
      // API 测试/连接测试页已下线（本轮 S1）：连通性内联到渠道列表行内，旧路径 redirect。
      {
        path: 'api-test',
        name: 'ResourceApiTest',
        redirect: '/resource/providers',
        meta: { title: 'API测试', role: 'admin' },
      },
      {
        path: 'anomalies',
        name: 'ResourceAnomalies',
        component: () => import('@/pages/resource/anomalies/index.vue'),
        meta: { title: '异常处理', role: 'admin', permission: 'resource:instance' },
      },
      // 任务队列（本轮 S3）：开通履约/实例动作/续费/同步四类任务的「是否到达上游」。
      {
        path: 'task-queue',
        name: 'ResourceTaskQueue',
        component: () => import('@/pages/resource/task-queue/index.vue'),
        meta: { title: '任务队列', role: 'admin', permission: 'resource:sync' },
      },
      // 实例对账（本轮 S4）：本地已开通实例与上游的售价/成本/到期时间比对。
      {
        path: 'reconcile',
        name: 'ResourceReconcile',
        component: () => import('@/pages/resource/reconcile/index.vue'),
        meta: { title: '实例对账', role: 'admin', permission: 'resource:instance' },
      },
      // 模块配置并入系统配置（doc16 §9.2），旧路径 redirect。
      {
        path: 'settings',
        name: 'ResourceSettings',
        redirect: '/system/config',
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
      // —— 商品管理（catalog）——
      {
        path: 'products',
        name: 'ProductProducts',
        component: () => import('@/pages/product/products/index.vue'),
        meta: { title: '商品列表', role: 'admin', permission: 'product:list' },
      },
      {
        path: 'products/create',
        name: 'ProductProductsCreate',
        component: () => import('@/pages/product/products/create.vue'),
        meta: { title: '创建商品', role: 'admin', permission: 'product:create' },
      },
      {
        path: 'products/:id/edit',
        name: 'ProductProductsEdit',
        component: () => import('@/pages/product/products/edit.vue'),
        meta: { title: '编辑商品', role: 'admin', permission: 'product:update' },
      },
      {
        path: 'products/:id',
        name: 'ProductProductsDetail',
        component: () => import('@/pages/product/products/detail.vue'),
        meta: { title: '商品详情', role: 'admin', permission: 'product:list' },
      },
      // —— 商品对接（T7.2，doc16 §9.3）：组件复用资源侧页面，仅菜单与路径归位 ——
      {
        path: 'binding',
        name: 'ProductBinding',
        redirect: '/product/upstream',
        meta: { title: '商品对接', role: 'admin' },
      },
      {
        path: 'upstream',
        name: 'ProductUpstreamCatalog',
        component: () => import('@/pages/resource/products/index.vue'),
        meta: { title: '上游商品目录', role: 'admin', permission: 'resource:product' },
      },
      {
        path: 'cost-pricing',
        name: 'ProductCostPricing',
        component: () => import('@/pages/resource/pricing/index.vue'),
        meta: { title: '成本与加价', role: 'admin', permission: 'product:update_price' },
      },
      // —— 规格管理（spec）——
      {
        path: 'spec/templates',
        name: 'ProductSpecTemplates',
        component: () => import('@/pages/product/spec/templates/index.vue'),
        meta: { title: '规格模板', role: 'admin', permission: 'spec:template:list' },
      },
      {
        path: 'spec/custom',
        name: 'ProductSpecCustom',
        component: () => import('@/pages/product/spec/custom/index.vue'),
        meta: { title: '自定义规格', role: 'admin', permission: 'spec:template:list' },
      },
      {
        path: 'spec/mappings',
        name: 'ProductSpecMappings',
        component: () => import('@/pages/product/spec/mappings/index.vue'),
        meta: { title: '规格映射', role: 'admin', permission: 'spec:mapping:list' },
      },
      // —— 定价与计费（pricing）——
      {
        path: 'pricing',
        name: 'ProductPricing',
        component: () => import('@/pages/product/pricing/index.vue'),
        meta: { title: '商品调价', role: 'admin', permission: 'pricing:list' },
      },
      {
        path: 'pricing/calculator',
        name: 'ProductPricingCalculator',
        component: () => import('@/pages/product/pricing/calculator/index.vue'),
        meta: { title: '价格计算器', role: 'admin', permission: 'pricing:list' },
      },
      {
        path: 'pricing/history',
        name: 'ProductPricingHistory',
        component: () => import('@/pages/product/pricing/history/index.vue'),
        meta: { title: '价格历史', role: 'admin', permission: 'pricing:list' },
      },
      {
        path: 'pricing/policies',
        name: 'ProductPricingPolicies',
        component: () => import('@/pages/product/pricing/policies/index.vue'),
        meta: { title: '折扣策略', role: 'admin', permission: 'pricing:list' },
      },
      {
        path: 'pricing/matrix',
        name: 'ProductPricingMatrix',
        component: () => import('@/pages/product/pricing/matrix/index.vue'),
        meta: { title: '周期价格', role: 'admin', permission: 'pricing:list' },
      },
      // —— 促销管理（promotion）——
      {
        path: 'promotion/coupons',
        name: 'ProductPromotionCoupons',
        component: () => import('@/pages/product/promotion/coupons/index.vue'),
        meta: { title: '优惠券管理', role: 'admin', permission: 'promotion:coupon:list' },
      },
      {
        path: 'promotion/activities',
        name: 'ProductPromotionActivities',
        component: () => import('@/pages/product/promotion/activities/index.vue'),
        meta: { title: '折扣活动', role: 'admin', permission: 'promotion:activity:list' },
      },
      {
        path: 'promotion/bundles',
        name: 'ProductPromotionBundles',
        component: () => import('@/pages/product/promotion/bundles/index.vue'),
        meta: { title: '套餐组合', role: 'admin', permission: 'promotion:activity:list' },
      },
      {
        path: 'promotion/recommends',
        name: 'ProductPromotionRecommends',
        component: () => import('@/pages/product/promotion/recommends/index.vue'),
        meta: { title: '推荐位管理', role: 'admin', permission: 'promotion:activity:list' },
      },
      // —— 商品分类（category）——
      {
        path: 'categories',
        name: 'ProductCategories',
        component: () => import('@/pages/product/categories/index.vue'),
        meta: { title: '分类管理', role: 'admin', permission: 'product:category' },
      },
      // —— 上游商品同步（T7.4 下线：任务/日志/差异由资源管理「同步与调度」合并页承接，doc16 §9.1）——
      {
        path: 'sync/tasks',
        name: 'ProductSyncTasks',
        redirect: '/resource/sync-center',
        meta: { title: '同步任务', role: 'admin' },
      },
      {
        path: 'sync/logs',
        name: 'ProductSyncLogs',
        redirect: '/resource/sync-center',
        meta: { title: '同步日志', role: 'admin' },
      },
      {
        path: 'sync/diff',
        name: 'ProductSyncDiff',
        redirect: '/resource/sync-center',
        meta: { title: '差异对比', role: 'admin' },
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
        meta: { title: '订单列表', role: 'admin', permission: 'order:list' },
      },
      {
        path: 'detail/:id',
        name: 'OrderDetail',
        component: () => import('@/pages/order/detail.vue'),
        meta: { title: '订单详情', role: 'admin', permission: 'order:list' },
      },
      {
        path: 'refunds',
        name: 'OrderRefunds',
        component: () => import('@/pages/order/refunds/index.vue'),
        meta: { title: '退款管理', role: 'admin', permission: 'order:refunds' },
      },
      {
        path: 'refunds/:id',
        name: 'OrderRefundDetail',
        component: () => import('@/pages/order/refunds/detail.vue'),
        meta: { title: '退款详情', role: 'admin', permission: 'order:refunds' },
      },
      {
        path: 'stats',
        name: 'OrderStats',
        component: () => import('@/pages/order/stats/index.vue'),
        meta: { title: '订单统计', role: 'admin', permission: 'order:stats' },
      },
    ],
  },
  {
    path: '/instances',
    component: () => import('@/layouts/index.vue'),
    redirect: '/instances/list',
    meta: { title: '实例管理' },
    children: [
      {
        path: 'list',
        name: 'InstanceList',
        component: () => import('@/pages/instances/index.vue'),
        meta: { title: '实例列表', role: 'admin', permission: 'resource:instance' },
      },
      {
        path: 'detail/:id',
        name: 'InstanceDetail',
        component: () => import('@/pages/instances/detail.vue'),
        meta: { title: '实例详情', role: 'admin', permission: 'resource:instance', hideInTabs: true },
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
        meta: { title: '工单列表', role: 'admin', permission: 'ticket:list' },
      },
      {
        path: 'detail/:id',
        name: 'TicketDetail',
        component: () => import('@/pages/ticket/detail.vue'),
        meta: { title: '工单详情', role: 'admin', permission: 'ticket:view' },
      },
      {
        path: 'categories',
        name: 'TicketCategories',
        component: () => import('@/pages/ticket/categories/index.vue'),
        meta: { title: '工单分类管理', role: 'admin', permission: 'ticket:category' },
      },
      {
        path: 'stats',
        name: 'TicketStats',
        component: () => import('@/pages/ticket/stats/index.vue'),
        meta: { title: '工单统计', role: 'admin', permission: 'ticket:stats' },
      },
    ],
  },
  {
    path: '/finance',
    component: () => import('@/layouts/index.vue'),
    redirect: '/finance/overview',
    meta: { title: '财务管理' },
    children: [
      // —— 财务总览 ——
      {
        path: 'overview',
        name: 'FinanceOverview',
        component: () => import('@/pages/finance/overview/index.vue'),
        meta: { title: '财务总览', role: 'admin', permission: 'finance:wallet' },
      },
      // —— 账户管理 ——
      {
        path: 'accounts/wallets',
        name: 'FinanceWallets',
        component: () => import('@/pages/finance/accounts/wallets/index.vue'),
        meta: { title: '用户钱包', role: 'admin', permission: 'finance:wallet' },
      },
      {
        path: 'accounts/adjust',
        name: 'FinanceAdjust',
        component: () => import('@/pages/finance/accounts/adjust.vue'),
        meta: { title: '人工调账', role: 'admin', permission: 'finance:adjust' },
      },
      // —— 交易流水 ——
      {
        path: 'transactions',
        name: 'FinanceTransactions',
        component: () => import('@/pages/finance/transactions/index.vue'),
        meta: { title: '资金流水', role: 'admin', permission: 'finance:wallet' },
      },
      // —— 充值提现 ——
      {
        path: 'recharges',
        name: 'FinanceRecharges',
        component: () => import('@/pages/finance/recharge/index.vue'),
        meta: { title: '充值管理', role: 'admin', permission: 'finance:recharge' },
      },
      {
        path: 'withdrawals',
        name: 'FinanceWithdrawals',
        component: () => import('@/pages/finance/withdraw/index.vue'),
        meta: { title: '提现管理', role: 'admin', permission: 'finance:withdraw' },
      },
      // —— 账单管理 ——
      {
        path: 'bills',
        name: 'FinanceBills',
        component: () => import('@/pages/finance/bills/index.vue'),
        meta: { title: '账单管理', role: 'admin', permission: 'finance:bill' },
      },
      {
        path: 'recon',
        name: 'FinanceReconciliation',
        component: () => import('@/pages/finance/bills/recon.vue'),
        meta: { title: '对账中心', role: 'admin', permission: 'finance:bill' },
      },
      // —— 财务报表 ——
      {
        path: 'report',
        name: 'FinanceReport',
        component: () => import('@/pages/finance/report/index.vue'),
        meta: { title: '财务报表', role: 'admin', permission: 'finance:wallet' },
      },
      // —— 财务配置 ——
      {
        path: 'config',
        name: 'FinanceConfig',
        component: () => import('@/pages/finance/config/index.vue'),
        meta: { title: '财务配置', role: 'admin', permission: 'finance:wallet' },
      },
    ],
  },
  {
    // 支付中心（doc35）：与财务管理并列的独立模块 —— 财务管账本，支付管钱进出的通道。
    path: '/payment',
    component: () => import('@/layouts/index.vue'),
    redirect: '/payment/overview',
    meta: { title: '支付中心' },
    children: [
      {
        path: 'overview',
        name: 'PaymentOverview',
        component: () => import('@/pages/payment/overview/index.vue'),
        meta: { title: '支付概览', role: 'admin', permission: 'payment:channel' },
      },
      {
        path: 'channels',
        name: 'PaymentChannels',
        component: () => import('@/pages/payment/channels/index.vue'),
        meta: { title: '支付渠道', role: 'admin', permission: 'payment:channel' },
      },
      {
        path: 'methods',
        name: 'PaymentMethods',
        component: () => import('@/pages/payment/methods/index.vue'),
        meta: { title: '支付方式', role: 'admin', permission: 'payment:method' },
      },
      {
        path: 'orders',
        name: 'PaymentOrders',
        component: () => import('@/pages/payment/orders/index.vue'),
        meta: { title: '支付订单', role: 'admin', permission: 'payment:order' },
      },
      {
        path: 'callbacks',
        name: 'PaymentCallbacks',
        component: () => import('@/pages/payment/callbacks/index.vue'),
        meta: { title: '回调日志', role: 'admin', permission: 'payment:callback' },
      },
      {
        path: 'refunds',
        name: 'PaymentRefunds',
        component: () => import('@/pages/payment/refunds/index.vue'),
        meta: { title: '渠道退款', role: 'admin', permission: 'payment:refund' },
      },
      {
        path: 'payouts',
        name: 'PaymentPayouts',
        component: () => import('@/pages/payment/payouts/index.vue'),
        meta: { title: '打款管理', role: 'admin', permission: 'payment:payout' },
      },
      {
        path: 'recon',
        name: 'PaymentRecon',
        component: () => import('@/pages/payment/recon/index.vue'),
        meta: { title: '渠道对账', role: 'admin', permission: 'payment:recon' },
      },
    ],
  },
  {
    path: '/referral',
    component: () => import('@/layouts/index.vue'),
    redirect: '/referral/cashbacks',
    meta: { title: '推广返现' },
    children: [
      {
        path: 'cashbacks',
        name: 'ReferralCashbacks',
        component: () => import('@/pages/referral/cashbacks/index.vue'),
        meta: { title: '返现台账', role: 'admin', permission: 'referral:cashback:list' },
      },
      {
        path: 'invitees',
        name: 'ReferralInvitees',
        component: () => import('@/pages/referral/invitees/index.vue'),
        meta: { title: '邀请关系', role: 'admin', permission: 'referral:cashback:list' },
      },
      {
        path: 'withdrawals',
        name: 'ReferralWithdrawals',
        component: () => import('@/pages/referral/withdrawals/index.vue'),
        meta: { title: '提现审核', role: 'admin', permission: 'referral:withdraw:list' },
      },
    ],
  },
  {
    path: '/lifecycle',
    component: () => import('@/layouts/index.vue'),
    redirect: '/lifecycle/expiring',
    meta: { title: '生命周期管理' },
    children: [
      {
        path: 'expiring',
        name: 'LifecycleExpiring',
        component: () => import('@/pages/lifecycle/expiring/index.vue'),
        meta: { title: '到期管理', role: 'admin', permission: 'lifecycle:expiring' },
      },
      {
        path: 'renewals',
        name: 'LifecycleRenewals',
        component: () => import('@/pages/lifecycle/renewals/index.vue'),
        meta: { title: '续费记录', role: 'admin', permission: 'lifecycle:renewals' },
      },
      {
        path: 'policy',
        name: 'LifecyclePolicy',
        component: () => import('@/pages/lifecycle/policy/index.vue'),
        meta: { title: '生命周期策略', role: 'admin', permission: 'lifecycle:policy' },
      },
    ],
  },
  {
    path: '/notification',
    component: () => import('@/layouts/index.vue'),
    redirect: '/notification/records',
    meta: { title: '消息中心', role: 'admin' },
    children: [
      {
        path: 'records',
        name: 'NotifyRecords',
        component: () => import('@/pages/notification/records/index.vue'),
        meta: { title: '通知记录', role: 'admin', permission: 'notify:record' },
      },
      {
        path: 'templates',
        name: 'NotifyTemplates',
        component: () => import('@/pages/notification/templates/index.vue'),
        meta: { title: '通知模板', role: 'admin', permission: 'notify:template' },
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
        meta: { title: '菜单管理', role: 'admin', permission: 'system:menu' },
      },
      {
        path: 'roles',
        name: 'SystemRoles',
        component: () => import('@/pages/system/roles/index.vue'),
        meta: { title: '角色列表', role: 'admin', permission: 'system:role:list' },
      },
      {
        path: 'permissions',
        name: 'SystemPermissions',
        component: () => import('@/pages/system/permissions/index.vue'),
        meta: { title: '权限分配', role: 'admin', permission: 'system:permission:view' },
      },
      {
        path: 'admins',
        name: 'SystemAdmins',
        component: () => import('@/pages/system/admins/index.vue'),
        meta: { title: '管理员列表', role: 'admin', permission: 'staff:list' },
      },
      {
        // 系统配置：键值型配置项的增删改查
        path: 'config',
        name: 'SystemConfig',
        component: () => import('@/pages/system/config/index.vue'),
        meta: { title: '系统配置', role: 'admin', permission: 'system:config:view' },
      },
      {
        // 操作审计：管理员操作日志查询与 CSV 导出
        path: 'audit-logs',
        name: 'SystemAuditLogs',
        component: () => import('@/pages/system/audit-logs/index.vue'),
        meta: { title: '操作审计', role: 'admin', permission: 'security:audit:list' },
      },
      {
        // 公告管理（复用 notification 公告服务，归类到系统管理/安全审计）
        path: 'announcements',
        name: 'SystemAnnouncements',
        component: () => import('@/pages/notification/announcements/index.vue'),
        meta: { title: '公告管理', role: 'admin', permission: 'notify:announcement' },
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
