-- ============================================================================
-- 099_instances_customer_remark.sql
-- 实例「客户备注」列：与「管理员备注」分开存放。
--
-- 背景：
--   instances.remark 一直是**运营内部**备注（管理端「编辑备注」写入，用户在
--   任何界面都看不到）。但云主机是客户自己的资产，客户下单时常要把「这台机器
--   是给哪个客户/项目用的」写清楚；此前商品下单没有任何备注入口，运营只能
--   口口相传，续费/售后时无法回溯。
--
--   参考实现把两者分得很清楚（zjmf AgentController.php:430-431「代理商在资源池
--   处的客户备注 / 客户在代理商处的客户备注」，orders.notes 由 CartController
--   ::settle 从下单参数写入），因此这里也**不复用** remark：
--   混用会让运营的内部备注被客户看到，反过来客户填的说明也会污染运营视图。
--
-- 取值范围：客户在下单时填写的说明（orders.customer_remark 随履约带入实例），
--   以及后续自助修改。只读展示于用户端详情；管理端详情同时看到两份备注。
--
-- 回滚：ALTER TABLE instances DROP COLUMN IF EXISTS customer_remark;
-- ============================================================================

BEGIN;

ALTER TABLE instances ADD COLUMN IF NOT EXISTS customer_remark VARCHAR(255);
ALTER TABLE orders ADD COLUMN IF NOT EXISTS customer_remark VARCHAR(255);

COMMENT ON COLUMN instances.customer_remark IS '客户备注：客户下单填写或自助修改，与管理员备注（remark）分开';
COMMENT ON COLUMN orders.customer_remark IS '客户备注：下单时填写，履约时随实例落库';

COMMIT;
