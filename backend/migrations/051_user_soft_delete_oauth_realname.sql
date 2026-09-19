-- ============================================================================
-- 051_user_soft_delete_oauth_realname.sql
-- 背景（doc104 §4 / §5 / §6）：
--   1) 用户注销落地：users 此前没有任何删除标记，UserService.Delete 是硬删除死代码。
--      本迁移加软删除列 + 把 username/email 的普通唯一索引改成「仅未注销行唯一」的
--      部分唯一索引，使注销用户的账号名可被重新注册（doc104 §4.2）。
--   2) 实名认证补链：verification_applications 加证件号/手机号密文列与三方核验列；
--      users 加 real_name_verified_at 作为实名状态的唯一信任信号，堵掉
--      「改一次昵称即被判定为已实名」的后门（doc104 §5.3，F13）。
--   3) 第三方登录：新建 user_oauth_bindings（绑定关系）+ oauth_providers（渠道配置），
--      形态逐列对齐既有 captcha_providers（凭证 JSON + 字段级加密）。
--   4) 实名 provider：新建 realname_providers，同上形态。
--   5) 孤儿数据修复：3 行 verification_applications 的 user_id 全为 0（种子按
--      已不存在的演示用户名查找用户），按 username 尝试回绑；仍无法回绑的
--      打 risk_flags 标记保留，不删除（doc104 §7）。
--
-- 幂等：全部 IF NOT EXISTS / 列级 IF NOT EXISTS；索引重建前先探测重复行，
--       重复即 RAISE EXCEPTION 中止（绝不静默删数据）。
-- 顺序：本迁移先跑，再重启后端（AutoMigrate 与 seed 会把模型/菜单/配置对齐到代码）。
-- 回滚：DROP 新增列与新增表即可；被替换的唯一索引需按下方注释手工恢复为普通唯一索引
--       （注意：若已有注销用户占用同名账号，恢复会失败——这是软删除的必然代价）。
-- ============================================================================
BEGIN;

-- ---- 1. users：软删除列 + 实名信任信号列 ----
ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at            timestamptz;
ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_by            bigint       NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS delete_reason         varchar(255) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS status_before_delete  varchar(32)  NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS real_name_verified_at timestamptz;
ALTER TABLE users ADD COLUMN IF NOT EXISTS real_name_verified_source varchar(32) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users (deleted_at);

-- ---- 2. username / email 唯一索引改造为部分唯一索引 ----
-- 先探测重复：若 (username) 在未注销行中已有重复，说明存量数据本身违反新约束，
-- 必须人工介入，不能靠 DROP INDEX 掩盖。
DO $$
DECLARE
    dup_username int;
    dup_email    int;
BEGIN
    SELECT count(*) INTO dup_username FROM (
        SELECT username FROM users WHERE deleted_at IS NULL GROUP BY username HAVING count(*) > 1
    ) t;
    IF dup_username > 0 THEN
        RAISE EXCEPTION 'users.username 存在 % 组重复（未注销行），无法建立部分唯一索引，请先人工去重', dup_username;
    END IF;

    SELECT count(*) INTO dup_email FROM (
        SELECT email FROM users WHERE deleted_at IS NULL GROUP BY email HAVING count(*) > 1
    ) t;
    IF dup_email > 0 THEN
        RAISE EXCEPTION 'users.email 存在 % 组重复（未注销行），无法建立部分唯一索引，请先人工去重', dup_email;
    END IF;
END $$;

DROP INDEX IF EXISTS idx_users_username;
DROP INDEX IF EXISTS idx_users_email;
CREATE UNIQUE INDEX IF NOT EXISTS uk_users_username_active ON users (username) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_users_email_active    ON users (email)    WHERE deleted_at IS NULL;

-- ---- 3. verification_applications：证件密文列 + 三方核验列 ----
ALTER TABLE verification_applications ADD COLUMN IF NOT EXISTS id_number_encrypted text         NOT NULL DEFAULT '';
ALTER TABLE verification_applications ADD COLUMN IF NOT EXISTS mobile_encrypted    text         NOT NULL DEFAULT '';
ALTER TABLE verification_applications ADD COLUMN IF NOT EXISTS submitted_by        bigint       NOT NULL DEFAULT 0;
ALTER TABLE verification_applications ADD COLUMN IF NOT EXISTS provider            varchar(32)  NOT NULL DEFAULT '';
ALTER TABLE verification_applications ADD COLUMN IF NOT EXISTS provider_txn_no     varchar(128) NOT NULL DEFAULT '';
ALTER TABLE verification_applications ADD COLUMN IF NOT EXISTS provider_result     varchar(32)  NOT NULL DEFAULT '';
ALTER TABLE verification_applications ADD COLUMN IF NOT EXISTS provider_message    varchar(255) NOT NULL DEFAULT '';
ALTER TABLE verification_applications ADD COLUMN IF NOT EXISTS provider_checked_at timestamptz;
ALTER TABLE verification_applications ADD COLUMN IF NOT EXISTS review_round        int          NOT NULL DEFAULT 1;

-- 冷却期判定与「同一用户最近一次申请」查询走 (user_id, submitted_at) 复合索引。
CREATE INDEX IF NOT EXISTS idx_verification_applications_user_submitted
    ON verification_applications (user_id, submitted_at DESC);

-- ---- 4. 第三方登录：绑定关系表 ----
CREATE TABLE IF NOT EXISTS user_oauth_bindings (
    id             bigserial    PRIMARY KEY,
    user_id        bigint       NOT NULL,
    provider       varchar(32)  NOT NULL,
    openid         varchar(128) NOT NULL,
    unionid        varchar(128) NOT NULL DEFAULT '',
    nickname       varchar(64)  NOT NULL DEFAULT '',
    avatar         varchar(255) NOT NULL DEFAULT '',
    status         varchar(16)  NOT NULL DEFAULT 'active',
    bound_at       timestamptz  NOT NULL DEFAULT now(),
    last_login_at  timestamptz,
    created_at     timestamptz,
    updated_at     timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_user_oauth_bindings_provider_openid
    ON user_oauth_bindings (provider, openid);
CREATE INDEX IF NOT EXISTS idx_user_oauth_bindings_user ON user_oauth_bindings (user_id);

-- ---- 5. 第三方登录：渠道配置表（形态对齐 captcha_providers）----
CREATE TABLE IF NOT EXISTS oauth_providers (
    id            bigserial    PRIMARY KEY,
    provider      varchar(32)  NOT NULL,
    name          varchar(100) NOT NULL,
    enabled       boolean      NOT NULL DEFAULT false,
    mode          varchar(20)  NOT NULL DEFAULT 'api',
    descriptor    jsonb,
    credentials   jsonb,
    scopes        varchar(255) NOT NULL DEFAULT '',
    icon          varchar(64)  NOT NULL DEFAULT '',
    sort_order    bigint       NOT NULL DEFAULT 0,
    health_status varchar(20)  NOT NULL DEFAULT '',
    last_error    text,
    last_check_at timestamptz,
    remark        varchar(255) NOT NULL DEFAULT '',
    created_at    timestamptz,
    updated_at    timestamptz,
    deleted_at    timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_oauth_providers_provider
    ON oauth_providers (provider) WHERE deleted_at IS NULL;

-- ---- 6. 实名核验：服务商配置表（形态对齐 captcha_providers）----
CREATE TABLE IF NOT EXISTS realname_providers (
    id            bigserial    PRIMARY KEY,
    provider_type varchar(50)  NOT NULL,
    name          varchar(100) NOT NULL,
    mode          varchar(20)  NOT NULL DEFAULT 'manual',
    descriptor    jsonb,
    credentials   jsonb,
    endpoint      varchar(255) NOT NULL DEFAULT '',
    priority      bigint       NOT NULL DEFAULT 0,
    health_status varchar(20)  NOT NULL DEFAULT '',
    last_error    text,
    last_check_at timestamptz,
    status        int          NOT NULL DEFAULT 1,
    is_default    boolean      NOT NULL DEFAULT false,
    remark        varchar(255) NOT NULL DEFAULT '',
    created_at    timestamptz,
    updated_at    timestamptz,
    deleted_at    timestamptz
);
CREATE INDEX IF NOT EXISTS idx_realname_providers_type ON realname_providers (provider_type);
-- 同类型只允许一个默认 provider（对齐 captcha_providers 的口径）。
CREATE UNIQUE INDEX IF NOT EXISTS uk_realname_providers_default_per_type
    ON realname_providers (provider_type) WHERE is_default = true AND deleted_at IS NULL;

-- ---- 7. 孤儿实名数据修复 ----
-- 7.1 按 username 回绑到现存用户。
UPDATE verification_applications va
SET user_id = u.id
FROM users u
WHERE va.user_id = 0 AND u.username = va.username;

-- 7.2 仍无法回绑的（演示种子遗留）打标记保留：宁可让管理端显示「孤儿数据」提示，
--     也不静默删除——删除会让待审核列表凭空少记录，反而难以解释。
--
-- 注意 WHERE 里的 `risk_flags IS NULL` 分支不可省：`NULL NOT LIKE '%x%'` 求值为
-- NULL 而不是 TRUE，只写 NOT LIKE 会把 risk_flags 为空的孤儿行全部漏掉 ——
-- 而那恰恰是最常见的情况（种子数据不写这一列）。
UPDATE verification_applications
SET risk_flags = CASE
        WHEN risk_flags IS NULL OR risk_flags = '' THEN 'orphan_seed'
        ELSE risk_flags || ',orphan_seed'
    END
WHERE user_id = 0
  AND (risk_flags IS NULL OR risk_flags NOT LIKE '%orphan_seed%');

COMMIT;
