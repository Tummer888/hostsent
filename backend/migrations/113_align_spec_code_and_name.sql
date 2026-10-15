-- 113_align_spec_code_and_name.sql
--
-- SKU 编码/名称与原子取值对齐（T4.7 配套数据修正）。
-- 等价 Go 侧回填 alignProductSpecCodeAndName（internal/pkg/db/db.go），两边口径一致。
--
-- 起因（实测）：商品 113 名下有一条编码 general-2c4g-30g、名称「通用型2核4G 30G」的规格，
-- 但它的原子取值是 4核8G/60G 盘 —— 编码、名称、取值三者各说各话。
-- 成因是旧版建品页允许就地改取值，而编码/名称照**模板**派生，改完没重算。
--
-- 处理范围（严格收紧，只修"数字确实对不上"的行）：
--   * 名称必须是「纯配置型」（^[^0-9]*数字核数字G，且不含 | · （） 等营销文案）；
--     带文案的名称（「香港|四区·轻量 8-8 9Mbps（4核6G）」）是运营有意起的商品名，不动。
--   * **只在名称/编码里的数字与取值矛盾时才改**。数字已对、仅编码少个磁盘段这类
--     "格式不统一但不矛盾"的情况不动 —— 那属于历史格式差异，改了会打断运营对编码的记忆。
--   * 跳过被 orders / order_items 引用的编码（改编码会断历史订单引用）。
--
-- 幂等：重复执行结果一致（已对齐的行不再命中 WHERE 条件）。

-- 名称重写：把与取值矛盾的名称改成「N核NG」（与建品页 renderNameTemplate 的
-- {cpu}核{memory}G 同口径）。
UPDATE product_specs
SET name = t.cpu::text || '核' || t.mem_gb::text || 'G'
FROM (
    SELECT
        s.id,
        (s.specs::jsonb ->> 'compute.cpu')::int                                   AS cpu,
        GREATEST(((s.specs::jsonb ->> 'compute.memory')::numeric / 1024)::int, 1) AS mem_gb
    FROM product_specs s
    WHERE s.specs IS NOT NULL
      AND s.specs <> ''
      AND s.specs::jsonb ? 'compute.cpu'
      AND s.specs::jsonb ? 'compute.memory'
      AND (s.specs::jsonb ->> 'compute.cpu')::int > 0
      AND (s.specs::jsonb ->> 'compute.memory')::int > 0
      AND s.name ~ '^[^0-9]*[0-9]+核[0-9]+G'
      AND s.name !~ '[|·（）()]'
      -- 名称里的核数与内存必须与取值不符才修（数字一致的格式差异不算错）。
      AND (
            (regexp_match(s.name, '([0-9]+)核'))[1]::int
              IS DISTINCT FROM (s.specs::jsonb ->> 'compute.cpu')::int
         OR (regexp_match(s.name, '核([0-9]+)G'))[1]::int
              IS DISTINCT FROM GREATEST(((s.specs::jsonb ->> 'compute.memory')::numeric / 1024)::int, 1)
      )
      AND s.spec_code NOT IN (
            SELECT spec_code FROM orders      WHERE spec_code IS NOT NULL AND spec_code <> ''
            UNION
            SELECT spec_code FROM order_items WHERE spec_code IS NOT NULL AND spec_code <> ''
      )
) t
WHERE product_specs.id = t.id;

-- 编码重写：同样只在编码里的数字与取值矛盾时改，且新编码未被同商品占用。
UPDATE product_specs
SET spec_code = t.family || CASE WHEN t.family <> '' THEN '-' ELSE '' END
                || t.cpu::text || 'c' || t.mem_gb::text || 'g'
FROM (
    SELECT
        s.id,
        s.product_id,
        split_part(s.spec_code, '-', 1)                                           AS family,
        (s.specs::jsonb ->> 'compute.cpu')::int                                   AS cpu,
        GREATEST(((s.specs::jsonb ->> 'compute.memory')::numeric / 1024)::int, 1) AS mem_gb
    FROM product_specs s
    WHERE s.specs IS NOT NULL
      AND s.specs <> ''
      AND s.specs::jsonb ? 'compute.cpu'
      AND s.specs::jsonb ? 'compute.memory'
      AND (s.specs::jsonb ->> 'compute.cpu')::int > 0
      AND (s.specs::jsonb ->> 'compute.memory')::int > 0
      -- 编码里的数字与取值不符才修（如 general-2c4g-30g 实为 4核8G）。
      -- 匹配 NcNG 形态；不含该形态的编码（如 mfy-2c4g）留给名称层处理。
      AND s.spec_code ~ '[0-9]+c[0-9]+g'
      AND (
            (regexp_match(s.spec_code, '([0-9]+)c[0-9]+g'))[1]::int
              IS DISTINCT FROM (s.specs::jsonb ->> 'compute.cpu')::int
         OR (regexp_match(s.spec_code, '[0-9]+c([0-9]+)g'))[1]::int
              IS DISTINCT FROM GREATEST(((s.specs::jsonb ->> 'compute.memory')::numeric / 1024)::int, 1)
      )
      AND s.spec_code NOT IN (
            SELECT spec_code FROM orders      WHERE spec_code IS NOT NULL AND spec_code <> ''
            UNION
            SELECT spec_code FROM order_items WHERE spec_code IS NOT NULL AND spec_code <> ''
      )
) t
WHERE product_specs.id = t.id
  AND product_specs.spec_code <> (
        t.family || CASE WHEN t.family <> '' THEN '-' ELSE '' END
                 || t.cpu::text || 'c' || t.mem_gb::text || 'g')
  AND NOT EXISTS (
        SELECT 1 FROM product_specs o
        WHERE o.product_id = t.product_id
          AND o.spec_code = (
                t.family || CASE WHEN t.family <> '' THEN '-' ELSE '' END
                         || t.cpu::text || 'c' || t.mem_gb::text || 'g')
          AND o.id <> t.id
  );
