-- ============================================================================
-- 098_orders_renewal_price.sql
-- 订单上的续费价承诺（代客下单随单锁定，开通后落到实例）。
--
-- 背景：
--   097 给 instances 加了 renewal_price 绑定价，但实例行不总是随订单立即产生：
--     · 下单即建「待开通」占位实例（assembly_instance_pending.go）——需要从订单取价；
--     · 开通成功由异步 worker 把占位行转正（AdoptPendingInstance）——worker 手里
--       只有订单（重读 orders），拿不到创建请求。
--   因此绑定价必须在订单上留一份快照，否则异步链路一断，运营承诺的续费价就丢了。
--
-- 本迁移给 orders 增加一列：
--   renewal_price decimal(15,2) NULL
--     · NULL  = 未指定，实例续费按商品定价实时算（全部存量行为不变）；
--     · 非 NULL = 随该订单开通的实例绑定此续费单价（元/周期）。
--   用 NULL 而非 0 区分"未指定"与"0 元续费"。
--
-- 幂等：IF NOT EXISTS，重复执行无副作用。
-- 顺序：本迁移先跑；列由 GORM AutoMigrate 在服务启动时补齐对齐
--      （Order.RenewalPrice 与 DDL 双写，见 internal/pkg/db/db.go）。
-- 回滚：ALTER TABLE orders DROP COLUMN renewal_price;
-- ============================================================================

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS renewal_price DECIMAL(15, 2) NULL;

COMMENT ON COLUMN orders.renewal_price IS
    '随单绑定的续费单价（元/周期）；NULL=未指定，开通后落 instances.renewal_price';
