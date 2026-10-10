-- ============================================================================
-- 074_remove_finance_wallets_page.sql
-- 下线「财务管理 → 资金管理 → 用户钱包」页（/finance/accounts/wallets）：
--   该页只是「按 user_id 查一次余额」的只读页，且长期硬编码演示用户（doc34 F-09）；
--   单用户余额在「用户管理 → 用户详情 → 财务」已有同一数据源的面板
--   （FinancePanel 复用 GET /finance/wallets/:user_id），余额变动在「资金流水」按用户/关键词检索。
--
-- 改动（只删页面与菜单，不动接口与权限码）：
--   ① 删除菜单行：admin + /finance/accounts/wallets（连同其 role_permissions 无关）
--   ② 组内排序补空洞：资金管理组只剩「资金流水(1)」「人工调账(3)」→ 人工调账 归 2
--
-- 保留（刻意的）：
--   - GET /api/v1/admin/finance/wallets/:user_id 与 permissions 里的 finance:wallet 继续存在：
--     用户详情页财务面板依赖该接口；finance:wallet 同时是财务总览/资金流水/财务报表/统计接口的口径。
--   - /finance/accounts（资金管理）目录保留：下线后仍有 2 个启用叶子，满足 R1（≥2 才留目录）。
--
-- 幂等：DELETE 带 component 约束（只删本次下线的这一行，未来若有同名页面不会被误删），
--   重复执行第二次删 0 行；UPDATE 带旧值判断。
-- 顺序：本迁移先跑，再重启后端（seed 不再声明该行，且会把人工调账排序对齐为 2）。
-- 回滚：把 db.go 的「用户钱包」行加回 admin:/finance/accounts（SortOrder=2、人工调账改 3）
--   并在 router 恢复组件路由后重启；本迁移的 DELETE 无法回滚（页面代码已删）。
-- ============================================================================

BEGIN;

-- ① 删除已下线的菜单行
DELETE FROM menus
 WHERE platform = 'admin'
   AND path = '/finance/accounts/wallets'
   AND component = 'finance/accounts/wallets/index';

-- ② 组内排序补空洞（资金流水 1 → 人工调账 2）
UPDATE menus SET sort_order = 2
 WHERE platform = 'admin'
   AND path = '/finance/accounts/adjust'
   AND sort_order <> 2;

COMMIT;

-- 验收查询（人工执行）
--   菜单行已删除：
--   SELECT count(*) FROM menus WHERE platform = 'admin' AND path = '/finance/accounts/wallets';
--   期望：0
--   组内只剩两个叶子且排序连续：
--   SELECT m.path, m.name, m.sort_order FROM menus m
--     JOIN menus p ON p.id = m.parent_id
--    WHERE m.platform = 'admin' AND p.path = '/finance/accounts' ORDER BY m.sort_order;
--   期望：/finance/transactions(1, 资金流水)、/finance/accounts/adjust(2, 人工调账)
--   接口仍在（用户详情页财务面板依赖）：
--   SELECT count(*) FROM permissions WHERE code = 'finance:wallet';
--   期望：1
