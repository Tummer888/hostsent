-- ============================================================================
-- 031_sync_framework.sql
-- 资源管理双链路重构 · P3 同步框架（T3.2 / T3.3 / T3.4 / T3.5）
--
-- 目的：把「一个渠道一个 sync_interval、固定三类任务（product/pool/instance）」
--   升级为「渠道 × scope 多节奏调度」，并为增量游标、全量对账、上游改价待确认、
--   同步差异落库提供权威存储（详见 docs/实施计划/16 §6、17 §P3）。
--
-- 内容：
--   1. sync_schedules       —— 渠道 × scope 调度配置（独立节奏 / 优先级 / 时间窗 / 全量周期）
--   2. sync_cursors         —— 渠道 × scope 增量游标与全量对账时间
--   3. price_change_events  —— 上游改价事件（阈值内自动应用，超阈值进待确认队列）
--   4. sync_diffs           —— 每次同步的增/删/改差异（对账页与差异页的真实数据源）
--   5. resource_providers.price_change_threshold —— 渠道级调价阈值（默认 5%）
--
-- 幂等（R4）：CREATE TABLE/INDEX IF NOT EXISTS；ADD COLUMN IF NOT EXISTS；
--   不写入业务种子（调度行由后端按能力描述符懒创建，见 sync/service/scheduler.go）。
-- 时序：先执行本迁移，再部署读取新表的后端；AutoMigrate 只保证列存在，
--   索引与唯一键以本文件为准。
-- 回滚：DROP TABLE IF EXISTS sync_diffs, price_change_events, sync_cursors, sync_schedules;
--   ALTER TABLE resource_providers DROP COLUMN IF EXISTS price_change_threshold;
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. resource_providers：渠道级调价阈值
-- ---------------------------------------------------------------------------
-- 上游成本价变动幅度 ≤ 阈值时自动应用并留痕；超过则写入 price_change_events
-- 等待人工确认，绝不静默改售价（T3.4）。0.05 = 5%。
ALTER TABLE resource_providers
  ADD COLUMN IF NOT EXISTS price_change_threshold numeric(10,4) NOT NULL DEFAULT 0.05;

-- ---------------------------------------------------------------------------
-- 2. sync_schedules：渠道 × scope 调度配置（T3.2）
-- ---------------------------------------------------------------------------
-- 现状是 resource_providers.sync_interval 一个值统管三类同步，粒度太粗：
-- 目录/价格要小时级、实例状态要分钟级、区域/镜像一天一次即可。改为每
-- (provider_id, scope) 一行，各自 interval_seconds / enabled / priority / window。
-- window_start/window_end 为允许执行的小时区间（0-23），NULL 表示不限；
-- full_sync_interval_seconds 控制增量拉取之外的全量对账周期（T3.3）。
CREATE TABLE IF NOT EXISTS sync_schedules (
  id                         bigserial PRIMARY KEY,
  provider_id                bigint       NOT NULL,
  scope                      varchar(32)  NOT NULL,
  interval_seconds           integer      NOT NULL DEFAULT 3600,
  full_sync_interval_seconds integer      NOT NULL DEFAULT 604800,
  enabled                    boolean      NOT NULL DEFAULT true,
  priority                   integer      NOT NULL DEFAULT 0,
  window_start               smallint,
  window_end                 smallint,
  last_run_at                timestamptz,
  next_run_at                timestamptz,
  last_status                varchar(20),
  last_error                 text,
  created_at                 timestamptz,
  updated_at                 timestamptz
);

-- 唯一性用「命名唯一索引」而非列内联 UNIQUE：GORM 的 uniqueIndex 标签按
-- uk_<table>_<cols> 查找/维护约束，内联 UNIQUE 会生成 <table>_<cols>_key
-- 导致启动期 DropConstraint 报 42704（P2 已踩，见 030 中同类注释）。
CREATE UNIQUE INDEX IF NOT EXISTS uk_sync_schedules_provider_scope
  ON sync_schedules (provider_id, scope);
-- 调度扫描：enabled + next_run_at（到期判定），priority 参与排序
CREATE INDEX IF NOT EXISTS idx_sync_schedules_due
  ON sync_schedules (enabled, next_run_at, priority DESC);

-- ---------------------------------------------------------------------------
-- 3. sync_cursors：增量游标与全量对账时间（T3.3）
-- ---------------------------------------------------------------------------
-- cursor 为上游分页/时间戳游标（各适配器自解释，存字符串）；增量拉取为主，
-- 超过 full_sync_interval 做一次全量对账：全量结果与本地差集 → 本地多出的
-- 记录置 offline（软删除），不物理删除。
CREATE TABLE IF NOT EXISTS sync_cursors (
  id                  bigserial PRIMARY KEY,
  provider_id         bigint       NOT NULL,
  scope               varchar(32)  NOT NULL,
  cursor              text,
  last_incremental_at timestamptz,
  last_full_at        timestamptz,
  last_seen_count     integer      NOT NULL DEFAULT 0,
  created_at          timestamptz,
  updated_at          timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_sync_cursors_provider_scope
  ON sync_cursors (provider_id, scope);

-- ---------------------------------------------------------------------------
-- 4. price_change_events：上游调价事件（T3.4）
-- ---------------------------------------------------------------------------
-- 上游调价绝不静默改售价，同步发现 cost_price/sale_price 变化即写事件：
--   · 幅度 ≤ threshold  → status=auto_applied（自动应用成本价并留审计）
--   · 幅度 >  threshold → status=pending（进待确认队列，后台批量确认/驳回）
-- 确认后写 product_history（products 侧）并发通知（T3.4）。
CREATE TABLE IF NOT EXISTS price_change_events (
  id                  bigserial PRIMARY KEY,
  provider_id         bigint        NOT NULL,
  scope               varchar(32)   NOT NULL DEFAULT 'price',
  resource_product_id bigint,
  upstream_id         varchar(128),
  product_id          bigint,
  field               varchar(32)   NOT NULL DEFAULT 'cost_price',
  old_value           numeric(12,4),
  new_value           numeric(12,4),
  change_ratio        numeric(10,4),
  threshold           numeric(10,4),
  status              varchar(16)   NOT NULL DEFAULT 'pending',
  applied             boolean       NOT NULL DEFAULT false,
  handled_by          bigint,
  handled_at          timestamptz,
  remark              varchar(255),
  created_at          timestamptz,
  updated_at          timestamptz
);

CREATE INDEX IF NOT EXISTS idx_price_change_events_provider_status
  ON price_change_events (provider_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_price_change_events_product
  ON price_change_events (product_id);

-- ---------------------------------------------------------------------------
-- 5. sync_diffs：同步差异记录（T3.5）
-- ---------------------------------------------------------------------------
-- 每次同步的增/删/改与调价明细（scope + 本地/外部 id + 字段 + 旧值/新值 +
-- 处置结果）。现有「对账报告」「差异对比」两页都是从同步日志硬凑的，
-- 有了本表才有真实数据源。
CREATE TABLE IF NOT EXISTS sync_diffs (
  id           bigserial PRIMARY KEY,
  task_id      bigint,
  provider_id  bigint       NOT NULL,
  scope        varchar(32)  NOT NULL,
  action       varchar(16)  NOT NULL,
  local_id     bigint,
  external_id  varchar(128),
  field        varchar(64),
  old_value    text,
  new_value    text,
  disposition  varchar(16)  NOT NULL DEFAULT 'applied',
  remark       varchar(255),
  created_at   timestamptz
);

CREATE INDEX IF NOT EXISTS idx_sync_diffs_provider_scope_created
  ON sync_diffs (provider_id, scope, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sync_diffs_task ON sync_diffs (task_id);
CREATE INDEX IF NOT EXISTS idx_sync_diffs_action ON sync_diffs (action);

COMMIT;
