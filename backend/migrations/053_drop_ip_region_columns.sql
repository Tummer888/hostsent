-- ============================================================================
-- 053_drop_ip_region_columns.sql
-- 背景：
--   此前用 ip-api.com 把登录 IP 反查成中文归属地，写进 login_logs.ip_region /
--   user_sessions.ip_region / users.last_login_ip_region，并在用户列表提供归属地
--   筛选、在总览页提供「登录 IP 归属地分布」图。该设计整体下线，理由有三：
--     1) 归属地本身不可信：解析依赖第三方 HTTP 服务，超时/限流/被墙时静默降级，
--        同一 IP 在不同时间可能得到不同结果，作为统计维度不具备可复现性；
--     2) 冗余且不构成隐私保护：三张表都同时存了**未脱敏的原始 IP**
--        （login_logs.ip / user_sessions.ip / users.last_login_ip），
--        只对归属地做聚合展示，既不减少暴露面，也不提供任何额外信息量 ——
--        需要地域时由原始 IP 现算即可，没必要持久化一份会过期的派生数据；
--     3) 归属地不是安全信号：IP 维度的撞库锁定（login_guard failIPKey）用的是
--        原始 IP，归属地从未参与任何判定。
--   替代实现：总览页改为「在线用户 / 最近登录用户」两张卡（数据源为
--   user_sessions 与 users.last_login_*，均为本地事实，无外部依赖）。
--
-- 幂等：DROP COLUMN IF EXISTS，可重复执行。
-- 顺序：本迁移先跑，再重启后端（AutoMigrate 不再声明这三列，seed 也不再写入）。
-- 回滚：ALTER TABLE ... ADD COLUMN 重新加回即可，但历史归属地数据不可恢复
--       （本迁移不备份）。如需保留一份快照，请在执行前自行
--       `CREATE TABLE _bak_053_ip_region AS SELECT id, ip, ip_region FROM login_logs;`
-- ============================================================================
BEGIN;

ALTER TABLE login_logs    DROP COLUMN IF EXISTS ip_region;
ALTER TABLE user_sessions DROP COLUMN IF EXISTS ip_region;
ALTER TABLE users         DROP COLUMN IF EXISTS last_login_ip_region;

COMMIT;
