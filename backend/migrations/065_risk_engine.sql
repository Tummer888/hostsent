-- ============================================================================
-- 065_risk_engine
-- 异常行为监控与黑名单管理的能力补齐（doc06 §4.3/§4.4）。
--
-- 为什么改（改造前的实际状态）：
--   1) risk_events 只有演示数据写入方（seedDemoRiskEvents），登录链路**从不写** ——
--      页面上永远是那三条假事件，运营看不到真实的异常行为；
--   2) blacklists 只在管理页读写，登录链路**从不读** —— 拉黑一个 IP 之后
--      对方照样能登录，等于一个摆设；
--   3) 两张表都缺主体域判别列，而 user_id 承载 users.id 与 admins.id 两个
--      ID 空间（实测 7 组撞号），员工后台的异常会被算到同 ID 的客户头上。
--
-- 本迁移只做结构补齐，规则判定与拦截都在 Go 侧（security/service/risk_engine.go）。
--
-- 幂等：ADD COLUMN IF NOT EXISTS + CREATE INDEX IF NOT EXISTS，
-- 重复执行安全（迁移脚本要求可反复跑）。
-- 回滚：
--   DROP INDEX IF EXISTS idx_risk_events_subject_user;
--   ALTER TABLE risk_events DROP COLUMN IF EXISTS subject_type;
--   DELETE FROM system_configs WHERE config_key LIKE 'risk_%';
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- A. risk_events 增加主体域（user / admin）
--
--    默认 'user'：既有写入方（演示数据、未来的新链路）不关心本列时天然落在
--    客户域，与 login_logs.subject_type 的口径完全一致。
-- ---------------------------------------------------------------------------
ALTER TABLE risk_events
    ADD COLUMN IF NOT EXISTS subject_type VARCHAR(16) NOT NULL DEFAULT 'user';

COMMENT ON COLUMN risk_events.subject_type IS '事件主体域：user（客户）/ admin（员工后台）。user_id 承载两个 ID 空间，必须带域判别（doc06）';

-- 按「主体 + 用户 + 最近发生」查未处置事件是规则引擎聚合时最热的一条路径
-- （每次命中规则都会走一次），给它一条复合索引。
CREATE INDEX IF NOT EXISTS idx_risk_events_subject_user ON risk_events (subject_type, user_id);

-- 规则引擎按 rule_code + 用户 + 状态找「同规则未处置事件」做聚合，
-- 现有的单列索引在多规则场景下选择率很差。
CREATE INDEX IF NOT EXISTS idx_risk_events_rule_open
    ON risk_events (rule_code, subject_type, user_id, status, last_occurred_at DESC);

COMMENT ON INDEX idx_risk_events_rule_open IS '规则引擎同类事件聚合的查询索引（规则+主体+用户+状态+最近发生）';

-- ---------------------------------------------------------------------------
-- B. 黑名单命中留痕索引
--
--    「命中记录」抽屉按黑名单的 type/target_value 去关联 login_logs：
--      - ip     → login_logs.ip
--      - device → login_logs.device_fingerprint
--      - phone/email/user → login_logs.username
--    ip 与 username 已有单列索引；device_fingerprint 此前完全没有索引，
--    按设备类型的黑名单点开「命中记录」就是一次全表扫描。
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_login_logs_device_fingerprint ON login_logs (device_fingerprint);

COMMENT ON INDEX idx_login_logs_device_fingerprint IS '黑名单按设备指纹查命中记录（doc06 §4.4「查看命中记录」）';

-- ---------------------------------------------------------------------------
-- C. 风险规则开关与阈值（system_configs，group=security）
--
--    与 captchaSystemConfigs 同一套种子机制：键不存在即写入，已存在不动 ——
--    绝不覆盖运营改过的值。默认值取「宁可漏报不要误报」：
--      - risk_rules_enabled=true：这条链路只写事件、不阻断登录，开着不会有副作用；
--      - risk_device_change_enabled=true：改设备用户自己能感知，误报代价只是看一眼；
--      - risk_new_ip_enabled=false：移动网络/企业 NAT 下「新 IP」是常态，默认关。
-- ---------------------------------------------------------------------------
INSERT INTO system_configs (config_key, config_value, value_type, config_group, description, sort_order, status, created_at, updated_at) VALUES
    ('risk_rules_enabled',                   'true',  'bool', 'security', '异常行为规则引擎总开关（只写事件、不阻断登录）', 50, 'active', NOW(), NOW()),
    ('risk_fail_threshold',                  '5',     'int',  'security', '连续登录失败触发风险事件的次数阈值',             51, 'active', NOW(), NOW()),
    ('risk_fail_window_minutes',             '10',    'int',  'security', '连续登录失败统计窗口（分钟）',                   52, 'active', NOW(), NOW()),
    ('risk_multi_account_fail_accounts',     '8',     'int',  'security', '同一 IP 失败的不同账号数阈值（撞库判定）',       53, 'active', NOW(), NOW()),
    ('risk_multi_account_fail_window_minutes','10',   'int',  'security', '撞库统计窗口（分钟）',                           54, 'active', NOW(), NOW()),
    ('risk_login_burst_count',               '30',    'int',  'security', '同一 IP 登录请求次数阈值（频率异常）',           55, 'active', NOW(), NOW()),
    ('risk_login_burst_window_minutes',      '5',     'int',  'security', '登录频率统计窗口（分钟）',                       56, 'active', NOW(), NOW()),
    ('risk_device_change_enabled',           'true',  'bool', 'security', '是否对设备指纹变更生成风险事件',                 57, 'active', NOW(), NOW()),
    ('risk_new_ip_enabled',                  'false', 'bool', 'security', '是否对新 IP 登录生成风险事件（NAT 环境易误报）', 58, 'active', NOW(), NOW()),
    ('risk_notify_critical',                 'false', 'bool', 'security', '严重级风险事件是否额外推送站内通知',             59, 'active', NOW(), NOW())
ON CONFLICT (config_key) DO NOTHING;

COMMIT;
