-- ============================================================================
-- 041_notification_channels.sql
--
-- 背景（docs/实施计划/90 消息中心与多渠道通知实施文档）：
--   1) 通知渠道从「SMTP 参数写死在 system_configs」升级为「渠道类型注册表 + 渠道实例 + 字段级加密凭证」；
--   2) 短信通道落地（短信模板 + 模板变量注册表），四家服务商先注册描述符占位、后接实现；
--   3) 外发通道（邮件/短信）从 notifications 表拆出，统一进投递队列 notification_deliveries，
--      notifications 从此只装站内信（channel=inbox）；
--   4) 修复五个既有通知 bug 的数据库侧前置：幂等唯一索引、偏好表短信列。
--
-- 内容：
--   1. notification_channel_types   —— 渠道类型注册表（descriptor 驱动前端动态表单）
--   2. notification_channels        —— 渠道实例（凭证字段级加密 enc:v1:）
--   3. sms_templates                —— 短信模板
--   4. sms_template_vars            —— 模板变量注册表（D7）
--   5. notification_deliveries      —— 投递队列（含幂等唯一索引）
--   6. 既有表改造                    —— notification_templates / notifications / notification_preferences
--   7. seed 数据                     —— 5 个渠道类型 + 15 个模板变量
--
-- 幂等：CREATE TABLE IF NOT EXISTS / ADD COLUMN IF NOT EXISTS /
--       CREATE INDEX IF NOT EXISTS / ON CONFLICT DO NOTHING。
-- 执行前建议 pg_dump 备份。
--
-- 回滚：
--   DROP TABLE IF EXISTS notification_deliveries;
--   DROP TABLE IF EXISTS sms_template_vars;
--   DROP TABLE IF EXISTS sms_templates;
--   DROP TABLE IF EXISTS notification_channels;
--   DROP TABLE IF EXISTS notification_channel_types;
--   ALTER TABLE notification_templates
--     DROP COLUMN IF EXISTS sms_on, DROP COLUMN IF EXISTS sms_template_id,
--     DROP COLUMN IF EXISTS mail_format, DROP COLUMN IF EXISTS title_show;
--   ALTER TABLE notifications
--     DROP COLUMN IF EXISTS content_format, DROP COLUMN IF EXISTS delivery_id,
--     DROP COLUMN IF EXISTS migrated_to_delivery;
--   ALTER TABLE notification_preferences DROP COLUMN IF EXISTS sms_on;
--   DROP INDEX IF EXISTS uk_notifications_source;
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. notification_channel_types：渠道类型注册表
-- ---------------------------------------------------------------------------
-- 与 payment_types 同构：一行 = 一种可接入的通道类型，descriptor 存凭证 schema。
CREATE TABLE IF NOT EXISTS notification_channel_types (
    id              BIGSERIAL PRIMARY KEY,
    type            varchar(50)  NOT NULL,              -- smtp/aliyun_sms/tencent_sms/saiyou_sms/duanxinbao_sms
    name            varchar(100) NOT NULL,
    category        varchar(20)  NOT NULL,              -- mail / sms
    mode            varchar(20)  NOT NULL DEFAULT 'api',-- api / manual
    descriptor      jsonb,                              -- CapabilityDescriptor（凭证字段 schema）
    icon            varchar(64)  NOT NULL DEFAULT '',
    doc_url         varchar(255) NOT NULL DEFAULT '',
    adapter_version varchar(32)  NOT NULL DEFAULT '',
    sort_order      int          NOT NULL DEFAULT 0,
    status          int          NOT NULL DEFAULT 1,
    created_at      timestamptz,
    updated_at      timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_notify_channel_types_type ON notification_channel_types (type);
CREATE INDEX IF NOT EXISTS idx_notify_channel_types_category ON notification_channel_types (category);

-- ---------------------------------------------------------------------------
-- 2. notification_channels：渠道实例
-- ---------------------------------------------------------------------------
-- credentials 为字段级加密密文（internal/pkg/credentials，enc:v1: 前缀），
-- 任何接口不得回显明文，回显一律走 credentials.Map.Mask。
CREATE TABLE IF NOT EXISTS notification_channels (
    id             BIGSERIAL PRIMARY KEY,
    channel_code   varchar(64)  NOT NULL,               -- 唯一标识，如 sms_main / mail_default
    name           varchar(100) NOT NULL,
    category       varchar(20)  NOT NULL,               -- mail / sms
    type           varchar(50)  NOT NULL,               -- 对应 notification_channel_types.type
    credentials    jsonb,                               -- 字段级加密后的凭证
    endpoint       varchar(255) NOT NULL DEFAULT '',
    sign_name      varchar(64)  NOT NULL DEFAULT '',    -- 短信签名
    sender         varchar(128) NOT NULL DEFAULT '',    -- 邮件发件人（From）
    template_code  varchar(64)  NOT NULL DEFAULT '',    -- 短信服务商侧模板号（可选）
    scenes         jsonb,                               -- 启用场景数组：otp/notify/alert/marketing
    priority       int          NOT NULL DEFAULT 0,
    weight         int          NOT NULL DEFAULT 0,
    daily_limit    int          NOT NULL DEFAULT 0,     -- 0=不限
    health_status  varchar(20)  NOT NULL DEFAULT '',    -- ''/healthy/down/pending
    last_error     text,
    last_check_at  timestamptz,
    status         int          NOT NULL DEFAULT 1,
    is_default     boolean      NOT NULL DEFAULT false,
    remark         varchar(255) NOT NULL DEFAULT '',
    created_at     timestamptz,
    updated_at     timestamptz,
    deleted_at     timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_notify_channels_code
    ON notification_channels (channel_code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_notify_channels_category ON notification_channels (category);
CREATE INDEX IF NOT EXISTS idx_notify_channels_type ON notification_channels (type);
-- 同一 category 内至多一个默认渠道（用部分唯一索引落地约束，代码无需事务清空再插入）。
CREATE UNIQUE INDEX IF NOT EXISTS uk_notify_channels_default
    ON notification_channels (category) WHERE is_default = true AND deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- 3. sms_templates：短信模板
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sms_templates (
    id            BIGSERIAL PRIMARY KEY,
    code          varchar(64)  NOT NULL,                -- 模板编码，如 verify_login / instance_expiring
    name          varchar(100) NOT NULL,
    scene         varchar(32)  NOT NULL DEFAULT 'notify',-- otp/notify/alert/marketing
    content       varchar(1000) NOT NULL,               -- 短信正文，含 {var}
    upstream_code varchar(64)  NOT NULL DEFAULT '',     -- 服务商侧模板号
    var_names     jsonb,                                -- 使用到的变量名数组，保存时后端解析回写
    status        varchar(20)  NOT NULL DEFAULT 'active',
    remark        varchar(255) NOT NULL DEFAULT '',
    created_at    timestamptz,
    updated_at    timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_sms_templates_code ON sms_templates (code);
CREATE INDEX IF NOT EXISTS idx_sms_templates_scene ON sms_templates (scene);

-- ---------------------------------------------------------------------------
-- 4. sms_template_vars：模板变量注册表（D7）
-- ---------------------------------------------------------------------------
-- 变量不是自由文本，而是可枚举、可选、可校验的注册项。
CREATE TABLE IF NOT EXISTS sms_template_vars (
    id          BIGSERIAL PRIMARY KEY,
    var_key     varchar(64)  NOT NULL,                  -- 变量名：hostname / display_ip / expire_date
    label       varchar(64)  NOT NULL,                  -- 中文名：主机名 / 显示IP / 到期日期
    category    varchar(32)  NOT NULL DEFAULT 'instance',-- instance/order/user/system/common/otp
    value_type  varchar(20)  NOT NULL DEFAULT 'string', -- string/number/date/datetime/amount
    sample      varchar(255) NOT NULL DEFAULT '',       -- 预览用样例值
    description varchar(255) NOT NULL DEFAULT '',
    scenes      jsonb,                                  -- 适用场景：["otp","notify"]
    sort_order  int          NOT NULL DEFAULT 0,
    status      varchar(20)  NOT NULL DEFAULT 'active',
    created_at  timestamptz,
    updated_at  timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_sms_template_vars_key ON sms_template_vars (var_key);

-- ---------------------------------------------------------------------------
-- 5. notification_deliveries：投递队列
-- ---------------------------------------------------------------------------
-- 一行 = 一个「模板事件 × 目标 × 通道」的投递任务，统一承接站内信/邮件/短信。
CREATE TABLE IF NOT EXISTS notification_deliveries (
    id              BIGSERIAL PRIMARY KEY,
    batch_id        varchar(40)  NOT NULL DEFAULT '',    -- 群发批次号；单发为空
    event           varchar(64)  NOT NULL,               -- 事件/模板编码
    channel         varchar(20)  NOT NULL,               -- inbox / mail / sms
    target_type     varchar(10)  NOT NULL DEFAULT 'user',-- user / admin
    target_id       bigint       NOT NULL DEFAULT 0,     -- 用户/管理员 ID；0=广播
    target_name     varchar(128) NOT NULL DEFAULT '',    -- 冗余显示名
    recipient       varchar(191) NOT NULL DEFAULT '',    -- 实际收件地址（邮箱/手机号，站内信为空）
    channel_id      bigint       NOT NULL DEFAULT 0,     -- 使用的渠道实例；站内信为 0
    title           varchar(255) NOT NULL DEFAULT '',
    content         text,
    content_format  varchar(10)  NOT NULL DEFAULT 'text',-- text / html
    vars            jsonb,                               -- 渲染变量（供重投时重渲染）
    send_status     varchar(20)  NOT NULL DEFAULT 'pending', -- pending/sending/sent/failed/skipped/dead
    attempts        int          NOT NULL DEFAULT 0,
    max_attempts    int          NOT NULL DEFAULT 5,
    next_retry_at   timestamptz,
    locked_until    timestamptz,
    provider_msg_id varchar(128) NOT NULL DEFAULT '',    -- 上游回执 ID
    provider_code   varchar(64)  NOT NULL DEFAULT '',    -- 上游业务码
    cost_fen        int          NOT NULL DEFAULT 0,     -- 短信计费（分）
    fail_reason     varchar(500) NOT NULL DEFAULT '',
    source_module   varchar(32)  NOT NULL DEFAULT '',
    source_id       varchar(64)  NOT NULL DEFAULT '',
    sent_at         timestamptz,
    created_at      timestamptz,
    updated_at      timestamptz
);
CREATE INDEX IF NOT EXISTS idx_notify_deliveries_created ON notification_deliveries (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notify_deliveries_due ON notification_deliveries (send_status, next_retry_at)
    WHERE send_status IN ('pending','failed');
CREATE INDEX IF NOT EXISTS idx_notify_deliveries_event ON notification_deliveries (event);
CREATE INDEX IF NOT EXISTS idx_notify_deliveries_target ON notification_deliveries (target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_notify_deliveries_batch ON notification_deliveries (batch_id) WHERE batch_id <> '';
-- 幂等唯一索引（bug ③ 的落点）：带 channel 与 target_id，
-- 避免「同一订单的邮件与短信互相顶掉」。
CREATE UNIQUE INDEX IF NOT EXISTS uk_notify_deliveries_idem
    ON notification_deliveries (source_module, source_id, channel, target_type, target_id)
    WHERE source_id <> '';

-- ---------------------------------------------------------------------------
-- 6. 改造既有表
-- ---------------------------------------------------------------------------

-- 6.1 通知模板扩展：支持短信通道与 HTML 格式
ALTER TABLE notification_templates
  ADD COLUMN IF NOT EXISTS sms_on          boolean     NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS sms_template_id bigint      NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS mail_format     varchar(10) NOT NULL DEFAULT 'text',
  ADD COLUMN IF NOT EXISTS title_show      boolean     NOT NULL DEFAULT true;

-- 6.2 站内信内容格式 + 关联投递记录
ALTER TABLE notifications
  ADD COLUMN IF NOT EXISTS content_format varchar(10) NOT NULL DEFAULT 'text',
  ADD COLUMN IF NOT EXISTS delivery_id    bigint      NOT NULL DEFAULT 0;

-- 6.3 幂等唯一索引（bug ③）：必须先清存量重复行，否则迁移直接失败。
--     清理段保留 id 最小的一条；只清 source_id <> '' 的行（历史自由行不受影响）。
DELETE FROM notifications a USING notifications b
WHERE a.id > b.id
  AND a.source_module = b.source_module AND a.source_id = b.source_id
  AND a.channel = b.channel AND a.target_type = b.target_type AND a.user_id = b.user_id
  AND a.source_id <> '';

CREATE UNIQUE INDEX IF NOT EXISTS uk_notifications_source
    ON notifications (source_module, source_id, channel, target_type, user_id)
    WHERE source_id <> '';

-- 6.4 偏好表补短信列（用户端偏好页三列布局）
ALTER TABLE notification_preferences
  ADD COLUMN IF NOT EXISTS sms_on boolean NOT NULL DEFAULT false;

-- 6.5 存量 mail 行标注（只标注不删数据；删除归日志中心 doc92）。
--     用户端「我的消息」查询据此过滤，避免出现英文邮件记录。
ALTER TABLE notifications
  ADD COLUMN IF NOT EXISTS migrated_to_delivery boolean NOT NULL DEFAULT false;

UPDATE notifications SET migrated_to_delivery = true
WHERE channel = 'mail' AND migrated_to_delivery = false;

-- ---------------------------------------------------------------------------
-- 7. seed：渠道类型 + 模板变量（与 db.go 的 seed 双写，保证既有库升级也有数据）
-- ---------------------------------------------------------------------------
INSERT INTO notification_channel_types (type, name, category, mode, icon, doc_url, adapter_version, sort_order, status)
VALUES
    ('smtp',           'SMTP 邮件', 'mail', 'api', 'mail',    '',                                              'v1', 1, 1),
    ('aliyun_sms',     '阿里云短信', 'sms',  'api', 'aliyun',  'https://help.aliyun.com/zh/sms/',              'v1', 2, 1),
    ('tencent_sms',    '腾讯云短信', 'sms',  'api', 'tencent', 'https://cloud.tencent.com/document/product/382', 'v1', 3, 1),
    ('saiyou_sms',     '赛游短信',   'sms',  'api', 'sms',     '',                                              'v1', 4, 1),
    ('duanxinbao_sms', '短信宝',     'sms',  'api', 'sms',     '',                                              'v1', 5, 1)
ON CONFLICT (type) DO NOTHING;

INSERT INTO sms_template_vars (var_key, label, category, value_type, sample, description, scenes, sort_order, status)
VALUES
    ('code',         '验证码',     'otp',      'string', '5283',           '一次性验证码',       '["otp"]',              1,  'active'),
    ('minutes',      '有效分钟数', 'otp',      'number', '5',              '验证码有效期（分钟）','["otp"]',              2,  'active'),
    ('hostname',     '主机名',     'instance', 'string', 'web-01',         '实例主机名',         '["notify","alert"]',   3,  'active'),
    ('display_ip',   '显示 IP',    'instance', 'string', '203.0.113.10',   '实例显示 IP',        '["notify","alert"]',   4,  'active'),
    ('expire_date',  '到期日期',   'instance', 'date',   '2026-12-31',     '实例到期日期',       '["notify","alert"]',   5,  'active'),
    ('instance_mark','实例标识',   'instance', 'string', 'i-8f3a91',       '实例标识',           '["notify","alert"]',   6,  'active'),
    ('days_left',    '剩余天数',   'instance', 'number', '7',              '距到期剩余天数',     '["notify","alert"]',   7,  'active'),
    ('order_no',     '订单号',     'order',    'string', 'SO20260913001',  '订单号',             '["notify"]',           8,  'active'),
    ('amount',       '金额',       'order',    'amount', '128.00',         '订单/交易金额',      '["notify"]',           9,  'active'),
    ('balance',      '账户余额',   'user',     'amount', '320.50',         '账户余额',           '["notify","alert"]',   10, 'active'),
    ('threshold',    '预警阈值',   'user',     'amount', '100.00',         '余额预警阈值',       '["alert"]',            11, 'active'),
    ('username',     '用户名',     'user',     'string', 'demo_user',      '用户名/昵称',        '["notify","otp","marketing"]', 12, 'active'),
    ('ticket_no',    '工单号',     'system',   'string', 'TK20260913007',  '工单编号',           '["notify"]',           13, 'active'),
    ('reason',       '失败原因',   'system',   'string', '上游超时',       '失败/异常原因',      '["alert","notify"]',   14, 'active'),
    ('site_name',    '站点名称',   'common',   'string', 'HostSent',       '站点名称（品牌）',   '["notify","otp","alert","marketing"]', 15, 'active')
ON CONFLICT (var_key) DO NOTHING;

COMMIT;
