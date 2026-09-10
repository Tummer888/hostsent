-- 020_p3_user_tiering.sql
-- 账号体系与权限分级重构 P3：用户分层（等级 / 用户组）+ 累计消费落列
-- 依据：docs/实施计划/82-账号体系与权限分级重构执行清单.md P3-01
--       docs/实施计划/81-账号体系与权限分级重构设计.md（D3：等级不参与折扣）
--
-- 注意（R1 schema 双写）：本文件仅作版本留痕，运行时不执行；
-- 实际建列由 internal/pkg/db/db.go 的 AutoMigrate 完成，数据回填由
-- db.backfillUserConsumeTotals 在启动时幂等补齐。
-- 对应 Go model：admin/user/account/model/user.go、admin/user/account/model/user_group.go、
--               admin/user/level/model/user_level.go、uc/auth/model/user.go
--
-- 幂等：ADD COLUMN IF NOT EXISTS + UPDATE 带 WHERE 条件，可重复执行。

BEGIN;

-- users：等级外键 + 累计消费落列（原先靠 wallet_transactions 实时聚合）
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS user_level_id BIGINT,
  ADD COLUMN IF NOT EXISTS total_consume_amount DECIMAL(15,2) NOT NULL DEFAULT 0;

-- user_groups：折扣策略绑定 + 组序 + 默认组 / 代理组标记
ALTER TABLE user_groups
  ADD COLUMN IF NOT EXISTS price_policy_id BIGINT,
  ADD COLUMN IF NOT EXISTS priority INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS is_agent_group BOOLEAN NOT NULL DEFAULT false;

-- user_levels：权益 / 升级门槛 / 子账号上限
ALTER TABLE user_levels
  ADD COLUMN IF NOT EXISTS benefits JSONB,
  ADD COLUMN IF NOT EXISTS upgrade_threshold DECIMAL(15,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS max_sub_accounts INT NOT NULL DEFAULT 0;

-- 历史数据回填：按归一后的流水类型 consume 聚合（P0-02 已统一为该类型）。
-- 仅回填仍为 0 的用户，重复执行不会叠加。
UPDATE users u
SET total_consume_amount = stats.total
FROM (
    SELECT user_id, COALESCE(SUM(ABS(amount)), 0) AS total
    FROM wallet_transactions
    WHERE type = 'consume'
    GROUP BY user_id
) AS stats
WHERE stats.user_id = u.id
  AND u.total_consume_amount = 0
  AND stats.total > 0;

COMMIT;
