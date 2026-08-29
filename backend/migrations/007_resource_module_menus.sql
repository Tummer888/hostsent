-- 007_resource_module_menus.sql
-- 资源管理模块菜单树落库（对照 docs/实施计划/10-资源管理模块总体架构和实施计划.md 第5章）。
-- 图标字段使用 tdesign-icons-vue-next 原生图标名（kebab-case），
-- 前端由 frontend-admin/src/store/modules/menu.ts 的 iconMap 映射到按需引入的图标组件。
-- 幂等策略：先 UPDATE 后 INSERT（WHERE NOT EXISTS），不使用 TRUNCATE，避免破坏已有菜单数据。
-- 结构：采用「顶级分组 → 二级大类(directory) → 三级子菜单(menu)」三层树，
-- 与用户管理子树结构保持一致；非叶子目录(component=NULL) 仅作导航分组。

-- 1) 清理历史遗留：003 中旧的 /resources 资源树已被 /upstream 口径取代（仅清理管理员平台）
DELETE FROM menus WHERE platform = 'admin' AND (path = '/resources' OR path LIKE '/resources/%');

-- 2) 顶级目录：资源管理
UPDATE menus
SET name = '资源管理', type = 'directory', component = NULL,
    icon = 'resource', sort_order = 3, status = 'active'
WHERE platform = 'admin' AND path = '/upstream';

INSERT INTO menus (parent_id, platform, name, type, path, component, icon, sort_order, status)
SELECT 0, 'admin', '资源管理', 'directory', '/upstream', NULL, 'resource', 3, 'active'
WHERE NOT EXISTS (SELECT 1 FROM menus WHERE platform = 'admin' AND path = '/upstream');

-- 3) 二级大类：directory 节点补插（不存在才插，保持幂等）
INSERT INTO menus (parent_id, platform, name, type, path, component, icon, sort_order, status)
SELECT p.id, 'admin', v.name, 'directory', v.path, NULL, v.icon, v.sort_order, 'active'
FROM (
    VALUES
    -- —— 资源总览
    ('资源总览',       '/upstream/overview',        'dashboard',      1),
    -- —— 上游对接管理
    ('上游对接管理',    '/upstream/connection',      'cloud',          2),
    -- —— 资源同步与对账
    ('资源同步与对账',  '/upstream/sync-center',     'refresh',        3),
    -- —— 资源商品管理
    ('资源商品管理',    '/upstream/products-center', 'product',        4),
    -- —— 实例资源
    ('实例资源',        '/upstream/instance',        'server',         5),
    -- —— 运维工具
    ('运维工具',        '/upstream/ops',             'setting',        6)
) AS v(name, path, icon, sort_order)
JOIN menus AS p ON p.platform = 'admin' AND p.path = '/upstream'
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
    ('资源总览',   '/upstream/dashboard',      'upstream/dashboard/index',      'dashboard',     1, '/upstream/overview'),
    ('同步监控',   '/upstream/sync-monitor',   'upstream/sync-monitor/index',   'data-checked',  2, '/upstream/overview'),
    -- 上游对接管理
    ('上游提供商', '/upstream/providers',      'upstream/providers/index',      'cloud',         1, '/upstream/connection'),
    ('资源池管理', '/upstream/pools',          'upstream/pools/index',          'layers',        2, '/upstream/connection'),
    ('连接测试',   '/upstream/connectivity',   'upstream/connectivity/index',   'link',          3, '/upstream/connection'),
    -- 资源同步与对账
    ('同步任务',   '/upstream/sync',           'upstream/sync/index',           'refresh',       1, '/upstream/sync-center'),
    ('同步日志',   '/upstream/logs',           'upstream/logs/index',           'history',       2, '/upstream/sync-center'),
    ('对账报告',   '/upstream/reconciliation', 'upstream/reconciliation/index', 'verify',        3, '/upstream/sync-center'),
    -- 资源商品管理
    ('商品列表',   '/upstream/products',       'upstream/products/index',       'product',       1, '/upstream/products-center'),
    ('商品同步',   '/upstream/product-sync',   'upstream/product-sync/index',   'cloud-download',2, '/upstream/products-center'),
    ('定价管理',   '/upstream/pricing',        'upstream/pricing/index',        'money',         3, '/upstream/products-center'),
    -- 实例资源
    ('云主机实例', '/upstream/instances',      'upstream/instances/index',      'server',        1, '/upstream/instance'),
    -- 运维工具
    ('API测试',    '/upstream/api-test',       'upstream/api-test/index',       'ai-tool',       1, '/upstream/ops'),
    ('异常处理',   '/upstream/anomalies',      'upstream/anomalies/index',      'error-circle',  2, '/upstream/ops'),
    ('系统配置',   '/upstream/settings',       'upstream/settings/index',       'setting',       3, '/upstream/ops')
) AS v(name, path, component, icon, sort_order, group_path)
JOIN menus AS d ON d.platform = 'admin' AND d.path = v.group_path
WHERE m.platform = 'admin' AND m.path = v.path;

-- 5) 三级子菜单：缺失节点补插
INSERT INTO menus (parent_id, platform, name, type, path, component, icon, sort_order, status)
SELECT d.id, 'admin', v.name, 'menu', v.path, v.component, v.icon, v.sort_order, 'active'
FROM (
    VALUES
    ('资源总览',   '/upstream/dashboard',      'upstream/dashboard/index',      'dashboard',     1, '/upstream/overview'),
    ('同步监控',   '/upstream/sync-monitor',   'upstream/sync-monitor/index',   'data-checked',  2, '/upstream/overview'),
    ('上游提供商', '/upstream/providers',      'upstream/providers/index',      'cloud',         1, '/upstream/connection'),
    ('资源池管理', '/upstream/pools',          'upstream/pools/index',          'layers',        2, '/upstream/connection'),
    ('连接测试',   '/upstream/connectivity',   'upstream/connectivity/index',   'link',          3, '/upstream/connection'),
    ('同步任务',   '/upstream/sync',           'upstream/sync/index',           'refresh',       1, '/upstream/sync-center'),
    ('同步日志',   '/upstream/logs',           'upstream/logs/index',           'history',       2, '/upstream/sync-center'),
    ('对账报告',   '/upstream/reconciliation', 'upstream/reconciliation/index', 'verify',        3, '/upstream/sync-center'),
    ('商品列表',   '/upstream/products',       'upstream/products/index',       'product',       1, '/upstream/products-center'),
    ('商品同步',   '/upstream/product-sync',   'upstream/product-sync/index',   'cloud-download',2, '/upstream/products-center'),
    ('定价管理',   '/upstream/pricing',        'upstream/pricing/index',        'money',         3, '/upstream/products-center'),
    ('云主机实例', '/upstream/instances',      'upstream/instances/index',      'server',        1, '/upstream/instance'),
    ('API测试',    '/upstream/api-test',       'upstream/api-test/index',       'ai-tool',       1, '/upstream/ops'),
    ('异常处理',   '/upstream/anomalies',      'upstream/anomalies/index',      'error-circle',  2, '/upstream/ops'),
    ('系统配置',   '/upstream/settings',       'upstream/settings/index',       'setting',       3, '/upstream/ops')
) AS v(name, path, component, icon, sort_order, group_path)
JOIN menus AS d ON d.platform = 'admin' AND d.path = v.group_path
WHERE NOT EXISTS (
    SELECT 1 FROM menus e WHERE e.platform = 'admin' AND e.path = v.path
);
