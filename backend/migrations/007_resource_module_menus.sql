-- 007_resource_module_menus.sql
-- 资源管理模块菜单树落库（对照 docs/实施计划/10-资源管理模块总体架构和实施计划.md 第5章）。
-- 图标字段使用 tdesign-icons-vue-next 原生图标名（kebab-case），
-- 前端由 frontend-admin/src/store/modules/menu.ts 的 iconMap 映射到按需引入的图标组件。
-- 幂等策略：先 UPDATE 后 INSERT（WHERE NOT EXISTS），不使用 TRUNCATE，避免破坏已有菜单数据。
-- 说明：布局侧边栏仅支持「顶级分组 → 二级叶子」两级结构，
-- 故文档中 5 个分类按 sort_order 扁平化挂在「资源管理」目录下。

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

-- 3) 二级菜单：已有节点原地更新（含改名/重排序）
UPDATE menus AS m
SET parent_id   = p.id,
    name        = v.name,
    type        = v.type,
    component   = v.component,
    icon        = v.icon,
    sort_order  = v.sort_order,
    status      = 'active'
FROM (
    VALUES
    -- —— 资源总览
    ('资源总览',   'menu', '/upstream/dashboard',      'upstream/dashboard/index',      'dashboard',     1),
    ('同步监控',   'menu', '/upstream/sync-monitor',   'upstream/sync-monitor/index',   'data-checked',  2),
    -- —— 上游对接管理
    ('上游提供商', 'menu', '/upstream/providers',      'upstream/providers/index',      'cloud',         3),
    ('资源池管理', 'menu', '/upstream/pools',          'upstream/pools/index',          'layers',        4),
    ('连接测试',   'menu', '/upstream/connectivity',   'upstream/connectivity/index',   'link',          5),
    -- —— 资源同步与对账
    ('同步任务',   'menu', '/upstream/sync',           'upstream/sync/index',           'refresh',       6),
    ('同步日志',   'menu', '/upstream/logs',           'upstream/logs/index',           'history',       7),
    ('对账报告',   'menu', '/upstream/reconciliation', 'upstream/reconciliation/index', 'verify',        8),
    -- —— 资源商品管理
    ('商品列表',   'menu', '/upstream/products',       'upstream/products/index',       'product',       9),
    ('商品同步',   'menu', '/upstream/product-sync',   'upstream/product-sync/index',   'cloud-download',10),
    ('定价管理',   'menu', '/upstream/pricing',        'upstream/pricing/index',        'money',        11),
    -- —— 实例资源
    ('云主机实例', 'menu', '/upstream/instances',      'upstream/instances/index',      'server',       12),
    -- —— 运维工具
    ('API测试',    'menu', '/upstream/api-test',       'upstream/api-test/index',       'ai-tool',      13),
    ('异常处理',   'menu', '/upstream/anomalies',      'upstream/anomalies/index',      'error-circle', 14),
    ('系统配置',   'menu', '/upstream/settings',       'upstream/settings/index',       'setting',      15)
) AS v(name, type, path, component, icon, sort_order)
JOIN menus AS p ON p.platform = 'admin' AND p.path = '/upstream'
WHERE m.platform = 'admin' AND m.path = v.path;

-- 4) 二级菜单：缺失节点补插
INSERT INTO menus (parent_id, platform, name, type, path, component, icon, sort_order, status)
SELECT p.id, 'admin', v.name, v.type, v.path, v.component, v.icon, v.sort_order, 'active'
FROM (
    VALUES
    ('资源总览',   'menu', '/upstream/dashboard',      'upstream/dashboard/index',      'dashboard',     1),
    ('同步监控',   'menu', '/upstream/sync-monitor',   'upstream/sync-monitor/index',   'data-checked',  2),
    ('上游提供商', 'menu', '/upstream/providers',      'upstream/providers/index',      'cloud',         3),
    ('资源池管理', 'menu', '/upstream/pools',          'upstream/pools/index',          'layers',        4),
    ('连接测试',   'menu', '/upstream/connectivity',   'upstream/connectivity/index',   'link',          5),
    ('同步任务',   'menu', '/upstream/sync',           'upstream/sync/index',           'refresh',       6),
    ('同步日志',   'menu', '/upstream/logs',           'upstream/logs/index',           'history',       7),
    ('对账报告',   'menu', '/upstream/reconciliation', 'upstream/reconciliation/index', 'verify',        8),
    ('商品列表',   'menu', '/upstream/products',       'upstream/products/index',       'product',       9),
    ('商品同步',   'menu', '/upstream/product-sync',   'upstream/product-sync/index',   'cloud-download',10),
    ('定价管理',   'menu', '/upstream/pricing',        'upstream/pricing/index',        'money',        11),
    ('云主机实例', 'menu', '/upstream/instances',      'upstream/instances/index',      'server',       12),
    ('API测试',    'menu', '/upstream/api-test',       'upstream/api-test/index',       'ai-tool',      13),
    ('异常处理',   'menu', '/upstream/anomalies',      'upstream/anomalies/index',      'error-circle', 14),
    ('系统配置',   'menu', '/upstream/settings',       'upstream/settings/index',       'setting',      15)
) AS v(name, type, path, component, icon, sort_order)
JOIN menus AS p ON p.platform = 'admin' AND p.path = '/upstream'
WHERE NOT EXISTS (
    SELECT 1 FROM menus e WHERE e.platform = 'admin' AND e.path = v.path
);
