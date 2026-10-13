-- ============================================================================
-- 092_merge_ops_into_sync_center.sql
-- doc120：把「运维」二级目录及其三个叶子（异常处理 / 任务队列 / 实例对账）
--   并入「同步与调度」，后者更名「同步状态运维控制台」。
--
-- 动机（用户反馈）：这两块本来是同一个问题的两面 —— 同步与调度管「从上游拉什么」，
--   运维三件套管「动作有没有送到上游 / 拉回来的东西对不对」，却在侧边栏分成两项，
--   且 anomalies 页只是 sync-center 任务/日志按 status=failed 再查一遍。
--
-- 改动（归属调整，HTTP API / 权限码均不变）：
--   菜单种子（db.go SeedMenus）删除 admin:/resource/ops 目录与 /resource/anomalies、
--   /resource/task-queue、/resource/reconcile 三个叶子；
--   /resource/sync-center 名称改为「同步状态运维控制台」（同一行，仅 name 变化）。
--   旧路径由 router redirect 到 /resource/sync-center?tab=anomalies|actions|reconcile 兼容书签。
--
-- seed 只增不删（按 (platform,path) 匹配已有行），旧行必须在这里显式删除，
-- 否则侧边栏会同时出现旧「运维」目录（其子项已不在种子中，成为孤儿行）。
--
-- 幂等：条件 DELETE，重复执行删 0 行。
-- 顺序：本迁移先跑，再重启后端（seed 会把 /resource/sync-center 的 name 对齐为新名称）。
-- 回滚：把 db.go 的菜单种子改回旧结构后重启即可（seed 会重建被删的行）；无数据损失。
-- ============================================================================

BEGIN;

-- 1) 删除三个叶子（它们的 parent 指向 /resource/ops，目录行随后一并删除）。
DELETE FROM menus
 WHERE platform = 'admin'
   AND path IN ('/resource/anomalies', '/resource/task-queue', '/resource/reconcile');

-- 2) 删除空的「运维」二级目录行本身。
DELETE FROM menus
 WHERE platform = 'admin'
   AND path = '/resource/ops';

COMMIT;

-- 验收查询（人工执行）
--   SELECT path,name,type,sort_order FROM menus
--    WHERE platform='admin' AND path LIKE '/resource%' ORDER BY sort_order, path;
--   期望：/resource（目录）、/resource/channels（目录）、/resource/providers、
--         /resource/platforms、/resource/pools、/resource/sync-center
--         —— 不再有 /resource/ops、/resource/anomalies、/resource/task-queue、/resource/reconcile
