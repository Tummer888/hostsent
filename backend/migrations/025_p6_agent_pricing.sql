-- 025_p6_agent_pricing.sql
-- P6-01/P6-02：代理价策略绑定 + 佣金自动计提的订单幂等键。
-- 幂等：可重复执行（ADD COLUMN IF NOT EXISTS / CREATE INDEX IF NOT EXISTS）。

-- 代理等级绑定折扣策略（P6-01）：算价管线第一步命中 agent → agent_levels.price_policy_id。
ALTER TABLE agent_levels
  ADD COLUMN IF NOT EXISTS price_policy_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_agent_levels_price_policy_id ON agent_levels (price_policy_id);

-- 佣金关联订单（P6-02）：与 agent_id 组成唯一键，保证订单完成自动计提幂等。
-- 历史手工佣金无关联订单，保持 NULL（Postgres 唯一索引对 NULL 不去重，不会互相冲突）。
ALTER TABLE distribution_commissions
  ADD COLUMN IF NOT EXISTS order_id BIGINT;
CREATE UNIQUE INDEX IF NOT EXISTS uk_commissions_order_agent
  ON distribution_commissions (order_id, agent_id);
