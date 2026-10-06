-- ============================================================================
-- 063_move_user_levels_under_accounts.sql
-- 背景（用户反馈）：用户等级从「用户管理」一级子项归入「账户管理」子模块 ——
--   前端页面目录 users/levels → users/accounts/levels，后端模块
--   modules/admin/user/level → modules/admin/user/account/level（归属调整，
--   HTTP API / 权限码均不变）。菜单种子（db.go SeedMenus）同步改挂
--   admin:/users/accounts、路径 /users/accounts/levels；旧路径由 router
--   redirect 兼容书签。seed 只增不删，旧行必须在这里显式删除。
-- 幂等：DELETE ... WHERE，重复执行删 0 行。
-- 顺序：本迁移先跑，再重启后端（seed 会插入新的 /users/accounts/levels 行）。
-- 回滚：菜单行可由旧版 seed 重建；无数据损失。
-- ============================================================================
BEGIN;

DELETE FROM menus WHERE platform = 'admin' AND path = '/users/levels';

COMMIT;

-- 验收查询（人工执行）
--   SELECT path,name FROM menus WHERE path LIKE '/users/accounts%';
--   期望：/users/accounts（目录）、/users/accounts/list、/users/accounts/groups、
--         /users/accounts/levels
