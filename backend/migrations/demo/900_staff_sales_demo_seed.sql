-- ============================================================================
-- 900_staff_sales_demo_seed.sql · 年度跑通造数（doc86 §8.2）
-- ----------------------------------------------------------------------------
-- 用途：S8 端到端联调与年度数据回归。覆盖 12 个月订单/提成，使业绩排行、
--       目标达成率、解冻期、提现四态都有可核对的真实数据。
--
-- 执行方式（手动，不会被 scripts/migrate.sh 自动应用 —— 该脚本只 glob
-- migrations/*.sql，不含 migrations/demo/）：
--   docker exec -i backend-postgres-1 psql -U hostsent -d hostsent -v ON_ERROR_STOP=1 \
--     < backend/migrations/demo/900_staff_sales_demo_seed.sql
--
-- 幂等性：全部 INSERT 带 ON CONFLICT DO NOTHING；账户余额在末尾由台账重算，
--         重复执行不产生翻倍资金。
-- 统一口令：Demo@123456（演示员工与演示客户同口令，仅联调环境使用）。
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. 部门（doc86 §8.2：销售一部 / 客服组 / 技术组）
--    注：技术组额外配 1 名技术员工（demo_tech_01），否则 S8 场景 2
--    「部门派单自动派给负载最低在岗员工」无候选可派，只能落未分配池。
-- ---------------------------------------------------------------------------
INSERT INTO departments (name, code, kind, parent_id, leader_admin_id, remark, sort_order, status)
VALUES
  ('销售一部', 'demo-sales-1', 'sales',   0, 0, '年度造数·销售一部', 11, 'active'),
  ('客服组',   'demo-support', 'support', 0, 0, '年度造数·客服组',   12, 'active'),
  ('技术组',   'demo-tech',    'tech',    0, 0, '年度造数·技术组',   13, 'active')
ON CONFLICT (code) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 2. 员工 7 名（2 销售 + 1 销售主管 + 2 客服 + 1 客服主管 + 1 技术）
-- ---------------------------------------------------------------------------
INSERT INTO admins (username, email, password_hash, role, status, real_name, phone,
                    department_id, staff_type, sales_enabled, must_change_password, joined_at)
VALUES
  ('demo_sales_01',    'demo_sales_01@hostsent.local',    '$2b$10$hB84ZQ4pVb0eP4/b3zyOguNiIpN3ECUKtWMRk9AOer4cJ9bF7BgH2', 'sales',        'active', '销售甲', '13800000001', (SELECT id FROM departments WHERE code='demo-sales-1'),   'sales',   true,  false, now()),
  ('demo_sales_02',    'demo_sales_02@hostsent.local',    '$2b$10$hB84ZQ4pVb0eP4/b3zyOguNiIpN3ECUKtWMRk9AOer4cJ9bF7BgH2', 'sales',        'active', '销售乙', '13800000002', (SELECT id FROM departments WHERE code='demo-sales-1'),   'sales',   true,  false, now()),
  ('demo_sales_mgr',   'demo_sales_mgr@hostsent.local',   '$2b$10$hB84ZQ4pVb0eP4/b3zyOguNiIpN3ECUKtWMRk9AOer4cJ9bF7BgH2', 'sales_manager','active', '销售主管', '13800000003', (SELECT id FROM departments WHERE code='demo-sales-1'),  'sales',   true,  false, now()),
  ('demo_support_01',  'demo_support_01@hostsent.local',  '$2b$10$hB84ZQ4pVb0eP4/b3zyOguNiIpN3ECUKtWMRk9AOer4cJ9bF7BgH2', 'support',      'active', '客服甲', '13800000004', (SELECT id FROM departments WHERE code='demo-support'), 'support', false, false, now()),
  ('demo_support_02',  'demo_support_02@hostsent.local',  '$2b$10$hB84ZQ4pVb0eP4/b3zyOguNiIpN3ECUKtWMRk9AOer4cJ9bF7BgH2', 'support',      'active', '客服乙', '13800000005', (SELECT id FROM departments WHERE code='demo-support'), 'support', false, false, now()),
  ('demo_support_lead','demo_support_lead@hostsent.local','$2b$10$hB84ZQ4pVb0eP4/b3zyOguNiIpN3ECUKtWMRk9AOer4cJ9bF7BgH2', 'support_lead', 'active', '客服主管','13800000006', (SELECT id FROM departments WHERE code='demo-support'), 'support', false, false, now()),
  ('demo_tech_01',     'demo_tech_01@hostsent.local',     '$2b$10$hB84ZQ4pVb0eP4/b3zyOguNiIpN3ECUKtWMRk9AOer4cJ9bF7BgH2', 'tech',         'active', '技术甲', '13800000007', (SELECT id FROM departments WHERE code='demo-tech'),    'tech',    false, false, now())
ON CONFLICT (username) DO NOTHING;

-- 角色绑定（roles 表种子 id：sales=8 / sales_manager=9 / support=10 / support_lead=11 / tech=12）
INSERT INTO admin_roles (admin_id, role_id)
SELECT a.id, r.id
FROM admins a
JOIN roles r ON r.code = a.role
WHERE a.username LIKE 'demo\_%'
ON CONFLICT DO NOTHING;

-- 部门负责人（销售主管带销售一部，客服主管带客服组）
UPDATE departments SET leader_admin_id = (SELECT id FROM admins WHERE username='demo_sales_mgr')
 WHERE code = 'demo-sales-1' AND leader_admin_id = 0;
UPDATE departments SET leader_admin_id = (SELECT id FROM admins WHERE username='demo_support_lead')
 WHERE code = 'demo-support' AND leader_admin_id = 0;

-- ---------------------------------------------------------------------------
-- 3. 工单分类（technical 需绑定产品 / authorization 需实名+双人复核 / consult 无前置）
-- ---------------------------------------------------------------------------
INSERT INTO ticket_categories (name, code, description, sort_order, status, department_id,
                               require_realname, require_binding, need_review, visible_role_codes)
VALUES
  ('技术支持', 'technical',     '实例故障、网络与性能问题', 1, 'active', (SELECT id FROM departments WHERE code='demo-tech'),    false, true,  false, '[]'::jsonb),
  ('授权申请', 'authorization', '软件授权与额度开通',       2, 'active', (SELECT id FROM departments WHERE code='demo-support'), true,  false, true,  '[]'::jsonb),
  ('业务咨询', 'consult',       '售前与业务咨询',           3, 'active', (SELECT id FROM departments WHERE code='demo-support'), false, false, false, '[]'::jsonb)
ON CONFLICT (code) DO UPDATE
  SET department_id    = EXCLUDED.department_id,
      require_realname = EXCLUDED.require_realname,
      require_binding  = EXCLUDED.require_binding,
      need_review      = EXCLUDED.need_review;

-- ---------------------------------------------------------------------------
-- 4. 客户 12 名：8 名归属销售（甲 4 / 乙 4），4 名未归属
-- ---------------------------------------------------------------------------
INSERT INTO users (username, email, password_hash, status, real_name, phone, tier, created_at, updated_at)
SELECT
  'demo_user_' || lpad(i::text, 2, '0'),
  'demo_user_' || lpad(i::text, 2, '0') || '@hostsent.local',
  '$2b$10$hB84ZQ4pVb0eP4/b3zyOguNiIpN3ECUKtWMRk9AOer4cJ9bF7BgH2',
  'active',
  '演示客户' || lpad(i::text, 2, '0'),
  '1390000' || lpad(i::text, 4, '0'),
  'free',
  now() - ((13 - i) * interval '30 days'),
  now() - ((13 - i) * interval '30 days')
FROM generate_series(1, 12) i
ON CONFLICT (username) DO NOTHING;

-- 归属回写：1–4 → 销售甲，5–8 → 销售乙，9–12 未归属
UPDATE users SET sales_admin_id = (SELECT id FROM admins WHERE username='demo_sales_01')
 WHERE username IN ('demo_user_01','demo_user_02','demo_user_03','demo_user_04');
UPDATE users SET sales_admin_id = (SELECT id FROM admins WHERE username='demo_sales_02')
 WHERE username IN ('demo_user_05','demo_user_06','demo_user_07','demo_user_08');
UPDATE users SET sales_admin_id = 0
 WHERE username IN ('demo_user_09','demo_user_10','demo_user_11','demo_user_12');

-- ---------------------------------------------------------------------------
-- 5. 归属关系 8 条：5 条在保护期内，3 条已过保护期（50 天前生效、保护 90 天）
-- ---------------------------------------------------------------------------
INSERT INTO staff_sales_relations (admin_id, user_id, status, reason, operator_id,
                                   protect_until, effective_at, released_at, created_at, updated_at)
SELECT
  u.sales_admin_id,
  u.id,
  'active',
  '年度造数初始分配',
  (SELECT id FROM admins WHERE username='demo_sales_mgr'),
  CASE WHEN right(u.username, 2) IN ('01','02','05','06','07')
       THEN now() + interval '90 days'          -- 保护期内
       ELSE now() - interval '40 days' END,     -- 保护期已过（生效 130 天前）
  CASE WHEN right(u.username, 2) IN ('01','02','05','06','07')
       THEN now() - interval '10 days'
       ELSE now() - interval '130 days' END,
  NULL,
  now(),
  now()
FROM users u
WHERE u.username LIKE 'demo\_user\_%' AND u.sales_admin_id > 0
ON CONFLICT (user_id) WHERE status = 'active' DO NOTHING;

-- ---------------------------------------------------------------------------
-- 6. 订单 36 单（12 个月 × 每月 3 单），带 sales_admin_id 快照
--    近 3 个月中每月第 3 单标记为续费单（单号 DEMOR 前缀）
-- ---------------------------------------------------------------------------
WITH gen AS (
  SELECT m, s,
         date_trunc('month', now()) - ((11 - m) * interval '1 month') + (s * 3 || ' days')::interval AS created_at,
         (200 + ((m * 7 + s * 131) % 2801))::numeric AS amount,
         CASE WHEN (m + s) % 2 = 0 THEN 1 ELSE 2 END AS sales_seq,
         ((m * 3 + s) % 4) + 1 AS cust_seq,
         (m >= 9 AND s = 3) AS is_renewal
  FROM generate_series(0, 11) m, generate_series(1, 3) s
),
sales_map AS (
  SELECT id AS admin_id, row_number() OVER (ORDER BY username) AS seq
  FROM admins WHERE username IN ('demo_sales_01','demo_sales_02')
),
cust_map AS (
  SELECT u.id AS user_id, u.sales_admin_id AS admin_id,
         row_number() OVER (PARTITION BY u.sales_admin_id ORDER BY u.username) AS seq
  FROM users u WHERE u.username LIKE 'demo\_user\_%' AND u.sales_admin_id > 0
)
INSERT INTO orders (order_no, user_id, product_id, product_name, quantity, price_model,
                    total_amount, paid_amount, final_amount, original_amount, discount_amount,
                    status, pay_method, pay_time, cycle, created_at, updated_at, sales_admin_id)
SELECT
  (CASE WHEN g.is_renewal THEN 'DEMOR' ELSE 'DEMO' END)
    || to_char(g.created_at, 'YYYYMM') || '-' || g.s,
  c.user_id, 1, '年度造数云主机 2C4G', 1, 'fixed',
  g.amount, g.amount, g.amount, g.amount, 0,
  'active', 'balance', g.created_at, 'month', g.created_at, g.created_at, sm.admin_id
FROM gen g
JOIN sales_map sm ON sm.seq = g.sales_seq
JOIN cust_map  c  ON c.admin_id = sm.admin_id AND c.seq = g.cust_seq
ON CONFLICT (order_no) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 7. 提成台账：按订单生成计提；已过解冻期的补一条 release 流水并把
--    release_at 置 NULL（与 service 的幂等标记一致）
-- ---------------------------------------------------------------------------
WITH ranked AS (
  SELECT o.id, o.order_no, o.user_id, o.sales_admin_id, o.paid_amount, o.created_at,
         row_number() OVER (PARTITION BY o.user_id ORDER BY o.created_at, o.id) AS rn
  FROM orders o
  WHERE o.order_no LIKE 'DEMO%' AND o.sales_admin_id > 0
),
calc AS (
  SELECT r.*,
         CASE WHEN r.order_no LIKE 'DEMOR%' THEN 'accrue_renewal'
              WHEN r.rn = 1               THEN 'accrue_first'
              ELSE 'accrue_subsequent' END AS tx_type,
         CASE WHEN r.order_no LIKE 'DEMOR%' THEN 0.03
              WHEN r.rn = 1               THEN 0.08
              ELSE 0.05 END AS rate
  FROM ranked r
)
INSERT INTO sales_commission_transactions
  (tx_no, admin_id, type, direction, amount, balance_before, balance_after,
   biz_type, ref_no, order_id, order_no, customer_user_id, release_at, remark, operator_id, created_at)
SELECT
  'DMO-A-' || c.order_no, c.sales_admin_id, c.tx_type, 1,
  round(c.paid_amount * c.rate, 2), 0, 0,
  c.tx_type, 'DMO-A-' || c.order_no, c.id, c.order_no, c.user_id,
  CASE WHEN c.created_at + interval '30 days' > now()
       THEN c.created_at + interval '30 days' END,
  '年度造数计提（' || c.tx_type || '）', 0, c.created_at
FROM calc c
ON CONFLICT DO NOTHING;

-- 已过解冻期：补解冻流水，并将计提行的 release_at 清空（解冻完成标记）
WITH ranked AS (
  SELECT o.id, o.order_no, o.user_id, o.sales_admin_id, o.paid_amount, o.created_at,
         row_number() OVER (PARTITION BY o.user_id ORDER BY o.created_at, o.id) AS rn
  FROM orders o
  WHERE o.order_no LIKE 'DEMO%' AND o.sales_admin_id > 0
),
calc AS (
  SELECT r.*,
         CASE WHEN r.order_no LIKE 'DEMOR%' THEN 0.03 WHEN r.rn = 1 THEN 0.08 ELSE 0.05 END AS rate
  FROM ranked r
)
INSERT INTO sales_commission_transactions
  (tx_no, admin_id, type, direction, amount, balance_before, balance_after,
   biz_type, ref_no, order_id, order_no, customer_user_id, release_at, remark, operator_id, created_at)
SELECT
  'DMO-R-' || c.order_no, c.sales_admin_id, 'release', 1,
  round(c.paid_amount * c.rate, 2), 0, 0,
  'release', 'DMO-A-' || c.order_no, c.id, c.order_no, c.user_id,
  NULL, '年度造数解冻转可用', 0, c.created_at + interval '30 days'
FROM calc c
WHERE c.created_at + interval '30 days' <= now()
ON CONFLICT DO NOTHING;

UPDATE sales_commission_transactions t
   SET release_at = NULL
 WHERE t.tx_no LIKE 'DMO-A-%'
   AND t.release_at IS NOT NULL
   AND EXISTS (SELECT 1 FROM sales_commission_transactions r
                WHERE r.tx_no = 'DMO-R-' || t.order_no);

-- ---------------------------------------------------------------------------
-- 8. 提现单 4 张（pending / approved / paid / rejected）+ 对应打款单
-- ---------------------------------------------------------------------------
INSERT INTO sales_withdrawals (withdraw_no, admin_id, amount, channel, account, account_name,
                               bank_name, payout_mode, payout_no, payout_id, status,
                               audit_by, audit_by_name, audited_at, paid_at, remark, created_at, updated_at)
VALUES
  ('DMO-SW-0001', (SELECT id FROM admins WHERE username='demo_sales_01'), 120.00, 'alipay', '****0001', '销售甲',
   '', 'manual', '', 0, 'pending', 0, '', NULL, NULL, '年度造数·待审核', now() - interval '5 days', now() - interval '5 days'),
  ('DMO-SW-0002', (SELECT id FROM admins WHERE username='demo_sales_01'), 80.00, 'bank', '****0002', '销售甲',
   '招商银行深圳分行', 'manual', 'DMO-PO-0001', 0, 'approved',
   (SELECT id FROM admins WHERE username='demo_sales_mgr'), '销售主管', now() - interval '4 days', NULL, '年度造数·待打款', now() - interval '6 days', now() - interval '4 days'),
  ('DMO-SW-0003', (SELECT id FROM admins WHERE username='demo_sales_02'), 200.00, 'alipay', '****0003', '销售乙',
   '', 'manual', 'DMO-PO-0002', 0, 'paid',
   (SELECT id FROM admins WHERE username='demo_sales_mgr'), '销售主管', now() - interval '20 days', now() - interval '18 days', '年度造数·已打款', now() - interval '25 days', now() - interval '18 days'),
  ('DMO-SW-0004', (SELECT id FROM admins WHERE username='demo_sales_02'), 60.00, 'alipay', '****0004', '销售乙',
   '', 'manual', '', 0, 'rejected',
   (SELECT id FROM admins WHERE username='demo_sales_mgr'), '销售主管', now() - interval '9 days', NULL, '年度造数·已驳回', now() - interval '10 days', now() - interval '9 days')
ON CONFLICT (withdraw_no) DO NOTHING;

-- 打款单（biz_type=sales_withdraw，验证 S6 的 biz_type 解耦）
INSERT INTO payment_payouts (payout_no, withdraw_id, withdraw_no, user_id, amount_fen, channel_id, channel_code,
                             mode, status, biz_type, channel_tx, paid_at, operator_id, remark, created_at, updated_at)
SELECT p.payout_no, w.id, w.withdraw_no, 0, (w.amount * 100)::bigint, 0, '', 'manual',
       CASE WHEN w.status = 'paid' THEN 'paid' ELSE 'pending' END,
       'sales_withdraw',
       CASE WHEN w.status = 'paid' THEN 'DEMO-TX-0002' END,
       CASE WHEN w.status = 'paid' THEN w.paid_at END,
       (SELECT id FROM admins WHERE username='demo_sales_mgr'),
       '年度造数打款单', w.created_at, w.updated_at
FROM sales_withdrawals w
JOIN (VALUES ('DMO-SW-0002','DMO-PO-0001'), ('DMO-SW-0003','DMO-PO-0002')) AS p(withdraw_no, payout_no)
  ON p.withdraw_no = w.withdraw_no
ON CONFLICT (payout_no) DO NOTHING;

-- 提现单回写 payout_id
UPDATE sales_withdrawals w SET payout_id = p.id
  FROM payment_payouts p
 WHERE p.payout_no = w.payout_no AND w.payout_id = 0;

-- 提现台账流水：冻结 / 退回 / 打款
INSERT INTO sales_commission_transactions
  (tx_no, admin_id, type, direction, amount, balance_before, balance_after, biz_type, ref_no, remark, operator_id, created_at)
SELECT 'DMO-WF-' || w.withdraw_no, w.admin_id, 'withdraw_freeze', -1, w.amount, 0, 0,
       'withdraw_freeze', w.withdraw_no, '年度造数·提现申请冻结', 0, w.created_at
FROM sales_withdrawals w WHERE w.withdraw_no LIKE 'DMO-SW-%'
ON CONFLICT DO NOTHING;

INSERT INTO sales_commission_transactions
  (tx_no, admin_id, type, direction, amount, balance_before, balance_after, biz_type, ref_no, remark, operator_id, created_at)
SELECT 'DMO-WR-' || w.withdraw_no, w.admin_id, 'withdraw_return', 1, w.amount, 0, 0,
       'withdraw_return', w.withdraw_no, '年度造数·驳回解冻退回', 0, coalesce(w.audited_at, w.updated_at)
FROM sales_withdrawals w WHERE w.withdraw_no LIKE 'DMO-SW-%' AND w.status = 'rejected'
ON CONFLICT DO NOTHING;

INSERT INTO sales_commission_transactions
  (tx_no, admin_id, type, direction, amount, balance_before, balance_after, biz_type, ref_no, remark, operator_id, created_at)
SELECT 'DMO-WP-' || w.withdraw_no, w.admin_id, 'withdraw_paid', -1, w.amount, 0, 0,
       'withdraw_paid', w.withdraw_no, '年度造数·提现已打款', 0, coalesce(w.paid_at, w.updated_at)
FROM sales_withdrawals w WHERE w.withdraw_no LIKE 'DMO-SW-%' AND w.status = 'paid'
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- 9. 账户余额重算（以台账为准，重复执行不改结果）
-- ---------------------------------------------------------------------------
INSERT INTO sales_commission_accounts (admin_id, balance, frozen, pending_release, total_income, total_out, version, created_at, updated_at)
SELECT t.admin_id,
       coalesce(sum(CASE WHEN t.type IN ('accrue_first','accrue_subsequent','accrue_renewal') AND t.release_at IS NULL THEN t.amount
                         WHEN t.type = 'withdraw_freeze' THEN -t.amount
                         WHEN t.type = 'withdraw_return' THEN t.amount
                         ELSE 0 END), 0),
       coalesce(sum(CASE WHEN t.type = 'withdraw_freeze' THEN t.amount
                         WHEN t.type IN ('withdraw_return','withdraw_paid') THEN -t.amount
                         ELSE 0 END), 0),
       coalesce(sum(CASE WHEN t.type IN ('accrue_first','accrue_subsequent','accrue_renewal') AND t.release_at IS NOT NULL THEN t.amount ELSE 0 END), 0),
       coalesce(sum(CASE WHEN t.type IN ('accrue_first','accrue_subsequent','accrue_renewal') THEN t.amount ELSE 0 END), 0),
       coalesce(sum(CASE WHEN t.type = 'withdraw_paid' THEN t.amount ELSE 0 END), 0),
       1, now(), now()
FROM sales_commission_transactions t
WHERE t.tx_no LIKE 'DMO-%'
GROUP BY t.admin_id
ON CONFLICT (admin_id) DO UPDATE
  SET balance          = EXCLUDED.balance,
      frozen           = EXCLUDED.frozen,
      pending_release  = EXCLUDED.pending_release,
      total_income     = EXCLUDED.total_income,
      total_out        = EXCLUDED.total_out,
      version          = sales_commission_accounts.version + 1,
      updated_at       = now();

-- 台账展示：按时间回填每行的变动前/后可用余额
UPDATE sales_commission_transactions t
   SET balance_before = s.running - s.effect,
       balance_after  = s.running
  FROM (
    SELECT id,
           sum(CASE WHEN type IN ('accrue_first','accrue_subsequent','accrue_renewal') AND release_at IS NULL THEN amount
                    WHEN type = 'withdraw_freeze' THEN -amount
                    WHEN type = 'withdraw_return' THEN amount
                    ELSE 0 END)
             OVER (PARTITION BY admin_id ORDER BY created_at, id ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS running,
           (CASE WHEN type IN ('accrue_first','accrue_subsequent','accrue_renewal') AND release_at IS NULL THEN amount
                 WHEN type = 'withdraw_freeze' THEN -amount
                 WHEN type = 'withdraw_return' THEN amount
                 ELSE 0 END) AS effect
      FROM sales_commission_transactions
     WHERE tx_no LIKE 'DMO-%'
  ) s
 WHERE t.id = s.id;

-- ---------------------------------------------------------------------------
-- 10. 当月业绩目标：销售一部 3 人各 50000 / 部门目标 150000
-- ---------------------------------------------------------------------------
INSERT INTO sales_targets (period, scope, admin_id, department_id, target_amount, target_orders, created_by, created_at, updated_at)
SELECT to_char(now(), 'YYYY-MM'), 'admin', a.id, d.id, 50000, 30, 1, now(), now()
FROM admins a
JOIN departments d ON d.code = 'demo-sales-1'
WHERE a.username IN ('demo_sales_01','demo_sales_02','demo_sales_mgr')
ON CONFLICT (period, scope, admin_id, department_id) DO NOTHING;

INSERT INTO sales_targets (period, scope, admin_id, department_id, target_amount, target_orders, created_by, created_at, updated_at)
SELECT to_char(now(), 'YYYY-MM'), 'department', 0, d.id, 150000, 90, 1, now(), now()
FROM departments d WHERE d.code = 'demo-sales-1'
ON CONFLICT (period, scope, admin_id, department_id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 11. 工单 10 条（覆盖 open/in_progress/waiting_user/resolved；含双人复核样本）
-- ---------------------------------------------------------------------------
INSERT INTO tickets (ticket_no, user_id, title, description, category, category_id, priority, status,
                     assigned_to, assigned_name, department_id, review_status, created_at, updated_at)
SELECT
  'DMO-TK-' || lpad(i::text, 2, '0'),
  (SELECT id FROM users WHERE username = 'demo_user_' || lpad(((i - 1) % 12 + 1)::text, 2, '0')),
  '年度造数工单 ' || lpad(i::text, 2, '0'),
  'S8 年度跑通造数生成的演示工单，覆盖状态机与复核分支。',
  c.code, c.id,
  CASE WHEN i % 3 = 0 THEN 'high' WHEN i % 3 = 1 THEN 'medium' ELSE 'low' END,
  CASE i % 4 WHEN 0 THEN 'resolved' WHEN 1 THEN 'open' WHEN 2 THEN 'in_progress' ELSE 'waiting_user' END,
  CASE WHEN i % 4 = 2 THEN (SELECT id FROM admins WHERE username='demo_tech_01')
       WHEN i % 4 = 3 THEN (SELECT id FROM admins WHERE username='demo_support_01') END,
  CASE WHEN i % 4 = 2 THEN '技术甲' WHEN i % 4 = 3 THEN '客服甲' END,
  c.department_id,
  CASE WHEN i = 6 THEN 'pending' WHEN i = 7 THEN 'approved' END,
  now() - ((11 - i) * interval '3 days'),
  now() - ((11 - i) * interval '3 days')
FROM generate_series(1, 10) i
JOIN ticket_categories c
  ON c.code = CASE WHEN i <= 4 THEN 'technical' WHEN i <= 7 THEN 'authorization' ELSE 'consult' END
ON CONFLICT (ticket_no) DO NOTHING;

-- 复核样本：authorization 分类下一条待复核回复 + 一条已通过回复 + 一条内部备注
-- （ticket_replies 无业务唯一键，用 NOT EXISTS 保证重复执行不产生重复回复）
INSERT INTO ticket_replies (ticket_id, sender_type, sender_id, sender_name, content, is_internal, review_status, created_at)
SELECT t.id, 'admin', (SELECT id FROM admins WHERE username='demo_support_01'), '客服甲',
       '您的授权申请已受理，请确认额度生效。', false,
       CASE WHEN t.ticket_no = 'DMO-TK-06' THEN 'pending' ELSE 'approved' END,
       now() - interval '1 day'
FROM tickets t
WHERE t.ticket_no IN ('DMO-TK-05','DMO-TK-06')
  AND NOT EXISTS (SELECT 1 FROM ticket_replies r WHERE r.ticket_id = t.id AND r.sender_name = '客服甲');

INSERT INTO ticket_replies (ticket_id, sender_type, sender_id, sender_name, content, is_internal, created_at)
SELECT t.id, 'admin', (SELECT id FROM admins WHERE username='demo_support_02'), '客服乙',
       '内部备注：该客户历史退款较多，授权额度请从严。', true, now() - interval '2 days'
FROM tickets t
WHERE t.ticket_no = 'DMO-TK-06'
  AND NOT EXISTS (SELECT 1 FROM ticket_replies r WHERE r.ticket_id = t.id AND r.sender_name = '客服乙');

-- 复核人回填（已通过的回复记复核人 = 客服主管）
UPDATE ticket_replies r
   SET reviewer_id = (SELECT id FROM admins WHERE username='demo_support_lead'),
       reviewed_at = now() - interval '12 hours'
 WHERE r.review_status = 'approved'
   AND r.ticket_id IN (SELECT id FROM tickets WHERE ticket_no LIKE 'DMO-TK-%')
   AND r.reviewer_id IS NULL;

COMMIT;

-- ============================================================================
-- 回滚段（需要清理造数时手动执行；先删子表再删主表）
-- ----------------------------------------------------------------------------
-- BEGIN;
--   DELETE FROM sales_commission_transactions WHERE tx_no LIKE 'DMO-%';
--   DELETE FROM sales_commission_accounts WHERE admin_id IN (SELECT id FROM admins WHERE username LIKE 'demo\_%');
--   DELETE FROM payment_payouts WHERE payout_no LIKE 'DMO-PO-%';
--   DELETE FROM sales_withdrawals WHERE withdraw_no LIKE 'DMO-SW-%';
--   DELETE FROM sales_targets WHERE created_by = 1 AND period >= to_char(now(), 'YYYY-MM')
--     AND (admin_id IN (SELECT id FROM admins WHERE username LIKE 'demo\_%')
--          OR department_id IN (SELECT id FROM departments WHERE code LIKE 'demo-%'));
--   DELETE FROM ticket_replies WHERE ticket_id IN (SELECT id FROM tickets WHERE ticket_no LIKE 'DMO-TK-%');
--   DELETE FROM orders WHERE order_no LIKE 'DEMO%';
--   DELETE FROM tickets WHERE ticket_no LIKE 'DMO-TK-%';
--   DELETE FROM staff_sales_relations WHERE user_id IN (SELECT id FROM users WHERE username LIKE 'demo\_user\_%');
--   DELETE FROM users WHERE username LIKE 'demo\_user\_%';
--   DELETE FROM admin_roles WHERE admin_id IN (SELECT id FROM admins WHERE username LIKE 'demo\_%');
--   DELETE FROM admins WHERE username LIKE 'demo\_%';
--   -- 分类 technical / 客服组等如需还原，按需手工处理
--   DELETE FROM departments WHERE code LIKE 'demo-%';
-- COMMIT;
-- ============================================================================
