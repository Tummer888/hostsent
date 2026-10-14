-- ============================================================================
-- 105_orders_verify_fields.sql
-- 订单核验标记（运营人工核对开通结果）：verified_at / verified_by / verified_by_name / verify_remark。
--
-- 背景：
--   开通是「支付 → 上游下单 → 主机就绪 → 服务中」的自动链路，但运营还需要一道
--   人工核对：上游实际开出来的配置/价格/时长是否与订单一致（换机型、补差价、
--   上游错发等只能靠人看）。此前系统没有承载这道工序的地方。
--
--   设计选择：核验做成**订单上的独立标记**而不是新增订单状态。
--     · 核验不阻断开通/支付/退款任何一种流转，塞进 status 会把状态机、
--       统计口径（paidStatuses）、uc/前端状态映射、续费链路全部拖下水；
--     · 是否需要核验由系统配置 order_verify_required 控制（默认关闭）；
--       开启时「服务中且 verified_at 为空」在**读取期**推导为「待核验」，
--       不产生任何后台写入，也不影响资金与履约链路。
--
-- 幂等：IF NOT EXISTS，重复执行无副作用。
-- 顺序：列由 GORM AutoMigrate 启动时补齐对齐（Order.VerifiedAt 等已建字段）。
-- 回滚：ALTER TABLE orders DROP COLUMN IF EXISTS verified_at, ... (四列);
-- ============================================================================

BEGIN;

ALTER TABLE orders ADD COLUMN IF NOT EXISTS verified_at TIMESTAMP;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS verified_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS verified_by_name VARCHAR(50);
ALTER TABLE orders ADD COLUMN IF NOT EXISTS verify_remark VARCHAR(255);

COMMENT ON COLUMN orders.verified_at IS
  '人工核验时间（NULL=未核验）；仅标记，不参与订单状态机流转';
COMMENT ON COLUMN orders.verified_by IS
  '核验人（admins.id）；0 表示未核验';
COMMENT ON COLUMN orders.verified_by_name IS
  '核验人姓名快照（admins 改名/删除后仍可追溯）';
COMMENT ON COLUMN orders.verify_remark IS
  '核验备注（核对结论，如"配置与订单一致"）';

COMMIT;
