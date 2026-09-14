-- ============================================================================
-- 045_retire_flat_site_keys.sql
--
-- 背景（docs/实施计划/100 内容中心与站点内容管理设计 §8.1 P1-1）：
--   system_configs 里同一份站点品牌信息存在两套键名：
--     - 历史扁平键（`site_name` / `site_logo` / `contact_phone` …），由管理端
--       「系统配置 → 基础配置」旧表单写入；
--     - 规范点号键（`site.name` / `site.logo` / `site.contact_phone` …），
--       由 seed 写入、公开接口与门户 schema 消费。
--   两行同时 active 时，门户 `fromFlatConfig` 的第二轮写入让**扁平键覆盖点号键**，
--   于是「官网名称」实际显示的是 09-05 建库时的旧占位值，而不是运营在 site 分组维护的值。
--
-- 内容：
--   1. 逐对补齐：点号键为空（或行不存在）而扁平键有值时，把扁平键的值迁到点号键；
--   2. 扁平键置 disabled：门户公开接口只取 active 行，覆盖随之消失；
--      不物理删除——回滚只需把 status 改回 active，且不销毁任何运营填过的值。
--
-- 配套代码改动（本次一并落地）：
--   - 管理端系统配置页改为只写点号键（frontend-admin/.../system/config/index.vue）；
--   - 邮件外壳 / 群发取品牌名改为「先点号键、后扁平键」
--     （internal/modules/admin/notification/service/{render,broadcast_service}.go）；
--   - 用户中心品牌 store 改为点号键优先（frontend-user/src/store/modules/brand.ts）。
--
-- 幂等：INSERT ... ON CONFLICT (config_key) DO UPDATE 只在点号键为空时写入；
--       UPDATE ... SET status='disabled' 可重复执行。
--
-- 回滚：
--   UPDATE system_configs SET status='active' WHERE config_key IN (…扁平键列表…) AND config_group='site';
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. 扁平键 → 点号键 补齐（仅当点号键为空，绝不覆盖已有规范值）
-- ---------------------------------------------------------------------------
INSERT INTO system_configs (config_key, config_value, value_type, config_group, description, sort_order, status, created_at, updated_at)
SELECT d.dotted_key, f.config_value, f.value_type, 'site', d.description, d.sort_order, 'active', now(), now()
  FROM (VALUES
        ('site_name',           'site.name',            '官网名称',       1),
        ('site_slogan',         'site.slogan',          '品牌标语',       2),
        ('site_logo',           'site.logo',            '站点 Logo 地址', 3),
        ('site_favicon',        'site.favicon',         '站点 favicon',   4),
        ('site_icp',            'site.icp',             'ICP 备案号',     5),
        ('site_copyright',      'site.copyright',       '版权信息',       6),
        ('site_license_no',     'site.license_no',      '增值电信业务经营许可证号', 7),
        ('site_license_org',    'site.license_org',     '代理域名注册服务机构',     8),
        ('site_public_security','site.public_security', '公网安备号',     9),
        ('contact_phone',       'site.contact_phone',   '客服电话',      10),
        ('contact_email',       'site.contact_email',   '客服邮箱',      11),
        ('contact_address',     'site.contact_address', '联系地址',      12),
        ('contact_wechat',      'site.wechat',          '公众号名称',    15)
       ) AS d(flat_key, dotted_key, description, sort_order)
  JOIN system_configs f ON f.config_key = d.flat_key
 WHERE btrim(coalesce(f.config_value, '')) <> ''
ON CONFLICT (config_key) DO UPDATE
   SET config_value = EXCLUDED.config_value,
       updated_at   = now()
 WHERE btrim(coalesce(system_configs.config_value, '')) = '';

-- ---------------------------------------------------------------------------
-- 2. 扁平键停用（两套键同时存在时的覆盖问题至此消失）
-- ---------------------------------------------------------------------------
UPDATE system_configs
   SET status = 'disabled', updated_at = now()
 WHERE config_key IN (
        'site_name', 'site_slogan', 'site_logo', 'site_favicon',
        'site_icp', 'site_copyright', 'site_license_no', 'site_license_org',
        'site_public_security', 'contact_phone', 'contact_email',
        'contact_address', 'contact_wechat'
       )
   AND status <> 'disabled';

COMMIT;
