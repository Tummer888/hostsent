-- ============================================================================
-- 073_finance_module_reorg.sql
-- 财务管理模块整理（本轮）：
--   ① 资金流水改挂「资金管理」组 —— 余额与余额变动（用户钱包 / 人工调账 / 资金流水）
--      是同一件事的三个视角；此前「资金流水」孤悬在域根，与钱包/调账割裂。
--      路径不变（/finance/transactions），沿用 /finance/recharges 挂
--      /finance/recharge-center 下的既有先例 —— 因此 router 无需 redirect、权限码不变。
--   ② 目录改名（只改 Name）：
--      /finance/accounts    钱包与调账 → 资金管理
--      /finance/bill-center 账单管理   → 账单与对账（其下同时挂账单/发票/对账中心）
--   ③ 清除 5 个历史死键：finance_billing_cycle / finance_tax_rate /
--      finance_recon_threshold / finance_balance_warning /
--      finance_manual_adjust_enabled —— 整仓零引用（无任何后端读取点），
--      由旧「财务配置」页写库、改了不生效；新参数键见 db.go seed（finance 分组）。
--
-- 幂等：UPDATE 按 (platform, path) 定位且带旧值判断；DELETE 按键名精确匹配。
--   重复执行第二次 UPDATE 影响 0 行（已是新值）、DELETE 删 0 行。
-- 顺序：本迁移先跑，再重启后端（seed 会按 (platform, path) 覆盖 Name/ParentID 并
--   播种 finance 分组三个新键；menus 表是 seed 的投影，二者结果一致）。
-- 回滚：把下面两处 UPDATE 的新旧值对调、DELETE 无法回滚（死键无消费方），
--   新键由 seed 保留。
-- ============================================================================

BEGIN;

-- ① 资金流水改挂「资金管理」组（组路径 /finance/accounts 不变）
UPDATE menus
   SET parent_id = (SELECT id FROM menus WHERE platform = 'admin' AND path = '/finance/accounts'),
       sort_order = 1
 WHERE platform = 'admin'
   AND path = '/finance/transactions'
   AND parent_id IS DISTINCT FROM (SELECT id FROM menus WHERE platform = 'admin' AND path = '/finance/accounts');

-- 组内既有叶子让位排序：资金流水(1) → 用户钱包(2) → 人工调账(3)
UPDATE menus SET sort_order = 2
 WHERE platform = 'admin' AND path = '/finance/accounts/wallets' AND sort_order <> 2;
UPDATE menus SET sort_order = 3
 WHERE platform = 'admin' AND path = '/finance/accounts/adjust' AND sort_order <> 3;

-- ② 目录改名（只改展示名，路径/权限码/组件不变）
UPDATE menus SET name = '资金管理'
 WHERE platform = 'admin' AND path = '/finance/accounts' AND name <> '资金管理';
UPDATE menus SET name = '账单与对账'
 WHERE platform = 'admin' AND path = '/finance/bill-center' AND name <> '账单与对账';

-- ③ 清除历史死键（整仓零引用；新键由 seed 播种）
DELETE FROM system_configs
 WHERE config_key IN (
   'finance_billing_cycle',
   'finance_tax_rate',
   'finance_recon_threshold',
   'finance_balance_warning',
   'finance_manual_adjust_enabled'
 );

COMMIT;

-- 验收查询（人工执行）
--   菜单归属：
--   SELECT m.path, m.name, p.path AS parent, m.sort_order
--     FROM menus m JOIN menus p ON p.id = m.parent_id
--    WHERE m.platform = 'admin' AND m.path IN
--          ('/finance/transactions','/finance/accounts','/finance/bill-center')
--    ORDER BY m.path;
--   期望：/finance/transactions → parent=/finance/accounts, sort_order=1
--         /finance/accounts     → name=资金管理（一级域 /finance 之下）
--         /finance/bill-center  → name=账单与对账
--   死键清零：
--   SELECT count(*) FROM system_configs WHERE config_key LIKE 'finance\_%' ESCAPE '\';
--   期望：0（新键为 finance. 前缀，见下）
--   新键就位（重启后端后）：
--   SELECT config_key, config_value, config_group FROM system_configs
--    WHERE config_group = 'finance' ORDER BY sort_order;
--   期望：finance.adjust_enabled / finance.recon_tolerance / finance.balance_warning
