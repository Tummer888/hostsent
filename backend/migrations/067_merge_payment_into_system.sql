-- ============================================================================
-- 067_merge_payment_into_system.sql
-- doc35 支付中心整域并入「系统管理」（用户反馈）：收款渠道、支付订单、打款与对账
--   都是后台配置与运维动作，独立一级域让侧边栏多一项，也让「系统相关配置」分成两处。
--
-- 改动（归属调整，HTTP API / 权限码均不变）：
--   后端模块 modules/admin/payment  → modules/admin/system/payment
--   前端页面 pages/payment          → pages/system/payment
--   菜单种子（db.go SeedMenus）改挂到 admin:/system 下的「支付中心」二级目录，
--   叶子路径 /payment/* → /system/payment/*；旧路径由 router redirect 兼容书签。
--   注：侧栏只渲染三层（一级域 + 二级分组 + 叶子），原「渠道管理 / 交易管理 /
--   出款与对账」三个二级分类在并入后必须压平，否则第四层叶子在侧栏点不到。
--   seed 只增不删（按 (platform,path) 匹配已有行），旧行必须在这里显式删除，
--   否则侧边栏会同时出现新旧两套入口。
--
-- 幂等：条件 DELETE，重复执行删 0 行。
-- 顺序：本迁移先跑，再重启后端（seed 会插入新的 /system/payment 行）。
-- 回滚：把 db.go 的菜单种子改回 /payment 一级域后重启即可；无数据损失。
-- ============================================================================

BEGIN;

DELETE FROM menus
 WHERE platform = 'admin'
   AND (path = '/payment' OR path LIKE '/payment/%');

COMMIT;

-- 验收查询（人工执行）
--   SELECT path,name FROM menus WHERE path LIKE '/system/payment%' ORDER BY path;
--   期望：/system/payment（目录）+ 8 个叶子
--         overview / channels / methods / orders / callbacks / refunds / payouts / recon
