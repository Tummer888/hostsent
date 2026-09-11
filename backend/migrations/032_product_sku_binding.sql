-- ============================================================================
-- 032_product_sku_binding.sql
-- 资源管理双链路重构 · P4 商品规格与定价（T4.2 / T4.3 / T4.4）
--
-- 目的：把 SKU 矩阵接入"规格模板 + 平台绑定"，并泛化配置项来源，
--   使自营链路（规格来自我方）与代理链路（规格镜像上游）共用同一套模型。
--   详见 docs/实施计划/16 §3.3/§7.2/§7.5、17 §5 T4.2~T4.4。
--
-- 内容：
--   1. product_specs.spec_template_id —— SKU 引用的标准规格模板（§7.4 统一可售单元）。
--   2. spec_bindings.product_spec_id  —— 自营 SKU 的出站平台绑定锚点。
--      external_spec_id 放开 NOT NULL（自营 SKU 没有外部规格），唯一键改为
--      两个部分唯一索引：外部规格绑定 (external_spec_id, direction) 与
--      SKU 绑定 (product_spec_id, direction) 各自唯一、互不干扰。
--   3. product_config_options / _sub 增加 source + source_key（§7.5）：
--      source=upstream 时 source_key 是上游配置项 id（沿用 upstream_key 语义）；
--      source=self 时 source_key 是平台参数名（area/node/store/os...）。
--   4. orders.spec_code —— 订单所选 SKU 编码。开通履约据此取 SKU 的
--      confirmed 平台绑定（spec_bindings.product_spec_id → platform_params）。
--   5. products.spec_passthrough —— 代理商品"仅透传"标记（§7.6.3）：
--      上游归一规格未确认绑定时，显式声明只透传上游参数、不改写，方可上架。
--
-- 幂等（R4）：ADD COLUMN IF NOT EXISTS；索引 DROP 后重建 IF NOT EXISTS；
--   DROP NOT NULL 可重复执行。旧列 upstream_key 保留一版（P8 清理），
--   迁移时把存量行回填为 source='upstream' / source_key=upstream_key::text。
-- 回滚：DROP INDEX ...；ALTER TABLE ... DROP COLUMN IF EXISTS <各新增列>；
--   ALTER TABLE spec_bindings ALTER COLUMN external_spec_id SET NOT NULL（需先清空自营绑定）。
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. product_specs：SKU → 标准规格模板
-- ---------------------------------------------------------------------------
ALTER TABLE product_specs ADD COLUMN IF NOT EXISTS spec_template_id bigint;
CREATE INDEX IF NOT EXISTS idx_product_specs_spec_template_id ON product_specs (spec_template_id);

-- ---------------------------------------------------------------------------
-- 2. spec_bindings：自营 SKU 出站绑定锚点
-- ---------------------------------------------------------------------------
ALTER TABLE spec_bindings ADD COLUMN IF NOT EXISTS product_spec_id bigint;
ALTER TABLE spec_bindings ALTER COLUMN external_spec_id DROP NOT NULL;

-- 原唯一索引按整体 (external_spec_id, direction) 约束；放开 NULL 后需改为部分唯一索引，
-- 否则多条 product_spec_id 绑定会因 external_spec_id 均为 NULL 而互相冲突。
DROP INDEX IF EXISTS uk_spec_bindings_external_direction;
CREATE UNIQUE INDEX IF NOT EXISTS uk_spec_bindings_external_direction
  ON spec_bindings (external_spec_id, direction) WHERE external_spec_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_spec_bindings_product_direction
  ON spec_bindings (product_spec_id, direction) WHERE product_spec_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_spec_bindings_product_spec_id ON spec_bindings (product_spec_id);

-- ---------------------------------------------------------------------------
-- 3. product_config_options / _sub：配置项来源泛化（§7.5）
-- ---------------------------------------------------------------------------
ALTER TABLE product_config_options ADD COLUMN IF NOT EXISTS source varchar(16) NOT NULL DEFAULT 'upstream';
ALTER TABLE product_config_options ADD COLUMN IF NOT EXISTS source_key varchar(128);
ALTER TABLE product_config_options_sub ADD COLUMN IF NOT EXISTS source varchar(16) NOT NULL DEFAULT 'upstream';
ALTER TABLE product_config_options_sub ADD COLUMN IF NOT EXISTS source_key varchar(128);

-- 存量回填：老数据全部来自上游克隆，source_key 取旧的 upstream_key 文本。
UPDATE product_config_options
SET source_key = upstream_key::text
WHERE source_key IS NULL AND upstream_key IS NOT NULL AND upstream_key <> 0;

UPDATE product_config_options_sub
SET source_key = upstream_key::text
WHERE source_key IS NULL AND upstream_key IS NOT NULL AND upstream_key <> 0;

CREATE INDEX IF NOT EXISTS idx_pco_source ON product_config_options (source);
CREATE INDEX IF NOT EXISTS idx_pcos_source ON product_config_options_sub (source);

-- ---------------------------------------------------------------------------
-- 4. orders：所选 SKU 编码
-- ---------------------------------------------------------------------------
-- 履约开通时按 (product_id, spec_code) 回查 SKU 与 confirmed 平台绑定；
-- 为空表示走商品级配置（存量订单语义不变）。
ALTER TABLE orders ADD COLUMN IF NOT EXISTS spec_code varchar(64);
CREATE INDEX IF NOT EXISTS idx_orders_spec_code ON orders (spec_code);

-- ---------------------------------------------------------------------------
-- 5. products：代理商品"仅透传"标记
-- ---------------------------------------------------------------------------
ALTER TABLE products ADD COLUMN IF NOT EXISTS spec_passthrough boolean NOT NULL DEFAULT false;

COMMIT;
