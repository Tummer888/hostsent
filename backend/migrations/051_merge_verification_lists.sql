-- ============================================================================
-- 051_merge_verification_lists.sql
-- 背景（用户反馈）：实名认证的「待审核列表 / 审核通过列表 / 审核拒绝列表」三个
--   页面已在前端合并为一页页签切换（/users/verification/list?tab=...）。
--   菜单种子（db.go SeedMenus）同步三行合一；旧路径由 router redirect 兼容书签。
--   seed 只增不删，因此旧的三行必须在这里显式删除，否则残留为孤儿菜单。
-- 幂等：DELETE ... WHERE ... IN，重复执行删 0 行。
-- 顺序：本迁移先跑，再重启后端（seed 会插入新的 /users/verification/list 行）。
-- 回滚：菜单行可由旧版 seed 重建；无数据损失。
-- ============================================================================
BEGIN;

DELETE FROM menus WHERE platform = 'admin' AND path IN (
    '/users/verification/pending',
    '/users/verification/approved',
    '/users/verification/rejected');

COMMIT;

-- 验收查询（人工执行）
--   SELECT path,name FROM menus WHERE path LIKE '/users/verification%';
--   期望只剩两行：/users/verification（目录）、/users/verification/list、/users/verification/config
