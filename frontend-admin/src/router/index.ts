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
        // 与「系统管理 → 操作审计」是同一件事（后者是前者的超集：双 Tab 含用户审计 + 管理审计），
        // 重复页已删（doc102 §4.1 M2-5），旧路径保留 redirect。
        path: 'security/audit-logs',
        name: 'UserSecurityAuditLogs',
        // 直指最终落点：操作审计归属「日志中心」组（迁移 072），路径沿用原 /system/audit-logs。
        redirect: '/system/audit-logs',
        meta: { title: '操作审计', role: 'admin' },
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
        // 用户等级归入「账户管理」子模块（归属调整，API/权限码不变），旧路径 redirect。
        path: 'accounts/levels',
        name: 'UserLevels',
        component: () => import('@/pages/users/accounts/levels/index.vue'),
        meta: { title: '用户等级管理', role: 'admin', permission: 'level:list' },
      },
      {
        path: 'levels',
        redirect: '/users/accounts/levels',
        meta: { title: '用户等级管理', role: 'admin' },
      },
      {
        // 待审核 / 审核通过 / 审核拒绝三个列表合并为一页页签切换（用户反馈），
        // 旧路径 redirect 到对应页签，书签不失效。
        path: 'verification/list',
        name: 'UserVerificationList',
        component: () => import('@/pages/users/verification/list/index.vue'),
        meta: { title: '实名认证审核', role: 'admin', permission: 'verification:list' },
      },
      {
        path: 'verification/pending',
        redirect: '/users/verification/list?tab=pending',
        meta: { title: '实名认证待审核', role: 'admin' },
      },
      {
        path: 'verification/approved',
        redirect: '/users/verification/list?tab=approved',
        meta: { title: '实名认证审核通过', role: 'admin' },
      },
      {
        path: 'verification/rejected',
        redirect: '/users/verification/list?tab=rejected',
        meta: { title: '实名认证审核拒绝', role: 'admin' },
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
        // 云主机实例随「实例」二级目录迁入一级域 /instances（doc102 §3.3），旧路径 redirect。
        path: 'instances',
        name: 'ResourceInstances',
        redirect: '/instances/inventory',
        meta: { title: '云主机实例', role: 'admin' },
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
      // 「折扣策略」(pricing/policies) 已随 doc108 §8J 下线：它对应的 price_policies
      // 自迁移 059 起就不再参与算价，页面还写着「上线折扣请用折扣策略」，把运营带偏。
      // 面向用户的限时折扣见「促销管理 → 折扣活动」。
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
      {
        // 云主机实例（只读资产视图）：原「资源管理 → 实例 → 云主机实例」，doc102 §3.3 迁入本域。
        path: 'inventory',
        name: 'InstanceInventory',
        component: () => import('@/pages/resource/instances/index.vue'),
        meta: { title: '云主机实例', role: 'admin', permission: 'resource:instance' },
      },
      {
        // 生命周期管理（doc60）整域并入实例管理（迁移 066）：目录只做分组，落地页在 expiring。
        path: 'lifecycle',
        redirect: '/instances/lifecycle/expiring',
        meta: { title: '生命周期管理', role: 'admin' },
      },
      {
        path: 'lifecycle/expiring',
        name: 'LifecycleExpiring',
        component: () => import('@/pages/instances/lifecycle/expiring/index.vue'),
        meta: { title: '到期管理', role: 'admin', permission: 'lifecycle:expiring' },
      },
      {
        path: 'lifecycle/renewals',
        name: 'LifecycleRenewals',
        component: () => import('@/pages/instances/lifecycle/renewals/index.vue'),
        meta: { title: '续费记录', role: 'admin', permission: 'lifecycle:renewals' },
      },
      {
        path: 'lifecycle/policy',
        name: 'LifecyclePolicy',
        component: () => import('@/pages/instances/lifecycle/policy/index.vue'),
        meta: { title: '生命周期策略', role: 'admin', permission: 'lifecycle:policy' },
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
        path: 'reviews',
        name: 'TicketReviews',
        component: () => import('@/pages/ticket/reviews/index.vue'),
        meta: { title: '复核中心', role: 'admin', permission: 'ticket:review' },
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
      // —— 账户管理（资金管理组，迁移 073：资金流水改挂本组） ——
      {
        path: 'transactions',
        name: 'FinanceTransactions',
        component: () => import('@/pages/finance/transactions/index.vue'),
        meta: { title: '资金流水', role: 'admin', permission: 'finance:wallet' },
      },
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
      // —— 账单与对账（迁移 073：目录改名，叶子不变） ——
      {
        path: 'bills',
        name: 'FinanceBills',
        component: () => import('@/pages/finance/bills/index.vue'),
        meta: { title: '账单列表', role: 'admin', permission: 'finance:bill' },
      },
      {
        path: 'recon',
        name: 'FinanceReconciliation',
        component: () => import('@/pages/finance/bills/recon.vue'),
        meta: { title: '对账中心', role: 'admin', permission: 'finance:bill' },
      },
      // 发票管理（doc36 §3.3）：用户申请 → 管理端开票/驳回。
      {
        path: 'invoices',
        name: 'FinanceInvoices',
        component: () => import('@/pages/finance/invoices/index.vue'),
        meta: { title: '发票管理', role: 'admin', permission: 'finance:invoice' },
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
        meta: { title: '财务配置', role: 'admin', permission: 'finance:config' },
      },
      // 推广返现（doc84）整域并入财务管理（迁移 070）：作为「推广返现」二级目录的子模块。
      {
        path: 'referral',
        redirect: '/finance/referral/cashbacks',
        meta: { title: '推广返现', role: 'admin' },
      },
      {
        path: 'referral/cashbacks',
        name: 'ReferralCashbacks',
        component: () => import('@/pages/finance/referral/cashbacks/index.vue'),
        meta: { title: '返现台账', role: 'admin', permission: 'referral:cashback:list' },
      },
      {
        path: 'referral/invitees',
        name: 'ReferralInvitees',
        component: () => import('@/pages/finance/referral/invitees/index.vue'),
        meta: { title: '邀请关系', role: 'admin', permission: 'referral:cashback:list' },
      },
      {
        path: 'referral/withdrawals',
        name: 'ReferralWithdrawals',
        component: () => import('@/pages/finance/referral/withdrawals/index.vue'),
        meta: { title: '提现审核', role: 'admin', permission: 'referral:withdraw:list' },
      },
      // 积分中心（doc36）整域并入财务管理（迁移 070）：作为「积分中心」二级目录的子模块。
      {
        path: 'points',
        redirect: '/finance/points/overview',
        meta: { title: '积分中心', role: 'admin' },
      },
      {
        path: 'points/overview',
        name: 'PointsOverview',
        component: () => import('@/pages/finance/points/overview/index.vue'),
        meta: { title: '积分概览', role: 'admin', permission: 'point:account' },
      },
      {
        path: 'points/rules',
        name: 'PointsRules',
        component: () => import('@/pages/finance/points/rules/index.vue'),
        meta: { title: '积分规则', role: 'admin', permission: 'point:rule' },
      },
      {
        path: 'points/accounts',
        name: 'PointsAccounts',
        component: () => import('@/pages/finance/points/accounts/index.vue'),
        meta: { title: '积分账户', role: 'admin', permission: 'point:account' },
      },
      {
        path: 'points/transactions',
        name: 'PointsTransactions',
        component: () => import('@/pages/finance/points/transactions/index.vue'),
        meta: { title: '积分流水', role: 'admin', permission: 'point:transaction' },
      },
    ],
  },
  // 支付中心整域并入「系统管理」（迁移 067）：页面/菜单已迁到 /system/payment/*，
  // 旧路径保留 redirect 兼容书签与已保存的页签（归属调整，HTTP API / 权限码不变）。
  {
    path: '/payment',
    redirect: '/system/payment/overview',
    meta: { title: '支付中心' },
  },
  {
    path: '/payment/overview',
    redirect: '/system/payment/overview',
    meta: { title: '支付概览' },
  },
  {
    path: '/payment/channels',
    redirect: '/system/payment/channels',
    meta: { title: '支付渠道' },
  },
  {
    path: '/payment/methods',
    redirect: '/system/payment/methods',
    meta: { title: '支付方式' },
  },
  {
    path: '/payment/orders',
    redirect: '/system/payment/orders',
    meta: { title: '支付订单' },
  },
  {
    path: '/payment/callbacks',
    redirect: '/system/payment/callbacks',
    meta: { title: '回调日志' },
  },
  {
    path: '/payment/refunds',
    redirect: '/system/payment/refunds',
    meta: { title: '渠道退款' },
  },
  {
    path: '/payment/payouts',
    redirect: '/system/payment/payouts',
    meta: { title: '打款管理' },
  },
  {
    path: '/payment/recon',
    redirect: '/system/payment/recon',
    meta: { title: '渠道对账' },
  },
  // 销售中心（doc86 S4–S6）整域并入「系统管理」（迁移 068）：页面/菜单已迁到
  // /system/sales/*，旧路径保留 redirect 兼容书签与已保存的页签（归属调整，权限码不变）。
  {
    path: '/sales',
    redirect: '/system/sales/customers',
    meta: { title: '销售中心' },
  },
  {
    path: '/sales/customers',
    redirect: '/system/sales/customers',
    meta: { title: '客户归属' },
  },
  {
    path: '/sales/commissions',
    redirect: '/system/sales/commissions',
    meta: { title: '提成台账' },
  },
  {
    path: '/sales/withdrawals',
    redirect: '/system/sales/withdrawals',
    meta: { title: '提成审核' },
  },
  {
    path: '/sales/performance',
    redirect: '/system/sales/performance',
    meta: { title: '业绩排行' },
  },
  // 推广返现、积分中心整域并入「财务管理」（迁移 070）：页面/菜单已迁到 /finance/referral/*、
  // /finance/points/*，旧路径保留 redirect 兼容书签与已保存的页签（归属调整，权限码不变）。
  // 注意：用户端（frontend-user）另有同名的 /referral 与 /points 路由，与此互不影响。
  {
    path: '/referral',
    redirect: '/finance/referral/cashbacks',
    meta: { title: '推广返现' },
  },
  {
    path: '/referral/cashbacks',
    redirect: '/finance/referral/cashbacks',
    meta: { title: '返现台账' },
  },
  {
    path: '/referral/invitees',
    redirect: '/finance/referral/invitees',
    meta: { title: '邀请关系' },
  },
  {
    path: '/referral/withdrawals',
    redirect: '/finance/referral/withdrawals',
    meta: { title: '提现审核' },
  },
  {
    path: '/points',
    redirect: '/finance/points/overview',
    meta: { title: '积分中心' },
  },
  {
    path: '/points/overview',
    redirect: '/finance/points/overview',
    meta: { title: '积分概览' },
  },
  {
    path: '/points/rules',
    redirect: '/finance/points/rules',
    meta: { title: '积分规则' },
  },
  {
    path: '/points/accounts',
    redirect: '/finance/points/accounts',
    meta: { title: '积分账户' },
  },
  {
    path: '/points/transactions',
    redirect: '/finance/points/transactions',
    meta: { title: '积分流水' },
  },
  // 生命周期管理整域并入「实例管理」（迁移 066）：页面/菜单已迁到 /instances/lifecycle，
  // 旧路径保留 redirect 兼容书签与已保存的页签（归属调整，API/权限码不变）。
  {
    path: '/lifecycle',
    redirect: '/instances/lifecycle/expiring',
    meta: { title: '生命周期管理' },
  },
  {
    path: '/lifecycle/expiring',
    redirect: '/instances/lifecycle/expiring',
    meta: { title: '到期管理' },
  },
  {
    path: '/lifecycle/renewals',
    redirect: '/instances/lifecycle/renewals',
    meta: { title: '续费记录' },
  },
  {
    path: '/lifecycle/policy',
    redirect: '/instances/lifecycle/policy',
    meta: { title: '生命周期策略' },
  },
  // 消息中心（doc70/doc90）整域并入「系统管理」（迁移 068）：页面/菜单已迁到
  // /system/notification/*，旧路径保留 redirect 兼容书签与已保存的页签（归属调整，权限码不变）。
  {
    path: '/notification',
    redirect: '/system/notification/records',
    meta: { title: '消息中心' },
  },
  {
    path: '/notification/records',
    redirect: '/system/notification/records',
    meta: { title: '通知记录' },
  },
  {
    path: '/notification/templates',
    redirect: '/system/notification/templates',
    meta: { title: '通知模板' },
  },
  {
    path: '/notification/channels',
    redirect: '/system/notification/channels',
    meta: { title: '渠道配置' },
  },
  {
    path: '/notification/sms-templates',
    redirect: '/system/notification/sms-templates',
    meta: { title: '短信模板' },
  },
  {
    path: '/notification/broadcast',
    redirect: '/system/notification/broadcast',
    meta: { title: '消息群发' },
  },
  {
    path: '/notification/deliveries',
    redirect: '/system/notification/deliveries',
    meta: { title: '发送日志' },
  },
  // 内容管理（doc100 §7.1）整域并入「系统管理」（迁移 069）：页面/菜单已迁到
  // /system/content/*，旧路径保留 redirect 兼容书签与已保存的页签（归属调整，权限码不变）。
  {
    path: '/content',
    redirect: '/system/content/articles',
    meta: { title: '内容管理' },
  },
  {
    path: '/content/articles',
    redirect: '/system/content/articles',
    meta: { title: '内容文章' },
  },
  {
    path: '/content/categories',
    redirect: '/system/content/categories',
    meta: { title: '内容分类' },
  },
  {
    path: '/content/links',
    redirect: '/system/content/links',
    meta: { title: '友情链接' },
  },
  {
    path: '/content/announcements',
    redirect: '/system/content/announcements',
    meta: { title: '公告管理' },
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
        meta: { title: '员工管理', role: 'admin', permission: 'staff:list' },
      },
      {
        // 部门管理（S1 员工体系）：组织架构维护，决定工单派单与销售归属范围
        path: 'departments',
        name: 'SystemDepartments',
        component: () => import('@/pages/system/departments/index.vue'),
        meta: { title: '部门管理', role: 'admin', permission: 'department:list' },
      },
      // 系统配置组（迁移 071）：原 /system/config、/system/captcha、/system/oauth 收进同一
      // 二级目录，运营在一处完成系统设置（操作审计同期收进、迁移 072 已改挂日志中心）。
      // 目录本身只做分组，落地页是基础配置；旧路径都保留 redirect 兼容书签。
      {
        path: 'config',
        redirect: '/system/config/basic',
        meta: { title: '系统配置', role: 'admin' },
      },
      {
        // 基础配置：键值型配置项的增删改查（原 /system/config 页面）
        path: 'config/basic',
        name: 'SystemConfig',
        component: () => import('@/pages/system/config/index.vue'),
        meta: { title: '基础配置', role: 'admin', permission: 'system:config:view' },
      },
      {
        // 验证码配置（doc91）：服务商 / 场景策略 / 统计
        path: 'config/captcha',
        name: 'SystemCaptcha',
        component: () => import('@/pages/system/captcha/index.vue'),
        meta: { title: '验证码配置', role: 'admin', permission: 'captcha:config' },
      },
      {
        // 第三方登录（doc104 §6.7）：微信/QQ/支付宝渠道凭证与启用开关。
        path: 'config/oauth',
        name: 'SystemOAuth',
        component: () => import('@/pages/system/oauth/index.vue'),
        meta: { title: '第三方登录', role: 'admin', permission: 'oauth:config' },
      },
      {
        // 操作审计在迁移 072 改挂「日志中心」并恢复原路径 /system/audit-logs；
        // 这里只保留短暂存在过的 /system/config/audit-logs 作为兼容跳转。
        path: 'config/audit-logs',
        redirect: '/system/audit-logs',
        meta: { title: '操作审计', role: 'admin' },
      },
      {
        path: 'captcha',
        redirect: '/system/config/captcha',
        meta: { title: '验证码配置', role: 'admin' },
      },
      {
        path: 'oauth',
        redirect: '/system/config/oauth',
        meta: { title: '第三方登录', role: 'admin' },
      },
      {
        // 操作审计（admin 平台）：管理员操作日志查询与 CSV 导出。
        // 归属「日志中心」组（迁移 072），与日志浏览/清理任务/保留策略同组，语义上都是日志查看。
        path: 'audit-logs',
        name: 'SystemAuditLogs',
        component: () => import('@/pages/system/audit-logs/index.vue'),
        meta: { title: '操作审计', role: 'admin', permission: 'security:audit:list' },
      },
      {
        // 公告管理已归位到「内容管理」（doc102 §4.1 M2-4，推翻 doc100 §7.1 的「保持原位」）：
        // 页面、后端模块与权限码 notify:announcement 都属 notification，旧路径 redirect。
        path: 'announcements',
        name: 'SystemAnnouncements',
        // 直指最终落点：公告管理随内容管理并入系统域（迁移 069）。
        redirect: '/system/content/announcements',
        meta: { title: '公告管理', role: 'admin' },
      },
      {
        // 日志中心（doc92）：26 类日志统一浏览，列与筛选项由后端 catalog 下发
        path: 'logs',
        name: 'SystemLogs',
        component: () => import('@/pages/system/logs/index.vue'),
        meta: { title: '日志中心', role: 'admin', permission: 'log:center' },
      },
      {
        // 清理任务（doc92）：预演 / 执行 / 任务列表 / 导出文件下载
        path: 'logs/cleanup',
        name: 'SystemLogsCleanup',
        component: () => import('@/pages/system/logs/cleanup/index.vue'),
        meta: { title: '清理任务', role: 'admin', permission: 'log:cleanup' },
      },
      {
        // 保留策略（doc92）：逐源配置 action / 保留天数 / 批次
        path: 'logs/policy',
        name: 'SystemLogsPolicy',
        component: () => import('@/pages/system/logs/policy/index.vue'),
        meta: { title: '保留策略', role: 'admin', permission: 'log:policy' },
      },
      // 支付中心（doc35）整域并入系统管理（迁移 067）：作为「支付中心」二级目录的子模块，
      // 目录只做分组、不做落地页，默认落在 overview。原 /payment/* 保留 redirect 兼容书签。
      // 注意：侧栏只渲染「一级域 + 二级分组 + 叶子」三层，因此原渠道/交易/出款三个二级分类
      // 已压平到本层 —— 再嵌一层的话第四层叶子的路径在侧栏点不到。
      {
        path: 'payment',
        redirect: '/system/payment/overview',
        meta: { title: '支付中心', role: 'admin' },
      },
      {
        path: 'payment/overview',
        name: 'PaymentOverview',
        component: () => import('@/pages/system/payment/overview/index.vue'),
        // 与 permission_map.go 口径统一为 payment:order（doc102 M3-1）：该页聚合
        // channels/orders/callbacks/payouts/refunds 五个接口，各自在服务端单独鉴权。
        meta: { title: '支付概览', role: 'admin', permission: 'payment:order' },
      },
      {
        path: 'payment/channels',
        name: 'PaymentChannels',
        component: () => import('@/pages/system/payment/channels/index.vue'),
        meta: { title: '支付渠道', role: 'admin', permission: 'payment:channel' },
      },
      {
        path: 'payment/methods',
        name: 'PaymentMethods',
        component: () => import('@/pages/system/payment/methods/index.vue'),
        meta: { title: '支付方式', role: 'admin', permission: 'payment:method' },
      },
      {
        path: 'payment/orders',
        name: 'PaymentOrders',
        component: () => import('@/pages/system/payment/orders/index.vue'),
        meta: { title: '支付订单', role: 'admin', permission: 'payment:order' },
      },
      {
        path: 'payment/callbacks',
        name: 'PaymentCallbacks',
        component: () => import('@/pages/system/payment/callbacks/index.vue'),
        meta: { title: '回调日志', role: 'admin', permission: 'payment:callback' },
      },
      {
        path: 'payment/refunds',
        name: 'PaymentRefunds',
        component: () => import('@/pages/system/payment/refunds/index.vue'),
        meta: { title: '渠道退款', role: 'admin', permission: 'payment:refund' },
      },
      {
        path: 'payment/payouts',
        name: 'PaymentPayouts',
        component: () => import('@/pages/system/payment/payouts/index.vue'),
        meta: { title: '打款管理', role: 'admin', permission: 'payment:payout' },
      },
      {
        path: 'payment/recon',
        name: 'PaymentRecon',
        component: () => import('@/pages/system/payment/recon/index.vue'),
        meta: { title: '渠道对账', role: 'admin', permission: 'payment:recon' },
      },
      // 消息中心（doc70/doc90）整域并入系统管理（迁移 068）：作为「消息中心」二级目录的子模块。
      {
        path: 'notification',
        redirect: '/system/notification/records',
        meta: { title: '消息中心', role: 'admin' },
      },
      {
        path: 'notification/records',
        name: 'NotifyRecords',
        component: () => import('@/pages/system/notification/records/index.vue'),
        meta: { title: '通知记录', role: 'admin', permission: 'notify:record' },
      },
      {
        path: 'notification/templates',
        name: 'NotifyTemplates',
        component: () => import('@/pages/system/notification/templates/index.vue'),
        meta: { title: '通知模板', role: 'admin', permission: 'notify:template' },
      },
      {
        path: 'notification/channels',
        name: 'NotifyChannels',
        component: () => import('@/pages/system/notification/channels/index.vue'),
        meta: { title: '渠道配置', role: 'admin', permission: 'notify:channel' },
      },
      {
        path: 'notification/sms-templates',
        name: 'NotifySmsTemplates',
        component: () => import('@/pages/system/notification/sms-templates/index.vue'),
        meta: { title: '短信模板', role: 'admin', permission: 'notify:sms-template' },
      },
      {
        path: 'notification/broadcast',
        name: 'NotifyBroadcast',
        component: () => import('@/pages/system/notification/broadcast/index.vue'),
        meta: { title: '消息群发', role: 'admin', permission: 'notify:broadcast' },
      },
      {
        path: 'notification/deliveries',
        name: 'NotifyDeliveries',
        component: () => import('@/pages/system/notification/deliveries/index.vue'),
        meta: { title: '发送日志', role: 'admin', permission: 'notify:delivery' },
      },
      // 销售中心（doc86）整域并入系统管理（迁移 068）：作为「销售中心」二级目录的子模块。
      {
        path: 'sales',
        redirect: '/system/sales/customers',
        meta: { title: '销售中心', role: 'admin' },
      },
      {
        path: 'sales/customers',
        name: 'SalesCustomers',
        component: () => import('@/pages/system/sales/customers/index.vue'),
        meta: { title: '客户归属', role: 'admin', permission: 'sales:customer:list' },
      },
      {
        path: 'sales/commissions',
        name: 'SalesCommissions',
        component: () => import('@/pages/system/sales/commissions/index.vue'),
        meta: { title: '提成台账', role: 'admin', permission: 'sales:commission:list' },
      },
      {
        path: 'sales/withdrawals',
        name: 'SalesWithdrawals',
        component: () => import('@/pages/system/sales/withdrawals/index.vue'),
        meta: { title: '提成审核', role: 'admin', permission: 'sales:commission:audit' },
      },
      {
        path: 'sales/performance',
        name: 'SalesPerformance',
        component: () => import('@/pages/system/sales/performance/index.vue'),
        meta: { title: '业绩排行', role: 'admin', permission: 'sales:performance:view' },
      },
      // 内容管理（doc100 §7.1）整域并入系统管理（迁移 069）：作为「内容管理」二级目录的子模块。
      {
        path: 'content',
        redirect: '/system/content/articles',
        meta: { title: '内容管理', role: 'admin' },
      },
      {
        path: 'content/articles',
        name: 'ContentArticles',
        component: () => import('@/pages/system/content/articles/index.vue'),
        meta: { title: '内容文章', role: 'admin', permission: 'content:article:list' },
      },
      {
        path: 'content/categories',
        name: 'ContentCategories',
        component: () => import('@/pages/system/content/categories/index.vue'),
        meta: { title: '内容分类', role: 'admin', permission: 'content:category:list' },
      },
      {
        path: 'content/links',
        name: 'ContentLinks',
        component: () => import('@/pages/system/content/links/index.vue'),
        meta: { title: '友情链接', role: 'admin', permission: 'content:link:list' },
      },
      {
        // 公告管理（复用 notification 公告服务与权限码）：页面仍随 notification 模块放在
        // pages/system/notification/announcements（doc102 §4.1 M2-4），菜单随内容管理迁移。
        path: 'content/announcements',
        name: 'ContentAnnouncements',
        component: () => import('@/pages/system/notification/announcements/index.vue'),
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
