-- 019_p2_ticket_workflow.sql
-- 账号体系与权限分级重构 P2：工单流转（日志/自动派单/认领/转派/SLA）+ 管理端操作审计落库
-- 依据：docs/实施计划/82-账号体系与权限分级重构执行清单.md P2-02/P2-03/P2-05/P2-06
--
-- 注意（R1 schema 双写）：本文件仅作版本留痕，运行时不执行；
-- 实际建表/建列由 internal/pkg/db/db.go 的 AutoMigrate 完成。
-- 对应 Go model：ticket/model/ticket_log.go、ticket/model/ticket_category.go、
--               manager/model/admin.go（AdminAuditLog）

-- 工单操作日志（时间线）
CREATE TABLE IF NOT EXISTS ticket_logs (
    id            BIGSERIAL PRIMARY KEY,
    ticket_id     BIGINT NOT NULL,
    operator_id   BIGINT NOT NULL DEFAULT 0,
    operator_name VARCHAR(64) NOT NULL DEFAULT '',
    action        VARCHAR(32) NOT NULL,   -- create/assign/claim/transfer/reply/status/close/cancel
    from_value    VARCHAR(128),
    to_value      VARCHAR(128),
    note          VARCHAR(255),
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_ticket_logs_ticket ON ticket_logs (ticket_id);
CREATE INDEX IF NOT EXISTS idx_ticket_logs_created ON ticket_logs (created_at);

-- 分类派单策略与 SLA
ALTER TABLE ticket_categories
  ADD COLUMN IF NOT EXISTS default_role_code VARCHAR(64),
  ADD COLUMN IF NOT EXISTS default_group_id BIGINT,
  ADD COLUMN IF NOT EXISTS sla_hours INT NOT NULL DEFAULT 0;

-- 管理端操作审计：补齐中间件上报字段（原表仅有 detail/IP/UA）
ALTER TABLE admin_audit_logs
  ADD COLUMN IF NOT EXISTS module VARCHAR(64),
  ADD COLUMN IF NOT EXISTS request_method VARCHAR(16),
  ADD COLUMN IF NOT EXISTS request_path VARCHAR(255),
  ADD COLUMN IF NOT EXISTS response_code INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS trace_id VARCHAR(64);
CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_created ON admin_audit_logs (created_at);
