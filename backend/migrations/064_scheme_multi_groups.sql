-- ============================================================================
-- 064_scheme_multi_groups.sql
-- 折扣组支持绑定**多个**商品分组（需求方反馈：一个折扣组只能绑一个商品分组太窄）。
--
-- 为什么改：
--   改造前 discount_schemes.product_group_id 是单值，想让 3 个商品分组共用同一套
--   折扣阶梯，就必须建 3 个折扣组、把同一批费率抄 3 遍。费率一旦要调，三处都得改 ——
--   这是定价系统里最容易出错的重复劳动。
--
-- 模型变化：
--   单值列 product_group_id → 关联表 discount_scheme_groups（一个折扣组 N 个分组）。
--   同时保留「一个商品分组至多被一个折扣组绑定」这条既有约束（唯一索引），
--   分组仍是可复用的命名集合，但不会出现两个折扣组抢同一批商品。
--
-- 去重的两条规则（见 service 层）：
--   1) 同一折扣组内多个分组的成员目标取**并集去重**（同一单品被两个分组都包含时只写一次）；
--   2) 不同折扣组之间**不允许目标重叠** —— 否则同一个 (代理分组, 目标) 单元格会被两个
--      折扣组先后写入，后应用的静默覆盖前面的，矩阵上只看得到一个值、无法解释来源。
--      服务层在「绑定分组」和「应用」两处校验（应用时是权威口径：分组随后被加成员
--      也可能产生新重叠）。
--
-- 幂等：CREATE TABLE IF NOT EXISTS + 回填走 ON CONFLICT DO NOTHING，可重复执行。
-- 回滚：
--   ALTER TABLE discount_schemes ADD COLUMN IF NOT EXISTS product_group_id BIGINT;
--   UPDATE discount_schemes ds SET product_group_id = (SELECT min(group_id)
--     FROM discount_scheme_groups g WHERE g.scheme_id = ds.id);
--   DROP TABLE IF EXISTS discount_scheme_groups;
--   （多分组信息在回滚时只剩第一个分组，其余绑定关系需人工恢复。）
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- A. 折扣组 ↔ 商品分组 关联表
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS discount_scheme_groups (
    id         BIGSERIAL PRIMARY KEY,
    scheme_id  BIGINT NOT NULL,
    group_id   BIGINT NOT NULL,
    created_at TIMESTAMPTZ
);

-- 同一折扣组内不重复绑同一个分组。
CREATE UNIQUE INDEX IF NOT EXISTS uk_discount_scheme_groups
    ON discount_scheme_groups (scheme_id, group_id);
-- 一个商品分组至多被一个折扣组绑定（沿用改造前 checkGroupBindable 的语义）。
CREATE UNIQUE INDEX IF NOT EXISTS uk_discount_scheme_groups_group
    ON discount_scheme_groups (group_id);

COMMENT ON TABLE  discount_scheme_groups IS '折扣组 ↔ 商品分组：一个折扣组可绑多个分组，一个分组只归一个折扣组（doc108 §8K）';

-- ---------------------------------------------------------------------------
-- B. 回填既有单值绑定（幂等：ON CONFLICT DO NOTHING）
--
--    必须带存在性守卫：本迁移末尾会 DROP product_group_id，重复执行时这个 INSERT
--    会引用不存在的列直接报 42703（迁移要求可重复执行）。与迁移 059 处理
--    user_groups.price_policy_id 的做法一致。
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_name = 'discount_schemes' AND column_name = 'product_group_id'
    ) THEN
        INSERT INTO discount_scheme_groups (scheme_id, group_id, created_at)
        SELECT ds.id, ds.product_group_id, now()
          FROM discount_schemes ds
         WHERE ds.product_group_id IS NOT NULL
           AND ds.product_group_id > 0
        ON CONFLICT DO NOTHING;
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- C. 保留旧的单值列，但停止使用
--
--    为什么不 DROP：迁移 061 里有 `CREATE INDEX ... ON discount_schemes (product_group_id)`，
--    删列会让 061 从此不能重复执行（报 42703），而「每个迁移文件都能重复执行」是本仓库
--    迁移的硬要求（migrations_live_verify_test.go 逐个文件跑两遍）。061 已经上线应用过，
--    改它的语义风险更大，因此这里选择保留该列、由模型侧不再映射它（AutoMigrate 只加不删，
--    列会安静地留着）。
--
--    模型侧已移除 ProductGroupID 字段，全仓不再读这一列；绑定关系一律以
--    discount_scheme_groups 为准，两者不会产生第二份真相。
-- ---------------------------------------------------------------------------
UPDATE discount_schemes SET product_group_id = NULL WHERE product_group_id IS NOT NULL;

COMMIT;
