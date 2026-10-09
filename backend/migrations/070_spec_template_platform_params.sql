-- ============================================================================
-- 070_spec_template_platform_params.sql
-- 自营链路打通 · 规格模板升级为「可复用固定规格 + 平台映射」
--
-- 背景（本轮用户反馈）：
--   新建自营商品后，商品下没有任何 SKU，也就无从做"规格 → 平台字段"映射，
--   上架门禁（T4.6）必然拦截，链路走不通。运营需要先配好一个固定规格模板，
--   新建商品时直接选用模板、改掉少量参数即可生成带平台映射的 SKU。
--
-- 改动：
--   product_spec_templates 增 platform_params（jsonb）——模板级的平台写参数
--   （魔方云的 area/node/os/store/cpu/memory/system_disk_size 等），
--   生成 SKU 时原样复制成 spec_bindings.platform_params 并置 confirmed。
--   spec_values / source 两列由 030 迁移创建，本迁移不重复处理（IF NOT EXISTS 兜底）。
--   为便于按模板名检索/生成 SKU 编码，补 (status, spec_family) 复合索引。
--
-- 幂等：ADD COLUMN IF NOT EXISTS / CREATE INDEX IF NOT EXISTS。
-- 顺序：先跑本迁移，再重建后端（GORM AutoMigrate 会补列但不写索引）。
-- 回滚：ALTER TABLE product_spec_templates DROP COLUMN IF EXISTS platform_params;
-- ============================================================================

BEGIN;

ALTER TABLE product_spec_templates ADD COLUMN IF NOT EXISTS spec_values jsonb;
ALTER TABLE product_spec_templates ADD COLUMN IF NOT EXISTS source varchar(16) DEFAULT 'self';
ALTER TABLE product_spec_templates ADD COLUMN IF NOT EXISTS platform_params jsonb;

CREATE INDEX IF NOT EXISTS idx_product_spec_templates_family_status
  ON product_spec_templates (spec_family, status);

COMMIT;

-- 验收查询（人工执行）
--   \d product_spec_templates
--   期望含 spec_values / source / platform_params 三列。
