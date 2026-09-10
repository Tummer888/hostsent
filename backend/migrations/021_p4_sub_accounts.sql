-- 021_p4_sub_accounts.sql
-- 账号体系与权限分级重构 P4：子账号（成员）
-- 依据：docs/实施计划/82-账号体系与权限分级重构执行清单.md P4-01
--       docs/实施计划/81-账号体系与权限分级重构设计.md §4.6
--
-- 注意（R1 schema 双写）：本文件仅作版本留痕，运行时不执行；
-- 实际建列/建表由 internal/pkg/db/db.go 的 AutoMigrate 完成。
-- 对应 Go model：admin/user/account/model/user.go、uc/auth/model/user.go、
--               admin/user/account/model/sub_account_permission.go
--
-- 幂等：ADD COLUMN IF NOT EXISTS / CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS。

BEGIN;

-- users：子账号归属与标识
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS owner_user_id BIGINT,
  ADD COLUMN IF NOT EXISTS is_sub_account BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS sub_account_remark VARCHAR(64);
CREATE INDEX IF NOT EXISTS idx_users_owner ON users (owner_user_id);

-- 子账号权限授予（覆盖式设置）
CREATE TABLE IF NOT EXISTS sub_account_permissions (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT      NOT NULL,
    permission_code VARCHAR(64) NOT NULL,
    created_at      TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_sub_account_perms ON sub_account_permissions (user_id, permission_code);

COMMIT;
