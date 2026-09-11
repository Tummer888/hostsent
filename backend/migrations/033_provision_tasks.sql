-- ============================================================================
-- 033_provision_tasks.sql
-- 资源管理双链路重构 · P5 履约双通道（T5.1 / T5.2 / T5.4）
--
-- 目的：把「下单即同步调上游开通（约 50s，超过 app.write_timeout=10s）」
--   改为「订单落库 paid → 投递任务 → 进程内工作池异步履约 → 前端轮询订单状态」。
--   同时为续费补齐上游回执字段，并为生命周期阶段落库扫描补索引。
--
-- 内容：
--   1. provision_tasks                —— 开通履约任务表（唯一 order_id 保证幂等）
--   2. instance_renewals.upstream_order_id / sync_state —— 续费上游回执与同步状态
--   3. instances.lifecycle_stage 索引 —— 生命周期推进器按阶段扫描
--
-- 不引入消息队列中间件：以 DB 任务表 + 进程内工作池实现，与既有 sync_tasks 同构（§8 边界）。
-- 幂等（R4）：CREATE TABLE/INDEX IF NOT EXISTS；ADD COLUMN IF NOT EXISTS。
-- 回滚：DROP TABLE IF EXISTS provision_tasks;
--   ALTER TABLE instance_renewals DROP COLUMN IF EXISTS upstream_order_id, DROP COLUMN IF EXISTS sync_state;
--   DROP INDEX IF EXISTS idx_instances_lifecycle_stage;
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. provision_tasks：开通履约任务（T5.1）
-- ---------------------------------------------------------------------------
-- order_id 唯一：重试复用同一行并累加 attempts，避免重复投递造成重复开通；
-- status ∈ pending / running / success / failed / manual（manual=连续失败转人工队列）。
-- locked_until 用于工作池领取时的租约，避免多协程重复处理同一任务；
-- 超过租约仍 running 的任务由工作池回收重试。
CREATE TABLE IF NOT EXISTS provision_tasks (
  id           bigserial PRIMARY KEY,
  order_id     bigint       NOT NULL,
  user_id      bigint       NOT NULL,
  product_id   bigint,
  source_mode  varchar(16),
  status       varchar(20)  NOT NULL DEFAULT 'pending',
  attempts     integer      NOT NULL DEFAULT 0,
  max_attempts integer      NOT NULL DEFAULT 5,
  last_error   text,
  locked_until timestamptz,
  started_at   timestamptz,
  completed_at timestamptz,
  created_at   timestamptz  NOT NULL DEFAULT now(),
  updated_at   timestamptz  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_provision_tasks_order
  ON provision_tasks (order_id);
-- 工作池领取：按 status + created_at 取最早待办
CREATE INDEX IF NOT EXISTS idx_provision_tasks_claim
  ON provision_tasks (status, created_at);
-- 人工队列/失败视图
CREATE INDEX IF NOT EXISTS idx_provision_tasks_status
  ON provision_tasks (status);
-- 用户维度排查
CREATE INDEX IF NOT EXISTS idx_provision_tasks_user
  ON provision_tasks (user_id);

-- ---------------------------------------------------------------------------
-- 2. instance_renewals：上游续费回执与同步状态（T5.2）
-- ---------------------------------------------------------------------------
-- 链路 A 续费以上游返回的账单/主机回执为准，落 upstream_order_id 便于对账；
-- sync_state ∈ pending / upstream_ok / local_only / failed，
-- local_only 表示链路 B 无上游续费接口时的本地账期顺延。
ALTER TABLE instance_renewals
  ADD COLUMN IF NOT EXISTS upstream_order_id varchar(128);
ALTER TABLE instance_renewals
  ADD COLUMN IF NOT EXISTS sync_state varchar(32);
CREATE INDEX IF NOT EXISTS idx_instance_renewals_sync_state
  ON instance_renewals (sync_state);

-- ---------------------------------------------------------------------------
-- 3. instances：生命周期阶段扫描索引（T5.4）
-- ---------------------------------------------------------------------------
-- 推进器按 (lifecycle_stage, expire_at) 扫描待推进实例；阶段为派生目标落库值。
CREATE INDEX IF NOT EXISTS idx_instances_lifecycle_stage
  ON instances (lifecycle_stage, expire_at);

COMMIT;
