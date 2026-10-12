-- ============================================================================
-- 088_provider_type_rename_dcimcloud_zjmf.sql
--
-- 背景：两个魔方系平台的渠道类型码原先用拼音（mofangyun / mofangfinance），
-- 既不是厂商官方标识，也无法与上游文档对照。按上游正式名称改名：
--   mofangyun     → dcimcloud   （魔方云，DCIMCloud，自营算力平台）
--   mofangfinance → zjmf        （魔方财务，ZJMF，上游转售）
--
-- 影响面（存了类型码的全部位置）：
--   1. 列式 provider_type：resource_providers / provider_types（列名 type）/
--      spec_mappings / external_specs / upstream_api_logs /
--      provider_option_specs / platform_option_values /
--      product_config_options / product_spec_templates；
--   2. jsonb 平台键：provider_types.descriptor、spec_atoms.platform_fields；
--   3. 自由文本：upstream_api_logs.provider_name、sync_diffs.remark。
--
-- 执行方式：本目录迁移不被独立 runner 执行（见 migrations/README.md），
-- 等价回填由 internal/pkg/db/db.go 的 backfillProviderTypeCodes 在启动期幂等完成。
-- 本文件是演进记录与人工执行依据，两者口径必须一致。
--
-- 幂等：所有语句都是「等于旧码才改」，重复执行无影响。
-- provider_types 先删旧行再改名，避免 (type) 唯一索引与新码行冲突。
--
-- 回滚：把下面每一处的 'dcimcloud' ↔ 'mofangyun'、'zjmf' ↔ 'mofangfinance'
-- 对调后重跑即可；jsonb 的 replace(...::jsonb) 同样对调。
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. provider_types：唯一索引保护，先删旧行再改名
-- ---------------------------------------------------------------------------
DELETE FROM provider_types
WHERE type = 'mofangyun'
  AND EXISTS (SELECT 1 FROM provider_types t2 WHERE t2.type = 'dcimcloud');
DELETE FROM provider_types
WHERE type = 'mofangfinance'
  AND EXISTS (SELECT 1 FROM provider_types t2 WHERE t2.type = 'zjmf');

UPDATE provider_types SET type = 'dcimcloud' WHERE type = 'mofangyun';
UPDATE provider_types SET type = 'zjmf'      WHERE type = 'mofangfinance';

-- descriptor 内的平台键（field_dictionary 等）
UPDATE provider_types
SET descriptor = replace(descriptor::text, '"mofangyun"', '"dcimcloud"')::jsonb
WHERE descriptor::text LIKE '%mofangyun%';
UPDATE provider_types
SET descriptor = replace(descriptor::text, '"mofangfinance"', '"zjmf"')::jsonb
WHERE descriptor::text LIKE '%mofangfinance%';

-- ---------------------------------------------------------------------------
-- 2. 其余列式 provider_type
-- ---------------------------------------------------------------------------
UPDATE resource_providers SET provider_type = 'dcimcloud' WHERE provider_type = 'mofangyun';
UPDATE resource_providers SET provider_type = 'zjmf'      WHERE provider_type = 'mofangfinance';

UPDATE spec_mappings SET provider_type = 'dcimcloud' WHERE provider_type = 'mofangyun';
UPDATE spec_mappings SET provider_type = 'zjmf'      WHERE provider_type = 'mofangfinance';

UPDATE external_specs SET provider_type = 'dcimcloud' WHERE provider_type = 'mofangyun';
UPDATE external_specs SET provider_type = 'zjmf'      WHERE provider_type = 'mofangfinance';

UPDATE upstream_api_logs SET provider_type = 'dcimcloud' WHERE provider_type = 'mofangyun';
UPDATE upstream_api_logs SET provider_type = 'zjmf'      WHERE provider_type = 'mofangfinance';

UPDATE provider_option_specs SET provider_type = 'dcimcloud' WHERE provider_type = 'mofangyun';
UPDATE provider_option_specs SET provider_type = 'zjmf'      WHERE provider_type = 'mofangfinance';

UPDATE platform_option_values SET provider_type = 'dcimcloud' WHERE provider_type = 'mofangyun';
UPDATE platform_option_values SET provider_type = 'zjmf'      WHERE provider_type = 'mofangfinance';

UPDATE product_config_options SET provider_type = 'dcimcloud' WHERE provider_type = 'mofangyun';
UPDATE product_config_options SET provider_type = 'zjmf'      WHERE provider_type = 'mofangfinance';

UPDATE product_spec_templates SET provider_type = 'dcimcloud' WHERE provider_type = 'mofangyun';
UPDATE product_spec_templates SET provider_type = 'zjmf'      WHERE provider_type = 'mofangfinance';

-- ---------------------------------------------------------------------------
-- 3. spec_atoms.platform_fields：平台 → 原子写字段名映射
-- ---------------------------------------------------------------------------
UPDATE spec_atoms
SET platform_fields = replace(platform_fields::text, '"mofangyun"', '"dcimcloud"')::jsonb
WHERE platform_fields::text LIKE '%mofangyun%';
UPDATE spec_atoms
SET platform_fields = replace(platform_fields::text, '"mofangfinance"', '"zjmf"')::jsonb
WHERE platform_fields::text LIKE '%mofangfinance%';

-- ---------------------------------------------------------------------------
-- 4. 自由文本里的旧码（日志展示用，不改也能跑，但界面会出现旧码字样）
-- ---------------------------------------------------------------------------
UPDATE upstream_api_logs SET provider_name = replace(provider_name, 'mofangyun', 'dcimcloud')
WHERE provider_name LIKE '%mofangyun%';
UPDATE upstream_api_logs SET provider_name = replace(provider_name, 'mofangfinance', 'zjmf')
WHERE provider_name LIKE '%mofangfinance%';

COMMIT;
