-- ============================================================================
-- 093_upstream_push_and_reconcile_findings.sql
-- doc120：上游推送接收端 + 差异预警持久化，两块一起落库。
--
-- 背景：
--   1) 魔方财务（zjmf）上游会向 {downstream_url}/api/host/sync 与
--      /api/ticket_reply/sync 推送主机状态与工单回复（源码 pushHostInfo /
--      pushTicketReply，失败落 zjmf_pushhost 重试至多 5 次）。我方此前只下发
--      downstream_url/token 而没有接收端，上游推过来直接 404，重试 5 次后放弃。
--      接收端要按上游 tid 反查本地工单，故 tickets 需要 upstream_tid 列；
--      工单回复要按上游 rid 去重（上游重试会重复 POST），故 ticket_replies
--      需要 upstream_reply_id 列 + 部分唯一索引。
--   2) 实例对账目前是纯只读页面：差异算出来了但没人看就流失了。本迁移建
--      reconcile_findings 把「售价低于成本 / 毛利过薄 / 到期错位 / 状态不同步」
--      落库并跟踪生命周期（open → ack/ignored → resolved），供预警通知与
--      持续时长统计使用。
--
-- 幂等：IF NOT EXISTS / 条件 DELETE，重复执行无副作用。
-- 顺序：本迁移先跑；列表结构由 GORM AutoMigrate 在服务启动时补列对齐
--      （tickets.upstream_tid / ticket_replies.upstream_reply_id / reconcile_findings
--       三处模型与 DDL 双写，见 internal/pkg/db/db.go 的 AutoMigrate 列表）。
-- 回滚：DROP TABLE reconcile_findings；DROP INDEX uk_ticket_replies_upstream；
--      ALTER TABLE tickets DROP COLUMN upstream_tid;
--      ALTER TABLE ticket_replies DROP COLUMN upstream_reply_id;
--      （列可留空字段，回滚不影响既有数据。）
-- ============================================================================

BEGIN;

-- ---- 1. 工单：上游工单号（zjmf tid）----
-- 上游推送 /api/ticket_reply/sync 时带的 id 是「下游工单 id」，但真实反查键是
-- 上游自己的 tid（源码 where("upstream_tid", $tid)）。这里给本地工单加一列存它。
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS upstream_tid VARCHAR(64) NOT NULL DEFAULT '';
COMMENT ON COLUMN tickets.upstream_tid IS '上游工单号（zjmf tid）；转投上游人工处理的工单才有值，空表示纯本地工单';
CREATE INDEX IF NOT EXISTS idx_tickets_upstream_tid ON tickets (upstream_tid);

-- ---- 2. 工单回复：上游回复号（zjmf ticket_reply.id）----
-- 上游推送失败会重试（zjmf_pushhost 至多 5 次），每次 POST 内容相同。
-- 用 (ticket_id, upstream_reply_id) 部分唯一索引做精确幂等，避免同一句回复入库多条。
ALTER TABLE ticket_replies ADD COLUMN IF NOT EXISTS upstream_reply_id VARCHAR(64) NOT NULL DEFAULT '';
COMMENT ON COLUMN ticket_replies.upstream_reply_id IS '上游回复号；非空表示该回复由上游推送写入，用于重试去重';
CREATE UNIQUE INDEX IF NOT EXISTS uk_ticket_replies_upstream
    ON ticket_replies (ticket_id, upstream_reply_id) WHERE upstream_reply_id <> '';

-- ---- 3. 对账发现（差异预警的事实表）----
-- 一行 = 某实例的某一类差异。UNIQUE(instance_id, kind) 保证同类差异只留一行，
-- 重复检出只更新 last_seen_at；条件消失置 status=resolved。
--   kind：price_below_cost / price_thin_margin / expire_early / expire_late /
--         status_mismatch / status_stale
--   severity：danger（资金/账期直接受损、上游已停服而本地仍计费）/ warning（薄利、状态漂移）
--   status：open / ack（已确认，暂不处理）/ ignored（忽略）/ resolved（条件已消失）
--   notice_seq：已发出的预警通知序号。通知的幂等键是 notify.unique(source_id)，
--              重开同一差异必须换 source_id 才能再收到提醒，故每次发出即自增。
CREATE TABLE IF NOT EXISTS reconcile_findings (
    id              BIGSERIAL PRIMARY KEY,
    provider_id     BIGINT       NOT NULL DEFAULT 0,
    instance_id     BIGINT       NOT NULL,
    kind            VARCHAR(32)  NOT NULL,
    severity        VARCHAR(16)  NOT NULL,
    local_value     TEXT         NOT NULL DEFAULT '',
    upstream_value  TEXT         NOT NULL DEFAULT '',
    delta           NUMERIC(18,4),
    first_seen_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    last_seen_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    status          VARCHAR(16)  NOT NULL DEFAULT 'open',
    notice_seq      INTEGER      NOT NULL DEFAULT 0,
    notified_at     TIMESTAMPTZ,
    ack_by          BIGINT,
    ack_at          TIMESTAMPTZ,
    note            VARCHAR(255) NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
COMMENT ON TABLE reconcile_findings IS '实例对账差异的持久化台账：open 差异供预警通知与时长统计，条件消失自动 resolved';
CREATE UNIQUE INDEX IF NOT EXISTS uk_reconcile_findings_instance_kind
    ON reconcile_findings (instance_id, kind);
CREATE INDEX IF NOT EXISTS idx_reconcile_findings_open
    ON reconcile_findings (status, severity, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_reconcile_findings_provider
    ON reconcile_findings (provider_id, status);

COMMIT;

-- 验收查询（人工执行）
--   \d tickets            -- 期望含 upstream_tid
--   \d ticket_replies     -- 期望含 upstream_reply_id 与 uk_ticket_replies_upstream
--   \d reconcile_findings
