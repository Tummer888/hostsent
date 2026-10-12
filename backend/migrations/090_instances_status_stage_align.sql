-- ============================================================================
-- 090_instances_status_stage_align.sql
-- 收敛实例 status 与 lifecycle_stage 的存量矛盾数据。
--
-- 背景：两列语义不同，但都是「实例当前处境」的一面，必须互相自洽：
--   instances.status          上游/平台服务状态：running/stopped/creating/error/deleting/deleted
--   instances.lifecycle_stage 本系统到期处置阶段：active/grace/suspended/destroyed
--
-- 写库路径此前不对称，实测出三类矛盾行（截至 2026-10-12）：
--   1) status=deleted 而 stage=active/grace/suspended（9 行）
--      手动销毁只落状态、不落阶段（instance_service.Destroy）。
--   2) stage=destroyed 而 status=running（17 行）
--      到期推进器只落阶段、不落状态（lifecycle_advancer.setStage）。
--   3) stage=suspended 而 status=running（4 行）
--      同上：推进器暂停只落阶段。
--
-- 代码侧（同批提交）已把两条写入路径对齐：
--   - 推进器 setStageWithStatus 一次写两列；手动 Destroy 同步落 stage=destroyed。
--   - 同步/回源遇 stage=destroyed 不再照抄上游 status（否则软删残留会把已销毁实例复活）。
-- 本迁移只负责把历史行收敛到自洽值。
--
-- 收敛方向按「哪个字段反映了真实发生过的动作」定：
--   status=deleted 只可能由我方 Destroy 写入 → 销毁确实发生过 → stage 应补为 destroyed；
--   stage=destroyed 是本系统认定的终态 → status 应补为 deleted。
--   （上游适配器从不产出 deleted，故不存在"上游删了但我们没销毁"的误判风险。）
--
-- 幂等：全部为条件 UPDATE，重复执行无副作用。
-- 回滚：无需回滚——收敛为一致值是修正，不是可逆配置。
-- ============================================================================

BEGIN;

-- 1) 销毁确实发生过（status=deleted）但阶段没跟上：补终态阶段。
UPDATE instances
SET lifecycle_stage = 'destroyed', updated_at = now()
WHERE status = 'deleted'
  AND coalesce(lifecycle_stage, '') <> 'destroyed';

-- 2) 阶段已判终态但状态没跟上：补 deleted（同时覆盖第 1 步刚补齐的行，幂等）。
UPDATE instances
SET status = 'deleted', updated_at = now()
WHERE lifecycle_stage = 'destroyed'
  AND status <> 'deleted';

-- 3) 已暂停：服务状态应为 stopped（deleted 终态优先，上一步已处理）。
UPDATE instances
SET status = 'stopped', updated_at = now()
WHERE lifecycle_stage = 'suspended'
  AND status NOT IN ('stopped', 'deleted');

COMMENT ON COLUMN instances.status IS
    '上游/平台服务状态：running/stopped/creating/error/deleting/deleted（由同步回源与本系统动作共同维护）';
COMMENT ON COLUMN instances.lifecycle_stage IS
    '本系统到期处置阶段：active（服务中）/ grace（宽限期）/ suspended（已暂停）/ destroyed（终态，已销毁）';

COMMIT;

-- 验收查询（人工执行）
--   SELECT count(*) FROM instances WHERE lifecycle_stage='destroyed' AND status<>'deleted';   -- 期望 0
--   SELECT count(*) FROM instances WHERE status='deleted' AND coalesce(lifecycle_stage,'')<>'destroyed'; -- 期望 0
--   SELECT count(*) FROM instances WHERE lifecycle_stage='suspended' AND status='running';    -- 期望 0
--   SELECT lifecycle_stage, status, count(*) FROM instances GROUP BY 1,2 ORDER BY 3 DESC;
