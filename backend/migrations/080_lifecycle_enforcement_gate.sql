-- ============================================================================
-- 080_lifecycle_enforcement_gate.sql
-- 到期阶段强制执行「总开关 + 预演 + 失败退避」（doc61 §8.4 要求）。
--
-- 背景：生命周期推进器（lifecycle_advancer）此前每 10 分钟无条件对宽延期
--   结束的实例下发上游暂停、对超过保留期的实例下发销毁，既没有总开关也没有
--   预演，且失败不做退避 —— 上游不可达时会把同一实例逐轮重试，把
--   instance_operations 打成失败日志（实测线上 unsuspend 失败 3974 条）。
--
-- 本迁移只加列，行为变更在代码侧（推进器读 policy 闸门 + 退避字段）：
--   1. lifecycle_policies.auto_enforce   —— 自动执行总开关，默认 FALSE：
--      false 时只做阶段登记与提醒，绝不下发上游。
--   2. lifecycle_policies.enforce_dry_run —— 预演开关，默认 TRUE：
--      即使总开关打开，也只计算「将执行什么」而不真正下发。
--   3. instances.enforce_attempts / enforce_next_at / last_enforce_error ——
--      连续失败次数、下次可重试时间（指数退避闸门）、最近失败原因。
--
-- 幂等：全部 ADD COLUMN IF NOT EXISTS；存量策略行只补默认值（不覆盖既有值）。
-- 回滚：
--   ALTER TABLE instances DROP COLUMN IF EXISTS last_enforce_error;
--   ALTER TABLE instances DROP COLUMN IF EXISTS enforce_next_at;
--   ALTER TABLE instances DROP COLUMN IF EXISTS enforce_attempts;
--   ALTER TABLE lifecycle_policies DROP COLUMN IF EXISTS enforce_dry_run;
--   ALTER TABLE lifecycle_policies DROP COLUMN IF EXISTS auto_enforce;
-- ============================================================================

BEGIN;

-- 1. 策略闸门
ALTER TABLE lifecycle_policies
    ADD COLUMN IF NOT EXISTS auto_enforce BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE lifecycle_policies
    ADD COLUMN IF NOT EXISTS enforce_dry_run BOOLEAN NOT NULL DEFAULT TRUE;

COMMENT ON COLUMN lifecycle_policies.auto_enforce IS
    '到期阶段自动执行总开关；false（默认）时推进器只登记阶段、不下发上游暂停/销毁';
COMMENT ON COLUMN lifecycle_policies.enforce_dry_run IS
    '预演开关；true（默认）时即使总开关开启也只计算不下发，用于评估影响面';

-- 存量单行策略：仅当该行存在时把默认值对齐（不改已有显式取值）。
UPDATE lifecycle_policies SET enforce_dry_run = TRUE WHERE enforce_dry_run IS NULL;

-- 2. 实例级失败退避
ALTER TABLE instances
    ADD COLUMN IF NOT EXISTS enforce_attempts INT NOT NULL DEFAULT 0;

ALTER TABLE instances
    ADD COLUMN IF NOT EXISTS enforce_next_at TIMESTAMPTZ;

ALTER TABLE instances
    ADD COLUMN IF NOT EXISTS last_enforce_error VARCHAR(500) NOT NULL DEFAULT '';

COMMENT ON COLUMN instances.enforce_attempts IS
    '到期阶段强制执行连续失败次数（成功或手动重置后归零），用于指数退避与告警';
COMMENT ON COLUMN instances.enforce_next_at IS
    '下次允许重试的时间（退避闸门），为空表示可立即处理';
COMMENT ON COLUMN instances.last_enforce_error IS
    '最近一次强制执行失败原因（截断 500 字），供运维在列表/详情直接看到';

-- 退避扫描索引：推进器按 (lifecycle_stage, expire_at) 取候选并按 enforce_next_at 过滤。
CREATE INDEX IF NOT EXISTS idx_instances_enforce_next
    ON instances (enforce_next_at)
    WHERE enforce_next_at IS NOT NULL;

COMMIT;

-- 验收查询（人工执行）
--   列已存在：
--   SELECT column_name FROM information_schema.columns
--    WHERE table_name IN ('instances','lifecycle_policies')
--      AND column_name IN ('auto_enforce','enforce_dry_run','enforce_attempts','enforce_next_at','last_enforce_error');
--   期望：5 行
--   默认闸门为「不自动执行 + 预演」：
--   SELECT auto_enforce, enforce_dry_run FROM lifecycle_policies WHERE id = 1;
--   期望：f | t
