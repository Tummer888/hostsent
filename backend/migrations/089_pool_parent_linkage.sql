-- ============================================================================
-- 089_pool_parent_linkage.sql
--
-- 背景：资源池原先把「区域」与「宿主机」平铺在同一张表里，两者之间没有关联字段，
-- 页面只能显示一张扁平列表，看不出「某区域下有几台宿主机、各自状态如何」——
-- 而这正是资源池的核心观察诉求（区域 → 宿主机 → 存储 的层级）。
--
-- 变更：resource_pools 增 parent_upstream_id（上级池的上游 ID）。
--   - 宿主机节点：parent_upstream_id = 所属区域的 /areas id（如 "1"）；
--   - 存储池：同口径挂到区域；
--   - 区域：空（顶层）。
--
-- 关联值口径：区域池的 upstream_id 就是 /areas 的裸 id（见 dcimcloud.ListRegions），
-- 因此父级直接写裸 id 字符串即可对上，无需额外映射表。
--
-- 执行方式：本目录迁移不被独立 runner 执行（见 migrations/README.md），
-- 列由启动期 GORM AutoMigrate 补齐（模型 internal/modules/admin/resource/provider/model/provider.go），
-- 存量行的回填在下一次该渠道同步时自然完成（节点行每轮 pool 同步都会带 area_id 重写）。
-- 本文件是演进记录与人工执行依据。
--
-- 回滚：ALTER TABLE resource_pools DROP COLUMN IF EXISTS parent_upstream_id;
-- ============================================================================

BEGIN;

ALTER TABLE resource_pools
  ADD COLUMN IF NOT EXISTS parent_upstream_id VARCHAR(64) NOT NULL DEFAULT '';

-- 存量回填：区域行按 name 匹配同渠道、同名的区域池，把宿主机挂上去。
-- 只在能唯一对上时才写（同名多区域时留空，避免挂错父级——宁可不显示层级，也不要错挂）。
WITH area_map AS (
  SELECT rp.provider_id, rp.name AS area_name, MIN(rp.upstream_id) AS area_upstream_id, COUNT(*) AS n
  FROM resource_pools rp
  WHERE rp.pool_type IN ('region', 'zone')
  GROUP BY rp.provider_id, rp.name
  HAVING COUNT(*) = 1
)
UPDATE resource_pools AS node
SET parent_upstream_id = am.area_upstream_id
FROM area_map am
WHERE node.provider_id = am.provider_id
  AND node.pool_type NOT IN ('region', 'zone')
  AND node.parent_upstream_id = ''
  AND node.region = am.area_name;

COMMIT;
