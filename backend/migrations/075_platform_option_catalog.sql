-- ============================================================================
-- 075_platform_option_catalog.sql
-- 自营链路规格配置化 · 平台配置项目录 + 平台取值库 + 配置档
--
-- 背景（本轮用户反馈）：
--   规格模板此前只有 CPU/内存/磁盘/带宽/OS 五个结构化字段，外加一个固定的
--   spec_family 枚举（通用型/计算型/内存型/存储型/GPU型）。运营既无法按对接平台
--   （魔方云）声明它实际支持的全部参数，也无法为每个参数预置「可选值」，新建商品时
--   只能手写 JSON。本次改造把「规格模板」升级为「配置档」：
--     平台配置项目录（provider_option_specs）
--       ↓ 平台/手工提供可选值
--     平台取值库（platform_option_values，镜像/CPU 档/区域/节点/存储统一一张表）
--       ↓ 运营勾选
--     配置档（product_spec_templates.option_selections）
--       ↓ 建品勾选
--     1 个 SKU + N 个客户可选配置项（product_config_options 结构化字段）
--
-- 改动清单：
--   1. provider_option_specs  平台可配置项声明（含运营自定义新增项）。
--   2. platform_option_values 平台取值库（镜像类型批量入库的落点）。
--   3. product_spec_templates 增 provider_type / option_selections /
--      name_template / description_template；spec_family 保留但不再写入。
--   4. product_config_options(_sub) 增结构化渲染与计价字段（不改变
--      config_groups[].options[].sub[] 的 API 形状，只加字段，兼容上游克隆链路）。
--   5. orders / order_items 增 config_options（jsonb）：客户选配选择是履约的唯一真源。
--
-- 幂等：全部 ADD COLUMN / CREATE TABLE / CREATE INDEX 带 IF NOT EXISTS。
-- 回滚：见文件末尾。
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1) 平台配置项目录
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS provider_option_specs (
    id            BIGSERIAL PRIMARY KEY,
    provider_type VARCHAR(64)  NOT NULL,             -- 平台类型（mofangyun / ...）
    option_key    VARCHAR(64)  NOT NULL,             -- 平台写参数名（cpu/memory/os/area...）
    label         VARCHAR(128) NOT NULL DEFAULT '',  -- 中文名（CPU / 操作系统 / 数据中心）
    group_name    VARCHAR(64)  NOT NULL DEFAULT '',  -- 分组（基础配置/网络/高级）
    required      BOOLEAN      NOT NULL DEFAULT false, -- 文档「必选」
    default_value VARCHAR(255) NOT NULL DEFAULT '',  -- 文档「未传递默认值」
    widget        VARCHAR(32)  NOT NULL DEFAULT 'select', -- select|radio|button_group|qty|bool|json
    value_source  VARCHAR(32)  NOT NULL DEFAULT 'static', -- static|manual|areas|nodes|stores|images
    options       JSONB,                              -- 静态枚举 [{label,value}]
    min_value     NUMERIC(16,4),                      -- 数量型下限
    max_value     NUMERIC(16,4),                      -- 数量型上限
    step_value    NUMERIC(16,4),                      -- 数量型步长
    unit          VARCHAR(16)  NOT NULL DEFAULT '',  -- 核/GB/Mbps/个
    help          TEXT         NOT NULL DEFAULT '',  -- 注释性标注（取值从哪来、怎么填）
    multi_value   BOOLEAN      NOT NULL DEFAULT false, -- 是否允许多选（配置档里做成多选列表）
    hidden        BOOLEAN      NOT NULL DEFAULT false, -- 后台列表隐藏（不删，仅收起）
    sort_order    INTEGER      NOT NULL DEFAULT 0,
    source        VARCHAR(16)  NOT NULL DEFAULT 'adapter', -- adapter 适配器导入 / custom 运营新增
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_provider_option_specs
  ON provider_option_specs (provider_type, option_key);
CREATE INDEX IF NOT EXISTS idx_provider_option_specs_list
  ON provider_option_specs (provider_type, hidden, sort_order);

-- ---------------------------------------------------------------------------
-- 2) 平台取值库（镜像/CPU 档/区域/节点/存储/自定义取值）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS platform_option_values (
    id            BIGSERIAL PRIMARY KEY,
    provider_type VARCHAR(64)  NOT NULL,
    option_key    VARCHAR(64)  NOT NULL,
    value         VARCHAR(255) NOT NULL,             -- 下发平台的原始取值（魔方云多为数字 ID）
    label         VARCHAR(255) NOT NULL DEFAULT '',  -- 展示名
    parent_value  VARCHAR(255) NOT NULL DEFAULT '',  -- 节点/存储挂区域；镜像挂节点
    group_label   VARCHAR(128) NOT NULL DEFAULT '',  -- 镜像家族（Ubuntu/Windows/CentOS）
    status        VARCHAR(16)  NOT NULL DEFAULT 'active', -- active|offline
    origin        VARCHAR(16)  NOT NULL DEFAULT 'manual', -- platform 平台拉取 / manual 手工或批量导入
    sort_order    INTEGER      NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_option_values
  ON platform_option_values (provider_type, option_key, value, parent_value);
CREATE INDEX IF NOT EXISTS idx_platform_option_values_lookup
  ON platform_option_values (provider_type, option_key, status);

-- ---------------------------------------------------------------------------
-- 3) 配置档（product_spec_templates）
-- ---------------------------------------------------------------------------
ALTER TABLE product_spec_templates ADD COLUMN IF NOT EXISTS provider_type varchar(64) NOT NULL DEFAULT '';
-- option_selections: 每参数勾选的可选值，形如
--   {"cpu":{"values":["2","4"]},"bw":{"range":[1,100],"default":"10"}}
ALTER TABLE product_spec_templates ADD COLUMN IF NOT EXISTS option_selections jsonb;
-- name_template / description_template：档位商品名与描述的渲染模板，如 "{cpu}核{memory}G {os}"
ALTER TABLE product_spec_templates ADD COLUMN IF NOT EXISTS name_template varchar(255) NOT NULL DEFAULT '';
ALTER TABLE product_spec_templates ADD COLUMN IF NOT EXISTS description_template text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_product_spec_templates_provider
  ON product_spec_templates (provider_type, status);
-- 说明：spec_family 列保留（存量数据不动），新流程不再写入，UI 与 SKU 编码也不再使用。

-- ---------------------------------------------------------------------------
-- 4) 商品配置项：结构化渲染 + 选配计价字段
-- ---------------------------------------------------------------------------
ALTER TABLE product_config_options ADD COLUMN IF NOT EXISTS provider_type varchar(64) NOT NULL DEFAULT '';
ALTER TABLE product_config_options ADD COLUMN IF NOT EXISTS option_key    varchar(64) NOT NULL DEFAULT '';
ALTER TABLE product_config_options ADD COLUMN IF NOT EXISTS widget        varchar(32) NOT NULL DEFAULT 'select';
ALTER TABLE product_config_options ADD COLUMN IF NOT EXISTS required      boolean     NOT NULL DEFAULT false;
ALTER TABLE product_config_options ADD COLUMN IF NOT EXISTS default_value varchar(255) NOT NULL DEFAULT '';
-- widget_group：同组下拉的分组标签（操作系统家族 Ubuntu/Windows），空则平铺
ALTER TABLE product_config_options ADD COLUMN IF NOT EXISTS widget_group  varchar(128) NOT NULL DEFAULT '';
ALTER TABLE product_config_options ADD COLUMN IF NOT EXISTS min_value     numeric(16,4);
ALTER TABLE product_config_options ADD COLUMN IF NOT EXISTS max_value     numeric(16,4);
ALTER TABLE product_config_options ADD COLUMN IF NOT EXISTS unit          varchar(16) NOT NULL DEFAULT '';
ALTER TABLE product_config_options ADD COLUMN IF NOT EXISTS help          text        NOT NULL DEFAULT '';

ALTER TABLE product_config_options_sub ADD COLUMN IF NOT EXISTS group_label varchar(128) NOT NULL DEFAULT '';
ALTER TABLE product_config_options_sub ADD COLUMN IF NOT EXISTS is_default  boolean      NOT NULL DEFAULT false;

-- ---------------------------------------------------------------------------
-- 5) 订单选配快照（履约真源）
-- ---------------------------------------------------------------------------
ALTER TABLE orders      ADD COLUMN IF NOT EXISTS config_options jsonb;
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS config_options jsonb;

COMMIT;

-- ============================================================================
-- 验收查询（人工执行）
--   \d provider_option_specs      -- 期望含 option_key/label/required/default_value/options
--   \d platform_option_values     -- 唯一键 (provider_type, option_key, value, parent_value)
--   \d product_spec_templates     -- 期望新增 provider_type/option_selections/name_template
--   \d product_config_options     -- 期望新增 widget/required/default_value/min_value/unit
--   \d orders                     -- 期望含 config_options jsonb
--
-- 回滚：
--   BEGIN;
--   ALTER TABLE orders DROP COLUMN IF EXISTS config_options;
--   ALTER TABLE order_items DROP COLUMN IF EXISTS config_options;
--   ALTER TABLE product_config_options_sub DROP COLUMN IF EXISTS group_label, DROP COLUMN IF EXISTS is_default;
--   ALTER TABLE product_config_options
--     DROP COLUMN IF EXISTS provider_type, DROP COLUMN IF EXISTS option_key,
--     DROP COLUMN IF EXISTS widget, DROP COLUMN IF EXISTS required,
--     DROP COLUMN IF EXISTS default_value, DROP COLUMN IF EXISTS widget_group,
--     DROP COLUMN IF EXISTS min_value, DROP COLUMN IF EXISTS max_value,
--     DROP COLUMN IF EXISTS unit, DROP COLUMN IF EXISTS help;
--   ALTER TABLE product_spec_templates
--     DROP COLUMN IF EXISTS provider_type, DROP COLUMN IF EXISTS option_selections,
--     DROP COLUMN IF EXISTS name_template, DROP COLUMN IF EXISTS description_template;
--   DROP TABLE IF EXISTS platform_option_values;
--   DROP TABLE IF EXISTS provider_option_specs;
--   COMMIT;
-- ============================================================================
