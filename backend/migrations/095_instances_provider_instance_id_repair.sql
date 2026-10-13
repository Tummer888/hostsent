-- ============================================================================
-- 095_instances_provider_instance_id_repair.sql
-- 修复被空值覆盖的 provider_instance_id（「已开通」被判成「待开通」的存量行）。
--
-- 背景：instances.provider_instance_id 是"是否已在上游开通"的唯一判据
-- （空 = 待开通占位行）。但 UpsertInstances 的 ON CONFLICT 更新列里包含该列，
-- 而部分调用方（如同步发现路径）本来就不知道该实例的上游实例号，写入空值就把
-- 已开通实例的上游号抹掉了。实测：实例 1003（instance_id=2049，来源订单 832）
-- 在 2026-10-12 17:39 被清空，此后该实例在详情页被判为"待开通"（能力位全关、
-- 出现「重新开通」按钮），回源刷新直接跳过（推进器日志长期是
-- "refresh skipped status: ''"）。
--
-- 代码侧（同批提交）已加固 UpsertInstances：仅在本次值非空时才覆盖该列。
-- 本迁移负责把已经被抹掉的存量行补回来。
--
-- 补值规则（只处理"实锤曾经开通过"的行）：
--   1) 非占位实例号（不以 pending- 开头）——占位行的 instance_id 是本地生成号，
--      不能当作上游实例号；
--   2) 上游链路（source_mode='upstream'）或已挂来源订单；
--   3) 该行确实有上游实例号可用（instance_id 为纯数字，即上游返回的 host id）。
--
-- 按上游实例号而不是 instance_id 判据的原因：占位转正时 instance_id 会被替换成
-- 真实上游号，因此"非 pending- 前缀 + 纯数字"等价于"这个号是上游给的"。
--
-- 幂等：条件 UPDATE，重复执行无副作用。
-- 回滚：无需回滚——恢复被误抹的实例号是修正，不是可逆配置。
-- ============================================================================

BEGIN;

UPDATE instances
SET provider_instance_id = instance_id,
    updated_at = now()
WHERE coalesce(provider_instance_id, '') = ''
  AND instance_id NOT LIKE 'pending-%'
  AND instance_id ~ '^[0-9]+$'
  AND coalesce(source_mode, '') = 'upstream';

COMMIT;

-- 验收查询（人工执行）
--   SELECT count(*) FROM instances
--   WHERE coalesce(provider_instance_id,'') = ''
--     AND instance_id NOT LIKE 'pending-%' AND instance_id ~ '^[0-9]+$';  -- 期望 0
