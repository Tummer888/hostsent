-- ============================================================================
-- 094_upstream_push_index_dedup.sql
-- 清扫 093 遗留的一个重复索引。
--
-- 背景：093 用 `CREATE INDEX IF NOT EXISTS idx_tickets_upstream_tid` 建了
--   tickets(upstream_tid) 的普通索引；同一批代码里 GORM 的 autoIndex 又为该列
--   生成了一个 `idx_tickets_upstream_t_id`（gorm 的命名截断规则不同）。两个索引
--   列相同、功能重复，写入时白白多维护一棵 B 树。
--   后续已把模型上的 tag 固定为 index:idx_tickets_upstream_tid，GORM 不再另建名，
--   这里把历史库上那条多余索引删掉。
--
-- 幂等：DROP INDEX IF EXISTS，重复执行无副作用。
-- 回滚：无需回滚（索引可随时重建，且两个索引等价）。
-- ============================================================================

BEGIN;

DROP INDEX IF EXISTS idx_tickets_upstream_t_id;

COMMIT;

-- 验收查询（人工执行）
--   SELECT indexname FROM pg_indexes WHERE tablename = 'tickets' AND indexname LIKE '%upstream%';
--   期望：只有 idx_tickets_upstream_tid 一行
