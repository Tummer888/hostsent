-- ============================================================================
-- 068_merge_sales_and_notification_into_system.sql
-- 销售中心（doc86）与消息中心（doc70/doc90）整域并入「系统管理」（用户反馈）：
--   两者都是后台运营配置类模块，独立一级域让侧边栏多出两项。
--
-- 改动（归属调整，HTTP API / 权限码均不变）：
--   后端模块 modules/admin/sales        → modules/admin/system/sales
--              modules/admin/notification → modules/admin/system/notification
--   前端页面 pages/sales                → pages/system/sales
--              pages/notification       → pages/system/notification
--   菜单种子（db.go SeedMenus）改挂到 admin:/system 下的「销售中心」「消息中心」两个二级目录，
--   叶子路径 /sales/* 、/notification/* → /system/sales/* 、/system/notification/*；
--   旧路径由 router redirect 兼容书签。
--   注：公告管理（菜单在 /content/announcements）的页面随 notification 模块一起搬到
--   pages/system/notification/announcements（doc102 M2-4 的决定：菜单归内容管理、
--   页面/模块/权限码随 notification），seed 的 component 已同步。
--   seed 只增不删（按 (platform,path) 匹配已有行），旧行必须在这里显式删除，
--   否则侧边栏会同时出现新旧两套入口。
--
-- 幂等：条件 DELETE，重复执行删 0 行。
-- 顺序：本迁移先跑，再重启后端（seed 会插入新的 /system/{sales,notification} 行）。
-- 回滚：把 db.go 的菜单种子改回 /sales、/notification 一级域后重启即可；无数据损失。
-- ============================================================================

BEGIN;

DELETE FROM menus
 WHERE platform = 'admin'
   AND (path = '/sales' OR path LIKE '/sales/%');

DELETE FROM menus
 WHERE platform = 'admin'
   AND (path = '/notification' OR path LIKE '/notification/%');

COMMIT;

-- 验收查询（人工执行）
--   SELECT path,name FROM menus WHERE path LIKE '/system/sales%' OR path LIKE '/system/notification%' ORDER BY path;
--   期望：/system/notification（目录）+ 6 叶子（templates/sms-templates/channels/records/deliveries/broadcast）
--         /system/sales（目录）+ 4 叶子（customers/commissions/withdrawals/performance）
