-- 003_seed_menus.sql
-- 完整菜单树初始化数据（管理员后台 + 用户中心）。
-- 对照 docs/项目架构.md 第5章 + 用户管理子树设计（7 二级 + 22 三级）。
-- 图标字段使用 tdesign-icons 名称（kebab-case），前端 store/modules/menu.ts 的 iconMap 映射到按需引入的图标组件。
-- 幂等：TRUNCATE RESTART IDENTITY 后重新插入，可重复执行（会清空现有菜单数据）。

TRUNCATE TABLE menus RESTART IDENTITY;

-- ============ 管理员后台菜单（52 项） ============
INSERT INTO menus (id, parent_id, platform, name, type, path, component, icon, sort_order, status) VALUES
-- 1. 仪表盘
(1,  0,  'admin', '仪表盘',   'directory', '/dashboard',                NULL,                           'dashboard',  1, 'active'),
(2,  1,  'admin', '概览',     'menu',      '/dashboard/base',            'dashboard/base/index',         'dashboard',  1, 'active'),
(3,  1,  'admin', '数据分析', 'menu',      '/dashboard/analysis',        'dashboard/analysis/index',      'chart-bar',  2, 'active'),
-- 2. 用户管理（7 二级 + 22 三级）
(4,  0,  'admin', '用户管理',           'directory', '/users',                          NULL,                              'user',            2, 'active'),
-- 2.1 用户总览
(5,  4,  'admin', '用户总览',             'menu',      '/users/overview',                 'users/overview/index',            'dashboard',       1, 'active'),
-- 2.2 账户管理
(6,  4,  'admin', '账户管理',             'directory', '/users/accounts',                NULL,                              'usergroup',       2, 'active'),
(7,  6,  'admin', '用户列表',             'menu',      '/users/accounts/list',            'users/accounts/list/index',       'user-list',       1, 'active'),
(8,  6,  'admin', '用户组/组织管理',       'menu',      '/users/accounts/groups',          'users/accounts/groups/index',     'control-platform',2, 'active'),
-- 2.3 分销与代理商管理
(9,  4,  'admin', '分销与代理商管理',      'directory', '/users/partners',                 NULL,                              'link',            3, 'active'),
(10, 9,  'admin', '代理商列表',           'menu',      '/users/partners/agents',          'users/partners/agents/index',     'usergroup',       1, 'active'),
(11, 9,  'admin', '代理商等级配置',        'menu',      '/users/partners/levels',          'users/partners/levels/index',     'tag',             2, 'active'),
(12, 9,  'admin', '下级用户管理',          'menu',      '/users/partners/subordinates',   'users/partners/subordinates/index','user-list',       3, 'active'),
(13, 9,  'admin', '返利/佣金记录',         'menu',      '/users/partners/commissions',    'users/partners/commissions/index','money',           4, 'active'),
-- 2.4 权限与角色
(14, 4,  'admin', '权限与角色',           'directory', '/users/rbac',                     NULL,                              'lock-on',         4, 'active'),
(15, 14, 'admin', '角色列表',             'menu',      '/users/rbac/roles',               'users/rbac/roles/index',          'usergroup',       1, 'active'),
(16, 14, 'admin', '权限分配',             'menu',      '/users/rbac/permissions',         'users/rbac/permissions/index',    'setting',         2, 'active'),
(17, 14, 'admin', '管理员列表',           'menu',      '/users/rbac/admins',              'users/rbac/admins/index',         'user-list',       3, 'active'),
-- 2.5 安全与风控
(18, 4,  'admin', '安全与风控',           'directory', '/users/security',                 NULL,                              'key',             5, 'active'),
(19, 18, 'admin', '登录日志',             'menu',      '/users/security/login-logs',      'users/security/login-logs/index', 'history',         1, 'active'),
(20, 18, 'admin', '操作审计日志',         'menu',      '/users/security/audit-logs',      'users/security/audit-logs/index', 'file',            2, 'active'),
(21, 18, 'admin', '异常行为监控',         'menu',      '/users/security/risk',            'users/security/risk/index',       'chart-bar',       3, 'active'),
(22, 18, 'admin', '黑名单管理',           'menu',      '/users/security/blacklist',       'users/security/blacklist/index',  'stop',            4, 'active'),
(23, 18, 'admin', '会话管理',             'menu',      '/users/security/sessions',        'users/security/sessions/index',   'refresh',         5, 'active'),
-- 2.6 资源配额与等级
(24, 4,  'admin', '资源配额与等级',        'directory', '/users/quota',                    NULL,                              'layers',          6, 'active'),
(25, 24, 'admin', '配额模板管理',          'menu',      '/users/quota/templates',          'users/quota/templates/index',     'catalog',         1, 'active'),
(26, 24, 'admin', '用户等级管理',          'menu',      '/users/quota/tiers',              'users/quota/tiers/index',         'tag',             2, 'active'),
(27, 24, 'admin', '配额调整记录',          'menu',      '/users/quota/changes',            'users/quota/changes/index',       'history',         3, 'active'),
-- 2.7 实名认证
(28, 4,  'admin', '实名认证',             'directory', '/users/verification',            NULL,                              'verify',          7, 'active'),
(29, 28, 'admin', '待审核列表',           'menu',      '/users/verification/pending',     'users/verification/pending/index', 'history',         1, 'active'),
(30, 28, 'admin', '审核通过列表',         'menu',      '/users/verification/approved',   'users/verification/approved/index','check-circle',    2, 'active'),
(31, 28, 'admin', '审核拒绝列表',         'menu',      '/users/verification/rejected',   'users/verification/rejected/index','error-circle',    3, 'active'),
(32, 28, 'admin', '认证配置',             'menu',      '/users/verification/config',     'users/verification/config/index', 'setting',         4, 'active'),
-- 3. 资源管理
(33, 0,  'admin', '资源管理', 'directory', '/resources',                     NULL,                            'layers',     3, 'active'),
(34, 33, 'admin', '云主机',   'directory', '/resources/instances',            'resources/instances/index',     'server',     1, 'active'),
(35, 34, 'admin', '实例列表', 'menu',      '/resources/instances/list',        'resources/instances/index',     'server',     1, 'active'),
(36, 34, 'admin', '快照管理', 'menu',      '/resources/instances/snapshots',    'resources/instances/snapshots', 'file-paste', 2, 'active'),
(37, 33, 'admin', '镜像管理', 'menu',      '/resources/images',                'resources/images/index',        'image',      2, 'active'),
(38, 33, 'admin', '网络管理', 'menu',      '/resources/networks',              'resources/networks/index',      'internet',   3, 'active'),
-- 4. 产品管理
(39, 0,  'admin', '产品管理', 'directory', '/products',                       NULL,                            'app',        4, 'active'),
(40, 39, 'admin', '产品列表', 'menu',      '/products/list',                    'products/list/index',           'catalog',    1, 'active'),
(41, 39, 'admin', '产品分类', 'menu',      '/products/categories',             'products/categories/index',     'tag',        2, 'active'),
-- 5. 订单管理
(42, 0,  'admin', '订单管理', 'directory', '/orders',                         NULL,                            'order',      5, 'active'),
(43, 42, 'admin', '订单列表', 'menu',      '/orders/list',                     'orders/list/index',            'order',      1, 'active'),
-- 6. 财务管理
(44, 0,  'admin', '财务管理', 'directory', '/billing',                        NULL,                            'bill',       6, 'active'),
(45, 44, 'admin', '账单查询', 'menu',      '/billing/bills',                   'billing/bills/index',           'bill',       1, 'active'),
(46, 44, 'admin', '交易流水', 'menu',      '/billing/transactions',           'billing/transactions/index',   'money',      2, 'active'),
-- 7. 系统管理
(47, 0,  'admin', '系统管理', 'directory', '/system',                        NULL,                            'setting',    7, 'active'),
(48, 47, 'admin', '菜单管理', 'menu',      '/system/menus',                    'system/menus/index',           'menu',       1, 'active'),
(49, 47, 'admin', '审计日志', 'menu',      '/system/audit',                    'system/audit/index',           'history',    2, 'active'),
(50, 47, 'admin', '系统设置', 'menu',      '/system/settings',                 'system/settings/index',        'setting',    3, 'active'),
-- 8. 工单支持
(51, 0,  'admin', '工单支持', 'directory', '/support',                        NULL,                            'service',    8, 'active'),
(52, 51, 'admin', '工单列表', 'menu',      '/support/tickets',                'support/tickets/index',         'service',    1, 'active');

-- ============ 用户中心菜单（16 项） ============
INSERT INTO menus (parent_id, platform, name, type, path, component, icon, sort_order, status)
SELECT p.id, 'user', child.name, child.type, child.path, child.component, child.icon, child.sort_order, child.status
FROM menus p
JOIN (
  VALUES
  ('/user/dashboard', '用户中心',       'directory', NULL,                          'dashboard', 1, 'active'),
  ('/user/dashboard/home', '首页',      'menu',      'user/dashboard/home/index',    'dashboard', 1, 'active'),
  ('/user/profile', '个人资料',         'menu',      'user/profile/index',           'user',      2, 'active'),
  ('/user/orders', '我的订单',          'menu',      'user/orders/index',            'order',     3, 'active'),
  ('/user/resources', '我的资源',        'directory', NULL,                          'server',    4, 'active'),
  ('/user/resources/list', '云主机列表', 'menu',      'user/resources/list/index',   'server',    1, 'active'),
  ('/user/resources/snapshots', '快照管理','menu',    'user/resources/snapshots/index','file-paste',2, 'active'),
  ('/user/tickets', '工单支持',         'menu',      'user/tickets/index',           'service',   5, 'active'),
  ('/user/billing', '财务中心',         'directory', NULL,                          'bill',      6, 'active'),
  ('/user/billing/bills', '账单',       'menu',      'user/billing/bills/index',     'bill',      1, 'active'),
  ('/user/billing/transactions', '流水', 'menu',     'user/billing/transactions/index','money',    2, 'active'),
  ('/user/security', '安全中心',         'directory', NULL,                          'key',       7, 'active'),
  ('/user/security/login-history', '登录历史','menu', 'user/security/login-history', 'history',   1, 'active'),
  ('/user/security/realname', '实名认证', 'menu',     'user/security/realname/index', 'verify',    2, 'active'),
  ('/user/security/password', '修改密码', 'menu',     'user/security/password/index', 'lock-on',   3, 'active'),
  ('/user/security/sessions', '会话管理', 'menu',     'user/security/sessions/index', 'refresh',   4, 'active')
) AS child(path, name, type, component, icon, sort_order, status)
ON p.platform = 'user' AND p.path = CASE
  WHEN child.path LIKE '/user/resources/%' THEN '/user/resources'
  WHEN child.path LIKE '/user/billing/%' THEN '/user/billing'
  WHEN child.path LIKE '/user/security/%' THEN '/user/security'
  ELSE '/user/dashboard'
END
WHERE p.platform = 'user';
