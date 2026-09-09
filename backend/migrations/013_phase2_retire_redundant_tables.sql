-- ============================================================================
-- 013_phase2_retire_redundant_tables.sql
-- Phase 2 · T2.1/T2.2 退役冗余聚合表：user_instances / user_transactions / user_bills。
--
-- 前置（已在代码层完成并通过 build/vet/test）：
--   1) user_detail_service.GetAggregate / user列表 total_consume_amount 已改读权威表
--      instances / wallet_transactions / bills（见 user/account/repository/*.go）。
--   2) internal/pkg/db/db.go 已把这三个模型移出 AutoMigrate，并移除 db.Seed 填充。
-- 因此这三张表已无任何业务读路径，可安全 DROP。
--
-- 执行时序：先构建并部署新后端（新代码读权威表），再执行本迁移，否则运行中的旧后端
--           聚合接口会因表不存在而报错。
-- 安全：本迁移会先把三表快照到 retired_*_<yyyymmdd>，再 DROP；回滚 = 从快照 INSERT 回。
-- ============================================================================

BEGIN;

-- 快照（可回滚）
CREATE TABLE IF NOT EXISTS retired_user_instances_20260909 AS SELECT * FROM user_instances;
CREATE TABLE IF NOT EXISTS retired_user_transactions_20260909 AS SELECT * FROM user_transactions;
CREATE TABLE IF NOT EXISTS retired_user_bills_20260909 AS SELECT * FROM user_bills;

-- 退役
DROP TABLE IF EXISTS user_instances;
DROP TABLE IF EXISTS user_transactions;
DROP TABLE IF EXISTS user_bills;

COMMIT;

-- 回滚（Rollback）：
--   INSERT INTO user_instances SELECT * FROM retired_user_instances_20260909;
--   INSERT INTO user_transactions SELECT * FROM retired_user_transactions_20260909;
--   INSERT INTO user_bills SELECT * FROM retired_user_bills_20260909;
-- 注：完整 T0 基线备份见 migrations/backups/pre-phase2-*.sql。
