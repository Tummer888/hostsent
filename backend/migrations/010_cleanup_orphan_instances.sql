-- ============================================================================
-- 010_cleanup_orphan_instances.sql
-- Phase 0 · T0.2 清理脏数据：删除无真实上游的占位实例行。
--
-- 背景：instances 表曾混入 i-mfy-* / i-os-* / i-pxm-* / i-aws-* / i-ali-* 等
--      「占位种子行」，其 provider 在 upstream/factory 中并未注册（openstack/aws/
--      aliyun/proxmox 未接入）。同时存在 provider_id 指向已删除/不存在上游的行。
--
-- 口径：
--   1) instance_id 命中占位前缀（i-mfy- / i-os- / i-pxm- / i-aws- / i-ali-）；
--   2) provider_id 在 resource_providers 中不存在（孤儿/悬挂行）。
--
-- 安全措施：先备份整表到 instances_backup_<yyyymmdd>；在事务中执行；幂等；
--           回滚 = 从备份表还原。
--
-- 注意：本迁移不随应用启动自动执行。请先在测试库演练，备份后手动放行。
--       执行前务必确认 resource_providers 与 instances 的关联规则无误。
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 0) 备份：将实例表快照到备份表（带时间戳，避免冲突）。
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    backup_table text := 'instances_backup_' || to_char(current_date, 'YYYYMMDD');
    tbl_exists   boolean;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = backup_table
    ) INTO tbl_exists;

    IF NOT tbl_exists THEN
        EXECUTE format('CREATE TABLE %I AS SELECT * FROM instances', backup_table);
        RAISE NOTICE '已备份 instances -> %', backup_table;
    ELSE
        RAISE NOTICE '备份表 % 已存在，跳过备份（幂等）', backup_table;
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- 1) 目标行：占位前缀 或 上游不存在。
-- ---------------------------------------------------------------------------
CREATE TEMP TABLE tmp_orphan_instances AS
SELECT id
FROM instances i
WHERE instance_id LIKE 'i-mfy-%'
   OR instance_id LIKE 'i-os-%'
   OR instance_id LIKE 'i-pxm-%'
   OR instance_id LIKE 'i-aws-%'
   OR instance_id LIKE 'i-ali-%'
   OR NOT EXISTS (
        SELECT 1 FROM resource_providers p WHERE p.id = i.provider_id
   );

-- ---------------------------------------------------------------------------
-- 2) 删除（先记录数量，便于对账）。
-- ---------------------------------------------------------------------------
SELECT count(*) AS deleted_count FROM tmp_orphan_instances;

DELETE FROM instances i
USING tmp_orphan_instances t
WHERE i.id = t.id;

DROP TABLE tmp_orphan_instances;

-- ---------------------------------------------------------------------------
-- 3) 对账：确认无残留占位行。
-- ---------------------------------------------------------------------------
SELECT
    (SELECT count(*) FROM instances WHERE instance_id LIKE 'i-mfy-%')  AS mfy_left,
    (SELECT count(*) FROM instances WHERE instance_id LIKE 'i-os-%')   AS os_left,
    (SELECT count(*) FROM instances WHERE instance_id LIKE 'i-pxm-%')  AS pxm_left,
    (SELECT count(*) FROM instances WHERE instance_id LIKE 'i-aws-%')  AS aws_left,
    (SELECT count(*) FROM instances WHERE instance_id LIKE 'i-ali-%')  AS ali_left;

COMMIT;

-- ============================================================================
-- 回滚（Rollback）：从备份表还原被删除的行（冲突以备份为准）。
--   INSERT INTO instances (id, instance_id, provider_id, user_id, product_id,
--       name, cpu, memory, disk, disk_type, bandwidth, os, region, zone,
--       status, private_ip, public_ip, raw_data, billing_mode, created_at,
--       expire_at, updated_at)
--   SELECT id, instance_id, provider_id, user_id, product_id,
--       name, cpu, memory, disk, disk_type, bandwidth, os, region, zone,
--       status, private_ip, public_ip, raw_data, billing_mode, created_at,
--       expire_at, updated_at
--   FROM instances_backup_<yyyymmdd>
--   ON CONFLICT (instance_id) DO NOTHING;
-- 注：instance_id 为唯一索引，备份表恢复时以 NULLS NOT DISTINCT 兜底冲突即可。
-- ============================================================================
