-- ============================================================================
-- 026_drop_agent_domain_and_referral.sql
-- 一次性完成两件事：
--   A. 退役代理/分销域（5 张表 + 菜单 + 权限），折扣来源收敛为「用户组价格策略」；
--   B. 建立推广邀请返现体系（users 邀请字段 + referral_accounts/transactions/withdrawals）。
--
-- 背景与决策：见 docs/实施计划/84-推广邀请返现体系实施计划.md。
--   · 代理域整套下线（佣金/结算/代理等级/下级/代理记录），代理折扣改由用户组承载；
--   · 邀请改为单级返现：users.invite_code 邀请注册 → 邀请人按被邀请人订单实付拿返现；
--   · 返现余额独立成体系（本迁移的三张表），可提现（独立审核）或转入现金余额。
--
-- 前置（已在代码层完成并通过 build/vet/test）：
--   1) internal/modules/admin/user/distribution 与 internal/modules/uc/agent 整模块删除；
--   2) db.AutoMigrate 已移除这 5 张代理表模型，并新增 referral_* 三表模型；
--   3) db.Seed 已移除分销权限与角色预置。
-- 执行时序：先构建并部署新后端，再执行本迁移（与 013/017 同理），否则运行中的旧后端
--           访问 /admin/distribution/* 或 /uc/agent/* 会因表不存在而报错。
--
-- 幂等：全部 IF NOT EXISTS / IF EXISTS / 条件 UPDATE，可重复执行。
-- 回滚：代理 5 表已快照到 retired_*_20260910，可从快照 INSERT 回；
--       完整基线备份见 migrations/backups/pre-agent-drop-*.sql。
--
-- 注意：本迁移打破了「只增不删」的 R5 铁律——这是经确认的定向退役，
--       缓解措施是快照表 + 事先全库 dump，恢复步骤写在文件末尾。
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1) users 邀请字段：邀请码（唯一）+ 邀请人（单级）+ 绑定时间。
--    invite_code 允许 NULL（唯一索引在 Postgres 下不约束多个 NULL），
--    存量用户按确定性算法回填，新用户由注册流程生成。
-- ---------------------------------------------------------------------------
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS invite_code     VARCHAR(32),
  ADD COLUMN IF NOT EXISTS inviter_user_id BIGINT,
  ADD COLUMN IF NOT EXISTS invited_at      TIMESTAMPTZ;

DO $$
DECLARE
    r    RECORD;
    code TEXT;
    i    INT;
BEGIN
    FOR r IN SELECT id FROM users WHERE invite_code IS NULL OR invite_code = '' LOOP
        i := 0;
        LOOP
            code := UPPER(SUBSTR(MD5('hostsent-referral-' || r.id::text || '-' || i::text), 1, 8));
            EXIT WHEN NOT EXISTS (SELECT 1 FROM users u WHERE u.invite_code = code);
            i := i + 1;
            IF i > 20 THEN
                RAISE EXCEPTION 'invite_code 回填冲突过多（user_id=%）', r.id;
            END IF;
        END LOOP;
        UPDATE users SET invite_code = code, updated_at = now() WHERE id = r.id;
    END LOOP;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uk_users_invite_code ON users (invite_code);
CREATE INDEX IF NOT EXISTS idx_users_inviter_user_id ON users (inviter_user_id);

-- ---------------------------------------------------------------------------
-- 2) 返现体系三表。索引名与 GORM 模型显式名严格一致（R1）。
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS referral_accounts (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT        NOT NULL,
    balance      DECIMAL(15,2) NOT NULL DEFAULT 0,  -- 可用返现余额（退款冲减可为负）
    frozen       DECIMAL(15,2) NOT NULL DEFAULT 0,  -- 提现申请冻结
    total_income DECIMAL(15,2) NOT NULL DEFAULT 0,  -- 累计返现收入
    total_out    DECIMAL(15,2) NOT NULL DEFAULT 0,  -- 累计流出（冲减/冻结/转出）
    version      BIGINT        NOT NULL DEFAULT 0,  -- 乐观锁版本号
    created_at   TIMESTAMPTZ,
    updated_at   TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_referral_user ON referral_accounts (user_id);

CREATE TABLE IF NOT EXISTS referral_transactions (
    id              BIGSERIAL PRIMARY KEY,
    tx_no           VARCHAR(64)   NOT NULL,
    user_id         BIGINT        NOT NULL,          -- 返现归属人（邀请人）
    type            VARCHAR(32)   NOT NULL,          -- cashback/renewal_cashback/refund_clawback/withdraw_freeze/withdraw_return/transfer_out
    direction       BIGINT        NOT NULL,          -- 1=收入 -1=支出
    amount          DECIMAL(15,2) NOT NULL,
    balance_before  DECIMAL(15,2) NOT NULL,
    balance_after   DECIMAL(15,2) NOT NULL,
    biz_type        VARCHAR(32)   NOT NULL,          -- 幂等业务标识
    ref_no          VARCHAR(64),                     -- 关联单号（订单号/退款号/提现号/转账号）
    order_id        BIGINT,
    order_no        VARCHAR(64),
    inviter_user_id BIGINT,
    invitee_user_id BIGINT,
    remark          VARCHAR(255),
    operator_id     BIGINT,
    created_at      TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_referral_tx_no ON referral_transactions (tx_no);
CREATE UNIQUE INDEX IF NOT EXISTS uk_referral_biz ON referral_transactions (user_id, biz_type, ref_no);
CREATE INDEX IF NOT EXISTS idx_referral_tx_user ON referral_transactions (user_id);
CREATE INDEX IF NOT EXISTS idx_referral_tx_type ON referral_transactions (type);
CREATE INDEX IF NOT EXISTS idx_referral_tx_order_id ON referral_transactions (order_id);
CREATE INDEX IF NOT EXISTS idx_referral_tx_invitee ON referral_transactions (invitee_user_id);
CREATE INDEX IF NOT EXISTS idx_referral_tx_created ON referral_transactions (created_at);

CREATE TABLE IF NOT EXISTS referral_withdrawals (
    id            BIGSERIAL PRIMARY KEY,
    withdraw_no   VARCHAR(64)   NOT NULL,
    user_id       BIGINT        NOT NULL,
    amount        DECIMAL(15,2) NOT NULL,
    channel       VARCHAR(20),
    account       VARCHAR(128),
    status        VARCHAR(20)   NOT NULL DEFAULT 'pending',
    audit_by      BIGINT,
    audit_by_name VARCHAR(50),
    audited_at    TIMESTAMPTZ,
    remark        VARCHAR(255),
    created_at    TIMESTAMPTZ,
    updated_at    TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_referral_wd_no ON referral_withdrawals (withdraw_no);
CREATE INDEX IF NOT EXISTS idx_referral_wd_user ON referral_withdrawals (user_id);
CREATE INDEX IF NOT EXISTS idx_referral_wd_status ON referral_withdrawals (status);

-- ---------------------------------------------------------------------------
-- 3) 清理代理域的菜单与权限（003 种子已写入，须显式删除，否则前端仍显示入口）。
-- ---------------------------------------------------------------------------
DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE code = 'distribution' OR code LIKE 'distribution:%');

DELETE FROM menus WHERE path LIKE '/users/partners%' OR path LIKE '%/distribution%';

DELETE FROM permissions WHERE code = 'distribution' OR code LIKE 'distribution:%';

-- ---------------------------------------------------------------------------
-- 4) 退役代理域 5 张表：先快照（表存在时才做，保证可重复执行），再 DROP。
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF to_regclass('public.distribution_agents') IS NOT NULL THEN
        EXECUTE 'CREATE TABLE IF NOT EXISTS retired_distribution_agents_20260910 AS SELECT * FROM distribution_agents';
    END IF;
    IF to_regclass('public.agent_levels') IS NOT NULL THEN
        EXECUTE 'CREATE TABLE IF NOT EXISTS retired_agent_levels_20260910 AS SELECT * FROM agent_levels';
    END IF;
    IF to_regclass('public.distribution_subordinates') IS NOT NULL THEN
        EXECUTE 'CREATE TABLE IF NOT EXISTS retired_distribution_subordinates_20260910 AS SELECT * FROM distribution_subordinates';
    END IF;
    IF to_regclass('public.distribution_commissions') IS NOT NULL THEN
        EXECUTE 'CREATE TABLE IF NOT EXISTS retired_distribution_commissions_20260910 AS SELECT * FROM distribution_commissions';
    END IF;
    IF to_regclass('public.distribution_settlements') IS NOT NULL THEN
        EXECUTE 'CREATE TABLE IF NOT EXISTS retired_distribution_settlements_20260910 AS SELECT * FROM distribution_settlements';
    END IF;
END $$;

DROP TABLE IF EXISTS distribution_commissions;
DROP TABLE IF EXISTS distribution_settlements;
DROP TABLE IF EXISTS distribution_subordinates;
DROP TABLE IF EXISTS distribution_agents;
DROP TABLE IF EXISTS agent_levels;

COMMIT;

-- ============================================================================
-- 回滚（Rollback）：
--   1) 代理域表结构与数据：
--      CREATE TABLE distribution_agents AS SELECT * FROM retired_distribution_agents_20260910;（逐表同理）
--      注意：快照表不含主键/索引定义，重建后需补 PK、唯一键与序列（参考 001_init_schema.sql）。
--   2) 折扣回到代理线：从 git 历史恢复 distribution/uc agent 模块与 server 装配（提交见本次重构）。
--   3) 返现体系：DROP TABLE referral_withdrawals / referral_transactions / referral_accounts;
--      ALTER TABLE users DROP COLUMN IF EXISTS invite_code, DROP COLUMN IF EXISTS inviter_user_id, DROP COLUMN IF EXISTS invited_at;
-- ============================================================================
