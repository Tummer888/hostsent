-- ============================================================================
-- 106_purge_orphan_sync_schedules.sql
-- 清理已删渠道遗留的同步调度行与增量游标。
--
-- 背景（本轮用户反馈）：
--   调度配置页里出现了渠道名对不上号的条目 —— 渠道已被删除，但它的
--   sync_schedules 行还在，页面上只能显示成「渠道 #1」这种裸 ID，
--   看上去像配置坏了、数据自相矛盾。
--
-- 根因：
--   渠道删除走 GORM 软删除（resource_providers.deleted_at），而
--   sync_schedules / sync_cursors 是独立表、没有外键级联，删除渠道时不会连带清理。
--   实际受影响：provider 1 / 16 / 17 / 19（都是测试与演示期建的渠道）。
--
-- 两个执行面（同一条演进）：
--   1) 存量清理 —— 本文件；等价实现见 internal/pkg/db/db.go 的 purgeOrphanSyncSchedules
--      （迁移目录不被 runner 执行，见 migrations/README.md，故启动期幂等回填是主路径）。
--   2) 防复发 —— 渠道删除路径已补连带清理
--      （providerService.Delete / DeleteWithCascade → purgeSyncArtifacts）。
--
-- 判定：provider_id 在 resource_providers 里没有「未删除」的同 id 行。
--   用 NOT EXISTS 而非 NOT IN：resource_providers 有软删除，语义是
--   「不存在未删除的同 id 渠道」，NOT EXISTS 直接表达且不受 NULL 影响。
--
-- 幂等：只匹配已删渠道的行，重复执行第二次影响 0 行。
-- 回滚：被删的都是无主调度配置（渠道已不存在，调度器也不会再执行它们）；
--       如需恢复，按 provider 重建渠道后调度行会由 EnsureSchedulesForProvider 自动补齐。
-- ============================================================================

BEGIN;

-- 先看一眼要删什么（psql 会打印，便于事后核对）：
SELECT s.id, s.provider_id, s.scope, s.enabled, s.last_status, p.name AS provider_name, p.deleted_at
FROM sync_schedules s
LEFT JOIN resource_providers p ON p.id = s.provider_id
WHERE NOT EXISTS (
    SELECT 1 FROM resource_providers p2
    WHERE p2.id = s.provider_id AND p2.deleted_at IS NULL
)
ORDER BY s.provider_id, s.scope;

-- 删除无主调度行。
DELETE FROM sync_schedules
WHERE NOT EXISTS (
    SELECT 1 FROM resource_providers p
    WHERE p.id = sync_schedules.provider_id AND p.deleted_at IS NULL
);

-- 增量游标同理：指向已删渠道的游标永远读不到，留着只是噪音。
DELETE FROM sync_cursors
WHERE NOT EXISTS (
    SELECT 1 FROM resource_providers p
    WHERE p.id = sync_cursors.provider_id AND p.deleted_at IS NULL
);

COMMIT;
