-- 055_bill_source_split.sql
--
-- 背景（用户反馈）：人工代充值充两次，两次落在同一张账单上、金额累加。
-- 根因是账单只有「按 (user_id, period) 归集」一种形态：
--   uk_bills_user_period 唯一索引 + GenerateForUser 按期重算 upsert，
--   决定了同一账期的多笔充值必然被折叠成一行。
--
-- 修复思路：账单分两类，用 source_type 区分。
--   period   按期归集的消费账单：一个用户一个账期一张（仍受 (user_id, period) 唯一约束）。
--   recharge 单笔充值账单：一笔充值一张，source_no = 充值单号（recharge_no），可追溯、可逐笔核对。
--
-- 充值账单不进 total_amount（充值是平台收到的钱，不是欠款），金额记在 recharge_amount。
-- 因此按期账单不再承载充值额：本期充值额改由各自的充值账单表达，避免两边重复统计。
--
-- 回滚：DROP INDEX IF EXISTS uk_bills_source;
--       DROP INDEX IF EXISTS uk_bills_user_period;
--       CREATE UNIQUE INDEX IF NOT EXISTS uk_bills_user_period ON bills (user_id, period);
--       DELETE FROM bills WHERE source_type = 'recharge';
--       ALTER TABLE bills DROP COLUMN IF EXISTS source_no;
--       ALTER TABLE bills DROP COLUMN IF EXISTS source_type;
--       （回滚前须先处理同账期多行：按月合并 recharge_amount 后再重建全量唯一索引。）

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. 账单来源列
-- ---------------------------------------------------------------------------
ALTER TABLE bills ADD COLUMN IF NOT EXISTS source_type varchar(20) NOT NULL DEFAULT 'period';
ALTER TABLE bills ADD COLUMN IF NOT EXISTS source_no   varchar(64) NOT NULL DEFAULT '';

-- ---------------------------------------------------------------------------
-- 2. 唯一约束改部分索引
--    全量 (user_id, period) 唯一会让同账期的第二笔充值无处落脚；
--    改为只约束 period 类账单，充值类账单改由 (source_type, source_no) 约束幂等。
-- ---------------------------------------------------------------------------
DROP INDEX IF EXISTS uk_bills_user_period;
CREATE UNIQUE INDEX IF NOT EXISTS uk_bills_user_period ON bills (user_id, period) WHERE source_type = 'period';
CREATE UNIQUE INDEX IF NOT EXISTS uk_bills_source ON bills (source_type, source_no) WHERE source_no <> '';

-- ---------------------------------------------------------------------------
-- 3. 存量口径修正：按期账单不再承载充值额
--    充值额改由第 4 步补开的独立充值账单表达。
-- ---------------------------------------------------------------------------
UPDATE bills SET recharge_amount = 0, updated_at = now()
 WHERE source_type = 'period' AND recharge_amount <> 0;

-- 旧口径下当期只有充值时会把这行分类成 recharge；充值额清掉后按消费构成重判，
-- 否则一张没有任何充值额的消费账单还挂着「充值账单」的标签。
UPDATE bills SET bill_type = CASE
        WHEN renewal_amount > 0 AND consume_amount > 0 THEN 'mixed'
        WHEN renewal_amount > 0 THEN 'renewal'
        ELSE 'consumption'
    END,
    updated_at = now()
 WHERE source_type = 'period' AND bill_type = 'recharge';

-- 清掉旧口径留下的空壳：当期只有充值、没有任何消费/退款，金额全为 0 的按期账单
-- 不含任何信息（充值额已移到独立账单），留着只会在用户账单页显示一张「待缴 0 元」。
-- 已挂开票申请的不删：申请单以 bill_id 指向它，删了会让发票记录指向空账单。
DELETE FROM bills b
 WHERE b.source_type = 'period'
   AND b.total_amount = 0
   AND b.consume_amount = 0
   AND b.renewal_amount = 0
   AND b.refund_amount = 0
   AND b.channel_refund_amount = 0
   AND b.recharge_amount = 0
   AND b.status <> 'paid'
   AND NOT EXISTS (SELECT 1 FROM invoice_requests ir WHERE ir.bill_id = b.id);

-- ---------------------------------------------------------------------------
-- 4. 存量已到账充值补开独立账单
--    幂等：按 (source_type='recharge', source_no=recharge_no) 判重；
--    bill_no 由 paid_at + 充值单 id 拼成，重复执行不会撞唯一键。
-- ---------------------------------------------------------------------------
INSERT INTO bills (
    bill_no, user_id, period, total_amount, refund_amount, status, bill_type,
    recharge_amount, paid_amount_fen, paid_method, paid_at, detail,
    source_type, source_no, created_at, updated_at
)
SELECT 'BILLR' || to_char(COALESCE(r.paid_at, r.created_at), 'YYYYMMDDHH24MISS') || lpad(r.id::text, 5, '0'),
       r.user_id,
       to_char(COALESCE(r.paid_at, r.created_at), 'YYYYMM'),
       0, 0, 'paid', 'recharge',
       r.amount, round(r.amount * 100)::bigint, r.method, COALESCE(r.paid_at, r.created_at),
       jsonb_build_object('recharge', r.amount, 'recharge_no', r.recharge_no, 'method', r.method),
       'recharge', r.recharge_no, now(), now()
  FROM recharges r
 WHERE r.status = 'success'
   AND NOT EXISTS (
       SELECT 1 FROM bills b WHERE b.source_type = 'recharge' AND b.source_no = r.recharge_no
   );

COMMIT;
