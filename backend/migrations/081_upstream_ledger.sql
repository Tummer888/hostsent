-- ============================================================================
-- 081_upstream_ledger.sql
-- 上游账本流水（成本管理 doc111 §5.2）：把上游的「消费流水」与「充值流水」落库，
-- 让月度上游成本从「余额差推算」升级为「流水归集」——逐笔可查、可回填历史、
-- 并与余额快照互校（差额即待查项）。
--
-- 背景：上游（魔方财务）会员中心暴露了两条账：
--   GET /credit_record            余额支付消费（订购/续费，含上游账单号 relid）
--   GET /v1/transactions/funds    充值/入账（含支付方式与交易号）
-- 加上已实现的 GET /cart/credit（当前余额），成本模块三口径齐备：
--   ① 流水口径（本表，主口径）② 快照推算口径（期初+充值−期末，核对）③ 到期金额（余额水位告警）。
--
-- 幂等：CREATE TABLE/INDEX IF NOT EXISTS；写入侧以 (provider_id, kind, external_id)
--   唯一键 upsert（ON CONFLICT DO UPDATE），重复同步与全量回填都走同一入口。
-- 双写：运行时建表由 db.go 的 AutoMigrate（costmodel.UpstreamLedgerEntry）完成，
--   本文件是版本留痕，列/索引/默认值必须与模型一致（live 用例锁定）。
-- 回滚：DROP TABLE upstream_ledger_entries;（本表只用于成本展示与对账，无外键依赖）
-- ============================================================================

BEGIN;

CREATE TABLE IF NOT EXISTS upstream_ledger_entries (
    id            BIGSERIAL PRIMARY KEY,
    provider_id   BIGINT        NOT NULL,
    -- consume=余额支付消费（订购/续费）；topup=充值/入账
    kind          VARCHAR(16)   NOT NULL,
    -- 上游记录 ID（字符串化）：上游自增，既做幂等键也做增量同步断点
    external_id   VARCHAR(64)   NOT NULL,
    occurred_at   TIMESTAMPTZ   NOT NULL,
    -- 金额恒为正，方向由 kind 决定
    amount        DECIMAL(15,2) NOT NULL DEFAULT 0,
    -- 该笔消费对应的退款金额（正数=已冲回）；消费净额 = amount − refund_amount
    refund_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    -- 上游类型原文（订购产品 / 续费 / 用户充值 / 人工入账）
    category      VARCHAR(64)   NOT NULL DEFAULT '',
    ref_no        VARCHAR(64)   NOT NULL DEFAULT '',
    description   VARCHAR(255)  NOT NULL DEFAULT '',
    currency      VARCHAR(8)    NOT NULL DEFAULT 'CNY',
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_up_ledger_ext
    ON upstream_ledger_entries (provider_id, kind, external_id);
CREATE INDEX IF NOT EXISTS idx_up_ledger_provider_time
    ON upstream_ledger_entries (provider_id, occurred_at);

COMMIT;

-- 验收查询（人工执行）
--   表与索引齐备：
--   SELECT indexname FROM pg_indexes WHERE tablename = 'upstream_ledger_entries';
--   期望：uk_up_ledger_ext、idx_up_ledger_provider_time
--   同步后可核对流水口径与余额口径：
--   SELECT kind, count(*), SUM(amount) FROM upstream_ledger_entries
--    WHERE provider_id = 7 GROUP BY kind;
--   期望：consume 139 笔 / ¥7,236.92、topup 29 笔 / ¥8,012.00（haika 实测，2025-03 至 2026-10）
