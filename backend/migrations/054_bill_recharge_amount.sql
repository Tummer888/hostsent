-- ============================================================================
-- 054_bill_recharge_amount.sql
-- 背景：
--   管理端「用户列表 → 充值」此前直接调用人工调账（WalletService.Adjust），
--   只写一条 type=adjust 的资金流水：既不生成充值单，也不归集账单。结果是
--   同一笔钱在财务「充值管理」页查不到、在用户端「我的充值单」查不到，账单里
--   也看不到充值额 —— doc36 §3.4 定义了 bill_type=recharge 却从未有数据写入。
--
--   本迁移为 bills 增加 recharge_amount 列，承载「本期充值合计」，作为充值侧
--   的对账口径；充值到账后由充值服务触发当期账单归集（recharge_service）。
--
-- 口径（与 doc36 §3.4 一致，且不改动既有应结口径）：
--   total_amount 仍是消费口径（consume − 原路退回本金），充值**不参与**应结
--   金额 —— 充值是把钱打进平台，不是用户欠款，计入应结会让账单虚高。
--   recharge_amount 单独成列，仅作展示与对账。
--   有充值但当期无消费时 bill_type 记 recharge，有消费时保持原分类不变。
--
-- 幂等：ADD COLUMN IF NOT EXISTS，可重复执行。
-- 回滚：ALTER TABLE bills DROP COLUMN IF EXISTS recharge_amount;
-- ============================================================================

BEGIN;

ALTER TABLE bills ADD COLUMN IF NOT EXISTS recharge_amount numeric(15,2) NOT NULL DEFAULT 0;

COMMIT;
