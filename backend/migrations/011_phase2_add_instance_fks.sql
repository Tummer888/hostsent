-- ============================================================================
-- 011_phase2_add_instance_fks.sql
-- Phase 2 · T2.2 实例收敛（安全部分）：为 instances 补外键，确立单一真相。
--    - instances.user_id    -> users.id
--    - instances.provider_id -> resource_providers.id
-- 前置：instances.instance_id 已有唯一约束（见 model）。若存在悬挂用户/提供商，
--       本脚本会给出明确错误并中止，避免引入失败的外键。
-- 回滚：DROP CONSTRAINT ...（见文末）。
-- ============================================================================

BEGIN;

-- 前置校验：存在悬挂引用则拒绝加外键（先跑 010_cleanup_orphan_instances.sql 清理占位行）。
DO $$
DECLARE
    orphans bigint;
BEGIN
    SELECT count(*) INTO orphans
    FROM instances i
    WHERE i.user_id <> 0 AND NOT EXISTS (SELECT 1 FROM users u WHERE u.id = i.user_id)
       OR i.provider_id <> 0 AND NOT EXISTS (SELECT 1 FROM resource_providers p WHERE p.id = i.provider_id);
    IF orphans > 0 THEN
        RAISE EXCEPTION 'instances 存在 % 条悬挂 user_id/provider_id，请先清理占位行（010）再执行', orphans;
    END IF;
END $$;

ALTER TABLE instances DROP CONSTRAINT IF EXISTS fk_instances_user;
ALTER TABLE instances DROP CONSTRAINT IF EXISTS fk_instances_provider;
ALTER TABLE instances DROP CONSTRAINT IF EXISTS fk_instances_provider_provider_id;

ALTER TABLE instances
    ADD CONSTRAINT fk_instances_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE instances
    ADD CONSTRAINT fk_instances_provider FOREIGN KEY (provider_id) REFERENCES resource_providers(id);

COMMIT;

-- 回滚：
--   ALTER TABLE instances DROP CONSTRAINT fk_instances_user;
--   ALTER TABLE instances DROP CONSTRAINT fk_instances_provider;
