-- ============================================================================
-- 059_agent_levels_and_group_purify.sql
-- 落地 doc108：用户组回归纯分类，代理独立成"等级 + 折扣矩阵"体系。
--
-- 背景（需求方 2026-10-05 确认）：
--   1) 用户组只做分类，不涉及折扣；
--   2) 代理按等级管理折扣，"最高折扣"= 数值最小、让利最大；
--   3) 用户组保持单维（一个用户只属于一个组）。
--
-- 本次做四件事：
--   A. 新增 agent_levels + agent_level_discounts（代理等级与折扣矩阵）；
--   B. users 增 agent_level_id（NULL = 非代理）；
--   C. product_categories 增 cost_rate（分类级拿货折扣，作为毛利校验基准）；
--   D. user_groups 去折扣化：先把已绑定的折扣策略承接为代理等级，再删
--      price_policy_id / is_agent_group，最后清理演示组。
--
-- 为什么把折扣从用户组搬走：改造前 user_groups.price_policy_id 是"这客户打几折"
--   的唯一来源（doc81 D3），但用户组同时被当成"客户分类"，改分类会误伤价格。
--   代理域（迁移 026）下线时把代理折扣并进了用户组，而算价管线一直留着
--   pricing.Deps.AgentRule 未接线的后路、退役表 retired_agent_levels_20260910
--   本身就带 price_policy_id —— 本次即把折扣来源迁回代理等级。
--
-- 幂等：ADD COLUMN IF NOT EXISTS / CREATE TABLE IF NOT EXISTS / 条件 UPDATE，
--       可重复执行。
-- 顺序：先跑本迁移，再重启后端（AutoMigrate 补模型列 + seed 补默认代理等级与菜单）。
-- 回滚：
--   DROP TABLE IF EXISTS agent_level_discounts;
--   DROP TABLE IF EXISTS agent_levels;
--   ALTER TABLE users              DROP COLUMN IF EXISTS agent_level_id;
--   ALTER TABLE product_categories DROP COLUMN IF EXISTS cost_rate;
--   ALTER TABLE user_groups ADD COLUMN IF NOT EXISTS price_policy_id BIGINT;
--   ALTER TABLE user_groups ADD COLUMN IF NOT EXISTS is_agent_group BOOLEAN NOT NULL DEFAULT false;
--   （回滚会丢失代理等级配置；user_groups 的两列只能恢复结构、无法恢复绑定值，
--     回滚前请先导出 agent_levels / agent_level_discounts 与 users.agent_level_id。
--     若迁移前存在绑定策略的用户组，其折扣已被承接为 code=migrated_policy_* 的代理等级 ——
--     回滚后这些等级不会被 user_groups 重新引用，需要人工按快照恢复 price_policy_id。
--     演示组数据不回滚。）
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- A. 代理等级
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS agent_levels (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(64)  NOT NULL,
    code        VARCHAR(64)  NOT NULL,
    weight      BIGINT       NOT NULL DEFAULT 0,
    status      VARCHAR(32)  NOT NULL DEFAULT 'active',
    description VARCHAR(255),
    created_at  TIMESTAMPTZ,
    updated_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_agent_levels_code ON agent_levels (code);

COMMENT ON TABLE  agent_levels         IS '代理等级：代理的定价阶梯（doc108）';
COMMENT ON COLUMN agent_levels.weight  IS '权重，越大越优先（折扣最优）；矩阵列按此降序';
COMMENT ON COLUMN agent_levels.status  IS 'active / disabled；禁用后该等级不再参与算价';

-- 折扣矩阵：一格一行（等级 × 目标），target_type=all 时 target_id=0。
CREATE TABLE IF NOT EXISTS agent_level_discounts (
    id             BIGSERIAL PRIMARY KEY,
    agent_level_id BIGINT        NOT NULL,
    target_type    VARCHAR(16)   NOT NULL,
    target_id      BIGINT        NOT NULL DEFAULT 0,
    discount_rate  NUMERIC(10,4) NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ,
    updated_at     TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_agent_level_discounts
    ON agent_level_discounts (agent_level_id, target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_agent_level_discounts_target
    ON agent_level_discounts (target_type, target_id);

COMMENT ON TABLE  agent_level_discounts               IS '代理折扣矩阵：等级 × 商品/分类/全站（doc108）';
COMMENT ON COLUMN agent_level_discounts.discount_rate IS '折扣率，0.85=八五折（越小越优惠）；0=未配置（不打折）';
COMMENT ON COLUMN agent_level_discounts.target_type   IS 'all / category / product；命中优先级 product > category > all';

-- ---------------------------------------------------------------------------
-- B. users.agent_level_id（NULL = 非代理）
--    不加外键：与 user_group_id 一致，改为应用层校验"存在且启用"。
-- ---------------------------------------------------------------------------
ALTER TABLE users ADD COLUMN IF NOT EXISTS agent_level_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_users_agent_level_id ON users (agent_level_id);

COMMENT ON COLUMN users.agent_level_id IS '代理等级 ID；NULL = 非代理（普通客户，无折扣）';

-- ---------------------------------------------------------------------------
-- C. product_categories.cost_rate：分类级拿货折扣
--    0 = 未配置 → 折扣保存时跳过毛利校验（不阻断；存量分类都未配置）。
-- ---------------------------------------------------------------------------
ALTER TABLE product_categories ADD COLUMN IF NOT EXISTS cost_rate NUMERIC(10,4) NOT NULL DEFAULT 0;

COMMENT ON COLUMN product_categories.cost_rate IS '分类级拿货折扣率，如 0.60=六折进货；0=未配置（跳过毛利校验）';

-- ---------------------------------------------------------------------------
-- D. user_groups 去折扣化
-- ---------------------------------------------------------------------------
-- D1. 先做数据前移：把仍在演示组里的用户迁到默认组，避免下一步 DROP 后留悬空 ID。
--     默认组由 is_default 唯一标识（uk_user_groups_single_default 保证至多一个）。
DO $$
DECLARE
    default_group_id BIGINT;
    moved            INT;
BEGIN
    SELECT id INTO default_group_id FROM user_groups WHERE is_default = true LIMIT 1;

    IF default_group_id IS NOT NULL THEN
        UPDATE users
           SET user_group_id = default_group_id
         WHERE user_group_id IN (
             SELECT id FROM user_groups
              WHERE code IN ('platform_ops','east_sales','key_account_support','disabled_demo_group')
         );
        GET DIAGNOSTICS moved = ROW_COUNT;
        RAISE NOTICE '用户组去折扣化：演示组成员迁入默认组 % 的行数 = %', default_group_id, moved;
    ELSE
        -- 无默认组时不允许静默丢归属：把成员置为未分组（NULL），与"未分组"既有语义一致。
        UPDATE users
           SET user_group_id = NULL
         WHERE user_group_id IN (
             SELECT id FROM user_groups
              WHERE code IN ('platform_ops','east_sales','key_account_support','disabled_demo_group')
         );
        GET DIAGNOSTICS moved = ROW_COUNT;
        RAISE WARNING '用户组去折扣化：无默认用户组，% 个用户被置为未分组', moved;
    END IF;
END $$;

-- D1b. 折扣策略前移：改造前绑了 price_policy_id 的用户组，其折扣会随 D3 一起消失。
--      唯一的实例是 P6下游组（p6_downstream → 策略「P6下游85折」，绑商品 1），
--      它的语义正是"下游成本价 = owner 账号的折扣"（doc16），所以按代理等级原样承接：
--      一个绑定的组 → 一个代理等级，策略条目 → 折扣矩阵格子，组成员 → agent_level_id。
--      不承接就等于静默取消下游折扣，属于数据丢失，故必须在 DROP 之前做。
DO $$
DECLARE
    g            RECORD;
    v_level_id   BIGINT;
    v_target     RECORD;
    v_has_item   BOOLEAN;
BEGIN
    -- 幂等守卫：本块引用 user_groups.price_policy_id，而同一个迁移的 D3 会把它删掉。
    -- PL/pgSQL 按需解析语句，只有真的进入 FOR 才解析那条 SELECT，所以用列存在性
    -- 判断把整块包起来，重复执行时不会撞 "column does not exist"。
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_name = 'user_groups' AND column_name = 'price_policy_id'
    ) THEN
        RAISE NOTICE '迁移 059：user_groups.price_policy_id 已不存在，跳过折扣承接';
        RETURN;
    END IF;

    FOR g IN
        SELECT ug.id           AS group_id,
               ug.name         AS group_name,
               pp.id           AS policy_id,
               pp.code         AS policy_code,
               pp.discount_type,
               pp.discount_value,
               pp.scope
          FROM user_groups ug
          JOIN price_policies pp ON pp.id = ug.price_policy_id
         WHERE ug.price_policy_id IS NOT NULL
    LOOP
        -- 一策略一等级；code 固定前缀 + 策略 ID，重复执行命中 ON CONFLICT 不新建。
        INSERT INTO agent_levels (name, code, weight, status, description, created_at, updated_at)
        VALUES ('迁移·' || g.group_name,
                'migrated_policy_' || g.policy_id,
                0,
                'active',
                '迁移 059：承接用户组折扣策略 ' || g.policy_code || '（原组 ' || g.group_name || '）',
                now(), now())
        ON CONFLICT (code) DO NOTHING;

        SELECT id INTO v_level_id FROM agent_levels WHERE code = 'migrated_policy_' || g.policy_id;

        -- 策略条目 → 矩阵格子（仅 rate 类型可表达；amount 类型线下降价无法进矩阵，跳过并告警）。
        SELECT EXISTS (SELECT 1 FROM price_policy_items WHERE policy_id = g.policy_id)
          INTO v_has_item;

        IF v_has_item THEN
            FOR v_target IN
                SELECT target_type, target_id, discount_type, discount_value
                  FROM price_policy_items WHERE policy_id = g.policy_id
            LOOP
                IF v_target.discount_type <> 'rate' THEN
                    RAISE WARNING '迁移 059：策略 % 的条目 %.% 为 % 类型，代理矩阵只能表达 rate，已跳过',
                        g.policy_code, v_target.target_type, v_target.target_id, v_target.discount_type;
                    CONTINUE;
                END IF;
                INSERT INTO agent_level_discounts
                    (agent_level_id, target_type, target_id, discount_rate, created_at, updated_at)
                VALUES (v_level_id, v_target.target_type, v_target.target_id,
                        v_target.discount_value, now(), now())
                ON CONFLICT (agent_level_id, target_type, target_id) DO NOTHING;
            END LOOP;
        ELSIF g.scope = 'all' AND g.discount_type = 'rate' THEN
            -- 无条目 + scope=all：整站折扣，落到全站兜底行（target_id 固定 0）。
            INSERT INTO agent_level_discounts
                (agent_level_id, target_type, target_id, discount_rate, created_at, updated_at)
            VALUES (v_level_id, 'all', 0, g.discount_value, now(), now())
            ON CONFLICT (agent_level_id, target_type, target_id) DO NOTHING;
        ELSE
            RAISE WARNING '迁移 059：策略 % 为 % 类型/scope=% 且无条目，无法承接',
                g.policy_code, g.discount_type, g.scope;
        END IF;

        -- 组成员获得该代理等级（NULL 不算）。
        UPDATE users
           SET agent_level_id = v_level_id
         WHERE user_group_id = g.group_id
           AND agent_level_id IS NULL;

        RAISE NOTICE '迁移 059：用户组 % → 代理等级 %（策略 %）', g.group_name, v_level_id, g.policy_code;
    END LOOP;
END $$;

-- D2. 删除建库演示组（命名用的是员工部门，本就不该出现在客户分类里）。
DELETE FROM user_groups
 WHERE code IN ('platform_ops','east_sales','key_account_support','disabled_demo_group');

-- D3. 去掉折扣与代理标记两列。
--     price_policy_id 是改造前唯一的折扣来源（已由 agent_level_discounts 承接）；
--     is_agent_group 全仓零消费，是代理域下线后的死字段。
ALTER TABLE user_groups DROP COLUMN IF EXISTS price_policy_id;
ALTER TABLE user_groups DROP COLUMN IF EXISTS is_agent_group;

COMMIT;
