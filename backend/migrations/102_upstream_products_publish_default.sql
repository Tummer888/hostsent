-- ============================================================================
-- 102_upstream_products_publish_default.sql
-- 财务型上游导入商品的存量收敛：不再因「未开仅透传」卡草稿。
--
-- 背景（实测）：魔方财务（Kind=upstream）的商品开通参数由上游自带
-- （resource_products.raw_specs.config_groups 原样转发），不存在"本地规格 →
-- 上游开通参数"的映射需要确认。但上架门禁仍按通用规则要求 spec_bindings
-- confirmed 或逐商品手动开「仅透传」——导入 200+ 商品后全部落草稿，
-- 批量上架完全不可用，运营只能一台一台点。
--
-- 代码侧（同批提交）已按客观判据放行（2026-10-13）：
--   validatePublish → upstreamProvisionIsSelfContained：上游资源商品镜像里
--   带 config_groups 的（财务型）直接放行，不要求 spec_bindings。
--   CloneFromUpstreamBatch 默认 status=published（可显式传 0 要草稿）。
--
-- 本迁移负责存量：把**已经导入但仍是草稿**的此类商品补开「仅透传」标记，
-- 使其无需重新导入即可直接上架（标记语义与人工开启完全一致，留痕走
-- product_history 不适用于批量收敛，故只改标记列）。
--
-- 判定条件（只动"确定属于财务型上游"的行）：
--   1) source_mode = 'upstream'（代理链路）；
--   2) status = 0（草稿，已上架的不动）；
--   3) 其绑定的上游资源商品 raw_specs 含 config_groups（上游自带开通参数）。
--
-- 幂等：条件 UPDATE，重复执行无副作用。
-- 回滚：UPDATE product_history 无法反向；如需撤销可执行文末的注释语句。
-- ============================================================================

BEGIN;

UPDATE products p
SET spec_passthrough = true,
    updated_at = now()
FROM resource_products rp
WHERE p.source_product_id = rp.id
  AND p.source_mode = 'upstream'
  AND p.status = 0
  AND coalesce(p.spec_passthrough, false) = false
  AND rp.raw_specs LIKE '%config_groups%';

COMMIT;

-- 验收查询（人工执行）
--   SELECT count(*) FROM products p JOIN resource_products rp ON rp.id = p.source_product_id
--   WHERE p.source_mode='upstream' AND p.status=0 AND coalesce(p.spec_passthrough,false)=false
--     AND rp.raw_specs LIKE '%config_groups%';   -- 期望 0

-- 撤销（如确需）：
--   UPDATE products p SET spec_passthrough = false
--   FROM resource_products rp
--   WHERE p.source_product_id = rp.id AND p.source_mode='upstream' AND p.status=0
--     AND rp.raw_specs LIKE '%config_groups%';
