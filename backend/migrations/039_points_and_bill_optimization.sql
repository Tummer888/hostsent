-- ============================================================================
-- 039_points_and_bill_optimization.sql
--
-- 背景（docs/实施计划/36 积分体系与订单账单优化设计）：
--   1) 积分体系：独立于资金账本的积分账户/流水/规则。积分不可抵扣、不可提现，
--      与 wallet_accounts / wallet_transactions 零耦合（若混用会污染对账口径）。
--   2) 退款双模式：order_refunds 记录退款去向（余额 / 原路）与原路退回扣点，
--      使财务侧可显式统计原路退款的渠道手续费损失。
--   3) 账单分类与发票：bills 拆分消费/续费/原路退款扣点，并增加发票标记；
--      新增 invoice_requests 预埋开票申请（税务/邮件接口后续接入）。
--   4) 订单检索与合并支付：payment_orders 补 biz_no 索引，并支持一单多业务
--      （biz_items jsonb），解决"同时支付多单"的支付单号体验问题。
--
-- 幂等：CREATE TABLE IF NOT EXISTS / ADD COLUMN IF NOT EXISTS /
--       CREATE INDEX IF NOT EXISTS / INSERT ... ON CONFLICT DO NOTHING。
--
-- 回滚：
--   DROP TABLE IF EXISTS invoice_requests, point_transactions, point_accounts, point_rules;
--   DROP INDEX IF EXISTS idx_payment_orders_biz_no;
--   ALTER TABLE payment_orders DROP COLUMN IF EXISTS biz_items, DROP COLUMN IF EXISTS item_count;
--   ALTER TABLE order_refunds DROP COLUMN IF EXISTS refund_mode, DROP COLUMN IF EXISTS fee_amount,
--     DROP COLUMN IF EXISTS net_amount, DROP COLUMN IF EXISTS channel_refund_no,
--     DROP COLUMN IF EXISTS channel_refund_status;
--   ALTER TABLE bills DROP COLUMN IF EXISTS bill_type, DROP COLUMN IF EXISTS consume_amount,
--     DROP COLUMN IF EXISTS renewal_amount, DROP COLUMN IF EXISTS channel_refund_amount,
--     DROP COLUMN IF EXISTS refund_fee_amount, DROP COLUMN IF EXISTS paid_at,
--     DROP COLUMN IF EXISTS invoice_status, DROP COLUMN IF EXISTS invoice_no,
--     DROP COLUMN IF EXISTS invoiced_at;
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. 积分账户（与 users 1:1，独立于 wallet_accounts）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS point_accounts (
    id            BIGSERIAL PRIMARY KEY,
    user_id       bigint        NOT NULL,
    balance       bigint        NOT NULL DEFAULT 0,  -- 可用积分
    frozen        bigint        NOT NULL DEFAULT 0,  -- 冻结积分（预埋：兑换锁定额）
    total_earned  bigint        NOT NULL DEFAULT 0,  -- 累计获得
    total_spent   bigint        NOT NULL DEFAULT 0,  -- 累计消耗
    version       bigint        NOT NULL DEFAULT 0,  -- 乐观锁版本
    created_at    timestamptz,
    updated_at    timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_point_accounts_user ON point_accounts (user_id);

-- ---------------------------------------------------------------------------
-- 2. 积分流水（只增不改不删；幂等键 (user_id, biz_type, ref_no)）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS point_transactions (
    id             BIGSERIAL PRIMARY KEY,
    tx_no          varchar(64)  NOT NULL,
    user_id        bigint       NOT NULL,
    type           varchar(32)  NOT NULL,  -- earn_payment/earn_renewal/earn_activity/spend_exchange/adjust/expire
    direction      int          NOT NULL,  -- 1=获得 -1=消耗
    points         bigint       NOT NULL,  -- 变动积分（正数）
    balance_before bigint       NOT NULL DEFAULT 0,
    balance_after  bigint       NOT NULL DEFAULT 0,
    biz_type       varchar(32)  NOT NULL,  -- 幂等业务标识（order/bill/recharge/adjust/...）
    ref_no         varchar(64)  NOT NULL,  -- 关联单号（订单号/人工调整单号）
    remark         varchar(255) NOT NULL DEFAULT '',
    operator_id    bigint       NOT NULL DEFAULT 0,
    expire_at      timestamptz,            -- 预埋：积分有效期（过期回收任务用）
    created_at     timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_point_tx_no ON point_transactions (tx_no);
CREATE UNIQUE INDEX IF NOT EXISTS uk_point_biz ON point_transactions (user_id, biz_type, ref_no);
CREATE INDEX IF NOT EXISTS idx_point_tx_user_created ON point_transactions (user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_point_tx_type ON point_transactions (type);
CREATE INDEX IF NOT EXISTS idx_point_tx_expire ON point_transactions (expire_at);

-- ---------------------------------------------------------------------------
-- 3. 积分发放规则（规则可配，不在代码写死比例）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS point_rules (
    id                  BIGSERIAL PRIMARY KEY,
    code                varchar(50)   NOT NULL,
    name                varchar(100)  NOT NULL,
    scene               varchar(32)   NOT NULL,  -- order_purchase/order_renewal/bill_payment/activity
    earn_mode           varchar(20)   NOT NULL DEFAULT 'rate',  -- rate=按金额比例 fixed=固定值
    fixed_points        bigint        NOT NULL DEFAULT 0,
    points_per_yuan     numeric(12,4) NOT NULL DEFAULT 0,
    min_amount          numeric(15,2) NOT NULL DEFAULT 0,      -- 触发门槛（订单金额）
    max_points_per_order bigint       NOT NULL DEFAULT 0,      -- 单笔上限（0=不限）
    valid_days          int           NOT NULL DEFAULT 0,      -- 有效期天数（0=永久）
    status              int           NOT NULL DEFAULT 1,      -- 1=启用 0=停用
    sort_order          int           NOT NULL DEFAULT 0,
    remark              varchar(255)  NOT NULL DEFAULT '',
    created_at          timestamptz,
    updated_at          timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_point_rules_code ON point_rules (code);

-- 默认规则：下单得积分、续费双倍激励；账单支付给积分作为备选（默认停用，避免与下单积分重复计发）。
INSERT INTO point_rules (code, name, scene, earn_mode, fixed_points, points_per_yuan, min_amount, max_points_per_order, valid_days, status, sort_order, remark, created_at, updated_at)
VALUES
  ('order_purchase', '下单消费得积分', 'order_purchase', 'rate', 0, 1.0000, 0, 0, 0, 1, 1, '每消费 1 元得 1 积分', now(), now()),
  ('order_renewal',  '续费得积分',     'order_renewal',  'rate', 0, 2.0000, 0, 0, 0, 1, 2, '续费激励：每消费 1 元得 2 积分', now(), now()),
  ('bill_payment',   '账单支付得积分', 'bill_payment',   'rate', 0, 1.0000, 0, 0, 0, 0, 3, '备选规则：账单已由消费流水计发时启用会重复，默认停用', now(), now())
ON CONFLICT (code) DO NOTHING;

-- 幂等补正：规则若由早期 SQL 直插而未带时间戳，会显示为零值时间。
UPDATE point_rules SET created_at = now() WHERE created_at IS NULL;
UPDATE point_rules SET updated_at = now() WHERE updated_at IS NULL;

-- ---------------------------------------------------------------------------
-- 4. 退款双模式（doc36 §3.2）
--    balance = 退回余额（资金未流出平台，消费口径不变，无扣点）
--    channel = 原路退回（资金流出平台，渠道手续费不退，需在收入口径扣点）
-- ---------------------------------------------------------------------------
ALTER TABLE order_refunds
  ADD COLUMN IF NOT EXISTS refund_mode           varchar(20)   NOT NULL DEFAULT 'balance',
  ADD COLUMN IF NOT EXISTS fee_amount            numeric(15,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS net_amount            numeric(15,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS channel_refund_no     varchar(64)   NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS channel_refund_status varchar(20)   NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_order_refunds_mode ON order_refunds (refund_mode);

-- ---------------------------------------------------------------------------
-- 5. 账单分类 + 发票标记（doc36 §3.3/§3.4）
--    口径（用户确认）：total_amount = 消费 + 续费 - 原路退回本金 - 原路退回扣点。
--    余额退回不减少消费口径（资金仍在平台内，属钱包内部转移）。
-- ---------------------------------------------------------------------------
ALTER TABLE bills
  ADD COLUMN IF NOT EXISTS bill_type            varchar(20)   NOT NULL DEFAULT 'consumption',
  ADD COLUMN IF NOT EXISTS consume_amount       numeric(15,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS renewal_amount       numeric(15,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS channel_refund_amount numeric(15,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS refund_fee_amount    numeric(15,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS paid_at              timestamptz,
  ADD COLUMN IF NOT EXISTS invoice_status       varchar(20)   NOT NULL DEFAULT 'none',
  ADD COLUMN IF NOT EXISTS invoice_no           varchar(64)   NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS invoiced_at          timestamptz;

CREATE INDEX IF NOT EXISTS idx_bills_type ON bills (bill_type);
CREATE INDEX IF NOT EXISTS idx_bills_invoice_status ON bills (invoice_status);

-- ---------------------------------------------------------------------------
-- 6. 发票申请（预埋税务/邮件接口）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS invoice_requests (
    id           BIGSERIAL PRIMARY KEY,
    request_no   varchar(64)   NOT NULL,
    bill_id      bigint        NOT NULL DEFAULT 0,
    bill_no      varchar(64)   NOT NULL DEFAULT '',
    user_id      bigint        NOT NULL,
    invoice_type varchar(20)   NOT NULL DEFAULT 'normal',  -- normal=普票 special=专票
    title        varchar(200)  NOT NULL DEFAULT '',        -- 发票抬头
    tax_no       varchar(64)   NOT NULL DEFAULT '',        -- 税号
    amount       numeric(15,2) NOT NULL DEFAULT 0,
    email        varchar(128)  NOT NULL DEFAULT '',        -- 接收邮箱（预埋邮件下发）
    status       varchar(20)   NOT NULL DEFAULT 'pending', -- pending/issued/rejected
    channel      varchar(20)   NOT NULL DEFAULT 'manual',  -- manual=人工 tax_api=税控（预埋）
    external_no  varchar(128)  NOT NULL DEFAULT '',        -- 税控回执号（预埋）
    file_url     varchar(512)  NOT NULL DEFAULT '',        -- 发票文件（预埋下载）
    reject_reason varchar(255) NOT NULL DEFAULT '',
    operator_id  bigint        NOT NULL DEFAULT 0,
    issued_at    timestamptz,
    created_at   timestamptz,
    updated_at   timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_invoice_requests_no ON invoice_requests (request_no);
CREATE INDEX IF NOT EXISTS idx_invoice_requests_user ON invoice_requests (user_id);
CREATE INDEX IF NOT EXISTS idx_invoice_requests_status ON invoice_requests (status);
CREATE INDEX IF NOT EXISTS idx_invoice_requests_bill ON invoice_requests (bill_id);

-- ---------------------------------------------------------------------------
-- 7. 合并支付 + 检索索引（doc36 §3.5/§3.1）
--    biz_items: [{"biz_type":"order","biz_id":1,"biz_no":"...","amount_fen":100,"status":"paid"}]
--    item_count: 业务单数量（1 为普通单，>1 为合并支付）
-- ---------------------------------------------------------------------------
ALTER TABLE payment_orders
  ADD COLUMN IF NOT EXISTS biz_items  jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS item_count int   NOT NULL DEFAULT 1;

-- 业务单号反查订单/账单/充值（原复合索引 (biz_type,biz_id) 无法按 biz_no 命中）
CREATE INDEX IF NOT EXISTS idx_payment_orders_biz_no ON payment_orders (biz_no);

COMMIT;
