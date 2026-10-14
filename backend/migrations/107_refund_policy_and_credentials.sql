-- ============================================================================
-- 107_refund_policy_and_credentials.sql
-- 退款策略（按商品来源+分类的可退时限/免审开关）、退款审核凭据、订单开通时间。
--
-- 背景（用户需求）：
--   此前退款没有任何时限与来源概念，CreateRefund 只校验状态/金额/去向，且只有
--   管理端能发起。业务要求：
--     · 自营商品：24 小时内可申请，退回余额且符合条件时免人工审核自动退，
--       之后仍要一道事后复核（自动退出的钱必须有人回头核对）；
--     · 对接上游商品：一律先人工审核，通过后才退回；
--     · 时限与是否可退按「商品来源 + 分类」客制化（如某分类买了不退、
--       某分类 7 天内可退）；
--     · 原路退回（channel）目前没有自动退款接口，一律人工审核 + 上传凭据。
--
-- 三处结构变更：
--   1) orders.activated_at：退款时限起算点。pay_time 是「付钱」，本列是「交付」，
--      时限按交付时间算；存量按 pay_time 回填（唯一可用的近似值）。
--   2) refund_policies 新表：策略行（来源 + 分类 + 时限 + 免审 + 状态）。
--   3) order_refunds 增列：origin（申请来源）/ audit_remark（审核备注，此前收了
--      参数却从未落库）/ evidence_no（凭据外部单号）/ review_*（事后复核四件套）。
--   4) refund_attachments 新表：审核凭据附件（本地存储 + 鉴权下载）。
--
-- 幂等：IF NOT EXISTS，重复执行无副作用。表由 GORM AutoMigrate 建，
-- 本文件是演进记录与对齐依据（见 migrations/README.md）。
-- 回滚：DROP TABLE refund_policies, refund_attachments;
--       ALTER TABLE orders DROP COLUMN activated_at;
--       ALTER TABLE order_refunds DROP COLUMN origin, audit_remark, evidence_no,
--         review_state, review_remark, reviewed_by, reviewed_by_name, reviewed_at;
-- ============================================================================

BEGIN;

-- 1) 订单开通成功时间（退款时限起算点）
ALTER TABLE orders ADD COLUMN IF NOT EXISTS activated_at TIMESTAMP;
COMMENT ON COLUMN orders.activated_at IS
  '开通成功时间：订单转为服务中的时刻，退款时限策略按此时间起算（与 pay_time 区分）';

-- 存量回填：已开通/退款/完成的订单用支付时间近似（没有更可靠的来源）。
UPDATE orders
   SET activated_at = pay_time
 WHERE activated_at IS NULL
   AND pay_time IS NOT NULL
   AND status IN ('active', 'refunding', 'refunded', 'completed');

-- 2) 退款策略表
CREATE TABLE IF NOT EXISTS refund_policies (
    id            BIGSERIAL PRIMARY KEY,
    source_mode   VARCHAR(16) NOT NULL DEFAULT '',   -- self / upstream / ''（不限）
    category_id   BIGINT      NOT NULL DEFAULT 0,    -- 0 = 该来源下全部商品；含子孙分类
    refundable    BOOLEAN     NOT NULL DEFAULT FALSE,-- 是否允许自助申请
    window_hours  INTEGER     NOT NULL DEFAULT 0,    -- 可退时限（小时）；0 = 不限
    auto_approve  BOOLEAN     NOT NULL DEFAULT FALSE,-- 免人工审核自动退款
    status        VARCHAR(16) NOT NULL DEFAULT 'active',
    sort_order    INTEGER     NOT NULL DEFAULT 0,
    remark        VARCHAR(255),
    created_at    TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_refund_policies_source_category
    ON refund_policies (source_mode, category_id);

COMMENT ON TABLE refund_policies IS
  '退款策略：按商品来源+分类配置可退时限与免审开关；未命中策略的商品不可自助退款';

-- 3) 退款单增列
ALTER TABLE order_refunds ADD COLUMN IF NOT EXISTS origin VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE order_refunds ADD COLUMN IF NOT EXISTS audit_remark VARCHAR(255);
ALTER TABLE order_refunds ADD COLUMN IF NOT EXISTS evidence_no VARCHAR(128);
ALTER TABLE order_refunds ADD COLUMN IF NOT EXISTS review_state VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE order_refunds ADD COLUMN IF NOT EXISTS review_remark VARCHAR(255);
ALTER TABLE order_refunds ADD COLUMN IF NOT EXISTS reviewed_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE order_refunds ADD COLUMN IF NOT EXISTS reviewed_by_name VARCHAR(50);
ALTER TABLE order_refunds ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMP;

-- 存量退款单都来自管理端。
UPDATE order_refunds SET origin = 'admin' WHERE origin = '';
CREATE INDEX IF NOT EXISTS idx_order_refunds_review_state ON order_refunds (review_state);

COMMENT ON COLUMN order_refunds.origin IS '申请来源：admin=运营发起 / user=客户自助申请';
COMMENT ON COLUMN order_refunds.audit_remark IS '审核备注（审核依据）';
COMMENT ON COLUMN order_refunds.evidence_no IS '退款凭据外部单号（转账/渠道流水号），查账定位用';
COMMENT ON COLUMN order_refunds.review_state IS '事后复核状态：''（无需）/pending（待复核）/done（已复核）';

-- 4) 审核凭据附件
CREATE TABLE IF NOT EXISTS refund_attachments (
    id          BIGSERIAL PRIMARY KEY,
    refund_id   BIGINT       NOT NULL,
    file_name   VARCHAR(255) NOT NULL,
    file_url    VARCHAR(512) NOT NULL,   -- 存储相对路径，下载走鉴权端点
    file_size   BIGINT       NOT NULL DEFAULT 0,
    file_type   VARCHAR(64),
    uploader_id BIGINT       NOT NULL DEFAULT 0,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_refund_attachments_refund ON refund_attachments (refund_id);

COMMENT ON TABLE refund_attachments IS '退款审核凭据附件（人工退款凭证，资金审计材料）';

COMMIT;
