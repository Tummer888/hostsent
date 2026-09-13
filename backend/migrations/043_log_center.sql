-- ============================================================================
-- 043_log_center.sql
--
-- 背景（docs/实施计划/92 日志中心与统一日志清理实施文档）：
--   1) 26 类日志源分散在 24 张既有表中，无统一查询/导出/清理入口；
--   2) 唯一既有清理先例（sync/service/scheduler.go purgeOnce）保留期硬编码 30 天、
--      无导出备份、不留 DB 记录，本模块将其收编；
--   3) 上游厂商 API 调用与定时任务执行此前无落库留痕，排障只能翻容器日志。
--
-- 内容：
--   1. upstream_api_logs     —— 上游厂商接口调用日志（摘要恒写，正文可采样/截断）
--   2. job_run_logs          —— 统一定时任务执行日志（含重入保护唯一索引）
--   3. log_retention_policies—— 逐源保留策略（clean/archive_only/keep）
--   4. log_export_files      —— 导出文件元数据（sha256 校验、权限留痕）
--   5. log_cleanup_jobs      —— 清理任务（状态机 + 全局互斥部分唯一索引）
--   6. 10 个时间列索引补齐   —— 删除/查询走 Index Scan
--
-- 注意：下面的 CREATE INDEX 非 CONCURRENTLY，会短暂持有表锁。当前各表最大约
--       1300 行（实测），锁表时间可忽略。若在生产库上执行时数据量已增长，
--       请改为 CREATE INDEX CONCURRENTLY 并在迁移之外手工执行
--       （CONCURRENTLY 不能在事务块内运行）。
--
-- 幂等：CREATE TABLE/INDEX IF NOT EXISTS。
-- 执行前建议 pg_dump 备份。
--
-- 回滚：
--   DROP TABLE IF EXISTS log_cleanup_jobs;
--   DROP TABLE IF EXISTS log_export_files;
--   DROP TABLE IF EXISTS log_retention_policies;
--   DROP TABLE IF EXISTS job_run_logs;
--   DROP TABLE IF EXISTS upstream_api_logs;
--   DROP INDEX IF EXISTS idx_login_logs_created;
--   DROP INDEX IF EXISTS idx_audit_logs_created;
--   DROP INDEX IF EXISTS idx_user_operation_logs_created;
--   DROP INDEX IF EXISTS idx_product_history_created;
--   DROP INDEX IF EXISTS idx_user_level_change_logs_created;
--   DROP INDEX IF EXISTS idx_payment_callback_logs_created;
--   DROP INDEX IF EXISTS idx_payment_recon_records_created;
--   DROP INDEX IF EXISTS idx_risk_events_created;
--   DROP INDEX IF EXISTS idx_verification_review_logs_created;
--   DROP INDEX IF EXISTS idx_notification_reads_read_at;
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. upstream_api_logs：上游厂商 API 日志（D5）
-- ---------------------------------------------------------------------------
-- request_digest/response_digest 恒写（sha256 全量摘要），正文受
-- system_configs.log_upstream_sample_rate 与 log_upstream_body_max_bytes 控制，
-- 可截断可采样：即使关闭正文采集，仍能用摘要比对「当时上游返回了什么」。
-- success 为独立列（不靠 status_code 推断）：上游常用 200 + body.error 表达失败。
CREATE TABLE IF NOT EXISTS upstream_api_logs (
    id                 BIGSERIAL PRIMARY KEY,
    provider_id        bigint       NOT NULL DEFAULT 0,
    provider_name      varchar(100) NOT NULL DEFAULT '',
    provider_type      varchar(50)  NOT NULL DEFAULT '',
    op                 varchar(64)  NOT NULL DEFAULT '',
    method             varchar(8)   NOT NULL DEFAULT '',
    url                varchar(500) NOT NULL DEFAULT '',
    request_digest     varchar(64)  NOT NULL DEFAULT '',
    request_body       text,
    request_bytes      int          NOT NULL DEFAULT 0,
    request_truncated  boolean      NOT NULL DEFAULT false,
    status_code        int          NOT NULL DEFAULT 0,
    response_digest    varchar(64)  NOT NULL DEFAULT '',
    response_body      text,
    response_bytes     int          NOT NULL DEFAULT 0,
    response_truncated boolean      NOT NULL DEFAULT false,
    success            boolean      NOT NULL DEFAULT false,
    error_code         varchar(64)  NOT NULL DEFAULT '',
    error_message      varchar(500) NOT NULL DEFAULT '',
    duration_ms        int          NOT NULL DEFAULT 0,
    retry_index        int          NOT NULL DEFAULT 0,
    trace_id           varchar(64)  NOT NULL DEFAULT '',
    created_at         timestamptz
);
CREATE INDEX IF NOT EXISTS idx_upstream_api_logs_created ON upstream_api_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_upstream_api_logs_provider ON upstream_api_logs (provider_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_upstream_api_logs_op ON upstream_api_logs (op, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_upstream_api_logs_failed
    ON upstream_api_logs (created_at DESC) WHERE success = false;

-- ---------------------------------------------------------------------------
-- 2. job_run_logs：统一定时任务日志（D6）
-- ---------------------------------------------------------------------------
-- uk_job_run_logs_running 是并发保护：time.NewTicker 的调度器若某轮执行超过间隔
-- 会重入。插入 running 行时若唯一索引冲突 → 说明上一轮未结束，本轮跳过并打 warn。
CREATE TABLE IF NOT EXISTS job_run_logs (
    id             BIGSERIAL PRIMARY KEY,
    job_name       varchar(64)  NOT NULL,
    job_group      varchar(32)  NOT NULL DEFAULT '',
    trigger_type   varchar(20)  NOT NULL DEFAULT 'ticker',
    status         varchar(20)  NOT NULL DEFAULT 'running',
    started_at     timestamptz  NOT NULL,
    finished_at    timestamptz,
    duration_ms    int          NOT NULL DEFAULT 0,
    items_scanned  int          NOT NULL DEFAULT 0,
    items_affected int          NOT NULL DEFAULT 0,
    summary        jsonb,
    error_message  varchar(1000) NOT NULL DEFAULT '',
    created_at     timestamptz
);
CREATE INDEX IF NOT EXISTS idx_job_run_logs_created ON job_run_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_job_run_logs_job ON job_run_logs (job_name, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_job_run_logs_failed
    ON job_run_logs (created_at DESC) WHERE status = 'failed';
CREATE UNIQUE INDEX IF NOT EXISTS uk_job_run_logs_running
    ON job_run_logs (job_name) WHERE status = 'running';

-- ---------------------------------------------------------------------------
-- 3. log_retention_policies：逐源保留策略
-- ---------------------------------------------------------------------------
-- source_key 与代码常量注册表（logcenter/catalog）一一对应；action=clean 仅允许
-- 出现在 Cleanable=true 的源上（API 与执行期双重校验）。
CREATE TABLE IF NOT EXISTS log_retention_policies (
    id             BIGSERIAL PRIMARY KEY,
    source_key     varchar(64)  NOT NULL,
    display_name   varchar(100) NOT NULL,
    action         varchar(20)  NOT NULL DEFAULT 'clean',
    retention_days int          NOT NULL DEFAULT 180,
    archive_cron   varchar(32)  NOT NULL DEFAULT '',
    batch_size     int          NOT NULL DEFAULT 5000,
    enabled        boolean      NOT NULL DEFAULT true,
    last_run_at    timestamptz,
    last_deleted   bigint       NOT NULL DEFAULT 0,
    updated_by     bigint       NOT NULL DEFAULT 0,
    remark         varchar(255) NOT NULL DEFAULT '',
    created_at     timestamptz,
    updated_at     timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_log_retention_policies_source ON log_retention_policies (source_key);

-- ---------------------------------------------------------------------------
-- 4. log_export_files：导出文件元数据
-- ---------------------------------------------------------------------------
-- 行永不清理（清理任务产生的备份是「删了哪些数据」的唯一凭据，只改 status）。
CREATE TABLE IF NOT EXISTS log_export_files (
    id            BIGSERIAL PRIMARY KEY,
    source_key    varchar(64)  NOT NULL,
    job_id        bigint       NOT NULL DEFAULT 0,
    format        varchar(10)  NOT NULL DEFAULT 'csv',
    storage_path  varchar(500) NOT NULL,
    file_name     varchar(255) NOT NULL,
    file_size     bigint       NOT NULL DEFAULT 0,
    row_count     bigint       NOT NULL DEFAULT 0,
    sha256        varchar(64)  NOT NULL DEFAULT '',
    period_from   timestamptz,
    period_to     timestamptz,
    filters       jsonb,
    operator_id   bigint       NOT NULL DEFAULT 0,
    operator_name varchar(64)  NOT NULL DEFAULT '',
    status        varchar(20)  NOT NULL DEFAULT 'success',
    fail_reason   varchar(500) NOT NULL DEFAULT '',
    expires_at    timestamptz,
    created_at    timestamptz,
    updated_at    timestamptz
);
CREATE INDEX IF NOT EXISTS idx_log_export_files_created ON log_export_files (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_log_export_files_source ON log_export_files (source_key, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_log_export_files_job ON log_export_files (job_id) WHERE job_id <> 0;

-- ---------------------------------------------------------------------------
-- 5. log_cleanup_jobs：清理任务
-- ---------------------------------------------------------------------------
-- uk_log_cleanup_jobs_active 用部分唯一索引 ((1)) 实现「全局至多一个在跑的任务」：
-- Redis 可用时另加分布式锁（多实例），Redis 不可用时这条索引就是兜底（doc89 §3.3）。
CREATE TABLE IF NOT EXISTS log_cleanup_jobs (
    id              BIGSERIAL PRIMARY KEY,
    job_no          varchar(40) NOT NULL,
    trigger_type    varchar(20) NOT NULL DEFAULT 'manual',
    status          varchar(20) NOT NULL DEFAULT 'pending',
    sources         jsonb,
    before_time     timestamptz NOT NULL,
    dry_run         boolean     NOT NULL DEFAULT false,
    export_required boolean     NOT NULL DEFAULT true,
    export_file_ids jsonb,
    scanned_rows    bigint      NOT NULL DEFAULT 0,
    deleted_rows    bigint      NOT NULL DEFAULT 0,
    expired_rows    bigint      NOT NULL DEFAULT 0,
    progress        int         NOT NULL DEFAULT 0,
    current_source  varchar(64) NOT NULL DEFAULT '',
    error_message   varchar(1000) NOT NULL DEFAULT '',
    operator_id     bigint      NOT NULL DEFAULT 0,
    operator_name   varchar(64) NOT NULL DEFAULT '',
    started_at      timestamptz,
    finished_at     timestamptz,
    created_at      timestamptz,
    updated_at      timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_log_cleanup_jobs_no ON log_cleanup_jobs (job_no);
CREATE INDEX IF NOT EXISTS idx_log_cleanup_jobs_created ON log_cleanup_jobs (created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uk_log_cleanup_jobs_active
    ON log_cleanup_jobs ((1)) WHERE status IN ('pending','exporting','verifying','deleting');

-- ---------------------------------------------------------------------------
-- 6. 索引补齐（L7）：10 个时间列
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_login_logs_created        ON login_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created        ON audit_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_operation_logs_created ON user_operation_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_product_history_created   ON product_history (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_level_change_logs_created ON user_level_change_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payment_callback_logs_created ON payment_callback_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payment_recon_records_created ON payment_recon_records (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_risk_events_created       ON risk_events (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_verification_review_logs_created ON verification_review_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notification_reads_read_at ON notification_reads (read_at DESC);

-- ---------------------------------------------------------------------------
-- 7. 逐源保留策略 seed（doc92 §4.2/§4.3）
-- ---------------------------------------------------------------------------
-- 14 个 ops 源可清理（notify_inbox/notify_read 例外为 365 天，D12），
-- 12 个 audit 源只备份不删（archive_only + monthly 归档节奏）。
-- 与 db.go 的 seedLogRetentionPolicies 同口径双写；ON CONFLICT DO NOTHING
-- 保证运营改过的保留期不被重启/重跑重置。
INSERT INTO log_retention_policies
    (source_key, display_name, action, retention_days, archive_cron, batch_size, enabled)
VALUES
    ('admin_audit',         '管理端操作审计',   'archive_only', 180, 'monthly', 5000, true),
    ('user_audit',          '用户操作审计',     'archive_only', 180, 'monthly', 5000, true),
    ('security_audit',      '安全审计日志',     'archive_only', 180, 'monthly', 5000, true),
    ('login',               '登录日志',         'archive_only', 180, 'monthly', 5000, true),
    ('ticket',              '工单操作日志',     'archive_only', 180, 'monthly', 5000, true),
    ('instance_ops',        '实例操作流水',     'archive_only', 180, 'monthly', 5000, true),
    ('verification_review', '实名审核日志',     'archive_only', 180, 'monthly', 5000, true),
    ('level_change',        '用户等级变更',     'archive_only', 180, 'monthly', 5000, true),
    ('payment_callback',    '支付回调日志',     'archive_only', 180, 'monthly', 5000, true),
    ('payment_recon',       '支付对账记录',     'archive_only', 180, 'monthly', 5000, true),
    ('risk_event',          '风控事件',         'archive_only', 180, 'monthly', 5000, true),
    ('product_history',     '商品变更历史',     'archive_only', 180, 'monthly', 5000, true),
    ('notify_inbox',        '站内信记录',       'clean',        365, '',        5000, true),
    ('notify_delivery',     '通知投递日志',     'clean',        180, '',        5000, true),
    ('notify_read',         '通知已读记录',     'clean',        365, '',        5000, true),
    ('verify_code',         '验证码记录',       'clean',        180, '',        5000, true),
    ('sync_log',            '同步日志',         'clean',        180, '',        5000, true),
    ('sync_task',           '同步任务',         'clean',        180, '',        5000, true),
    ('sync_diff',           '同步差异',         'clean',        180, '',        5000, true),
    ('price_change',        '价格变更事件',     'clean',        180, '',        5000, true),
    ('provision_task',      '开通任务',         'clean',        180, '',        5000, true),
    ('open_api',            '开放平台调用日志', 'clean',        180, '',        5000, true),
    ('open_request',        '开放平台幂等请求', 'clean',        180, '',        5000, true),
    ('open_notify',         '开放平台回调投递', 'clean',        180, '',        5000, true),
    ('upstream_api',        '上游厂商接口日志', 'clean',        180, '',        5000, true),
    ('job_run',             '定时任务日志',     'clean',        180, '',        5000, true)
ON CONFLICT (source_key) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 8. 新增配置开关（doc89 §9.1 日志部分）
-- ---------------------------------------------------------------------------
-- log_cleanup_enabled 默认 true：清理永远先导出后删除（log_export_before_delete
-- 另有一层互斥校验），不存在「无备份被删」的情形。
-- log_upstream_capture 默认 true + sample_rate=100：先有可观测性，按实际量调采样。
INSERT INTO system_configs (config_key, config_value, value_type, config_group, description, sort_order, status)
VALUES
    ('log_retention_days',          '180',  'int',  'log', '日志全局保留天数（硬地板 7 天）',             1, 'active'),
    ('log_cleanup_enabled',         'true', 'bool', 'log', '启用日志自动清理调度',                       2, 'active'),
    ('log_cleanup_hour',            '3',    'int',  'log', '日志清理执行小时（0-23，当日只跑一次）',      3, 'active'),
    ('log_export_before_delete',    'true', 'bool', 'log', '删除前必须导出备份（关闭时清理接口拒绝执行）', 4, 'active'),
    ('log_upstream_capture',        'true', 'bool', 'log', '采集上游厂商接口日志',                       5, 'active'),
    ('log_upstream_body_max_bytes', '4096', 'int',  'log', '上游正文采集上限（字节，超出截断）',          6, 'active'),
    ('log_upstream_sample_rate',    '100',  'int',  'log', '上游日志采样率（0-100，100=全量）',           7, 'active'),
    ('log_export_retention_days',   '365',  'int',  'log', '导出文件自身保留天数',                       8, 'active')
ON CONFLICT (config_key) DO NOTHING;

COMMIT;
