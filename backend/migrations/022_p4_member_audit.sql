-- 022_p4_member_audit.sql
-- 账号体系与权限分级重构 P4：子账号操作审计日志
-- 依据：docs/实施计划/82-账号体系与权限分级重构执行清单.md P4-08
--
-- 注意（R1 schema 双写）：本文件仅作版本留痕，运行时不执行；
-- 实际建表由 internal/pkg/db/db.go 的 AutoMigrate 完成。
-- 对应 Go model：uc/member/model/member.go（OperationLog）
--
-- 与 admin_audit_logs 的区别：本表是「客户侧」维度，actor 是真实操作人（可能是子账号），
-- account_user_id 是数据归属主账号，主账号按归属查看成员做了什么。
--
-- 幂等：CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS。

BEGIN;

CREATE TABLE IF NOT EXISTS user_operation_logs (
    id              BIGSERIAL PRIMARY KEY,
    actor_user_id   BIGINT       NOT NULL,
    actor_name      VARCHAR(64)  NOT NULL DEFAULT '',
    account_user_id BIGINT       NOT NULL,
    module          VARCHAR(32)  NOT NULL,
    action          VARCHAR(32)  NOT NULL,
    target          VARCHAR(128),
    detail          VARCHAR(255),
    ip              VARCHAR(64),
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_user_op_logs_actor ON user_operation_logs (actor_user_id);
CREATE INDEX IF NOT EXISTS idx_user_op_logs_account ON user_operation_logs (account_user_id);

COMMIT;
