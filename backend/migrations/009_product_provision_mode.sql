-- 009_product_provision_mode.sql
-- 商品供货模式字段迁移（双模式商品架构：self 自营 / clone 上游克隆）。
-- 新增列：products.provision_mode、products.config_options；
--         resource_products.group_id、resource_products.group_name（上游商品分类）。
-- 幂等策略：ALTER TABLE ... ADD COLUMN IF NOT EXISTS；并用 UPDATE 回填存量空值。

-- 1) 供货模式列（默认 self）
ALTER TABLE products ADD COLUMN IF NOT EXISTS provision_mode VARCHAR(20) NOT NULL DEFAULT 'self';

-- 2) 可配置项 JSON（自营映射 /clouds 参数；克隆可覆盖上游规格）
ALTER TABLE products ADD COLUMN IF NOT EXISTS config_options TEXT;

-- 3) 供货模式索引（列表页按模式筛选）
CREATE INDEX IF NOT EXISTS idx_products_provision_mode ON products (provision_mode);

-- 4) 上游商品分类列（cart/all 分组，供克隆弹窗按分类渲染/多选）
ALTER TABLE resource_products ADD COLUMN IF NOT EXISTS group_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE resource_products ADD COLUMN IF NOT EXISTS group_name VARCHAR(100) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_resource_products_provider_group ON resource_products (provider_id, group_id);

-- 5) 回填存量数据：空/未知模式归一为 self（自营），避免订单履约解析失败
UPDATE products
SET provision_mode = 'self'
WHERE provision_mode IS NULL
   OR provision_mode = ''
   OR provision_mode NOT IN ('self', 'clone');

