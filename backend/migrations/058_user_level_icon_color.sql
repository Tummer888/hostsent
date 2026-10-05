-- ============================================================================
-- 058_user_level_icon_color.sql
-- 背景（用户反馈）：等级的「名称 + 图标 + 颜色」应可由运营自定义，管理端配好后
--   用户端按等级显示不同配色。此前 user_levels 只有文案字段，前端只能按权重
--   硬编码色阶，运营想让「钻石会员」变紫没有任何入口。
--
-- 本次给 user_levels 增加两列：
--   icon  varchar(64) NOT NULL DEFAULT ''  —— 图标目录 key（如 star-filled）
--   color varchar(32) NOT NULL DEFAULT ''  —— 主题色 #RRGGBB
--
-- 存 key 不存图片 URL：配色要随主题（暗色模式、品牌色替换）走，组件图标能继承
--   当前前景色，存死的图片链接换主题就失效。
-- 空值有明确语义：前端回落到「按权重推导的默认图标与色阶」。因此本迁移不回填存量行 ——
--   六个内置等级的默认图标与配色由启动期 seedUserLevels 在「仍为空」时补齐
--   （与 benefits/门槛 同一套只补空值的策略），运营改过的值重启不会被还原。
--
-- 幂等：ADD COLUMN IF NOT EXISTS；可重复执行。
-- 顺序：本迁移先跑，再重启后端（AutoMigrate 补齐模型列 + seed 补默认图标配色）。
-- 回滚：ALTER TABLE user_levels DROP COLUMN IF EXISTS icon;
--       ALTER TABLE user_levels DROP COLUMN IF EXISTS color;
--       （会丢掉运营配过的图标与配色，回滚前先导出。）
-- ============================================================================
BEGIN;

ALTER TABLE user_levels ADD COLUMN IF NOT EXISTS icon  varchar(64) NOT NULL DEFAULT '';
ALTER TABLE user_levels ADD COLUMN IF NOT EXISTS color varchar(32) NOT NULL DEFAULT '';

COMMENT ON COLUMN user_levels.icon  IS '等级图标 key（前端图标目录，如 star-filled）；空 = 前端按权重推导默认图标';
COMMENT ON COLUMN user_levels.color IS '等级主题色 #RRGGBB；空 = 前端按权重推导默认色阶';

COMMIT;
