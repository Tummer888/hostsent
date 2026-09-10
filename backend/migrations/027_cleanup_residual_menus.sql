-- ============================================================================
-- 027_cleanup_residual_menus.sql
-- 清理菜单表中的历史残留，并补齐缺失的「用户等级」菜单，使
-- menus 表与前端路由 / 权限表三者一致。
--
-- 背景：管理端侧边栏完全由 menus 表驱动（GET /admin/menus/tree），
--   db.seedMenus 只做幂等 upsert、从不删除，因此历次重构在库里留下了
--   无页面 / 无路由 / 与新版重复的菜单行，共 5 类：
--     1) admin /dashboard/analysis      —— 页面与路由均不存在
--     2) admin /users/rbac[/**]         —— 与 /system/{roles,permissions,admins} 重复
--     3) admin /users/quota[/**]        —— 配额域已随 017 退役，页面已删除
--     4) admin /support[/tickets]       —— disabled 且与 /tickets 重复
--     5) user  /user/**                —— 旧版用户中心菜单，全部 disabled，已被
--                                          /dashboard /cloud /order /billing 取代
--   同时 /users/levels（用户等级管理）虽有页面、路由与 level:* 权限，却从未写入
--   菜单，导致该页不可达，这里一并补上。/users/accounts/detail 按产品决定改为
--   仅从用户列表点入，不再作为侧边栏菜单（页面与路由保留）。
--
-- 幂等（R4）：DELETE 天然幂等；INSERT 用 NOT EXISTS 守卫，可重复执行。
-- 时序：本迁移只动数据不动结构，可在任意时点执行；seedMenus 已同步补齐
--       /users/levels 与 security / verification 子菜单，二者结果一致。
-- ============================================================================

BEGIN;

-- 1. 数据分析：无页面无路由的失效菜单
DELETE FROM menus
WHERE platform = 'admin' AND path = '/dashboard/analysis';

-- 1b. 用户详情：页面与路由保留（由用户列表点击进入），但不再作为侧边栏一级入口
DELETE FROM menus
WHERE platform = 'admin' AND path = '/users/accounts/detail';

-- 2. 用户-权限与角色：与「系统管理 → 权限管理」下的 /system/* 完全重复
DELETE FROM menus
WHERE platform = 'admin' AND (path = '/users/rbac' OR path LIKE '/users/rbac/%');

-- 3. 用户-资源配额与等级：配额子系统已随 017 退役
DELETE FROM menus
WHERE platform = 'admin' AND (path = '/users/quota' OR path LIKE '/users/quota/%');

-- 4. 已停用的工单支持目录：被 /tickets 取代
DELETE FROM menus
WHERE platform = 'admin' AND path IN ('/support', '/support/tickets');

-- 5. 旧版用户中心菜单：全部 disabled，已被新用户端菜单取代
DELETE FROM menus
WHERE platform = 'user' AND path LIKE '/user/%';

-- 6. 补齐「用户等级」菜单（挂在 /users 下，权限 level:list）
INSERT INTO menus (parent_id, platform, name, type, path, component, icon, sort_order, status, created_at, updated_at)
SELECT p.id, 'admin', '用户等级', 'menu', '/users/levels', 'users/levels/index', 'tag', 6, 'active', now(), now()
FROM menus p
WHERE p.platform = 'admin' AND p.path = '/users'
  AND NOT EXISTS (
    SELECT 1 FROM menus WHERE platform = 'admin' AND path = '/users/levels'
  );

COMMIT;
