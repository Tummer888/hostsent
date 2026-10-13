-- ============================================================================
-- 097_instances_renewal_price.sql
-- 实例级续费价绑定（运营对单机的价格承诺）。
--
-- 背景：
--   后台代客下单时经常出现「首次价 ≠ 续费价」的成交口径：例如 4核4G 首开收 30 元，
--   后续每次续费按 18 元。此前 instances 表没有任何价格列，续费单价只能由
--   lifecycle 的 ResolveProduct 实时回查商品（products.price 或周期矩阵），
--   于是"这台机器续费 18"这种单机承诺无处安放——商品一调价，已承诺的机器跟着变，
--   与客服与客户谈定的价格不一致。
--
-- 本迁移给 instances 增加一列绑定价：
--   renewal_price decimal(15,2) NULL
--     · NULL  = 未绑定，续费按商品定价与折扣管线实时计算（全部存量行为不变）；
--     · 非 NULL = 该实例续费单价固定为此值（元/周期），续费算价优先取它。
--   刻意用 NULL 而不是 0 表示"未绑定"：0 是合法价（免费续费），两者语义必须分开。
--
-- 写入与消费：
--   · 写入：后台代客下单（admin create order）随请求带 renewal_price，落占位实例行；
--   · 消费：renewal_service.resolveRenewalQuote 优先取绑定价（跳过折扣管线）。
--
-- 幂等：IF NOT EXISTS，重复执行无副作用。
-- 顺序：本迁移先跑；列由 GORM AutoMigrate 在服务启动时补齐对齐
--      （Instance.RenewalPrice 与 DDL 双写，见 internal/pkg/db/db.go）。
-- 回滚：ALTER TABLE instances DROP COLUMN renewal_price;
--      （列可留空，回滚不影响既有数据，只是单机价格承诺失效。）
-- ============================================================================

ALTER TABLE instances
    ADD COLUMN IF NOT EXISTS renewal_price DECIMAL(15, 2) NULL;

COMMENT ON COLUMN instances.renewal_price IS
    '绑定的续费单价（元/周期）；NULL=未绑定，按商品定价实时计算';
