-- ============================================================================
-- 040_staff_sales_ticket_collab.sql
--
-- 背景（docs/实施计划/85 员工体系与销售提成及工单协同设计、
--       86 员工体系与销售提成及工单协同实施文档）：
--   1) 组织：新建 departments，收敛 admins.department 自由文本与裸客服组；
--      admins 扩展 department_id / staff_type / sales_enabled / 在职字段。
--   2) 销售：客户归属（staff_sales_relations + users.sales_admin_id）、
--      订单归属快照（orders.sales_admin_id，不可变）、
--      独立提成账本（sales_commission_accounts/transactions、sales_withdrawals）、
--      业绩目标（sales_targets）。
--   3) 工单：分类挂部门 + 提交前置条件 + 双人复核；回复内部备注与复核状态；
--      附件内部可见标记。
--   4) 打款单加 biz_type：提现出口复用 payment_payouts，需区分财务提现与销售提成提现。
--
-- 幂等：CREATE TABLE IF NOT EXISTS / ADD COLUMN IF NOT EXISTS /
--       CREATE INDEX IF NOT EXISTS / ON CONFLICT DO NOTHING。
-- 执行前建议 pg_dump 备份。
--
-- 回滚：
--   ALTER TABLE payment_payouts DROP COLUMN IF EXISTS biz_type;
--   ALTER TABLE ticket_attachments DROP COLUMN IF EXISTS is_internal;
--   ALTER TABLE ticket_replies DROP COLUMN IF EXISTS is_internal, DROP COLUMN IF EXISTS review_status,
--     DROP COLUMN IF EXISTS reviewer_id, DROP COLUMN IF EXISTS reviewed_at, DROP COLUMN IF EXISTS review_note;
--   ALTER TABLE tickets DROP COLUMN IF EXISTS department_id, DROP COLUMN IF EXISTS review_status,
--     DROP COLUMN IF EXISTS reviewer_id, DROP COLUMN IF EXISTS review_requested_by,
--     DROP COLUMN IF EXISTS reviewed_at, DROP COLUMN IF EXISTS review_note;
--   ALTER TABLE ticket_categories DROP COLUMN IF EXISTS department_id,
--     DROP COLUMN IF EXISTS require_realname, DROP COLUMN IF EXISTS require_binding,
--     DROP COLUMN IF EXISTS need_review, DROP COLUMN IF EXISTS visible_role_codes;
--   ALTER TABLE orders DROP COLUMN IF EXISTS sales_admin_id;
--   ALTER TABLE users DROP COLUMN IF EXISTS sales_admin_id;
--   ALTER TABLE admins DROP COLUMN IF EXISTS department_id, DROP COLUMN IF EXISTS staff_type,
--     DROP COLUMN IF EXISTS sales_enabled, DROP COLUMN IF EXISTS real_name, DROP COLUMN IF EXISTS phone,
--     DROP COLUMN IF EXISTS joined_at, DROP COLUMN IF EXISTS resigned_at;
--   DROP TABLE IF EXISTS sales_targets, sales_withdrawals, sales_commission_transactions,
--     sales_commission_accounts, staff_sales_relations, departments;
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. 组织：departments
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS departments (
    id              BIGSERIAL PRIMARY KEY,
    name            varchar(64)  NOT NULL,
    code            varchar(64)  NOT NULL,
    kind            varchar(32)  NOT NULL DEFAULT 'general',  -- sales/support/tech/ops/finance/general
    parent_id       bigint       NOT NULL DEFAULT 0,
    leader_admin_id bigint       NOT NULL DEFAULT 0,
    remark          varchar(255) NOT NULL DEFAULT '',
    sort_order      int          NOT NULL DEFAULT 0,
    status          varchar(32)  NOT NULL DEFAULT 'active',
    created_at      timestamptz,
    updated_at      timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_departments_code ON departments (code);
CREATE INDEX IF NOT EXISTS idx_departments_kind ON departments (kind);
CREATE INDEX IF NOT EXISTS idx_departments_parent ON departments (parent_id);

-- ---------------------------------------------------------------------------
-- 2. 员工：admins 扩展
-- ---------------------------------------------------------------------------
ALTER TABLE admins
  ADD COLUMN IF NOT EXISTS department_id bigint      NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS staff_type    varchar(32) NOT NULL DEFAULT 'admin',
  ADD COLUMN IF NOT EXISTS sales_enabled boolean     NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS real_name     varchar(64) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS phone         varchar(32) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS joined_at     timestamptz,
  ADD COLUMN IF NOT EXISTS resigned_at   timestamptz;

CREATE INDEX IF NOT EXISTS idx_admins_department ON admins (department_id);
CREATE INDEX IF NOT EXISTS idx_admins_staff_type ON admins (staff_type);

-- 兼容迁移：旧自由文本 department → departments（同名合并）并回填 department_id。
-- code 用 legacy-<md5(name)> 保证稳定且不与人工建的部门冲突。
INSERT INTO departments (name, code, kind, sort_order, status, created_at, updated_at)
SELECT DISTINCT btrim(a.department), 'legacy-' || md5(btrim(a.department)), 'general', 90, 'active', now(), now()
FROM admins a
WHERE btrim(coalesce(a.department, '')) <> ''
ON CONFLICT (code) DO NOTHING;

UPDATE admins a SET department_id = d.id
FROM departments d
WHERE btrim(coalesce(a.department, '')) <> ''
  AND a.department_id = 0
  AND d.code = 'legacy-' || md5(btrim(a.department));

-- 旧自由文本客服组：为 ticket_categories.default_group_id 预建 legacy-group-N 部门。
-- kind=support，便于迁移后自动派单按部门挑选在岗员工。
INSERT INTO departments (name, code, kind, sort_order, status, created_at, updated_at)
SELECT DISTINCT '客服组 ' || c.default_group_id::text, 'legacy-group-' || c.default_group_id::text,
       'support', 80, 'active', now(), now()
FROM ticket_categories c
WHERE c.default_group_id IS NOT NULL AND c.default_group_id > 0
ON CONFLICT (code) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 3. 销售归属
-- ---------------------------------------------------------------------------
ALTER TABLE users  ADD COLUMN IF NOT EXISTS sales_admin_id bigint NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS sales_admin_id bigint NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_users_sales_admin  ON users (sales_admin_id);
CREATE INDEX IF NOT EXISTS idx_orders_sales_admin ON orders (sales_admin_id);

CREATE TABLE IF NOT EXISTS staff_sales_relations (
    id            BIGSERIAL PRIMARY KEY,
    admin_id      bigint       NOT NULL,
    user_id       bigint       NOT NULL,
    status        varchar(20)  NOT NULL DEFAULT 'active',  -- active/released
    reason        varchar(255) NOT NULL DEFAULT '',
    operator_id   bigint       NOT NULL DEFAULT 0,
    protect_until timestamptz,
    effective_at  timestamptz  NOT NULL DEFAULT now(),
    released_at   timestamptz,
    created_at    timestamptz,
    updated_at    timestamptz
);
CREATE INDEX IF NOT EXISTS idx_ssr_admin ON staff_sales_relations (admin_id, status);
CREATE INDEX IF NOT EXISTS idx_ssr_user  ON staff_sales_relations (user_id, status);
-- 一个客户同时只能有一个生效归属
CREATE UNIQUE INDEX IF NOT EXISTS uk_ssr_user_active
  ON staff_sales_relations (user_id) WHERE status = 'active';

-- ---------------------------------------------------------------------------
-- 4. 提成账本（与 referral_* 同构，主体是 admin_id，物理隔离）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sales_commission_accounts (
    id              BIGSERIAL PRIMARY KEY,
    admin_id        bigint        NOT NULL,
    balance         numeric(15,2) NOT NULL DEFAULT 0,  -- 可用（已过解冻期）
    frozen          numeric(15,2) NOT NULL DEFAULT 0,  -- 提现冻结
    pending_release numeric(15,2) NOT NULL DEFAULT 0,  -- 解冻中
    total_income    numeric(15,2) NOT NULL DEFAULT 0,
    total_out       numeric(15,2) NOT NULL DEFAULT 0,
    version         bigint        NOT NULL DEFAULT 0,
    created_at      timestamptz,
    updated_at      timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_sca_admin ON sales_commission_accounts (admin_id);

CREATE TABLE IF NOT EXISTS sales_commission_transactions (
    id               BIGSERIAL PRIMARY KEY,
    tx_no            varchar(64)   NOT NULL,
    admin_id         bigint        NOT NULL,
    type             varchar(32)   NOT NULL,
    direction        int           NOT NULL,  -- 1=入账 -1=出账
    amount           numeric(15,2) NOT NULL,
    balance_before   numeric(15,2) NOT NULL DEFAULT 0,
    balance_after    numeric(15,2) NOT NULL DEFAULT 0,
    biz_type         varchar(32)   NOT NULL,
    ref_no           varchar(64)   NOT NULL,
    order_id         bigint        NOT NULL DEFAULT 0,
    order_no         varchar(64)   NOT NULL DEFAULT '',
    customer_user_id bigint        NOT NULL DEFAULT 0,
    release_at       timestamptz,             -- 非空=未解冻；解冻后置 NULL 作为幂等标记
    remark           varchar(255)  NOT NULL DEFAULT '',
    operator_id      bigint        NOT NULL DEFAULT 0,
    created_at       timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_sct_no  ON sales_commission_transactions (tx_no);
CREATE UNIQUE INDEX IF NOT EXISTS uk_sct_biz ON sales_commission_transactions (admin_id, biz_type, ref_no);
CREATE INDEX IF NOT EXISTS idx_sct_admin_created ON sales_commission_transactions (admin_id, created_at);
CREATE INDEX IF NOT EXISTS idx_sct_release ON sales_commission_transactions (release_at);
CREATE INDEX IF NOT EXISTS idx_sct_order ON sales_commission_transactions (order_id);

CREATE TABLE IF NOT EXISTS sales_withdrawals (
    id            BIGSERIAL PRIMARY KEY,
    withdraw_no   varchar(64)   NOT NULL,
    admin_id      bigint        NOT NULL,
    amount        numeric(15,2) NOT NULL,
    channel       varchar(20)   NOT NULL DEFAULT '',
    account       varchar(128)  NOT NULL DEFAULT '',
    account_name  varchar(64)   NOT NULL DEFAULT '',
    bank_name     varchar(100)  NOT NULL DEFAULT '',
    payout_mode   varchar(20)   NOT NULL DEFAULT 'manual',
    payout_no     varchar(64)   NOT NULL DEFAULT '',
    payout_id     bigint        NOT NULL DEFAULT 0,
    status        varchar(20)   NOT NULL DEFAULT 'pending',
    audit_by      bigint        NOT NULL DEFAULT 0,
    audit_by_name varchar(64)   NOT NULL DEFAULT '',
    audited_at    timestamptz,
    paid_at       timestamptz,
    remark        varchar(255)  NOT NULL DEFAULT '',
    created_at    timestamptz,
    updated_at    timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_sw_no ON sales_withdrawals (withdraw_no);
CREATE INDEX IF NOT EXISTS idx_sw_admin  ON sales_withdrawals (admin_id, status);
CREATE INDEX IF NOT EXISTS idx_sw_status ON sales_withdrawals (status);

-- ---------------------------------------------------------------------------
-- 5. 业绩目标
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sales_targets (
    id            BIGSERIAL PRIMARY KEY,
    period        varchar(7)    NOT NULL,                  -- YYYY-MM
    scope         varchar(20)   NOT NULL DEFAULT 'admin',  -- admin/department
    admin_id      bigint        NOT NULL DEFAULT 0,
    department_id bigint        NOT NULL DEFAULT 0,
    target_amount numeric(15,2) NOT NULL DEFAULT 0,
    target_orders int           NOT NULL DEFAULT 0,
    created_by    bigint        NOT NULL DEFAULT 0,
    created_at    timestamptz,
    updated_at    timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_sales_targets_scope
  ON sales_targets (period, scope, admin_id, department_id);

-- ---------------------------------------------------------------------------
-- 6. 工单：分类前置条件 + 复核 + 内部备注 + 附件
-- ---------------------------------------------------------------------------
ALTER TABLE ticket_categories
  ADD COLUMN IF NOT EXISTS department_id      bigint  NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS require_realname   boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS require_binding    boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS need_review        boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS visible_role_codes jsonb   NOT NULL DEFAULT '[]'::jsonb;
CREATE INDEX IF NOT EXISTS idx_ticket_categories_dept ON ticket_categories (department_id);

-- 旧裸客服组 → 部门回填（部门已在上方按 legacy-group-N 预建）。
UPDATE ticket_categories c SET department_id = d.id
FROM departments d
WHERE c.department_id = 0
  AND c.default_group_id IS NOT NULL AND c.default_group_id > 0
  AND d.code = 'legacy-group-' || c.default_group_id::text;

ALTER TABLE tickets
  ADD COLUMN IF NOT EXISTS department_id       bigint      NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS review_status       varchar(20) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS reviewer_id         bigint      NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS review_requested_by bigint      NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS reviewed_at         timestamptz,
  ADD COLUMN IF NOT EXISTS review_note         varchar(255) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_tickets_dept   ON tickets (department_id);
CREATE INDEX IF NOT EXISTS idx_tickets_review ON tickets (review_status);

-- 存量工单补部门：按分类编码回填，避免历史工单在部门隔离下「不可见」。
UPDATE tickets t SET department_id = c.department_id
FROM ticket_categories c
WHERE t.category = c.code AND t.department_id = 0;

ALTER TABLE ticket_replies
  ADD COLUMN IF NOT EXISTS is_internal   boolean     NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS review_status varchar(20) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS reviewer_id   bigint      NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS reviewed_at   timestamptz,
  ADD COLUMN IF NOT EXISTS review_note   varchar(255) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_ticket_replies_review ON ticket_replies (review_status);

ALTER TABLE ticket_attachments
  ADD COLUMN IF NOT EXISTS is_internal boolean NOT NULL DEFAULT false;
CREATE INDEX IF NOT EXISTS idx_ticket_attachments_ticket ON ticket_attachments (ticket_id);

-- ---------------------------------------------------------------------------
-- 7. 打款单加业务域：financial withdraw vs sales commission withdraw
-- ---------------------------------------------------------------------------
ALTER TABLE payment_payouts ADD COLUMN IF NOT EXISTS biz_type varchar(32) NOT NULL DEFAULT 'withdraw';
CREATE INDEX IF NOT EXISTS idx_payment_payouts_biz ON payment_payouts (biz_type, withdraw_id);

COMMIT;
