-- ============================================================================
-- 035_p8_cleanup.sql
-- 资源管理双链路重构 · P8 清理（T8.1 / T8.2）
--
-- 目的：退役双链路切换后保留只读一版的兼容列与遗留归档表，
--   使 source_mode / sell_product_id / upstream_product_id 成为唯一判据（D6 收口）。
--
-- 内容：
--   1. products.provision_mode      —— 供货模式兼容列（P1 起由 products.source_mode 单判据取代，
--      代码读写已全部切换，service 层空值归一 self 的兜底一并退役）
--   2. instances.product_id         —— 旧商品语义兼容列（P1 起由 sell_product_id /
--      upstream_product_id 双列取代，续费算价/订单落库/运维台输出已全部切换）
--   3. 遗留归档表：instances_backup_20260909、retired_user_instances_20260909（doc17 T8.2）
--
-- 前置：已 pg_dump 备份（破坏性迁移，见 doc17 §2.1）。
-- 回滚：列不可直接恢复，需从备份还原：
--   ALTER TABLE products ADD COLUMN IF NOT EXISTS provision_mode varchar(20) DEFAULT 'self';
--   ALTER TABLE instances ADD COLUMN IF NOT EXISTS product_id bigint;
--   -- 遗留表从备份恢复。
-- ============================================================================

BEGIN;

-- 1. products.provision_mode（数据已由 029 回填 + 代码双写保证 source_mode 完整）
ALTER TABLE products DROP COLUMN IF EXISTS provision_mode;

-- 2. instances.product_id（029 迁移已按 order_id 语义回填到 sell_product_id / upstream_product_id）
DROP INDEX IF EXISTS idx_instances_product;
ALTER TABLE instances DROP COLUMN IF EXISTS product_id;

-- 3. 遗留归档表（T8.2）
DROP TABLE IF EXISTS instances_backup_20260909;
DROP TABLE IF EXISTS retired_user_instances_20260909;

COMMIT;
