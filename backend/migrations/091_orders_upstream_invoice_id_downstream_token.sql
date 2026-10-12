-- ============================================================================
-- 091_orders_upstream_invoice_id_downstream_token.sql
--
-- 背景（实测事故，2026-10-12）：上游（zjmf）开通是「下单 → 结算生成账单 → 余额支付」
-- 三步。旧履约逻辑在余额不足时把整段流程当失败重试，每次重试都重新下单结算 ——
-- 一个订单在上游连生成 5 张待支付账单（余额不足一张也没付成），且每张账单都要人工
-- 去上游后台清理。双根因：
--   1) 账单 id 没有落库，重试时无从复用；
--   2) 上游协议的幂等握手（cart/clear 带 downstream 三件套按 token 反查既有单据）
--      从未被使用 —— 旧代码一直传空 token，上游 `LIKE '%%'` 误命中无关主机后
--      我们忽略其"已开通"答复继续下单。
--
-- 修复：
--   a) orders 增加 upstream_invoice_id：上游结算一返回账单 id 就立即落库（在支付之前），
--      支付失败后的重试带着该 id 走「支付同一张账单」，不再重复下单；
--   b) orders 增加 downstream_token（32 位，首次履约生成后固定）：下游身份三件套之一，
--      下单前 cart/clear 握手据此找回既有账单/主机；
--   c) 余额不足等 4xx/业务码错误改为首次即转人工（provision_tasks.status=manual），
--      不再自动重试；运营充值后点「重新开通」会把 manual 任务重置为 pending 重跑。
--
-- 执行方式：本目录迁移不被独立 runner 执行（见 migrations/README.md），
-- 两列由 internal/pkg/db/db.go 的 AutoMigrate（ordermodel.Order）在启动期幂等添加。
-- 本文件是演进记录与人工执行依据，两者口径必须一致。
--
-- 附带：系统配置新增 site.public_url（本站对外地址，握手时告知上游"本站在哪"），
-- 由 seedSystemConfigs 幂等种入。
-- ============================================================================

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS upstream_invoice_id bigint NOT NULL DEFAULT 0;

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS downstream_token varchar(32) NOT NULL DEFAULT '';

COMMENT ON COLUMN orders.upstream_invoice_id IS
    '本次开通在（zjmf 类）上游生成的账单 id；重试复用同一张账单再支付，避免重复下单';
COMMENT ON COLUMN orders.downstream_token IS
    '上游对接令牌（32 位，首次履约生成后固定）；上游据此反查既有账单/主机（幂等握手），重试必须沿用同一 token';

