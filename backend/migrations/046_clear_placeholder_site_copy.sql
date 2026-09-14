-- ============================================================================
-- 046_clear_placeholder_site_copy.sql
--
-- 背景（对照 docs/实施计划/80 §5 与 100 §8）：
--   建库时 seed 给 `site.contact_phone` / `site.contact_email` 填了示例值
--   （400-800-1234 / support@hostsent.com）。这些值会经公开白名单直接渲染到
--   官网页脚的「售前咨询热线」与首页底部通栏的「售前咨询」上 ——
--   一个打不通的 400 电话、一个收不到信的邮箱，比不显示更伤可信度：
--   用户打不通之后不会再验证第二个信息，直接就会认定整站是假的。
--
--   本次同时修正了 seed 默认值（internal/pkg/db/db.go 里改成空串），
--   但 seed 是 `ON CONFLICT DO NOTHING` —— 对已经建好的库不会覆盖，
--   因此需要本迁移把残留的**示例值**清掉。
--
-- 内容：
--   仅当值仍然**逐字等于**建库示例值时清空。运营已经填过真实号码/邮箱的库
--   不受影响（这正是不能用无条件 UPDATE 的原因）。
--
-- 幂等：UPDATE ... WHERE config_value = '<示例值>'，重复执行无副作用。
--
-- 回滚：无需回滚（清空后官网只是不渲染该行；若要恢复示例值，
--       UPDATE system_configs SET config_value='400-800-1234' WHERE config_key='site.contact_phone'）。
-- ============================================================================

BEGIN;

UPDATE system_configs
SET config_value = '', updated_at = NOW()
WHERE config_group = 'site'
  AND (
    (config_key = 'site.contact_phone' AND config_value = '400-800-1234')
    OR (config_key = 'site.contact_email' AND config_value = 'support@hostsent.com')
  );

COMMIT;
