-- ============================================================================
-- 104_resource_products_stock.sql
-- 上游商品库存：把上游商品详情里的 stock_control/qty 落成本地可读字段。
--
-- 背景（本轮用户反馈）：
--   上游商品列表显示的库存信息不准确、也没有从上游拉取下来；用户要求
--   "低于库存就无法下单"。此前 raw_specs 里其实已经同步到了上游的
--   stock_control / qty（zjmf 商品详情 profile 字段），但没有任何消费方，
--   页面自然读不到 —— 数据在库里躺着，只是没通路。
--
-- 字段语义（对齐 zjmf 上游 /api/product/prodetail 的 profile 语义）：
--   stock_control = 0：上游未开启库存控制 → 不限量（qty 无意义）。
--   stock_control = 1：上游限量 → qty 为可售余量。
--   因此本地用 `stock = -1` 表示"不限量"，>=0 表示上游剩余可售数量，
--   与 sales 商品 products.stock 的既有约定（-1=不限）保持同一口径。
--
-- 幂等：ADD COLUMN IF NOT EXISTS + 带条件的 UPDATE，重复执行无副作用。
-- ============================================================================

ALTER TABLE resource_products ADD COLUMN IF NOT EXISTS stock BIGINT NOT NULL DEFAULT -1;
ALTER TABLE resource_products ADD COLUMN IF NOT EXISTS stock_control BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN resource_products.stock IS '可售库存：-1=不限量（上游未开启库存控制）';
COMMENT ON COLUMN resource_products.stock_control IS '上游是否开启库存控制：0=不限，1=限量（stock 为余量）';

-- 回填：历史行的 raw_specs 里已有 stock_control/qty，直接展开，
-- 避免必须等下一次目录同步才显示库存。
UPDATE resource_products
SET stock_control = COALESCE(NULLIF(raw_specs::json->>'stock_control', '')::bigint, 0),
    stock = CASE
        WHEN COALESCE(NULLIF(raw_specs::json->>'stock_control', '')::bigint, 0) = 1
            THEN COALESCE(NULLIF(raw_specs::json->>'qty', '')::bigint, 0)
        ELSE -1
    END
WHERE raw_specs IS NOT NULL
  AND raw_specs <> ''
  AND raw_specs::json->>'stock_control' IS NOT NULL
  AND stock = -1;
