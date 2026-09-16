-- ============================================================================
-- 050_menu_tree_normalize.sql
-- 背景（doc102 M0/M2/M3）：
--   1) menus 表没有 (platform,path) 唯一约束，运营侧可造重复行，而 seed 用
--      First() 取第一条 → 重复行中剩余的行永不被更新（幽灵行）；
--   2) 18 个 admin 禁用菜单 + 1 个 user 禁用菜单全部删除（R8）：它们从不出现在
--      侧边栏（toFlatMenu 过滤 status!=='active'），书签兼容由 router 的 redirect
--      独立承担，且其中 5 组造成启用菜单重名、12 个 component 指向不存在的文件；
--   3) 8 个「只有一个启用子项」的二级目录压平（R1），子叶子改挂一级域；
--   4) 9 个死权限码、20 个 seed 已不再声明的孤儿权限行下线（先解绑）
--      —— 与 048/049 同一类静默失效：勾了不生效。
-- 幂等：全部 DELETE ... WHERE ... IN (...) / IF NOT EXISTS，重复执行删 0 行。
-- 顺序：本迁移先跑，再重启后端（seed 会按 (platform,path) 补写 name/parent/
--       sort/status/icon/component，把 DB 对齐到代码）。
-- 回滚：菜单行可由重启后端从 seed 重建（seed 会插入缺失行）；权限码需重新执行
--       seedPermissions + 手工恢复角色绑定（它们本就无效，恢复后行为一致）。
-- ============================================================================
BEGIN;

-- ---- 1. 删除禁用菜单行（19 条）----
DELETE FROM menus WHERE platform IN ('admin','user') AND status = 'disabled'
  AND path IN (
    '/product/sync-center','/product/sync/diff','/product/sync/logs','/product/sync/tasks',
    '/resource/api-test','/resource/connection','/resource/connectivity','/resource/dashboard',
    '/resource/logs','/resource/overview','/resource/pricing','/resource/product-sync',
    '/resource/products','/resource/products-center','/resource/reconciliation',
    '/resource/settings','/resource/sync','/resource/sync-monitor','/cloud/images');

-- ---- 2. 删除被压平/迁出的二级目录行（8 条）----
-- 直接删目录行即可：子叶子的 parent_id 会被 seed 覆盖为新父级（幂等键是 (platform,path)，
-- 路径没变所以命中既有行、只改 parent_id）。
DELETE FROM menus WHERE platform = 'admin' AND path IN (
    '/product/mgmt','/product/category','/finance/transactions-center','/resource/capacity',
    '/resource/instance','/resource/sync-group','/system/config-center','/system/audit-center');

-- ---- 3. 删除重复的「操作审计日志」菜单行 ----
-- 与 /system/audit-logs 是同一件事（同表同权限码），后者是超集（用户审计 + 管理审计）。
DELETE FROM menus WHERE platform = 'admin' AND path = '/users/security/audit-logs';

-- ---- 3b. 删除「路径已变更」的旧叶子行（必须显式删，否则成孤儿）----
-- /resource/instance 目录在「第 2 节」已删，若留着 /resource/instances，其 parent_id 指向
-- 不存在的行 → buildMenuTree 从 parent_id=0 构建，它不会出现在菜单树里，却在表中长期残留。
-- 同理 /system/announcements 的父级 /system/audit-center 已删。
DELETE FROM menus WHERE platform = 'admin' AND path IN (
    '/resource/instances','/system/announcements');

-- ---- 4. 唯一索引（去重后）----
-- 若此处报唯一冲突，说明库里存在 (platform,path) 重复行，先人工确认再执行：
--   SELECT platform, path, count(*) FROM menus GROUP BY 1,2 HAVING count(*) > 1;
CREATE UNIQUE INDEX IF NOT EXISTS uk_menus_platform_path
    ON menus (platform, path) WHERE path <> '';

-- ---- 5. 权限码下线：先解绑，再删行 ----
DELETE FROM role_permissions WHERE permission_id IN (
  SELECT id FROM permissions WHERE code IN (
    'menu:view','menu:create','menu:update','menu:delete','role:assign_permissions',
    'user:impersonate','verification:review','product:price','product:pricing',
    'permission:list','role:assign-permission','role:detail','role:list','role:permission-list',
    'system:permission','system:user:detail','system:user:group','user:assign-role',
    'user:detail:view','user:list','user:reset','user:reset-password','user:status',
    'user:update-status','user:view','user_group:create','user_group:delete',
    'user_group:update','user_group:view'));

-- 先把要保留的子节点改挂到 catalog `product`（否则删掉父行会留下悬空 parent_id，
-- 权限树构建时这些节点会消失，而它们承载的正是「修改价格/定价查看/定价维护」三个在用动作）
UPDATE permissions SET parent_id = (SELECT id FROM permissions WHERE code = 'product')
 WHERE code IN ('product:price:update','pricing:list','pricing:update');

-- 同理，`system:permission`（被删）下的三个 button 是**在用**的（router.go 的
-- /permissions POST/PUT/DELETE 直接要求 permission:create|update|delete，见 seed 中
-- 它们的 ParentCode 是 `system:permission:view`）。DB 里这三行的旧 parent_id 指向
-- 被删的 `system:permission`，会变成悬空节点；按 seed 声明的父级改挂回去。
UPDATE permissions SET parent_id = (SELECT id FROM permissions WHERE code = 'system:permission:view')
 WHERE code IN ('permission:create','permission:update','permission:delete');

DELETE FROM permissions WHERE code IN (
    'menu:view','menu:create','menu:update','menu:delete','role:assign_permissions',
    'user:impersonate','verification:review','product:price','product:pricing',
    'permission:list','role:assign-permission','role:detail','role:list','role:permission-list',
    'system:permission','system:user:detail','system:user:group','user:assign-role',
    'user:detail:view','user:list','user:reset','user:reset-password','user:status',
    'user:update-status','user:view','user_group:create','user_group:delete',
    'user_group:update','user_group:view');

-- ---- 6. 权限节点重名消歧（只改展示名，不动 code）----
UPDATE permissions SET name = '工单分类'     WHERE code = 'ticket:category';
UPDATE permissions SET name = '推广提现'     WHERE code = 'referral:withdraw:list';
UPDATE permissions SET name = '推广提现审核' WHERE code = 'referral:withdraw:audit';

COMMIT;

-- 验收查询（人工执行）
--   SELECT platform, status, count(*) FROM menus GROUP BY 1,2;              -- 期望 disabled 全为 0
--   SELECT count(*) FROM menus GROUP BY platform, path HAVING count(*) > 1; -- 期望空
--   SELECT count(*) FROM permissions;                                       -- 期望 215 - 29 = 186
