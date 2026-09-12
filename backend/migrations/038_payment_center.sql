-- ============================================================================
-- 038_payment_center.sql —— 支付中心基础表 + 财务/账单/提现支付方式字段补充
--
-- 背景（docs/实施计划/35 支付中心架构设计与实施规划 §2）：
--   财务管理当前只有「人工确认充值」与「审批即扣款无打款」两处断链，
--   且存在未鉴权充值回调（doc34 F-01）。本迁移落地独立支付模块的数据底座：
--   渠道类型注册表 / 渠道实例 / 支付单 / 回调日志 / 退款单 / 打款单 /
--   用户收款账户 / 用户支付方式偏好 / 渠道对账记录。
--
--   金额一律整数分（*_fen BIGINT），出口由适配器转换渠道要求的格式。
--   支付单以 biz_type + biz_id 与业务解耦；回调幂等靠 (channel_id, channel_tx) 唯一。
--
-- 同时修正 doc34 两项缺陷：
--   F-04  bills 缺 (user_id, period) 唯一约束 → Upsert 的 ON CONFLICT 必然失败；
--   F-11  账单/提现缺支付方式描述字段。
--
-- 幂等：CREATE TABLE IF NOT EXISTS / ADD COLUMN IF NOT EXISTS /
--       CREATE UNIQUE INDEX IF NOT EXISTS。回填前先判空避免重复。
-- 回滚：
--   DROP TABLE IF EXISTS payment_recon_records, user_payment_preferences,
--     user_payout_accounts, payment_payouts, payment_refunds,
--     payment_callback_logs, payment_orders, payment_channels, payment_types;
--   DROP INDEX IF EXISTS uk_bills_user_period;
--   ALTER TABLE bills DROP COLUMN IF EXISTS paid_amount_fen,
--     DROP COLUMN IF EXISTS paid_method, DROP COLUMN IF EXISTS paid_channel_id,
--     DROP COLUMN IF EXISTS paid_at;
--   ALTER TABLE recharges DROP COLUMN IF EXISTS payment_order_id,
--     DROP COLUMN IF EXISTS channel_code;
--   ALTER TABLE withdrawals DROP COLUMN IF EXISTS payout_id,
--     DROP COLUMN IF EXISTS payout_mode, DROP COLUMN IF EXISTS payout_account_id;
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. 渠道类型注册表（一行 = 一种可接入支付方式，descriptor 存能力描述符）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS payment_types (
    id               BIGSERIAL PRIMARY KEY,
    type             varchar(50)  NOT NULL,
    name             varchar(100) NOT NULL,
    mode             varchar(20)  NOT NULL DEFAULT 'api',
    descriptor       jsonb,
    icon             varchar(64),
    doc_url          varchar(255),
    adapter_version  varchar(32),
    sort_order       int          NOT NULL DEFAULT 0,
    status           int          NOT NULL DEFAULT 1,
    created_at       timestamptz  NOT NULL DEFAULT now(),
    updated_at       timestamptz  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_payment_types_type ON payment_types (type);

-- ---------------------------------------------------------------------------
-- 2. 渠道实例（同类型多商户号；凭证字段级加密）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS payment_channels (
    id              BIGSERIAL PRIMARY KEY,
    channel_code    varchar(64)  NOT NULL,
    name            varchar(100) NOT NULL,
    type            varchar(50)  NOT NULL,
    credentials     jsonb,
    endpoint        varchar(255),
    notify_url      varchar(255),
    return_url      varchar(255),
    scenes          jsonb,
    priority        int          NOT NULL DEFAULT 0,
    weight          int          NOT NULL DEFAULT 0,
    fee_rate        numeric(6,4) NOT NULL DEFAULT 0,
    settle_mode     varchar(20)  NOT NULL DEFAULT '',
    min_amount_fen  bigint       NOT NULL DEFAULT 0,
    max_amount_fen  bigint       NOT NULL DEFAULT 0,
    environment     varchar(20)  NOT NULL DEFAULT 'prod',
    health_status   varchar(20)  NOT NULL DEFAULT '',
    last_error      text,
    last_check_at   timestamptz,
    status          int          NOT NULL DEFAULT 1,
    remark          varchar(255),
    is_default      boolean      NOT NULL DEFAULT false,
    created_at      timestamptz  NOT NULL DEFAULT now(),
    updated_at      timestamptz  NOT NULL DEFAULT now(),
    deleted_at      timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_payment_channels_code ON payment_channels (channel_code);
CREATE INDEX IF NOT EXISTS idx_payment_channels_type ON payment_channels (type);
CREATE INDEX IF NOT EXISTS idx_payment_channels_status ON payment_channels (status);

-- ---------------------------------------------------------------------------
-- 3. 支付单（biz_type + biz_id 解耦；金额整数分）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS payment_orders (
    id             BIGSERIAL PRIMARY KEY,
    payment_no     varchar(64)  NOT NULL,
    user_id        bigint       NOT NULL,
    biz_type       varchar(32)  NOT NULL,
    biz_id         bigint       NOT NULL DEFAULT 0,
    biz_no         varchar(64),
    amount_fen     bigint       NOT NULL,
    currency       varchar(10)  NOT NULL DEFAULT 'CNY',
    channel_id     bigint       NOT NULL DEFAULT 0,
    channel_code   varchar(64)  NOT NULL DEFAULT '',
    channel_type   varchar(50)  NOT NULL DEFAULT '',
    scene          varchar(20)  NOT NULL DEFAULT '',
    status         varchar(20)  NOT NULL DEFAULT 'pending',
    channel_tx     varchar(128),
    pay_url        varchar(512),
    qrcode         varchar(512),
    prepay_params  jsonb,
    instructions   text,
    subject        varchar(255),
    client_ip      varchar(64),
    fee_fen        bigint       NOT NULL DEFAULT 0,
    expire_at      timestamptz,
    paid_at        timestamptz,
    remark         varchar(255),
    created_at     timestamptz  NOT NULL DEFAULT now(),
    updated_at     timestamptz  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_orders_no ON payment_orders (payment_no);
CREATE INDEX IF NOT EXISTS idx_payment_orders_user ON payment_orders (user_id);
CREATE INDEX IF NOT EXISTS idx_payment_orders_biz ON payment_orders (biz_type, biz_id);
CREATE INDEX IF NOT EXISTS idx_payment_orders_status ON payment_orders (status);
CREATE INDEX IF NOT EXISTS idx_payment_orders_channel_tx ON payment_orders (channel_tx);
-- 回调幂等：同渠道同渠道交易号只能对应一张支付单
CREATE UNIQUE INDEX IF NOT EXISTS uk_payment_orders_channel_tx
  ON payment_orders (channel_id, channel_tx) WHERE channel_tx IS NOT NULL AND channel_tx <> '';

-- ---------------------------------------------------------------------------
-- 4. 回调日志（原始报文留痕 + 验签结果 + 重放）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS payment_callback_logs (
    id             BIGSERIAL PRIMARY KEY,
    channel_id     bigint       NOT NULL DEFAULT 0,
    channel_code   varchar(64)  NOT NULL DEFAULT '',
    notify_id      varchar(128),
    payment_no     varchar(64),
    raw_body       text,
    headers        text,
    verify_ok      boolean      NOT NULL DEFAULT false,
    amount_fen     bigint       NOT NULL DEFAULT 0,
    handle_status  varchar(20)  NOT NULL DEFAULT '',
    handle_msg     text,
    source_ip      varchar(64),
    created_at     timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_payment_callback_logs_channel ON payment_callback_logs (channel_id);
CREATE INDEX IF NOT EXISTS idx_payment_callback_logs_payment ON payment_callback_logs (payment_no);
CREATE INDEX IF NOT EXISTS idx_payment_callback_logs_notify ON payment_callback_logs (notify_id);
-- 回调幂等：同渠道同 notify_id 只处理一次
CREATE UNIQUE INDEX IF NOT EXISTS uk_payment_callback_notify
  ON payment_callback_logs (channel_id, notify_id) WHERE notify_id IS NOT NULL AND notify_id <> '';

-- ---------------------------------------------------------------------------
-- 5. 渠道退款单
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS payment_refunds (
    id                 BIGSERIAL PRIMARY KEY,
    refund_no          varchar(64) NOT NULL,
    payment_no         varchar(64) NOT NULL,
    order_refund_no    varchar(64),
    user_id            bigint      NOT NULL,
    amount_fen         bigint      NOT NULL,
    channel_id         bigint      NOT NULL DEFAULT 0,
    channel_refund_id  varchar(128),
    status             varchar(20) NOT NULL DEFAULT 'pending',
    reason             varchar(255),
    fail_reason        text,
    refunded_at        timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_payment_refunds_no ON payment_refunds (refund_no);
CREATE INDEX IF NOT EXISTS idx_payment_refunds_payment ON payment_refunds (payment_no);
CREATE INDEX IF NOT EXISTS idx_payment_refunds_order_refund ON payment_refunds (order_refund_no);
CREATE INDEX IF NOT EXISTS idx_payment_refunds_status ON payment_refunds (status);

-- ---------------------------------------------------------------------------
-- 6. 打款单（人工登记 / API 自动双模）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS payment_payouts (
    id           BIGSERIAL PRIMARY KEY,
    payout_no    varchar(64) NOT NULL,
    withdraw_id  bigint      NOT NULL DEFAULT 0,
    withdraw_no  varchar(64),
    user_id      bigint      NOT NULL,
    amount_fen   bigint      NOT NULL,
    channel_id   bigint      NOT NULL DEFAULT 0,
    channel_code varchar(64) NOT NULL DEFAULT '',
    mode         varchar(20) NOT NULL DEFAULT 'manual',
    status       varchar(20) NOT NULL DEFAULT 'pending',
    channel_tx   varchar(128),
    receipt_url  varchar(512),
    fail_reason  text,
    attempts     int         NOT NULL DEFAULT 0,
    operator_id  bigint      NOT NULL DEFAULT 0,
    paid_at      timestamptz,
    remark       varchar(255),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_payment_payouts_no ON payment_payouts (payout_no);
CREATE INDEX IF NOT EXISTS idx_payment_payouts_withdraw ON payment_payouts (withdraw_id);
CREATE INDEX IF NOT EXISTS idx_payment_payouts_user ON payment_payouts (user_id);
CREATE INDEX IF NOT EXISTS idx_payment_payouts_status ON payment_payouts (status);

-- ---------------------------------------------------------------------------
-- 7. 用户收款账户（账号字段级加密）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS user_payout_accounts (
    id           BIGSERIAL PRIMARY KEY,
    user_id      bigint       NOT NULL,
    channel      varchar(20)  NOT NULL DEFAULT 'bank',
    account_no   varchar(255) NOT NULL,
    account_name varchar(64)  NOT NULL,
    bank_name    varchar(100),
    branch       varchar(100),
    is_default   boolean      NOT NULL DEFAULT false,
    verified     boolean      NOT NULL DEFAULT false,
    created_at   timestamptz  NOT NULL DEFAULT now(),
    updated_at   timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_user_payout_accounts_user ON user_payout_accounts (user_id);

-- ---------------------------------------------------------------------------
-- 8. 用户支付方式偏好（默认方式 + 优先级）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS user_payment_preferences (
    id           BIGSERIAL PRIMARY KEY,
    user_id      bigint      NOT NULL,
    scene        varchar(20) NOT NULL DEFAULT '',
    channel_code varchar(64) NOT NULL,
    priority     int         NOT NULL DEFAULT 0,
    is_default   boolean     NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_user_pay_pref
  ON user_payment_preferences (user_id, scene, channel_code);
CREATE INDEX IF NOT EXISTS idx_user_pay_pref_user ON user_payment_preferences (user_id);

-- ---------------------------------------------------------------------------
-- 9. 渠道对账记录
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS payment_recon_records (
    id                 BIGSERIAL PRIMARY KEY,
    recon_no           varchar(64) NOT NULL,
    channel_id         bigint      NOT NULL DEFAULT 0,
    channel_code       varchar(64) NOT NULL DEFAULT '',
    period             varchar(20) NOT NULL DEFAULT '',
    local_amount_fen   bigint      NOT NULL DEFAULT 0,
    local_count        int         NOT NULL DEFAULT 0,
    channel_amount_fen bigint      NOT NULL DEFAULT 0,
    channel_count      int         NOT NULL DEFAULT 0,
    diff_fen           bigint      NOT NULL DEFAULT 0,
    status             varchar(20) NOT NULL DEFAULT '',
    detail             jsonb,
    created_at         timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_payment_recon_records_no ON payment_recon_records (recon_no);
CREATE INDEX IF NOT EXISTS idx_payment_recon_records_channel ON payment_recon_records (channel_id);

-- ---------------------------------------------------------------------------
-- 10. 修正 F-04：bills 缺 (user_id, period) 唯一约束
--     先清理可能的重复行（保留 id 最大的一条），再加唯一索引。
-- ---------------------------------------------------------------------------
DELETE FROM bills a USING bills b
 WHERE a.user_id = b.user_id AND a.period = b.period AND a.id < b.id;

CREATE UNIQUE INDEX IF NOT EXISTS uk_bills_user_period ON bills (user_id, period);

-- 账单支付方式描述（F-11）
ALTER TABLE bills
  ADD COLUMN IF NOT EXISTS paid_amount_fen bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS paid_method     varchar(32) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS paid_channel_id bigint NOT NULL DEFAULT 0;

-- 充值单关联支付单（充值改走支付通道后回填）
ALTER TABLE recharges
  ADD COLUMN IF NOT EXISTS payment_order_id bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS channel_code     varchar(64) NOT NULL DEFAULT '';

-- 提现单关联打款单与收款账户（F-02 打款闭环）
ALTER TABLE withdrawals
  ADD COLUMN IF NOT EXISTS payout_id         bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS payout_mode       varchar(20) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS payout_account_id bigint NOT NULL DEFAULT 0;

COMMIT;
