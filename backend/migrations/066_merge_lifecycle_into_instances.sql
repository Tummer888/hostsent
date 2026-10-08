-- ============================================================================
-- 066_merge_lifecycle_into_instances.sql
-- doc60 生命周期管理整域并入「实例管理」（用户反馈）：到期与续费本就是实例的
--   售后动作，独立一级域让侧边栏多一项、也让「实例」这件事有两个入口。
--
-- 改动（归属调整，HTTP API / 权限码均不变）：
--   后端模块 modules/admin/lifecycle      → modules/admin/instance/lifecycle
--   前端页面 pages/lifecycle              → pages/instances/lifecycle
--   菜单种子（db.go SeedMenus）改挂到 admin:/instances 下的「生命周期管理」二级目录，
--   叶子路径 /lifecycle/* → /instances/lifecycle/*；旧路径由 router redirect 兼容书签。
--   seed 只增不删（按 (platform,path) 匹配已有行），旧行必须在这里显式删除，
--   否则侧边栏会同时出现新旧两套入口。
--
-- 幂等：条件 DELETE，重复执行删 0 行。
-- 顺序：本迁移先跑，再重启后端（seed 会插入新的 /instances/lifecycle 行）。
-- 回滚：把 db.go 的菜单种子改回 /lifecycle 一级域后重启即可；无数据损失。
-- ============================================================================

BEGIN;

DELETE FROM menus
 WHERE platform = 'admin'
   AND (path = '/lifecycle' OR path LIKE '/lifecycle/%');

COMMIT;

-- 验收查询（人工执行）
--   SELECT path,name FROM menus WHERE path LIKE '/instances%' ORDER BY path;
--   期望：/instances（目录）、/instances/list、/instances/inventory、
--         /instances/lifecycle（目录）、/instances/lifecycle/expiring、
--         /instances/lifecycle/renewals、/instances/lifecycle/policy
