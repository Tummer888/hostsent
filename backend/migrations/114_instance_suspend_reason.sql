-- ============================================================================
-- 114_instance_suspend_reason.sql
-- 实例暂停原因分类（到期暂停 / 违规暂停 / 管理员暂停）＋ 取消状态
--
-- 背景（用户反馈）：状态列表里需要一个「有违规」——管理员在暂停时写明原因，
--   因违规被暂停的实例要显示成违规而不是笼统的「已暂停」。
--   此前 Suspend 收 reason 参数，但只写进了 instance_operations 流水，
--   实例行上没有任何字段能区分"到期没续费自动暂停"与"管理员因违规暂停"，
--   于是界面只能一律显示「到期暂停」—— 对违规客户是错误信息。
--
-- 设计：加一列 suspend_reason 存**分类**（不是自由文本），取值收敛成三个：
--   expired  —— 到期未续费被平台自动暂停（生命周期推进器写入）
--   violation—— 管理员因违规暂停（人工介入，客户需联系客服）
--   manual   —— 管理员因其他原因暂停（工单、风控复核等）
--
-- 为什么不直接存自由文本、界面直接显示：
--   1. 界面需要按原因给不同的下一步提示（违规说"联系客服"，到期说"续费即恢复"），
--      靠文案匹配自由文本既脆弱又会被改写；
--   2. 上游同步、推进器复位等自动化路径需要结构化判断，文本判不了。
-- 自由文本仍在 instance_operations.params.reason 里完整保留（审计用途）。
--
-- 同时新增 cancelled 状态语义（见下方注释）：订单未付款超时/被取消时，
-- 占位实例不再直接物理删除，而是落 cancelled 让运营在列表里看到流失。
-- 本迁移只加列与注释，不改变既有行的行为（存量行 suspend_reason 为空）。
-- ============================================================================

BEGIN;

-- 暂停原因分类（空 = 未暂停，或暂停早于本迁移、原因未知）。
ALTER TABLE instances ADD COLUMN IF NOT EXISTS suspend_reason varchar(24);

COMMENT ON COLUMN instances.suspend_reason IS
    '暂停原因分类：expired=到期自动暂停 / violation=违规暂停 / manual=管理员其他暂停；空=未暂停或原因未知';

-- 部分索引：只有被暂停的行会查这一列（列表按"违规"筛选用），
-- 绝大多数行该列为空，全表索引没有价值。
CREATE INDEX IF NOT EXISTS idx_instances_suspend_reason
    ON instances (suspend_reason) WHERE suspend_reason IS NOT NULL;

-- 存量回填：阶段已是 suspended 但没有原因的行，按**唯一能自动判定的**判据回填 expired
-- （到期暂停是推进器按 expire_at 自动写的；人工暂停不可能从数据里认出来）。
--
-- 只回填"确实已过到期时间"的行：阶段是 suspended 但 expire_at 仍在未来，
-- 说明是人工暂停（或数据异常），留空比猜一个错误原因好 —— 界面会退化为
-- 「已暂停」，而不是谎报「到期暂停」。
UPDATE instances
   SET suspend_reason = 'expired'
 WHERE lifecycle_stage = 'suspended'
   AND suspend_reason IS NULL
   AND expire_at IS NOT NULL
   AND expire_at < now();

-- 验收查询（人工执行）
--   列已建：
--   SELECT column_name FROM information_schema.columns
--    WHERE table_name='instances' AND column_name='suspend_reason';
--   期望：1 行
--   回填后各原因计数：
--   SELECT COALESCE(suspend_reason,'(空)') AS reason, count(*)
--     FROM instances WHERE lifecycle_stage='suspended' GROUP BY 1;
--   期望：到期暂停的行落 expired；人工暂停（expire_at 未来或为空）落 (空)

COMMIT;

-- 回滚说明：
--   ALTER TABLE instances DROP COLUMN IF EXISTS suspend_reason;
--   DROP INDEX IF EXISTS idx_instances_suspend_reason;
-- （回滚会丢失暂停原因分类；instance_operations 里的原始 reason 文本不受影响）
