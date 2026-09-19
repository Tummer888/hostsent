-- ============================================================================
-- 052_oauth_realname_seeds.sql
-- 背景（doc104 §7）：迁移 051 建好了表与列，但库里还没有任何可用的默认数据，
--   运营打开「系统管理 → 第三方登录」「实名认证 → 认证配置」会看到空白页。
--   本迁移把三样东西铺上：
--     1) oauth_providers：微信/QQ/支付宝三家渠道（enabled=false，等运营填凭证）；
--     2) realname_providers：manual 为内置默认（启用、is_default），alipay 待配置（停用）；
--     3) verification_configs：8 条实名策略默认值；
--     4) system_configs：oauth.callback_base / oauth.frontend_callback / oauth.auto_register。
--
-- 幂等：全部 INSERT ... WHERE NOT EXISTS（按业务键判重），重复执行插入 0 行。
--       绝不 UPDATE 已存在的行 —— 运营改过的凭证/策略不能被迁移覆盖。
-- 顺序：051 之后、重启后端之前执行（seed 与迁移同口径双写，先跑哪个都不会冲突）。
-- 回滚：DELETE FROM oauth_providers WHERE provider IN ('wechat','qq','alipay');
--       DELETE FROM realname_providers WHERE provider_type IN ('manual','alipay');
--       DELETE FROM verification_configs WHERE config_group = 'verification';
--       DELETE FROM system_configs WHERE config_key LIKE 'oauth.%';
-- ============================================================================
BEGIN;

-- ---- 1. 第三方登录渠道（默认停用）----
-- descriptor 落一份能力描述符快照，供审计「当时运营看到的字段定义」；
-- credentials 留空，enabled=false：没有 AppID/AppSecret 的渠道若默认启用，
-- 登录页会出现点了就报错的图标。
INSERT INTO oauth_providers (provider, name, enabled, mode, icon, scopes, descriptor, credentials, sort_order, health_status, remark, created_at, updated_at)
SELECT 'wechat', '微信', false, 'qr', 'wechat', 'snsapi_login',
       '{"type":"wechat","name":"微信","mode":"qr","default_scopes":"snsapi_login","icon":"wechat","adapter_version":"1.0.0","implemented":true}',
       '{}'::jsonb, 1, '', '', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM oauth_providers WHERE provider = 'wechat' AND deleted_at IS NULL);

INSERT INTO oauth_providers (provider, name, enabled, mode, icon, scopes, descriptor, credentials, sort_order, health_status, remark, created_at, updated_at)
SELECT 'qq', 'QQ', false, 'api', 'qq', 'get_user_info',
       '{"type":"qq","name":"QQ","mode":"api","default_scopes":"get_user_info","icon":"qq","adapter_version":"1.0.0","implemented":true}',
       '{}'::jsonb, 2, '', '', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM oauth_providers WHERE provider = 'qq' AND deleted_at IS NULL);

INSERT INTO oauth_providers (provider, name, enabled, mode, icon, scopes, descriptor, credentials, sort_order, health_status, remark, created_at, updated_at)
SELECT 'alipay', '支付宝', false, 'api', 'alipay', 'auth_user',
       '{"type":"alipay","name":"支付宝","mode":"api","default_scopes":"auth_user","icon":"alipay","adapter_version":"1.0.0","implemented":true}',
       '{}'::jsonb, 3, '', '', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM oauth_providers WHERE provider = 'alipay' AND deleted_at IS NULL);

-- ---- 2. 实名核验服务商 ----
-- manual 内置兜底：未接入三方核验时的合法路径，恒可用、不可删（服务层拒绝删除 Builtin）。
INSERT INTO realname_providers (provider_type, name, mode, status, is_default, priority, endpoint, descriptor, credentials, health_status, remark, created_at, updated_at)
SELECT 'manual', '人工审核', 'manual', 1, true, 100, '',
       '{"type":"manual","name":"人工审核","mode":"manual","icon":"user-check","adapter_version":"1.0.0","implemented":true,"builtin":true}',
       '{}'::jsonb, 'healthy', '内置兜底，不可删除', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM realname_providers WHERE provider_type = 'manual' AND deleted_at IS NULL);

-- alipay 跳转式核验：只登记不预置凭证，status=0 待运营填好后启用。
INSERT INTO realname_providers (provider_type, name, mode, status, is_default, priority, endpoint, descriptor, credentials, health_status, remark, created_at, updated_at)
SELECT 'alipay', '支付宝实名认证', 'redirect', 0, false, 50, '',
       '{"type":"alipay","name":"支付宝实名认证","mode":"redirect","icon":"alipay","adapter_version":"1.0.0","implemented":true,"builtin":false}',
       '{}'::jsonb, '', '待填写应用凭证', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM realname_providers WHERE provider_type = 'alipay' AND deleted_at IS NULL);

-- ---- 3. 实名策略默认值（与 model.VerificationConfigDefaults 逐字一致）----
-- 口径取舍：安全侧从严（real_name_locked=true、冷却 24h 防止刷审核队列），
-- 自动化默认关闭（auto_approve_on_pass=false 是需要运营明确知情的策略）。
INSERT INTO verification_configs (config_key, config_group, config_value, value_type, status, description, updated_by, created_at, updated_at)
SELECT v.config_key, 'verification', v.config_value, v.value_type, 'active', v.description, 0, now(), now()
FROM (VALUES
    ('verification.enabled',                'true',                  'bool',   '是否开放用户端自助提交实名认证'),
    ('verification.allowed_types',          '["personal","enterprise"]', 'json', '允许的认证主体类型'),
    ('verification.provider',               'manual',                'string', '生效的核验服务商（manual=纯人工审核）'),
    ('verification.auto_approve_on_pass',   'false',                 'bool',   '三方核验通过即自动通过（跳过人工审核）'),
    ('verification.auto_reject_on_fail',    'false',                 'bool',   '三方核验明确失败即自动驳回'),
    ('verification.require_before_order',   'false',                 'bool',   '下单前强制实名认证'),
    ('verification.resubmit_cooldown_hours','24',                    'int',    '驳回后重新提交的冷却小时数'),
    ('verification.real_name_locked',       'true',                  'bool',   '通过后禁止再次提交（改名需走撤销）')
) AS v(config_key, config_value, value_type, description)
WHERE NOT EXISTS (SELECT 1 FROM verification_configs vc WHERE vc.config_key = v.config_key);

-- ---- 4. 第三方登录运行期配置 ----
-- 回调地址做成配置项而非写死：部署域名各异，写死等于换域名就要改代码发版。
INSERT INTO system_configs (config_key, config_value, value_type, config_group, description, sort_order, status, created_at, updated_at)
SELECT 'oauth.callback_base', '', 'string', 'security',
       '第三方登录后端回调基地址，如 https://api.example.com/api/v1/uc/oauth（留空用进程内兜底）', 20, 'active', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM system_configs WHERE config_key = 'oauth.callback_base');

INSERT INTO system_configs (config_key, config_value, value_type, config_group, description, sort_order, status, created_at, updated_at)
SELECT 'oauth.frontend_callback', '/oauth/callback', 'string', 'security',
       '第三方登录完成后前端回跳路径，如 https://www.example.com/oauth/callback', 21, 'active', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM system_configs WHERE config_key = 'oauth.frontend_callback');

INSERT INTO system_configs (config_key, config_value, value_type, config_group, description, sort_order, status, created_at, updated_at)
SELECT 'oauth.auto_register', 'true', 'bool', 'security',
       '第三方账号首次登录时自动创建平台账号（关闭则要求先绑定已有账号）', 22, 'active', now(), now()
WHERE NOT EXISTS (SELECT 1 FROM system_configs WHERE config_key = 'oauth.auto_register');

COMMIT;

-- 验收查询（人工执行）
--   SELECT provider, name, enabled, mode FROM oauth_providers ORDER BY sort_order;   -- 期望 3 行，enabled 全 false
--   SELECT provider_type, status, is_default FROM realname_providers;                -- 期望 manual=1/true, alipay=0/false
--   SELECT config_key, config_value FROM verification_configs ORDER BY id;           -- 期望 8 行
--   SELECT config_key FROM system_configs WHERE config_key LIKE 'oauth.%';           -- 期望 3 行
