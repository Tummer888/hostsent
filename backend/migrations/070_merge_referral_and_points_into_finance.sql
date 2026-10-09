-- ============================================================================
-- 070_merge_referral_and_points_into_finance.sql
-- 推广返现（doc84）与积分中心（doc36）整域并入「财务管理」（用户反馈）：
--   两者都是资金/权益账本类模块（返现台账、提现审核、积分账户与流水），
--   并入财务域后运营在一处看账，侧边栏也少两项。
--
-- 改动（归属调整，HTTP API / 权限码均不变）：
--   后端模块 modules/admin/referral → modules/admin/finance/referral
--            modules/admin/point    → modules/admin/finance/point
--   前端页面 pages/referral        → pages/finance/referral
--            pages/points          → pages/finance/points
--   菜单种子（db.go SeedMenus）改挂到 admin:/finance 下的「推广返现」「积分中心」两个二级目录，
--   叶子路径 /referral/* 、/points/* → /finance/referral/* 、/finance/points/*；
--   旧路径由 router redirect 兼容书签。
--   ⚠ 用户端（frontend-user）另有同名菜单与路由：platform=user 的 /points（我的积分）与
--   /referral（推广邀请 + 5 个子项）保持不动 —— 本迁移只删 platform='admin' 的行。
--   seed 只增不删（按 (platform,path) 匹配已有行），旧行必须在这里显式删除，
--   否则侧边栏会同时出现新旧两套入口。
--
-- 幂等：条件 DELETE，重复执行删 0 行。
-- 顺序：本迁移先跑，再重启后端（seed 会插入新的 /finance/{referral,points} 行）。
-- 回滚：把 db.go 的菜单种子改回 /referral、/points 一级域后重启即可；无数据损失。
-- ============================================================================

BEGIN;

DELETE FROM menus
 WHERE platform = 'admin'
   AND (path = '/referral' OR path LIKE '/referral/%');

DELETE FROM menus
 WHERE platform = 'admin'
   AND (path = '/points' OR path LIKE '/points/%');

COMMIT;

-- 验收查询（人工执行）
--   SELECT platform, path, name FROM menus WHERE path LIKE '/finance/referral%' OR path LIKE '/finance/points%' ORDER BY path;
--   期望：admin 下 /finance/referral（目录）+ 3 叶子、/finance/points（目录）+ 4 叶子
--   SELECT platform, path FROM menus WHERE path IN ('/points','/referral') OR path LIKE '/referral/%';
--   期望：只剩 platform=user 的 6 行（/points 与 /referral 及其 5 个子项）
