-- ============================================================================
-- 061_product_groups_and_discount_schemes.sql
-- doc108 §8I：二维折扣结构 —— 商品分组 × 代理等级。
--
-- 模型（与截图的三页签一致：客户分组 | 商品分组 | 折扣设置）：
--   product_groups            商品分组：分类和/或单个商品的命名集合；
--   product_group_items       商品分组成员（category / product 两种粒度）；
--   discount_schemes          折扣组：一行 = 一个组（如「6折组」），可绑定一个商品分组；
--   discount_scheme_items     折扣组成员：每个代理等级在该组下的折扣率。
--
-- 关键决策：**不改变算价**。折扣组×商品分组通过「应用」展开写入既有的
--   agent_level_discounts 逐格矩阵（成本线/单调性/目标校验全部复用），
--   算价链路只认逐格矩阵 —— 两张新表是录入与组织层，不是第二真相。
--
-- 幂等：CREATE TABLE IF NOT EXISTS，可重复执行。
-- 回滚：
--   DROP TABLE IF EXISTS discount_scheme_items, discount_schemes,
--                        product_group_items, product_groups;
--   （agent_level_discounts 中已展开的格子不受影响，需要的话按格清理。）
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- A. 商品分组
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS product_groups (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(64)  NOT NULL,
    code        VARCHAR(64)  NOT NULL,
    description VARCHAR(255),
    status      VARCHAR(32)  NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ,
    updated_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_product_groups_code ON product_groups (code);

CREATE TABLE IF NOT EXISTS product_group_items (
    id          BIGSERIAL PRIMARY KEY,
    group_id    BIGINT      NOT NULL,
    target_type VARCHAR(16) NOT NULL,  -- category / product
    target_id   BIGINT      NOT NULL,
    created_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_product_group_items
    ON product_group_items (group_id, target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_product_group_items_target
    ON product_group_items (target_type, target_id);

COMMENT ON TABLE  product_groups        IS '商品分组：分类/商品的命名集合（doc108 §8I）';
COMMENT ON TABLE  product_group_items   IS '商品分组成员：category 或 product 粒度';

-- ---------------------------------------------------------------------------
-- B. 折扣组（一行 = 一个组，列 = 代理等级；绑定一个商品分组）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS discount_schemes (
    id               BIGSERIAL PRIMARY KEY,
    name             VARCHAR(64) NOT NULL,
    code             VARCHAR(64) NOT NULL,
    description      VARCHAR(255),
    -- 绑定的商品分组；一个分组至多被一个启用中的折扣组绑定（服务层校验）。
    product_group_id BIGINT,
    status           VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at       TIMESTAMPTZ,
    updated_at       TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_discount_schemes_code ON discount_schemes (code);
CREATE INDEX IF NOT EXISTS idx_discount_schemes_group ON discount_schemes (product_group_id);

CREATE TABLE IF NOT EXISTS discount_scheme_items (
    id             BIGSERIAL PRIMARY KEY,
    scheme_id      BIGINT        NOT NULL,
    agent_level_id BIGINT        NOT NULL,
    discount_rate  NUMERIC(10,4) NOT NULL DEFAULT 0,  -- 0 = 该等级在此方案不打折（应用时跳过）
    created_at     TIMESTAMPTZ,
    updated_at     TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_discount_scheme_items
    ON discount_scheme_items (scheme_id, agent_level_id);
CREATE INDEX IF NOT EXISTS idx_discount_scheme_items_level
    ON discount_scheme_items (agent_level_id);

COMMENT ON TABLE  discount_schemes            IS '折扣组：绑一个商品分组 + 每等级一个折扣率（doc108 §8I）';
COMMENT ON TABLE  discount_scheme_items       IS '折扣组成员：等级 → 折扣率；应用时展开进 agent_level_discounts';

COMMIT;
