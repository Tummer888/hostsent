-- 116_reconcile_status_service_wording.sql
-- ============================================================================
-- 对账「状态向」站内信模板措辞修正（第二十一轮）。
--
-- 背景（用户实测误报）：控制台关机后本地 status=stopped、上游 domainstatus 仍 Active，
-- 对账把两者判成 status_mismatch，站内信标题写「运行状态与上游不同步」。
-- 根因是拿**上游服务状态**与**本地机器电源状态**机械比较 —— 财务型上游（zjmf）
-- 根本没有电源字段，关机只是机器没通电，服务与账单都还在。
-- 判据已在代码侧改为"服务维度 vs 服务维度"（reconcile/repository/reconcile_repo.go），
-- 本迁移只补齐文案：db.go 的默认模板种子对**已存在的 event 会跳过**，
-- 所以存量库必须靠迁移改，否则升级后管理员收到的仍是旧措辞。
--
-- 为什么不在本迁移里"清理误报台账"：不需要。台账的收敛由扫描器自动完成 ——
-- 新判据下这些行不再命中，下一轮扫描（10 分钟）会经 ResolveAbsent 置 resolved，
-- 误报台账已实证自行收敛；手工 UPDATE 反而会与扫描器抢同一批行。
-- 已 resolved 的历史行保留原值（那是当时的快照，不再展示、不参与判定）。
--
-- 回滚：
--   UPDATE notification_templates
--      SET title_tpl   = '运行状态与上游不同步',
--          content_tpl = '实例 {instance_mark}（{provider_name}）本地状态 {local_value} 与上游状态 {upstream_value} 不一致。请前往「资源管理 → 同步状态运维控制台 → 实例对账」核实服务可用性。',
--          updated_at  = now()
--    WHERE event = 'reconcile_status_mismatch';
-- ============================================================================

BEGIN;

UPDATE notification_templates
   SET title_tpl   = '服务状态与上游不一致',
       content_tpl = '实例 {instance_mark}（{provider_name}）本地服务 {local_value} 与上游服务 {upstream_value} 不一致。请前往「资源管理 → 同步状态运维控制台 → 实例对账」核实服务可用性。',
       updated_at  = now()
 WHERE event = 'reconcile_status_mismatch';

COMMIT;
