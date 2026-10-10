-- 079_option_catalog_value_help.sql
-- 平台配置项的「取值含义」列（T4.5 规格配置化细化）。
--
-- 背景：-1 / 0 / auto / 0,0,0,0 这类取值光看取值看不出含义（-1 是「不开通」还是
-- 「不支持」因字段而异），文档口径的口径又只有运营看得见。补一列 OptionsHelp
-- 把「值 → 一句话含义」随配置项一起存下来，一路带到商品配置项与客户选配项上。
--
-- 三处新列：
--   provider_option_specs.options_help        目录项：{值: 含义} 的 JSON 对象
--   product_config_options.options_help       商品配置项：随建品从目录复制
--   product_config_options_sub.help           商品配置项取值：含义的一句话（展示用）
--
-- 表结构之外还要回填**数据**（标签/说明/默认值都要跟官方参数表对齐）：那部分由后端
-- 启动时的 seedProviderOptionCatalog 完成（以适配器声明为准刷新 source='adapter' 的行，
-- 逐字段 diff，只在真有差异时更新），不在这里写静态 UPDATE —— 目录是代码的一部分，
-- 复制一份 SQL 回填会在下次改声明时立刻过时。
--
-- 回滚：
--   ALTER TABLE product_config_options_sub DROP COLUMN IF EXISTS help;
--   ALTER TABLE product_config_options     DROP COLUMN IF EXISTS options_help;
--   ALTER TABLE provider_option_specs      DROP COLUMN IF EXISTS options_help;
--   （仅丢展示用文案，不影响开通链路。）

BEGIN;

ALTER TABLE provider_option_specs
    ADD COLUMN IF NOT EXISTS options_help JSONB;

ALTER TABLE product_config_options
    ADD COLUMN IF NOT EXISTS options_help JSONB;

ALTER TABLE product_config_options_sub
    ADD COLUMN IF NOT EXISTS help VARCHAR(255) NOT NULL DEFAULT '';

COMMENT ON COLUMN provider_option_specs.options_help IS
    '取值含义表（JSON 对象：值 → 含义），解释 -1/0/auto 等特殊取值';
COMMENT ON COLUMN product_config_options.options_help IS
    '取值含义表，建品时从平台配置项目录复制';
COMMENT ON COLUMN product_config_options_sub.help IS
    '该取值的一句话含义（特殊值解释），用户侧选配时展示';

COMMIT;

-- 验收查询（人工执行）：
--   列已就位（3 行）：
--     SELECT table_name, column_name, data_type
--       FROM information_schema.columns
--      WHERE column_name IN ('options_help','help')
--        AND table_name IN ('provider_option_specs','product_config_options','product_config_options_sub')
--      ORDER BY table_name, column_name;
--
--   后端重启后目录已按声明刷新（应有 34 行、label/help 与代码一致）：
--     SELECT option_key, label, left(help, 40) AS help, default_value, options_help IS NOT NULL AS has_vh
--       FROM provider_option_specs
--      WHERE provider_type = 'mofangyun' AND source = 'adapter'
--      ORDER BY sort_order;
