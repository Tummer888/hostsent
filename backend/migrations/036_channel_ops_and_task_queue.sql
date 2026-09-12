-- ============================================================================
-- 036_channel_ops_and_task_queue.sql —— 渠道运维入口 + 任务队列索引
--
-- 背景（本轮需求）：
--   1. 渠道与平台拆分为「上游转售渠道 / 自营平台对接」两个页面，连接测试内联到列表，
--      并给渠道记录一个可一键跳转的运维平台地址（上游云资源池的运维页面）。
--   2. 新增「任务队列」页：聚合开通履约 / 实例动作 / 续费 / 同步四类任务的
--      "是否到达上游"状态，需要按 created_at 排序的索引。
--   3. 新增「实例对账」页：本地实例与上游实例的售价/成本/到期时间比对（纯读，无需新表）。
--
-- 幂等（R4）：ADD COLUMN IF NOT EXISTS / CREATE INDEX IF NOT EXISTS。
-- 时序：先执行本迁移，再部署读取新列的后端；AutoMigrate 亦会补齐列（索引以本文件为准）。
-- 回滚：
--   ALTER TABLE resource_providers DROP COLUMN IF EXISTS ops_console_url;
--   DROP INDEX IF EXISTS idx_provision_tasks_created_at;
--   DROP INDEX IF EXISTS idx_instance_operations_created_at;
--   DROP INDEX IF EXISTS idx_instance_renewals_created_at;
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. resource_providers.ops_console_url —— 运维平台跳转地址
-- ---------------------------------------------------------------------------
-- 上游云资源池的运维页面（探针/容量看板/工单后台等）。为空表示未配置，
-- 后台不展示跳转按钮。属于渠道级展示字段，不参与任何履约逻辑。
ALTER TABLE resource_providers
  ADD COLUMN IF NOT EXISTS ops_console_url varchar(255) NOT NULL DEFAULT '';

-- ---------------------------------------------------------------------------
-- 2. 任务队列表的排序索引
-- ---------------------------------------------------------------------------
-- 任务队列按 created_at DESC 聚合排序并支持按状态过滤；各表已有 status 索引，
-- 这里补 created_at 侧索引，避免聚合查询回退到全表排序。
CREATE INDEX IF NOT EXISTS idx_provision_tasks_created_at ON provision_tasks (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_instance_operations_created_at ON instance_operations (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_instance_renewals_created_at ON instance_renewals (created_at DESC);

COMMIT;
