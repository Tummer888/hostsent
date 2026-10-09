-- ============================================================================
-- 071_group_system_settings_under_config.sql
-- 「系统配置」由单叶子升级为二级目录，把三个设置类入口收进同一组（用户反馈）：
--   第三方登录 /system/oauth、验证码配置 /system/captcha、操作审计 /system/audit-logs
--   → 迁到 /system/config/{oauth,captcha,audit-logs}；
--   原 /system/config 页面（键值型配置）→ /system/config/basic（菜单名「基础配置」）。
--
-- 目的：/system 一级列表从 10 项降到 7 项，系统设置类入口在一处即可完成；
--   操作审计与「用户管理 → 安全与风控」共用 security:audit:list，权限码与 HTTP API 全部不变。
--
-- 注意：`/system/config` 这一行**不需要删**——seed 按 (platform,path) 匹配已有行并就地更新
--   （Type: menu → directory、Component 清空、ParentID 不变），所以本迁移只删三条旧路径，
--   否则会与新叶子 /system/config/* 并存造成重复入口。
--
-- 幂等：条件 DELETE，重复执行删 0 行。
-- 顺序：本迁移先跑，再重启后端（seed 会更新 /system/config 并插入 4 个新叶子）。
-- 回滚：把 db.go 的菜单种子改回四个平级叶子后重启；旧行可由 seed 重建，无数据损失。
-- ============================================================================

BEGIN;

-- 只删「旧结构」的行：迁移 071 之前这三个叶子直接挂在 /system 下（parent = /system）。
-- 必须带 parent 条件 —— 迁移 072 会把 /system/audit-logs 以「日志中心子项」的身份重新启用，
-- 若这里只按 path 删，重复执行本迁移就会把那条合法菜单一并删掉（曾实测踩到）。
DELETE FROM menus
 WHERE platform = 'admin'
   AND path IN ('/system/captcha', '/system/oauth', '/system/audit-logs')
   AND parent_id = (SELECT id FROM menus WHERE platform = 'admin' AND path = '/system');

COMMIT;

-- 验收查询（人工执行）
--   SELECT path,name,type,component FROM menus WHERE path LIKE '/system/config%' ORDER BY path;
--   期望：/system/config（directory，无 component）
--         /system/config/basic（system:config:view 对应的页面）
--         /system/config/captcha、/system/config/oauth、/system/config/audit-logs
--   SELECT count(*) FROM menus WHERE platform='admin' AND path IN ('/system/captcha','/system/oauth','/system/audit-logs');
--   期望：0
