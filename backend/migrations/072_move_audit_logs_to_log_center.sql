-- ============================================================================
-- 072_move_audit_logs_to_log_center.sql
-- 操作审计改挂「日志中心」（用户采纳建议）：它语义上是审计/日志查看，与日志中心的
--   日志浏览 / 清理任务 / 保留策略同源；迁移 071 曾把它收进「系统配置」组，本次归位。
--
-- 改动（归属调整，HTTP API / 权限码均不变）：
--   菜单 parent: admin:/system/config → admin:/system/log-center
--   **路径恢复为原 /system/audit-logs**（迁移 071 才短暂改成 /system/config/audit-logs，
--   此前长期使用的就是 /system/audit-logs），因此不存在对外 URL 变更，无需新 redirect；
--   只在 router 里为 /system/config/audit-logs 留一条兼容跳转。
--
-- 注意：/system/audit-logs 这一行由 seed 在重启时插入（迁移 071 已把它删过），
--   本迁移只负责删掉 071 建立的 /system/config/audit-logs，避免两行并存。
--
-- 幂等：条件 DELETE，重复执行删 0 行。
-- 顺序：本迁移先跑，再重启后端（seed 会插入 /system/audit-logs 并挂到日志中心）。
-- 回滚：把 db.go 该行改回 admin:/system/config 与 /system/config/audit-logs 后重启。
-- ============================================================================

BEGIN;

DELETE FROM menus
 WHERE platform = 'admin'
   AND path = '/system/config/audit-logs';

COMMIT;

-- 验收查询（人工执行）
--   SELECT m.path, m.name, p.path AS parent FROM menus m JOIN menus p ON p.id = m.parent_id
--    WHERE m.platform='admin' AND m.path = '/system/audit-logs';
--   期望：一行，parent = /system/log-center，component = system/audit-logs/index
--   SELECT count(*) FROM menus WHERE platform='admin' AND path = '/system/config/audit-logs';
--   期望：0
