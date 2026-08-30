-- 007_resource_module_menus.sql
-- 资源管理模块菜单树落库（对照 docs/实施计划/10-资源管理模块总体架构和实施计划.md 第5章）。
-- 图标字段使用 tdesign-icons-vue-next 原生图标名（kebab-case），
-- 前端由 frontend-admin/src/store/modules/menu.ts 的 iconMap 映射到按需引入的图标组件。
-- 幂等策略：先 UPDATE 后 INSERT（WHERE NOT EXISTS），不使用 TRUNCATE，避免破坏已有菜单数据。
-- 结构：采用「顶级分组 → 二级大类(directory) → 三级子菜单(menu)」三层树，
-- 与用户管理子树结构保持一致；非叶子目录(component=NULL) 仅作导航分组。

-- 1) 清理历史遗留：003 中旧的 /resources 资源树已被 /resource 口径取代（仅清理管理员平台）
DELETE FROM menus WHERE platform = 'admin' AND (path = '/resources' OR path LIKE '/resources/%');

-- 2) 顶级目录：资源管理
UPDATE menus
SET name = '资源管理', type = 'directory', component = NULL,
    icon = 'resource', sort_order = 3, status = 'active'
WHERE platform = 'admin' AND path = '/resource';

INSERT INTO menus (parent_id, platform, name, type, path, component, icon, sort_order, status)
SELECT 0, 'admin', '资源管理', 'directory', '/resource', NULL, 'resource', 3, 'active'
WHERE NOT EXISTS (SELECT 1 FROM menus WHERE platform = 'admin' AND path = '/resource');

-- 3) 二级大类：directory 节点补插（不存在才插，保持幂等）
INSERT INTO menus (parent_id, platform, name, type, path, component, icon, sort_order, status)
SELECT p.id, 'admin', v.name, 'directory', v.path, NULL, v.icon, v.sort_order, 'active'
FROM (
    VALUES
    -- —— 资源总览
    ('资源总览',       '/resource/overview',        'dashboard',      1),
    -- —— 上游对接管理
    ('上游对接管理',    '/resource/connection',      'cloud',          2),
    -- —— 资源同步与对账
    ('资源同步与对账',  '/resource/sync-center',     'refresh',        3),
    -- —— 资源商品管理
    ('资源商品管理',    '/resource/products-center', 'product',        4),
    -- —— 实例资源
    ('实例资源',        '/resource/instance',        'server',         5),
    -- —— 运维工具
    ('运维工具',        '/resource/ops',             'setting',        6)
) AS v(name, path, icon, sort_order)
JOIN menus AS p ON p.platform = 'admin' AND p.path = '/resource'
WHERE NOT EXISTS (SELECT 1 FROM menus e WHERE e.platform = 'admin' AND e.path = v.path);

-- 4) 三级子菜单：已有节点原地更新（重挂 parent_id + 改名/排序）
UPDATE menus AS m
SET parent_id   = d.id,
    name        = v.name,
    type        = 'menu',
    component   = v.component,
    icon        = v.icon,
    sort_order  = v.sort_order,
    status      = 'active'
FROM (
    VALUES
    -- 资源总览
    ('资源总览',   '/resource/dashboard',      'resource/dashboard/index',      'dashboard',     1, '/resource/overview'),
    ('同步监控',   '/resource/sync-monitor',   'resource/sync-monitor/index',   'data-checked',  2, '/resource/overview'),
    -- 上游对接管理
    ('上游提供商', '/resource/providers',      'resource/providers/index',      'cloud',         1, '/resource/connection'),
    ('资源池管理', '/resource/pools',          'resource/pools/index',          'layers',        2, '/resource/connection'),
    ('连接测试',   '/resource/connectivity',   'resource/connectivity/index',   'link',          3, '/resource/connection'),
    -- 资源同步与对账
    ('同步任务',   '/resource/sync',           'resource/sync/index',           'refresh',       1, '/resource/sync-center'),
    ('同步日志',   '/resource/logs',           'resource/logs/index',           'history',       2, '/resource/sync-center'),
    ('对账报告',   '/resource/reconciliation', 'resource/reconciliation/index', 'verify',        3, '/resource/sync-center'),
    -- 资源商品管理
    ('商品列表',   '/resource/products',       'resource/products/index',       'product',       1, '/resource/products-center'),
    ('商品同步',   '/resource/product-sync',   'resource/product-sync/index',   'cloud-download',2, '/resource/products-center'),
    ('定价管理',   '/resource/pricing',        'resource/pricing/index',        'money',         3, '/resource/products-center'),
    -- 实例资源
    ('云主机实例', '/resource/instances',      'resource/instances/index',      'server',        1, '/resource/instance'),
    -- 运维工具
    ('API测试',    '/resource/api-test',       'resource/api-test/index',       'ai-tool',       1, '/resource/ops'),
    ('异常处理',   '/resource/anomalies',      'resource/anomalies/index',      'error-circle',  2, '/resource/ops'),
    ('系统配置',   '/resource/settings',       'resource/settings/index',       'setting',       3, '/resource/ops')
) AS v(name, path, component, icon, sort_order, group_path)
JOIN menus AS d ON d.platform = 'admin' AND d.path = v.group_path
WHERE m.platform = 'admin' AND m.path = v.path;

-- 5) 三级子菜单：缺失节点补插
INSERT INTO menus (parent_id, platform, name, type, path, component, icon, sort_order, status)
SELECT d.id, 'admin', v.name, 'menu', v.path, v.component, v.icon, v.sort_order, 'active'
FROM (
    VALUES
    ('资源总览',   '/resource/dashboard',      'resource/dashboard/index',      'dashboard',     1, '/resource/overview'),
    ('同步监控',   '/resource/sync-monitor',   'resource/sync-monitor/index',   'data-checked',  2, '/resource/overview'),
    ('上游提供商', '/resource/providers',      'resource/providers/index',      'cloud',         1, '/resource/connection'),
    ('资源池管理', '/resource/pools',          'resource/pools/index',          'layers',        2, '/resource/connection'),
    ('连接测试',   '/resource/connectivity',   'resource/connectivity/index',   'link',          3, '/resource/connection'),
    ('同步任务',   '/resource/sync',           'resource/sync/index',           'refresh',       1, '/resource/sync-center'),
    ('同步日志',   '/resource/logs',           'resource/logs/index',           'history',       2, '/resource/sync-center'),
    ('对账报告',   '/resource/reconciliation', 'resource/reconciliation/index', 'verify',        3, '/resource/sync-center'),
    ('商品列表',   '/resource/products',       'resource/products/index',       'product',       1, '/resource/products-center'),
    ('商品同步',   '/resource/product-sync',   'resource/product-sync/index',   'cloud-download',2, '/resource/products-center'),
    ('定价管理',   '/resource/pricing',        'resource/pricing/index',        'money',         3, '/resource/products-center'),
    ('云主机实例', '/resource/instances',      'resource/instances/index',      'server',        1, '/resource/instance'),
    ('API测试',    '/resource/api-test',       'resource/api-test/index',       'ai-tool',       1, '/resource/ops'),
    ('异常处理',   '/resource/anomalies',      'resource/anomalies/index',      'error-circle',  2, '/resource/ops'),
    ('系统配置',   '/resource/settings',       'resource/settings/index',       'setting',       3, '/resource/ops')
) AS v(name, path, component, icon, sort_order, group_path)
JOIN menus AS d ON d.platform = 'admin' AND d.path = v.group_path
WHERE NOT EXISTS (
    SELECT 1 FROM menus e WHERE e.platform = 'admin' AND e.path = v.path
);
