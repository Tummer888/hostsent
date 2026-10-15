-- ============================================================================
-- 112_instance_transition_state.sql
-- 实例过渡态追踪（电源操作进行中）＋ 状态维度拆分的前提数据
--
-- 背景（用户反馈）：关机后详情页显示成「已暂停」，关机和暂停分不清；同时
--   重启/开机这类操作在下发后有一段"上游正在执行"的窗口，期间界面上看不到任何
--   变化 —— 运营以为没生效就反复点，同一台机器堆了多次重启指令。
--
-- 两个正交维度必须分开（这是本轮的根因）：
--   ① 机器电源态（power）：运行中 / 已关机 / 开机中 / 关机中 / 重启中 …
--      —— 机器本身通没通电，来自上游；
--   ② 账期服务态（service）：服务中 / 即将到期 / 已过期 / 宽限期 / 到期暂停 / 已销毁
--      —— 我们卖的这个服务有没有到期或被我方暂停。
--   此前前端把两者压成一句话，判据是 `stage === 'suspended' || status === 'stopped'`，
--   于是"手动关机"（status=stopped，账期正常）被显示成「已暂停、到期暂停中」。
--
-- 本迁移只加过渡态所需的列；电源态/服务态的拆分是纯代码改动（见下面说明）。
--
-- 为什么过渡态要落库而不是只靠前端 loading：上游下发是异步的（接口只回"已受理"），
--   真正生效要等十几秒到几分钟。这段窗口跨越了页面刷新与多标签页，只有服务端记住
--   "这台机器上有个操作正在进行"，才能做到：刷新后仍显示「重启中」、新指令被拦截。
--
-- 列语义：
--   transition_state   进行中的动作（starting/stopping/rebooting/reinstalling/
--                      resetting_password/rescue_enter/rescue_exit）；空 = 无进行中操作
--   transition_started_at 该动作的下发时间（审计与超时判定）
--   transition_until   过渡窗口截止时间：到此时间仍未回源确认，视为"结果未知"，
--                      不再阻止新指令（但会在界面提示上次结果未确认）
--
-- 幂等：ADD COLUMN IF NOT EXISTS；无数据回填（存量行无进行中操作，留空即正确）。
-- Go 侧：模型字段见 .../admin/resource/sync/model/sync.go；写入/清除见
--   .../admin/instance/service/instance_service.go（markTransition / clearTransition）。
-- 回滚：DROP COLUMN transition_state, transition_started_at, transition_until；
--   同步移除模型字段与 service 里的读写（不删也能跑，只是不再追踪过渡态）。
-- ============================================================================

BEGIN;

ALTER TABLE instances ADD COLUMN IF NOT EXISTS transition_state varchar(24);
ALTER TABLE instances ADD COLUMN IF NOT EXISTS transition_started_at timestamptz;
ALTER TABLE instances ADD COLUMN IF NOT EXISTS transition_until timestamptz;

COMMENT ON COLUMN instances.transition_state IS '进行中的操作（starting/stopping/rebooting/reinstalling/…）；空=空闲。用于过渡态展示与重复指令拦截';
COMMENT ON COLUMN instances.transition_started_at IS '进行中操作的下发时间';
COMMENT ON COLUMN instances.transition_until IS '过渡窗口截止时间：到期仍未回源确认则视为结果未知，不再拦截新指令';

-- 只给"正在过渡"的行建部分索引：绝大多数实例的该列为空，全表索引没有价值。
CREATE INDEX IF NOT EXISTS idx_instances_transition_active
    ON instances (transition_until) WHERE transition_state IS NOT NULL;

COMMIT;

-- 验收查询（人工执行）
--   列已建：
--   SELECT column_name FROM information_schema.columns
--    WHERE table_name='instances' AND column_name LIKE 'transition%';
--   期望：3 行
--   当前无过渡中的实例（存量数据不应有）：
--   SELECT count(*) FROM instances WHERE transition_state IS NOT NULL;
--   期望：0（执行一次开机/关机后应为 1，窗口过后回到 0）
