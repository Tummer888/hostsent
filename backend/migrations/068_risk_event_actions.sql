-- ============================================================================
-- 068_risk_event_actions
-- 风险事件处置流水（doc06 §4.3「风险事件处置联动」）。
--
-- 为什么建这张表（改造前的实际问题）：
--   risk_events 上只有一列 handle_note + handled_by + handled_at，任何一次操作都会
--   **覆盖**上一次的痕迹。于是运营点了「拉黑」再点「失效会话」，页面上只剩最后一次
--   的处置人，完全看不出这条事件被做过哪些管控。更糟的是「拉黑」「失效会话」这两个
--   动作此前**根本没有写回 risk_events**，点完列表状态仍显示「待处理」，运营以为没生效。
--
--   本表把每个动作记成一行，形成事件下的处置时间线：
--     level（调等级）/ handle（处置）/ ignore（忽略）/ blacklist（拉黑）/ revoke_sessions（失效会话）
--   risk_events.status 仍然只回答「这条待办关掉没有」，动作本身由本表承载 ——
--   两者是不同维度，混在一列里就会像改造前那样互相覆盖。
--
-- 幂等：CREATE TABLE / INDEX IF NOT EXISTS，重复执行安全。
-- 回滚：DROP TABLE IF EXISTS risk_event_actions;
-- ============================================================================

BEGIN;

CREATE TABLE IF NOT EXISTS risk_event_actions (
    id          BIGSERIAL   PRIMARY KEY,
    -- 不加外键约束：事件被归档/清理后流水仍应可查（追责场景要的就是历史），
    -- 且风控表的写入频率高，外键检查会拖慢登录旁路。
    event_id    BIGINT      NOT NULL,
    -- 动作码：level / handle / ignore / blacklist / revoke_sessions。
    action      VARCHAR(32) NOT NULL,
    operator_id BIGINT      NOT NULL DEFAULT 0,
    note        VARCHAR(255) NOT NULL DEFAULT '',
    -- 动作结果的 JSON 快照：拉黑了哪个维度哪个值、失效了几个会话、等级从什么改到什么。
    -- 存成文本而不是多列：不同动作的产物结构完全不同，拆列会变成一堆稀疏列。
    detail      TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 详情抽屉按事件查时间线（最近的在最前），列表页按一批事件批量取动作摘要。
CREATE INDEX IF NOT EXISTS idx_risk_event_actions_event
    ON risk_event_actions (event_id, created_at DESC);

COMMENT ON TABLE  risk_event_actions          IS '风险事件处置流水：每次处置动作一行，形成可追溯的处置时间线（doc06 §4.3）';
COMMENT ON COLUMN risk_event_actions.action   IS '动作码：level(调等级) / handle(处置) / ignore(忽略) / blacklist(拉黑) / revoke_sessions(失效会话)';
COMMENT ON COLUMN risk_event_actions.detail   IS '动作结果 JSON 快照（拉黑维度与值、失效会话数、等级变更前后）';

COMMIT;
