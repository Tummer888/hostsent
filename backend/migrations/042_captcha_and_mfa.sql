-- ============================================================================
-- 042_captcha_and_mfa.sql
--
-- 背景（docs/实施计划/91 验证码与二次验证体系实施文档）：
--   1) 图形验证码从「前端 canvas 假生成 + 后端不校验」改为服务端生成、服务端校验；
--   2) OTP 二次验证（邮箱/短信）落地，验证码不落明文；
--   3) 平台基线策略（captcha_policies）与用户加严设置（user_security_settings）双向解算；
--   4) 关键操作验证票据（Redis cap:vt:*）与登录失败锁定所需的 login_logs 索引。
--
-- 内容：
--   1. captcha_providers        —— 第三方/原生验证码服务商（descriptor 驱动前端表单）
--   2. captcha_policies         —— 17 个场景的平台基线策略（D4「强制」侧）
--   3. verification_codes       —— 验证码审计记录 + Redis 降级读（绝不存明文 code）
--   4. user_security_settings   —— 用户加严设置（D4「加严」侧）
--   5. admins / users 扩展列    —— 管理端 MFA 设置、用户邮箱/手机验证时间
--   6. login_logs 索引          —— 失败计数降级路径的范围扫描
--   7. system_configs 开关      —— 验证码/锁定/密码策略/注册/限流新增键
--
-- 幂等：CREATE TABLE IF NOT EXISTS / ADD COLUMN IF NOT EXISTS /
--       CREATE INDEX IF NOT EXISTS / ON CONFLICT DO NOTHING。
-- 执行前建议 pg_dump 备份。
--
-- 回滚：
--   DROP TABLE IF EXISTS user_security_settings;
--   DROP TABLE IF EXISTS verification_codes;
--   DROP TABLE IF EXISTS captcha_policies;
--   DROP TABLE IF EXISTS captcha_providers;
--   ALTER TABLE admins DROP COLUMN IF EXISTS mfa_enabled, DROP COLUMN IF EXISTS mfa_channel,
--         DROP COLUMN IF EXISTS mfa_scene_overrides;
--   ALTER TABLE users  DROP COLUMN IF EXISTS phone_verified_at, DROP COLUMN IF EXISTS email_verified_at;
--   DROP INDEX IF EXISTS idx_login_logs_created;
--   DROP INDEX IF EXISTS idx_login_logs_account_created;
--   DROP INDEX IF EXISTS idx_login_logs_ip_created;
--   DELETE FROM system_configs WHERE config_key IN (
--     'captcha_enabled','captcha_provider','captcha_image_ttl_seconds',
--     'verify_code_ttl_seconds','verify_code_send_interval_seconds','verify_code_daily_limit',
--     'verify_code_max_attempts','captcha_send_ip_hourly_limit','captcha_image_ip_minute_limit',
--     'login_fail_lock','login_fail_threshold','login_lock_minutes','mfa_required',
--     'register_enabled','api_rate_limit','password_min_length','password_max_length',
--     'password_require_upper','password_require_lower','password_require_digit','password_require_symbol');
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. captcha_providers：验证码服务商
-- ---------------------------------------------------------------------------
-- native 为内置兜底（真实现），第三方不可用时回落；credentials 为字段级加密密文
-- （internal/pkg/credentials，enc:v1: 前缀），任何接口不得回显明文。
CREATE TABLE IF NOT EXISTS captcha_providers (
    id              bigserial PRIMARY KEY,
    provider_type   varchar(50)  NOT NULL,               -- native/netease/aliyun/tencent/geetest/dingxiang
    name            varchar(100) NOT NULL,
    mode            varchar(20)  NOT NULL DEFAULT 'api', -- native / api / manual
    descriptor      jsonb,                               -- 凭证 schema（驱动前端表单）
    credentials     jsonb,                               -- 字段级加密凭证
    endpoint        varchar(255) NOT NULL DEFAULT '',
    scenes          jsonb,                               -- 启用场景
    priority        int          NOT NULL DEFAULT 0,
    health_status   varchar(20)  NOT NULL DEFAULT '',    -- ''/healthy/down/pending
    last_error      text,
    last_check_at   timestamptz,
    status          int          NOT NULL DEFAULT 1,
    is_default      boolean      NOT NULL DEFAULT false,
    remark          varchar(255) NOT NULL DEFAULT '',
    created_at      timestamptz  NOT NULL DEFAULT now(),
    updated_at      timestamptz  NOT NULL DEFAULT now(),
    deleted_at      timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_captcha_providers_default
    ON captcha_providers (provider_type) WHERE is_default = true AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_captcha_providers_status ON captcha_providers (status);

-- ---------------------------------------------------------------------------
-- 2. captcha_policies：场景策略（平台基线）
-- ---------------------------------------------------------------------------
-- 注意（doc91 §1.2.2 默认值说明）：全局总闸 captcha_enabled 默认 false，
-- 它关着时本表所有 true/false 都不生效，不阻断任何现有登录路径（默认安全上线）。
CREATE TABLE IF NOT EXISTS captcha_policies (
    id                 bigserial PRIMARY KEY,
    scene              varchar(64)  NOT NULL,
    name               varchar(100) NOT NULL,
    -- 图形验证码侧
    image_required     boolean      NOT NULL DEFAULT false,
    image_provider_id  bigint       NOT NULL DEFAULT 0,   -- 0=用默认 provider
    image_level        varchar(20)  NOT NULL DEFAULT 'normal',
    -- OTP 侧（二次验证）
    otp_required       boolean      NOT NULL DEFAULT false,
    otp_channel        varchar(20)  NOT NULL DEFAULT 'email',
    min_channel_level  int          NOT NULL DEFAULT 0,
    -- 用户自治
    user_can_tighten   boolean      NOT NULL DEFAULT true,
    user_can_choose_channel boolean NOT NULL DEFAULT true,
    -- 频控
    max_attempts       int          NOT NULL DEFAULT 5,
    ttl_seconds        int          NOT NULL DEFAULT 0,   -- 0=用全局 verify_code_ttl_seconds
    send_interval_seconds int       NOT NULL DEFAULT 0,   -- 0=用全局
    daily_limit_per_target int      NOT NULL DEFAULT 0,   -- 0=用全局
    -- 兼容旧开关
    bind_config_key    varchar(64)  NOT NULL DEFAULT '',
    status             varchar(20)  NOT NULL DEFAULT 'active',
    remark             varchar(255) NOT NULL DEFAULT '',
    created_at         timestamptz  NOT NULL DEFAULT now(),
    updated_at         timestamptz  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_captcha_policies_scene ON captcha_policies (scene);

-- 17 个场景 seed（与 db.go 的 seedCaptchaPolicies 双写；既有库升级靠这段）
INSERT INTO captcha_policies
    (scene, name, image_required, otp_required, otp_channel, min_channel_level, bind_config_key, remark)
VALUES
    ('admin_login',       '管理端登录',       false, false, 'sms',   0, '', '管理端登录图形码；默认关，建议开'),
    ('user_login',        '用户端登录',       false, false, 'email', 0, '', '密码登录图形码'),
    ('user_login_sms',    '用户端短信登录',   true,  false, 'sms',   0, '', '短信登录必有图形码（防短信轰炸）'),
    ('user_login_email',  '用户端邮箱登录',   true,  false, 'email', 0, '', '邮箱登录必有图形码（防邮件轰炸）'),
    ('user_register',     '用户注册',         true,  true,  'email', 0, '', '注册需邮箱验证码'),
    ('password_reset',    '忘记密码',         true,  true,  'email', 0, '', ''),
    ('password_change',   '修改密码',         false, true,  'sms',   0, '', '关键操作'),
    ('phone_bind',        '绑定手机',         false, true,  'sms',   0, '', '必须验短信（验的是要绑的号）'),
    ('email_bind',        '绑定邮箱',         false, true,  'email', 0, '', '必须验邮箱'),
    ('withdraw_apply',    '发起提现',         false, true,  'sms',   3, '', '资金类，最低通道等级 sms'),
    ('payout_apply',      '销售提成提现申请', false, true,  'sms',   3, '', '资金类（doc86）'),
    ('apikey_create',     '创建 API 密钥',    false, true,  'sms',   0, '', '开放平台凭据类'),
    ('apikey_view',       '查看 API 密钥明文',false, true,  'sms',   0, '', '凭据类'),
    ('instance_destroy',  '销毁实例',         false, true,  'sms',   0, '', '不可逆'),
    ('instance_resize',   '实例变配',         false, false, 'sms',   0, '', '可逆但影响服务'),
    ('admin_grant_change','变更员工权限',     false, true,  'sms',   0, '', '管理端高危'),
    ('realname_submit',   '提交实名认证',     true,  false, 'email', 0, '', '')
ON CONFLICT (scene) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 3. verification_codes：验证码审计 + 降级读
-- ---------------------------------------------------------------------------
-- 安全设计（关键）：
--   * 不存明文 code：降级校验用 sha256(input + code_salt) = code_hash 比对；
--   * target 存明文但频控走 target_hash，日志展示时打码（138****8888）；
--   * captcha_key 只用于关联 Redis 答案行，本身不携带答案信息。
CREATE TABLE IF NOT EXISTS verification_codes (
    id           bigserial PRIMARY KEY,
    scene        varchar(64)  NOT NULL,
    channel      varchar(20)  NOT NULL,               -- image/email/sms/totp
    target       varchar(191) NOT NULL DEFAULT '',    -- 手机号/邮箱；图形码为空
    target_hash  varchar(64)  NOT NULL DEFAULT '',    -- sha256(target+salt)
    code_hash    varchar(128) NOT NULL,               -- sha256(code + per-row salt)
    code_salt    varchar(32)  NOT NULL,
    captcha_key  varchar(64)  NOT NULL DEFAULT '',
    status       varchar(20)  NOT NULL DEFAULT 'pending', -- pending/used/expired/failed
    attempts     int          NOT NULL DEFAULT 0,
    max_attempts int          NOT NULL DEFAULT 5,
    provider_id  bigint       NOT NULL DEFAULT 0,
    provider_msg_id varchar(128) NOT NULL DEFAULT '',
    cost_fen     int          NOT NULL DEFAULT 0,
    request_ip   varchar(64)  NOT NULL DEFAULT '',
    user_agent   varchar(255) NOT NULL DEFAULT '',
    operator_id  bigint       NOT NULL DEFAULT 0,
    expire_at    timestamptz  NOT NULL,
    used_at      timestamptz,
    created_at   timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_verification_codes_created ON verification_codes (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_verification_codes_verify
    ON verification_codes (scene, target_hash, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_verification_codes_status ON verification_codes (status, expire_at)
    WHERE status = 'pending';

-- ---------------------------------------------------------------------------
-- 4. user_security_settings：用户加严设置
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS user_security_settings (
    id                  bigserial PRIMARY KEY,
    user_id             bigint      NOT NULL,
    mfa_enabled         boolean     NOT NULL DEFAULT false,
    mfa_channel         varchar(20) NOT NULL DEFAULT '',      -- email/sms/totp
    -- 场景级加严：{"withdraw_apply":{"otp_required":true,"otp_channel":"sms"}, ...}
    scene_overrides     jsonb,
    trust_window_minutes int        NOT NULL DEFAULT 0,
    updated_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_user_security_settings_user ON user_security_settings (user_id);

-- ---------------------------------------------------------------------------
-- 5. admins / users 扩展
-- ---------------------------------------------------------------------------
ALTER TABLE admins
  ADD COLUMN IF NOT EXISTS mfa_enabled boolean     NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS mfa_channel varchar(20) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS mfa_scene_overrides jsonb;

-- 绑定状态：用于「已绑手机/邮箱」展示与 OTP 目标选择。
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS phone_verified_at timestamptz,
  ADD COLUMN IF NOT EXISTS email_verified_at timestamptz;

-- ---------------------------------------------------------------------------
-- 6. login_logs 索引（C6 真实落库后的降级路径）
-- ---------------------------------------------------------------------------
-- 失败计数降级路径按 (账号/目标, 时间) 范围扫，无索引会每次登录全表扫描。
CREATE INDEX IF NOT EXISTS idx_login_logs_created ON login_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_login_logs_account_created
    ON login_logs (username, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_login_logs_ip_created
    ON login_logs (ip, created_at DESC);

-- ---------------------------------------------------------------------------
-- 7. 新增配置开关（doc91 §9.1 存量 + §9.2 新增）
-- ---------------------------------------------------------------------------
-- 默认值全部取「默认安全侧」：captcha_enabled=false 使升级后登录行为不变，
-- login_fail_lock=false 使失败锁定行为与升级前一致。
-- 存量键（enable_mfa_required / enable_user_register）保留不动，读取端做兼容回落。
INSERT INTO system_configs (config_key, config_value, value_type, config_group, description, sort_order, status)
VALUES
    ('captcha_enabled',                 'false', 'bool',   'security', '验证码总闸（关闭时所有场景策略不生效）', 20, 'active'),
    ('captcha_provider',                'native','string', 'security', '图形验证码兜底服务商类型',             21, 'active'),
    ('captcha_image_ttl_seconds',       '120',   'int',    'security', '图形验证码有效期（秒）',               22, 'active'),
    ('verify_code_ttl_seconds',         '300',   'int',    'security', '验证码有效期（秒）',                   23, 'active'),
    ('verify_code_send_interval_seconds','60',   'int',    'security', '验证码发送间隔（秒）',                 24, 'active'),
    ('verify_code_daily_limit',         '10',    'int',    'security', '单目标每日验证码发送上限',             25, 'active'),
    ('verify_code_max_attempts',        '5',     'int',    'security', '验证码单次校验次数上限',               26, 'active'),
    ('captcha_send_ip_hourly_limit',    '20',    'int',    'security', '单 IP 每小时验证码发送上限',           27, 'active'),
    ('captcha_image_ip_minute_limit',   '30',    'int',    'security', '单 IP 每分钟图形码获取上限',           28, 'active'),
    ('login_fail_lock',                 'false', 'bool',   'security', '登录失败是否锁定账号',                 29, 'active'),
    ('login_fail_threshold',            '5',     'int',    'security', '登录失败锁定阈值（次）',               30, 'active'),
    ('login_lock_minutes',              '15',    'int',    'security', '登录锁定时长（分钟）',                 31, 'active'),
    ('mfa_required',                    'false', 'bool',   'security', '是否强制所有场景二次验证（全局提升）', 32, 'active'),
    ('register_enabled',                'true',  'bool',   'feature',  '是否开放用户注册',                     2,  'active'),
    ('api_rate_limit',                  '0',     'int',    'security', 'API 每分钟请求上限（0=不限）',         33, 'active'),
    ('password_min_length',             '6',     'int',    'security', '密码最小长度',                         34, 'active'),
    ('password_max_length',             '64',    'int',    'security', '密码最大长度',                         35, 'active'),
    ('password_require_upper',          'false', 'bool',   'security', '密码是否要求大写字母',                 36, 'active'),
    ('password_require_lower',          'false', 'bool',   'security', '密码是否要求小写字母',                 37, 'active'),
    ('password_require_digit',          'false', 'bool',   'security', '密码是否要求数字',                     38, 'active'),
    ('password_require_symbol',         'false', 'bool',   'security', '密码是否要求特殊字符',                 39, 'active')
ON CONFLICT (config_key) DO NOTHING;

COMMIT;
