-- ============================================================================
-- 078_cost_management.sql
-- 成本管理（doc111）建表：成本项配置 / 上游余额快照 / 上游充值记录。
--
-- 要解决的问题：
--   ① 上游成本看不见 —— 在上游代理充了多少、消耗了多少，此前只能靠手工记账；
--   ② 自营/固定成本无处登记 —— 母机月费、员工工资、机房带宽、域名证书，
--      没有配置入口，也就没有「月度成本」与「利润率」；
--   ③ 月中不可见 —— 只有月末算总账，月中看不到「当月成本 vs 当月收入」。
--
-- 三张表的分工（口径见 cost/service 层注释与页面页脚）：
--   cost_items                 成本项配置（母机月费/工资/机房…按自然月计入，支持一次性项）
--   upstream_balance_snapshots 各渠道余额快照（同一渠道同日唯一，重复录入即覆盖）
--   upstream_balance_topups    向上游充值的记录（正=充值，负=退款冲正）
--   期间消耗 = 期初余额（上月末最近一条快照）+ 期间充值合计 − 期末余额（本月最后一条快照）。
--   因此不要求月初/月末当天一定有快照，也不依赖上游是否开放资金接口。
--
-- 幂等：全部 CREATE ... IF NOT EXISTS 与 CREATE INDEX IF NOT EXISTS，重复执行无副作用。
-- 双写：运行时建表由 db.go 的 AutoMigrate（costmodel 三个模型）完成，本文件是版本留痕，
--   两条路径的列/索引/默认值必须一致（migrations_live_verify_test.go 有对应 live 校验）。
-- 顺序：本迁移可随时执行；表为空时页面给出「尚未录入」引导，不做数据回填。
-- 回滚：DROP TABLE upstream_balance_topups, upstream_balance_snapshots, cost_items;
--   （三张表均为新增，无其它模块外键依赖。）
-- ============================================================================

BEGIN;

-- ① 成本项配置：母机月费 / 员工工资 / 机房带宽 / 域名证书 / 一次性采购…
CREATE TABLE IF NOT EXISTS cost_items (
    id             BIGSERIAL PRIMARY KEY,
    name           VARCHAR(100)  NOT NULL,
    -- self_hosted 自营宿主机 / upstream_ops 上游运营 / labor 人力 / infra 基础设施 / other 其他
    category       VARCHAR(32)   NOT NULL,
    amount         DECIMAL(15,2) NOT NULL DEFAULT 0,
    -- monthly=按自然月计入；once=一次性，计入 occurred_on 所在自然月
    cycle          VARCHAR(16)   NOT NULL DEFAULT 'monthly',
    occurred_on    DATE          NULL,
    -- 生效区间（含端点）：按月项在区间与自然月相交时整月计入；effective_to 为空=长期有效
    effective_from DATE          NOT NULL,
    effective_to   DATE          NULL,
    -- 成本对象（母机名/员工/线路），用于区分同名成本项的多条配置
    subject        VARCHAR(120)  NOT NULL DEFAULT '',
    remark         VARCHAR(255)  NOT NULL DEFAULT '',
    status         VARCHAR(16)   NOT NULL DEFAULT 'active',
    operator_id    BIGINT        NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_cost_items_category ON cost_items (category);
CREATE INDEX IF NOT EXISTS idx_cost_items_status ON cost_items (status);

-- ② 上游余额快照：同一渠道同一天只保留一条（唯一键），重复抓取/修正直接覆盖
CREATE TABLE IF NOT EXISTS upstream_balance_snapshots (
    id            BIGSERIAL PRIMARY KEY,
    provider_id   BIGINT        NOT NULL,
    snapshot_date DATE          NOT NULL,
    balance       DECIMAL(15,2) NOT NULL DEFAULT 0,
    currency      VARCHAR(8)    NOT NULL DEFAULT 'CNY',
    -- manual=人工录入；auto=定时任务或「抓取余额」按钮
    source        VARCHAR(16)   NOT NULL DEFAULT 'manual',
    remark        VARCHAR(255)  NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_ub_snapshot_provider_date
    ON upstream_balance_snapshots (provider_id, snapshot_date);
CREATE INDEX IF NOT EXISTS idx_ub_snapshot_provider_date
    ON upstream_balance_snapshots (provider_id, snapshot_date);

-- ③ 上游充值记录：正=向渠道充值，负=渠道退款/冲正（把余额差还原成消耗）
CREATE TABLE IF NOT EXISTS upstream_balance_topups (
    id          BIGSERIAL PRIMARY KEY,
    provider_id BIGINT        NOT NULL,
    occurred_on DATE          NOT NULL,
    amount      DECIMAL(15,2) NOT NULL DEFAULT 0,
    remark      VARCHAR(255)  NOT NULL DEFAULT '',
    operator_id BIGINT        NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ub_topup_provider_date
    ON upstream_balance_topups (provider_id, occurred_on);

COMMIT;

-- 验收查询（人工执行）
--   三张表齐备：
--   SELECT table_name FROM information_schema.tables
--    WHERE table_name IN ('cost_items','upstream_balance_snapshots','upstream_balance_topups')
--    ORDER BY table_name;
--   期望：3 行
--   同日唯一键生效（同渠道同日余额只有一条真值）：
--   SELECT indexname FROM pg_indexes WHERE tablename = 'upstream_balance_snapshots'
--     AND indexname = 'uk_ub_snapshot_provider_date';
--   期望：1 行
--   菜单与权限（由 db.go seed 播种，重启后端后可见）：
--   SELECT m.path, m.name, m.sort_order FROM menus m JOIN menus p ON p.id = m.parent_id
--    WHERE m.platform = 'admin' AND p.path = '/finance/cost' ORDER BY m.sort_order;
--   期望：/finance/cost/overview(1)、/finance/cost/items(2)、/finance/cost/upstreams(3)
--   SELECT code FROM permissions WHERE code LIKE 'finance:cost:%' ORDER BY code;
--   期望：finance:cost:balance、finance:cost:item、finance:cost:overview
