-- ============================================================================
-- 015_phase2_product_config_tables.sql
-- Phase 2 · T2.3 商品/规格规范化：把上游 config_groups（原存于 products.config_options JSON）
-- 拆为可查询子表 product_config_options / product_config_options_sub。
--
-- 目标：终结"上游分类丢段/价格解析崩"——把 configoption 及其子项（cpu/memory/os/area...
-- 与各自售价）上移为列，替代 JSON 文本直读。
--
-- 现状（2026-09-09）：products.config_options 为空（0 行），resource_products.raw_specs 为
-- 扁平对象（已被 cpu/memory/disk/... 列覆盖）。故本迁移为"先建目标结构 + 幂等回填"。
-- 待产品目录服务改为读写本子表后，克隆/上游导入流程会开始填充。
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1) 目标表
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS product_config_options (
    id           BIGSERIAL PRIMARY KEY,
    product_id   BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    upstream_key BIGINT   DEFAULT 0,           -- 上游 configoption 键（option.upstream_id）
    option_name  VARCHAR(64),                  -- cpu/memory/os/area/...
    option_type  INTEGER  DEFAULT 1,           -- 1 下拉 2 单选 3 开关 4 数量
    sort_order   INTEGER  DEFAULT 0,
    created_at   TIMESTAMPTZ DEFAULT now(),
    updated_at   TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_pco_product ON product_config_options(product_id);

CREATE TABLE IF NOT EXISTS product_config_options_sub (
    id            BIGSERIAL PRIMARY KEY,
    option_id     BIGINT NOT NULL REFERENCES product_config_options(id) ON DELETE CASCADE,
    upstream_key  BIGINT   DEFAULT 0,          -- 上游 configoption 值（sub.upstream_id）
    option_name   VARCHAR(255),                -- 如 "16|16核" / "HK" / "CentOS"
    hidden        INTEGER  DEFAULT 0,
    price_monthly   NUMERIC(15,2) DEFAULT 0,
    price_annually  NUMERIC(15,2) DEFAULT 0,
    price_quarterly NUMERIC(15,2) DEFAULT 0,
    price_onetime   NUMERIC(15,2) DEFAULT 0,
    sort_order    INTEGER DEFAULT 0,
    created_at    TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_pcos_option ON product_config_options_sub(option_id);

-- 2) 幂等回填：真正的上游 config_groups 来自 resource_products.raw_specs 的 'config_groups' 键
--    （由魔方财务 prodetail 同步写入，形如 [{"options":[{"option_name":"cpu","upstream_id":710549,
--     "sub":[{"option_name":"16|16核","upstream_id":4387511,"pricings":[...]}]}]}]）。
--    经 products.source_product_id -> resource_products.id 关联到具体销售商品（product_id）。
--    当前 dev 库 raw_specs 为扁平对象（无 config_groups），故为幂等空操作；有数据时逐组逐项提取。
--    注意：products.config_options 是扁平覆盖 map（{"cpu":"4"}），并非 config_groups 来源。

-- 提取单个配置组内的 options 到子表（供回填复用）。
CREATE OR REPLACE FUNCTION seed_product_config_options(prod_id BIGINT, grp jsonb) RETURNS void AS $$
DECLARE
    opt jsonb;
    sub jsonb;
    opt_id BIGINT;
    price jsonb;
BEGIN
    FOR opt IN SELECT value FROM jsonb_array_elements(COALESCE(grp->'options', '[]'::jsonb)) AS x LOOP
        INSERT INTO product_config_options(product_id, upstream_key, option_name, option_type, sort_order)
        VALUES (prod_id,
                COALESCE((opt->>'upstream_id')::bigint, 0),
                opt->>'option_name',
                COALESCE((opt->>'option_type')::integer, 1),
                COALESCE((opt->>'sort_order')::integer, 0))
        ON CONFLICT DO NOTHING
        RETURNING id INTO opt_id;

        FOR sub IN SELECT value FROM jsonb_array_elements(COALESCE(opt->'sub', '[]'::jsonb)) AS y LOOP
            price := COALESCE(sub->'pricings', '[]'::jsonb);
            INSERT INTO product_config_options_sub(
                option_id, upstream_key, option_name, hidden, sort_order,
                price_monthly, price_annually, price_quarterly, price_onetime)
            VALUES (
                opt_id,
                COALESCE((sub->>'upstream_id')::bigint, 0),
                sub->>'option_name',
                COALESCE((sub->>'hidden')::integer, 0),
                COALESCE((sub->>'sort_order')::integer, 0),
                (SELECT COALESCE((p->>'monthly')::numeric, 0) FROM jsonb_array_elements(price) p LIMIT 1),
                (SELECT COALESCE((p->>'annually')::numeric, 0) FROM jsonb_array_elements(price) p LIMIT 1),
                (SELECT COALESCE((p->>'quarterly')::numeric, 0) FROM jsonb_array_elements(price) p LIMIT 1),
                (SELECT COALESCE((p->>'onetime')::numeric, 0) FROM jsonb_array_elements(price) p LIMIT 1)
            ) ON CONFLICT DO NOTHING;
        END LOOP;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    prod RECORD;
    grp jsonb;
    cfg  jsonb;
BEGIN
    FOR prod IN SELECT id, source_product_id FROM products WHERE source_product_id > 0 LOOP
        SELECT raw_specs::jsonb -> 'config_groups' INTO cfg
        FROM resource_products WHERE id = prod.source_product_id;
        IF cfg IS NULL OR cfg = 'null'::jsonb THEN
            CONTINUE;
        END IF;
        FOR grp IN SELECT value FROM jsonb_array_elements(cfg) AS g LOOP
            PERFORM seed_product_config_options(prod.id, grp);
        END LOOP;
    END LOOP;
END $$;

COMMIT;

-- 回滚：DROP TABLE product_config_options_sub; DROP TABLE product_config_options;
--        DROP FUNCTION seed_product_config_options(bigint, jsonb);
-- 注：执行前先确认产品目录服务已完成对子表的读写迁移；否则保留旧 JSON 读取路径。
