-- 023_p4_instance_actor.sql
-- P4-09：实例「操作人」列。子账号下单开通的实例记录真实操作人（子账号 ID），
-- 资源归属仍为主账号（user_id），actor_user_id 仅用于展示与追溯。
-- 幂等：可重复执行。

ALTER TABLE instances ADD COLUMN IF NOT EXISTS actor_user_id BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_instances_actor_user_id ON instances (actor_user_id);

-- 历史数据回填：把已存在实例的操作人兜底为其归属账号（无更精确来源）。
UPDATE instances SET actor_user_id = user_id WHERE actor_user_id = 0;
