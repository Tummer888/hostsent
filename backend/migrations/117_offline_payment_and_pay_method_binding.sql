-- ============================================================================
-- 117_offline_payment_and_pay_method_binding.sql
-- 场外收款人工入账（确认收款）+ 订单下单即绑定支付方式。
--
-- 背景（用户需求）：
--   客户没走线上收银台（对公转账/线下现金）时，钱到了公司账户却没有支付台账，
--   后台也无法把待支付订单改为已支付。业务要求：
--     · 下单那一刻订单就要带上支付方式（用户在用户中心设的默认为第一优先，
--       没有则用平台默认渠道），使「订单 ↔ 支付方式」强绑定；
--     · 后台可对未支付订单人工入账：强制选支付方式、可改实收金额与到账时间、
--       可填收款凭证号、可选是否发提醒（邮件/短信/站内信）；
--     · 入账要进支付台账（payment_orders），后续退款/对账可追溯。
--
-- 两处结构变更：
--   1) payment_orders.origin：支付单来源（'' 收银台 / 'offline' 后台人工入账）。
--      人工入账单必须与渠道回调单区分：原路退回时没有渠道接口可调（只能人工汇款），
--      渠道对账也不该把人工入账单算进去（永远对不平）。
--   2) payment_orders.operator_id：人工入账的登记人（admins.id）。
--      场外收款是"人在系统外收了钱"，谁登记必须可追。
--
-- 注意：orders.pay_method 列早已存在（迁移前即有），本次只是改变写入时机
-- （下单即写，含待支付单），无需改表。
-- 列由 AutoMigrate 按模型补齐，本文件为记录文件。
-- ============================================================================

-- 幂等：ADD COLUMN IF NOT EXISTS / CREATE INDEX IF NOT EXISTS。

ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS origin varchar(16) NOT NULL DEFAULT '';
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS operator_id bigint NOT NULL DEFAULT 0;

COMMENT ON COLUMN payment_orders.origin IS '支付单来源：空=收银台渠道下单；offline=后台人工入账（场外收款）';
COMMENT ON COLUMN payment_orders.operator_id IS '人工入账登记人（admins.id）；渠道单为 0';

-- 场外收款的原路退回：退款单标记待人工汇款（区别于 failed=已回落余额回补）。
COMMENT ON COLUMN order_refunds.channel_refund_status IS
    '渠道退款状态：空/failed=未流出平台（failed 已回落余额）；pending/success=渠道流出；manual=待财务线下汇款（场外收款的原路退回）';

-- origin 上的检索索引：对账与退款分流都要按来源筛已支付单。
CREATE INDEX IF NOT EXISTS idx_payment_orders_origin ON payment_orders (origin) WHERE origin <> '';
