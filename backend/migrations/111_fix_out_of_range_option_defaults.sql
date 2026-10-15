-- ============================================================================
-- 111_fix_out_of_range_option_defaults.sql
-- 修正「数量型配置项的默认值落在区间之外」的脏数据（模板 + 商品两层）。
--
-- 背景（用户反馈）：
--   自营商品下单（后台代下单）时，带宽 / 数据盘的预填值显示为 0，
--   而模板里明明配的是区间 [10,100]、[50,100]。字段没有传过来。
--
-- 根因（已实测确认）：
--   模板的 option_selections 里，数量型项写成 {"range":[min,max],"default":d}，
--   而 d 可以落在 [min,max] 之外 —— 编辑器把「默认值」做成独立输入框，
--   初值为 0，运营只改了区间没改默认值，保存时无人校验。
--   下单表单按声明取默认值预填，于是显示 0（真实值区间下限是 10 / 50）。
--   受影响实测样本（模板 id=17）：
--     bw             {"range":[10,100],"default":"0"}   → 0 不在 [10,100]
--     data_disk_size {"range":[50,100],"default":"0"}   → 0 不在 [50,100]
--     system_disk_size {"range":[30,30],"default":"50"} → 50 不在 [30,30]
--
-- 两层都要修（缺一层故障仍在）：
--   1) 模板层 product_spec_templates.option_selections —— 运营下次建品还会照抄；
--   2) 商品层 product_config_options.default_value —— 建品时已快照的副本。
--      改模板**不会**回写已有商品（生成是一次性的），存量商品 206 的四条脏值在这一层。
--
-- 两个执行面（同一条演进）：
--   1) 存量修正 —— 本文件；等价实现见 internal/pkg/db/db.go 的
--      clampOptionDefaults（迁移目录不被 runner 执行，见 migrations/README.md，
--      故启动期幂等回填是主路径）。
--   2) 防复发 —— 后端保存校验（specService.validateOptionSelections 已加
--      「默认值必须落在区间内」）+ 模板编辑器改为「固定值 / 动态范围」二选一
--      （固定值不再单独填默认值，从源头消除越界）。
--
-- 修正口径：把默认值夹到区间内（clamp），而不是删掉默认值。
--   夹取保留运营"有默认值"的意图，只把它挪进合法范围；
--   删掉会让该项退回"未配置"，客户侧可能看到空值。
--
-- 幂等：只更新默认值越界的行，重复执行第二次影响 0 行。
-- ============================================================================

BEGIN;

-- 先看一眼要改什么（psql 会打印，便于事后核对）。
-- 模板层：default 在 jsonb 里，是字符串。
SELECT 'template' AS layer,
       t.id,
       t.name,
       kv.key AS option_key,
       kv.value -> 'range'   AS range,
       kv.value -> 'default' AS bad_default
FROM product_spec_templates t
CROSS JOIN LATERAL jsonb_each(t.option_selections::jsonb) AS kv(key, value)
WHERE jsonb_typeof(kv.value -> 'range') = 'array'
  AND jsonb_array_length(kv.value -> 'range') = 2
  AND COALESCE(kv.value ->> 'default', '') <> ''
  AND (kv.value ->> 'default')::numeric
      NOT BETWEEN (kv.value -> 'range' ->> 0)::numeric AND (kv.value -> 'range' ->> 1)::numeric;

-- 商品层：default_value 是 varchar 列，区间是 numeric 列。
SELECT 'product' AS layer,
       pco.product_id,
       pco.option_key,
       pco.default_value AS bad_default,
       pco.min_value,
       pco.max_value
FROM product_config_options pco
WHERE pco.default_value ~ '^-?[0-9]+(\.[0-9]+)?$'
  AND pco.min_value IS NOT NULL
  AND pco.max_value IS NOT NULL
  AND (pco.default_value::numeric < pco.min_value OR pco.default_value::numeric > pco.max_value);

-- 模板层：把越界默认值夹进区间 default = LEAST(GREATEST(default, min), max)
--
-- 两处防御性判断（都是实测踩到的）：
--   * option_selections 是 jsonb 列，不能与空串比较（会报 invalid input syntax for type json）；
--   * default 未必是数字（如 port 的 "auto"），直接 ::numeric 会报
--     invalid input syntax for type numeric。用正则先确认是数字再夹。
UPDATE product_spec_templates t
SET option_selections = (
    SELECT jsonb_object_agg(
               kv.key,
               CASE
                   WHEN jsonb_typeof(kv.value -> 'range') = 'array'
                        AND jsonb_array_length(kv.value -> 'range') = 2
                        AND (kv.value ->> 'default') ~ '^-?[0-9]+(\.[0-9]+)?$'
                        AND (kv.value -> 'range' ->> 0) ~ '^-?[0-9]+(\.[0-9]+)?$'
                        AND (kv.value -> 'range' ->> 1) ~ '^-?[0-9]+(\.[0-9]+)?$'
                        AND (kv.value ->> 'default')::numeric
                            NOT BETWEEN (kv.value -> 'range' ->> 0)::numeric
                                    AND (kv.value -> 'range' ->> 1)::numeric
                   THEN jsonb_set(
                            kv.value,
                            '{default}',
                            to_jsonb(LEAST(
                                GREATEST((kv.value ->> 'default')::numeric,
                                         (kv.value -> 'range' ->> 0)::numeric),
                                (kv.value -> 'range' ->> 1)::numeric
                            ))
                        )
                   ELSE kv.value
               END
           )
    FROM jsonb_each(t.option_selections) AS kv(key, value)
),
    updated_at = now()
WHERE t.option_selections IS NOT NULL
  AND jsonb_typeof(t.option_selections) = 'object'
  AND EXISTS (
      SELECT 1
      FROM jsonb_each(t.option_selections) AS kv(key, value)
      WHERE jsonb_typeof(kv.value -> 'range') = 'array'
        AND jsonb_array_length(kv.value -> 'range') = 2
        AND (kv.value ->> 'default') ~ '^-?[0-9]+(\.[0-9]+)?$'
        AND (kv.value -> 'range' ->> 0) ~ '^-?[0-9]+(\.[0-9]+)?$'
        AND (kv.value -> 'range' ->> 1) ~ '^-?[0-9]+(\.[0-9]+)?$'
        AND (kv.value ->> 'default')::numeric
            NOT BETWEEN (kv.value -> 'range' ->> 0)::numeric AND (kv.value -> 'range' ->> 1)::numeric
  );

-- 商品层：同一套夹取。快照层不修的话，存量商品（如 206）下单仍会预填出 0。
-- trim_scale 去掉夹取产生的尾零（30.0000 → 30），否则客户侧显示成 "30.0000 GB"；
-- 第二个条件兜的是「已被早期版本夹过、但留着尾零的整数」行（30.0000 不再越界，
-- 只按越界判定会永远停留在带尾零的状态）。只对整数值生效，形如 "1.50" 的
-- 小数默认保持原样 —— 它可能是被逐字符比对的标识串。
UPDATE product_config_options
SET default_value = trim_scale(LEAST(
        GREATEST(default_value::numeric, min_value),
        max_value
    ))::text,
    updated_at = now()
WHERE default_value ~ '^-?[0-9]+(\.[0-9]+)?$'
  AND min_value IS NOT NULL
  AND max_value IS NOT NULL
  AND (default_value::numeric < min_value
       OR default_value::numeric > max_value
       OR (default_value::numeric = trunc(default_value::numeric)
           AND trim_scale(default_value::numeric)::text <> default_value));

COMMIT;
