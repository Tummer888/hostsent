-- ============================================================================
-- 060_merge_agent_menu_into_groups.sql
-- doc108 第三轮收口：二级菜单「代理等级」并入「用户组管理 → 代理分组」分区。
--
-- 背景：代理等级的完整管理（折扣矩阵单格编辑 + 阶梯填充 + 等级增删改查）已
--   内嵌为用户组管理页的「代理分组」分区，独立的二级菜单入口只剩重复导航，
--   还让侧边栏多一项。本迁移只删菜单行；agent_level:* 权限码保留（分区内的
--   按钮与接口仍按它鉴权）。
--
-- 幂等：条件删除，可重复执行。
-- 回滚：INSERT INTO menus (parent_id, platform, name, type, path, component,
--   icon, sort_order, status) VALUES (4, 'admin', '代理等级', 'menu',
--   '/users/agents', 'users/agents/index', 'usergroup-add', 6, 'active');
--   （parent_id=4 为 admin:/users 目录，回滚前请先确认该目录仍在。）
-- ============================================================================

BEGIN;

DELETE FROM menus
 WHERE platform = 'admin'
   AND path = '/users/agents'
   AND component = 'users/agents/index';

COMMIT;
