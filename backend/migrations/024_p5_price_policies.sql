-- 024_p5_price_policies.sql
-- P5-01：统一算价管线的折扣策略表 + 订单/订单项折扣快照字段。
-- 幂等：可重复执行（IF NOT EXISTS / ADD COLUMN IF NOT EXISTS）。

-- 折扣策略：product_pricing 决定基础价，本表决定按客户打几折。
CREATE TABLE IF NOT EXISTS price_policies (
    id             BIGSERIAL PRIMARY KEY,
    name           VARCHAR(64)  NOT NULL,
    code           VARCHAR(64)  NOT NULL,
    discount_type  VARCHAR(16)  NOT NULL,                -- rate 折扣率 / amount 直减
    discount_value DECIMAL(10,4) NOT NULL,               -- 0.85 = 85 折
    scope          VARCHAR(16)  NOT NULL DEFAULT 'all',  -- all / category / product
    priority       INT NOT NULL DEFAULT 0,
    effective_from TIMESTAMP,
    effective_to   TIMESTAMP,
    status         VARCHAR(32) NOT NULL DEFAULT 'active',
    remark         VARCHAR(255),
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- code 唯一索引名与 GORM 模型 uniqueIndex 显式名保持一致（R1），
-- 否则 AutoMigrate 会尝试 DROP 名字不一致的索引而失败/反复重建。
-- 早期执行过本迁移或 AutoMigrate 的库可能留下别的命名，先改名对齐再兜底创建。
DO $$
DECLARE
    cname text;
    iname text;
BEGIN
    SELECT conname INTO cname FROM pg_constraint
     WHERE conrelid = 'price_policies'::regclass AND contype = 'u'
       AND conname IN ('price_policies_code_key', 'uni_price_policies_code', 'idx_price_policies_code');
    IF cname IS NOT NULL THEN
        EXECUTE format('ALTER TABLE price_policies RENAME CONSTRAINT %I TO uk_price_policies_code', cname);
    ELSE
        SELECT indexname INTO iname FROM pg_indexes
         WHERE schemaname = 'public' AND tablename = 'price_policies'
           AND indexname IN ('price_policies_code_key', 'uni_price_policies_code', 'idx_price_policies_code');
        IF iname IS NOT NULL THEN
            EXECUTE format('ALTER INDEX %I RENAME TO uk_price_policies_code', iname);
        END IF;
    END IF;
END $$;
CREATE UNIQUE INDEX IF NOT EXISTS uk_price_policies_code ON price_policies (code);

-- 策略条目：scope=category/product 时对具体目标覆盖折扣。
CREATE TABLE IF NOT EXISTS price_policy_items (
    id             BIGSERIAL PRIMARY KEY,
    policy_id      BIGINT NOT NULL,
    target_type    VARCHAR(16) NOT NULL,   -- category / product
    target_id      BIGINT NOT NULL,
    discount_type  VARCHAR(16) NOT NULL,
    discount_value DECIMAL(10,4) NOT NULL,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_pp_items ON price_policy_items (policy_id, target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_pp_items_target ON price_policy_items (target_type, target_id);

-- 订单折扣快照
ALTER TABLE orders
  ADD COLUMN IF NOT EXISTS original_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS discount_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS final_amount    DECIMAL(15,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS price_policy_id BIGINT,
  ADD COLUMN IF NOT EXISTS discount_source VARCHAR(32),
  ADD COLUMN IF NOT EXISTS price_snapshot  JSONB;

ALTER TABLE order_items
  ADD COLUMN IF NOT EXISTS original_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS discount_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS final_amount    DECIMAL(15,2) NOT NULL DEFAULT 0;

-- 存量订单回填：原价 = 应付总额，实付 = 实付金额，优惠为 0（无更精确来源）。
UPDATE orders
SET original_amount = total_amount,
    final_amount    = paid_amount
WHERE original_amount = 0
  AND final_amount = 0
  AND (total_amount > 0 OR paid_amount > 0);
