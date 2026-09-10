-- ============================================================================
-- 017_drop_quota_tables.sql
-- 退役资源配额子系统：resource_quotas / quota_template_items / quota_templates
-- / quota_adjustment_logs，并剥掉 user_levels 的配额列。
--
-- 背景：这套配额只有表、管理端页面与种子数据，没有任何执行逻辑——
--   · resource_quotas.used_value 全仓仅由种子写入，用户开通/销毁资源时从不更新；
--   · 下单、开通、生命周期流程均不读配额，超配不会拦截；
--   · 用户中心前端零引用，用户看不到自己的配额。
-- 经确认「移除配额、保留用户等级」：user_levels 继续作为消费升级与权益的载体
-- （见 docs/实施计划/81-账号体系与权限分级重构设计.md、82-...执行清单.md P3）。
--
-- 前置（已在代码层完成并通过 build）：
--   1) internal/modules/admin/user/quota 整模块删除，等级迁至
--      internal/modules/admin/user/level（model/repository/service/handler）。
--   2) db.AutoMigrate 已移除这四张表的模型注册；
--   3) db.Seed 已移除 seedDemoQuota / 配额模板 / 资源配额 / 调整记录，仅保留 seedUserLevels。
--
-- 执行时序：先构建并部署新后端，再执行本迁移（与 013 同理），否则运行中的旧后端
--           访问 /admin/quotas 等接口会因表不存在而报错。
--
-- 幂等：DROP TABLE IF EXISTS + DROP COLUMN IF EXISTS，可重复执行。
-- 数据：这四张表此前仅有演示种子数据；若你的库中存在需要保留的配额数据，请先自行备份。
--
-- 注意（实测行为）：若数据库里已有进程长期持有 user_levels 的预备语句缓存，
--   ALTER TABLE 改列后会有一条查询报 SQLSTATE 0A000
--   "cached plan must not change result type"。这是 PostgreSQL 对失效执行计划的
--   一次性报错，同一语句的下一次执行会自动重新规划并成功，无需重启。若使用连接池
--   且不希望线上出现这一条 500，建议在低峰执行本迁移。
-- ============================================================================

BEGIN;

DROP TABLE IF EXISTS quota_adjustment_logs;
DROP TABLE IF EXISTS resource_quotas;
DROP TABLE IF EXISTS quota_template_items;
DROP TABLE IF EXISTS quota_templates;

-- 用户等级去掉配额相关列，仅保留等级自身语义（weight / feature_flags / upgrade_condition）。
ALTER TABLE user_levels
  DROP COLUMN IF EXISTS default_template_id,
  DROP COLUMN IF EXISTS max_instance_count,
  DROP COLUMN IF EXISTS max_cpu_cores,
  DROP COLUMN IF EXISTS max_memory_gb,
  DROP COLUMN IF EXISTS max_disk_gb;

COMMIT;
