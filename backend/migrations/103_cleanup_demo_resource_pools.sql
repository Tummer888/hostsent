-- ============================================================================
-- 103_cleanup_demo_resource_pools.sql
-- 清理演示期遗留的假资源池（与对接上游链路冲突的脏数据）。
--
-- 背景（本轮用户反馈）：
--   资源池与容量列表里还挂着几条演示期的假池 —— "魔方云·通用计算池"
--   "魔方云·高内存池" "华东 OpenStack 主池" "AWS 新加坡计算池" 等，
--   pool_type 为 compute/memory/storage。它们的问题：
--     1) upstream_id 是人造串（pool_mfy_east_01 / pool_os_east_01 …），
--        不是平台真实 ID，永远同步不到，也就永远刷不掉；
--     2) 所属渠道是演示数据（api_endpoint 指向 example.com / amazonaws.com /
--        占位域名），与真实的「魔方云测试接口」混在一起，让人以为有两条链路；
--     3) 带虚构容量（CPU 320 / 内存 2048），把汇总数字抬虚，掩盖真实水位。
--   对接上游（代理）商品本就不涉及这些资源状态，这些行只会造成误读。
--
-- 判定依据（关键）：**upstream_id 的命名形态**，而不是渠道名或池类型。
--   平台同步写入的真实池，upstream_id 是平台真实 ID（如 "1"）或节点形态（如 "node:2"）；
--   演示数据统一用 pool_* 前缀的人造串。按形态匹配最精确，且不会误伤真实渠道。
--
-- 幂等：按前缀匹配删除，重复执行第二次影响 0 行。
-- 回滚：被删的都是演示数据；如需恢复请从 migrations/backups/ 的历史备份取。
-- ============================================================================

BEGIN;

-- 先看一眼要删什么（psql 会打印，便于事后核对）：
SELECT p.id, p.provider_id, r.name AS provider_name, p.upstream_id, p.name, p.pool_type
FROM resource_pools p
LEFT JOIN resource_providers r ON r.id = p.provider_id
WHERE p.upstream_id LIKE 'pool\_%'
ORDER BY p.provider_id, p.id;

-- 删除演示期人造池：upstream_id 为 pool_* 前缀。
-- 真实同步的池不会用这个前缀（数字 ID 或 node:<id>），因此这条规则不会误伤。
DELETE FROM resource_pools WHERE upstream_id LIKE 'pool\_%';

COMMIT;
