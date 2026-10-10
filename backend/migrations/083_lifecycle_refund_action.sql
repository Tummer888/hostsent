-- ============================================================================
-- 083_lifecycle_refund_action.sql
-- 订单退款审核通过后对关联实例的处置策略（doc61 §8 第 2 条）。
--
-- 背景：doc61 §8 写「仿 SetRefundHook 增加实例侧 hook，退款通过后按规则停服/销毁」
--   但从头到尾没有定义「规则」；而退款既可能全额（客户不要机器了）也可能部分
--   （补偿），自动销毁线上实例属不可逆动作。因此把规则外置成一列策略：
--
--   lifecycle_policies.refund_action  none（默认）/ suspend / destroy
--
-- 默认 none 是刻意的：升级当天行为与升级前完全一致（退款只走资金链路），
--   运营显式选择后才生效。只有**全额退款**才处置实例，部分退款一律不动机器
--   （该规则刻在代码里，不随配置改变）。
--
-- 幂等：ADD COLUMN IF NOT EXISTS + 存量行补默认值。
-- 回滚：ALTER TABLE lifecycle_policies DROP COLUMN IF EXISTS refund_action;
-- ============================================================================

BEGIN;

ALTER TABLE lifecycle_policies
    ADD COLUMN IF NOT EXISTS refund_action VARCHAR(16) NOT NULL DEFAULT 'none';

UPDATE lifecycle_policies SET refund_action = 'none' WHERE refund_action IS NULL OR refund_action = '';

COMMENT ON COLUMN lifecycle_policies.refund_action IS
    '订单全额退款审核通过后对关联实例的动作：none（默认，不动作）/ suspend（暂停）/ destroy（销毁，不可逆）';

COMMIT;

-- 验收查询（人工执行）
--   SELECT refund_action FROM lifecycle_policies WHERE id = 1;
--   期望：none
