-- ============================================================================
-- 084_move_recharge_withdraw_menus.sql
-- 财务菜单整理（2026-10-10）：撤销「充值提现」二级目录，两个叶子各自归位 ——
--   ① 充值管理 改挂「资金管理」(/finance/accounts)，排序 3；
--   ② 提现管理 改挂「推广返现」(/finance/referral)，排序 4；
--   ③ 删除 /finance/recharge-center 目录行及其残留子行（孤儿菜单，doc102 §5.1 的老坑）；
--   ④ /finance 域其余兄弟排序前移补位（原 5..9 → 4..8，与迁移 073 补位同一做法）。
--
-- 为什么这样归位：充值是「钱进钱包」的上游一步，与资金流水/人工调账同属资金管理；
-- 钱包提现与返现提现（referral_withdrawals）在运营上是同一批动作，集中在推广返现处理。
--
-- 路径不变（/finance/recharges、/finance/withdrawals）—— 与迁移 073 同一取舍：
-- URL / 收藏 / 权限映射稳定，只改归属与展示位置；权限码不变
-- （finance:recharge、finance:withdraw，登录角色授权不受影响）。
-- 同步改动：db.go 的 seed（menus 表是 seed 的投影）与前端 permission.ts navMenu
-- （去掉「充值提现」）；前端页面组件迁到 finance/accounts/recharges 与
-- finance/referral/wallet-withdrawals；后端子域迁到 finance/account/recharge 与
-- finance/referral/walletwithdraw（HTTP 路由不变）。
--
-- 幂等：UPDATE 按 (platform, path) 定位且带目标值判断；DELETE 按父目录精确定位，
--   重复执行第二次影响 0 行。顺序：本迁移先跑，再重启后端
--   （seed 按 (platform, path) 覆盖 ParentID/SortOrder/Component，二者结果一致）。
-- 回滚（手工）：重建目录行并改回归属 ——
--   INSERT INTO menus (parent_id, platform, name, type, path, component, icon, sort_order, status)
--     SELECT id, 'admin', '充值提现', 'directory', '/finance/recharge-center', '', 'download', 4, 'active'
--       FROM menus WHERE platform='admin' AND path='/finance';
--   UPDATE menus SET parent_id = (SELECT id FROM menus WHERE path='/finance/recharge-center'),
--     sort_order = CASE path WHEN '/finance/recharges' THEN 1 WHEN '/finance/withdrawals' THEN 2 END
--    WHERE path IN ('/finance/recharges','/finance/withdrawals');
-- ============================================================================

BEGIN;

-- ① 充值管理 → 资金管理（排在 资金流水(1) / 人工调账(2) 之后）
UPDATE menus
   SET parent_id = (SELECT id FROM menus WHERE platform = 'admin' AND path = '/finance/accounts'),
       sort_order = 3
 WHERE platform = 'admin'
   AND path = '/finance/recharges'
   AND (parent_id IS DISTINCT FROM (SELECT id FROM menus WHERE platform = 'admin' AND path = '/finance/accounts')
        OR sort_order <> 3);

-- ② 提现管理 → 推广返现（排在 返现台账(1) / 提现审核(2) / 邀请关系(3) 之后）
UPDATE menus
   SET parent_id = (SELECT id FROM menus WHERE platform = 'admin' AND path = '/finance/referral'),
       sort_order = 4
 WHERE platform = 'admin'
   AND path = '/finance/withdrawals'
   AND (parent_id IS DISTINCT FROM (SELECT id FROM menus WHERE platform = 'admin' AND path = '/finance/referral')
        OR sort_order <> 4);

-- ③ 撤销「充值提现」目录：先清其下残留子行（归位完成后正常为空），再删目录行本身
DELETE FROM menus
 WHERE platform = 'admin'
   AND parent_id = (SELECT id FROM menus WHERE platform = 'admin' AND path = '/finance/recharge-center');
DELETE FROM menus
 WHERE platform = 'admin'
   AND path = '/finance/recharge-center';

-- ④ /finance 域兄弟排序补位（空出的 4 号槽由账单与对账接上）
UPDATE menus SET sort_order = 4 WHERE platform = 'admin' AND path = '/finance/bill-center' AND sort_order <> 4;
UPDATE menus SET sort_order = 5 WHERE platform = 'admin' AND path = '/finance/report' AND sort_order <> 5;
UPDATE menus SET sort_order = 6 WHERE platform = 'admin' AND path = '/finance/config' AND sort_order <> 6;
UPDATE menus SET sort_order = 7 WHERE platform = 'admin' AND path = '/finance/referral' AND sort_order <> 7;
UPDATE menus SET sort_order = 8 WHERE platform = 'admin' AND path = '/finance/points' AND sort_order <> 8;

COMMIT;

-- 验收查询（人工执行）
--   归属与排序：
--   SELECT m.path, m.name, p.path AS parent, m.sort_order
--     FROM menus m JOIN menus p ON p.id = m.parent_id
--    WHERE m.platform='admin' AND m.path IN
--      ('/finance/recharges','/finance/withdrawals','/finance/bill-center','/finance/referral')
--    ORDER BY p.path, m.sort_order;
--   期望：/finance/recharges → parent=/finance/accounts, sort=3
--         /finance/withdrawals → parent=/finance/referral, sort=4
--         /finance/bill-center → sort=4（一级）
--   旧目录已不存在：
--   SELECT count(*) FROM menus WHERE platform='admin' AND path LIKE '/finance/recharge-center%';
--   期望：0
