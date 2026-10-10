-- ============================================================================
-- 076_platform_option_menu.sql
-- 产品管理 · 平台配置项目录页菜单与权限（T4.5 规格配置化）
--
-- 背景：新增 /product/spec/option-catalog 页面（按对接平台维护可配置项与取值库，
-- 含镜像批量入库）。菜单是 DB 种子驱动的，权限串需与 permission_map.go 及
-- db.go 的权限种子一致（spec:option:list / spec:option:update）。
--
-- 幂等：先 UPDATE 后 INSERT（WHERE NOT EXISTS）。
-- 回滚：
--   DELETE FROM role_permissions WHERE permission_code LIKE 'spec:option:%';
--   DELETE FROM permissions WHERE code LIKE 'spec:option:%';
--   DELETE FROM menus WHERE platform='admin' AND path='/product/spec/option-catalog';
-- ============================================================================

BEGIN;

-- 1) 三级菜单：平台配置项（挂在「规格管理」目录下，排序 4）
UPDATE menus AS m
SET parent_id  = d.id,
    name       = '平台配置项',
    type       = 'menu',
    component  = 'product/spec/option-catalog/index',
    icon       = 'layers',
    sort_order = 4,
    status     = 'active'
FROM (SELECT id FROM menus WHERE platform = 'admin' AND path = '/product/spec') AS d
WHERE m.platform = 'admin' AND m.path = '/product/spec/option-catalog';

INSERT INTO menus (parent_id, platform, name, type, path, component, icon, sort_order, status)
SELECT d.id, 'admin', '平台配置项', 'menu', '/product/spec/option-catalog',
       'product/spec/option-catalog/index', 'layers', 4, 'active'
FROM (SELECT id FROM menus WHERE platform = 'admin' AND path = '/product/spec') AS d
WHERE NOT EXISTS (
    SELECT 1 FROM menus e WHERE e.platform = 'admin' AND e.path = '/product/spec/option-catalog'
);

-- 2) 按钮权限：查看 / 维护。
-- 表结构是 parent_id（非 parent_code），父节点取「规格管理」目录权限 product:spec。
INSERT INTO permissions (parent_id, name, code, type, sort_order, status)
SELECT COALESCE((SELECT id FROM permissions WHERE code = 'product:spec'), 0),
       v.name, v.code, 'button', v.sort_order, 'active'
FROM (VALUES
    ('平台配置项查看', 'spec:option:list',   7),
    ('平台配置项维护', 'spec:option:update', 8)
) AS v(name, code, sort_order)
WHERE NOT EXISTS (SELECT 1 FROM permissions e WHERE e.code = v.code);

-- 3) 授权：超级管理员与运维（与 spec:template:list 同口径）
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
    ('super_admin', 'spec:option:list'),
    ('super_admin', 'spec:option:update'),
    ('ops_admin',   'spec:option:list')
) AS v(role_code, permission_code)
JOIN roles r ON r.code = v.role_code
JOIN permissions p ON p.code = v.permission_code
WHERE NOT EXISTS (
    SELECT 1 FROM role_permissions e
    WHERE e.role_id = r.id AND e.permission_id = p.id
);

COMMIT;

-- 验收：SELECT m.path, m.name FROM menus m WHERE m.path LIKE '/product/spec%';
--       SELECT code FROM permissions WHERE code LIKE 'spec:option:%';
