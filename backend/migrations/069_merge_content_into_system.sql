-- ============================================================================
-- 069_merge_content_into_system.sql
-- 内容管理（doc100 §7.1）整域并入「系统管理」（用户反馈）：门户展示型内容
--   （文章/分类/友链）与公告都属后台运营配置，独立一级域让侧边栏多出一项。
--
-- 改动（归属调整，HTTP API / 权限码均不变）：
--   后端模块 modules/admin/content → modules/admin/system/content
--   前端页面 pages/content          → pages/system/content
--   菜单种子（db.go SeedMenus）改挂到 admin:/system 下的「内容管理」二级目录，
--   叶子路径 /content/* → /system/content/*；旧路径由 router redirect 兼容书签。
--   公告管理（doc102 §4.1 M2-4 的 R5 例外）：菜单随内容管理迁移，页面仍随
--   notification 模块放在 pages/system/notification/announcements，权限码
--   notify:announcement 不变。
--   seed 只增不删（按 (platform,path) 匹配已有行），旧行必须在这里显式删除，
--   否则侧边栏会同时出现新旧两套入口。
--
-- 幂等：条件 DELETE，重复执行删 0 行。
-- 顺序：本迁移先跑，再重启后端（seed 会插入新的 /system/content 行）。
-- 回滚：把 db.go 的菜单种子改回 /content 一级域后重启即可；无数据损失。
-- ============================================================================

BEGIN;

DELETE FROM menus
 WHERE platform = 'admin'
   AND (path = '/content' OR path LIKE '/content/%');

COMMIT;

-- 验收查询（人工执行）
--   SELECT path,name,component FROM menus WHERE path LIKE '/system/content%' ORDER BY path;
--   期望：/system/content（目录）+ 4 叶子（articles / categories / links / announcements），
--         其中 announcements 的 component 为 system/notification/announcements/index
