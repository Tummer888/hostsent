-- ============================================================================
-- 037_pool_capacity_and_location.sql —— 资源池容量与位置检测 + 探针流预留
--
-- 背景（本轮需求 S2）：
--   容量与位置监测只做「资源池的容量 + 位置」两件事：
--     1. 容量：各池 CPU/内存/磁盘用量与配额（已有 total_*/used_* 列）。
--     2. 位置：池所属地域/可用区；此前位置只在渠道上（resource_providers.region），
--        池维度缺失，无法做「某地域哪个池快满」这类检测。
--   后期会接入资源池探针流，展示资源池状态/内存/硬盘真实用量，故预留探针列：
--     probe_status / probe_at / probe_message（探针未接入时保持默认，页面不展示）。
--
-- 幂等（R4）：ADD COLUMN IF NOT EXISTS / CREATE INDEX IF NOT EXISTS。
-- 回滚：
--   DROP INDEX IF EXISTS idx_resource_pools_region;
--   ALTER TABLE resource_pools
--     DROP COLUMN IF EXISTS region,
--     DROP COLUMN IF EXISTS zone,
--     DROP COLUMN IF EXISTS probe_status,
--     DROP COLUMN IF EXISTS probe_at,
--     DROP COLUMN IF EXISTS probe_message;
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. 位置：region / zone（上游地域与可用区）
-- ---------------------------------------------------------------------------
ALTER TABLE resource_pools
  ADD COLUMN IF NOT EXISTS region varchar(64) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS zone   varchar(64) NOT NULL DEFAULT '';

-- 历史数据回填：池未带位置时沿用所属渠道的 region（渠道 region 是运营手填的地域）。
UPDATE resource_pools p
   SET region = COALESCE(NULLIF(pr.region, ''), '')
  FROM resource_providers pr
 WHERE p.provider_id = pr.id
   AND p.region = ''; 

CREATE INDEX IF NOT EXISTS idx_resource_pools_region ON resource_pools (region);

-- ---------------------------------------------------------------------------
-- 2. 探针流预留列（后期接入，当前仅落库不消费）
-- ---------------------------------------------------------------------------
-- probe_status 探针健康：''=未接入（默认）/healthy/warning/down；
-- probe_at 最近一次探针采样时间；probe_message 探针异常说明。
ALTER TABLE resource_pools
  ADD COLUMN IF NOT EXISTS probe_status  varchar(20) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS probe_at      timestamptz,
  ADD COLUMN IF NOT EXISTS probe_message varchar(255) NOT NULL DEFAULT '';

COMMIT;
