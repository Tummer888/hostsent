-- ============================================================================
-- 082_drop_upstream_balance_topups.sql
-- 删除「上游充值记录」表（078 建）：手工记账被账本同步取代（doc111 §5.4）。
--
-- 背景：082 之前，上游成本靠「期初余额 + 手工充值记录 − 期末余额」推算，
-- 运营必须手工补录每一次充值，否则余额差会被当成消耗。2026-10-10 接入上游账本
-- （mofangfinance 的 credit_record / v1/transactions/funds）后：
--   - 消费与充值都由 `upstream_ledger_entries` 自动同步（首次全量回填、之后增量）；
--   - 余额快照只保留「当前余额」用途（台账展示 + 低余额/到期告警）；
--   - 手工录入余额与充值的前后端入口全部下线，本表随之失去写入方与读取方。
--
-- 幂等：DROP TABLE IF EXISTS；表内数据为本轮验收产生的演示记录（已清空），
-- 删除不影响任何真实数据。回滚：由 078_cost_management.sql 重建该表结构。
-- ============================================================================

BEGIN;

DROP TABLE IF EXISTS upstream_balance_topups;

COMMIT;

-- 验收查询（人工执行）
--   表已不存在：
--   SELECT to_regclass('public.upstream_balance_topups');   -- 期望 NULL
