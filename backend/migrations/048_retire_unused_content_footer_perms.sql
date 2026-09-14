-- ============================================================================
-- 048_retire_unused_content_footer_perms.sql
--
-- 背景（doc100 §7.1 与 §7.2 收口）：
--   044 之后 seed 了三个内容类权限码，其中 `content:footer:view` 与
--   `content:footer:update` 被设计成「站点页脚」页面的菜单权限 —— 但页脚
--   从来没有独立页面：四个 site.footer_* 配置键就躺在「系统管理 → 系统配置
--   → 页脚」分组里，权限走 system:config:*。
--
--   结果是这两个权限码在**整仓范围内零引用**：没有路由、没有按钮、没有
--   permission_map 登记，只在超管角色里被显式赋了值。一个永远赋不到、
--   也永远拦不住任何东西的权限码，会让配权限的人以为自己控制了页脚，
--   与 047 清理的那批消息模板配置键是同一类静默失效。
--
-- 内容：
--   1. 先解除角色绑定（role_permissions），再删权限行 —— 顺序不能反，
--      否则留下指向不存在权限的角色绑定行；
--   2. 只删这两个码，`content` 目录与 article/category/link 六个码保留。
--
-- 幂等：DELETE ... WHERE code IN (...)，重复执行无副作用（第二遍删 0 行）。
--
-- 回滚：如需恢复，重新执行 seedPermissions 或手工插回这两行并重新赋权；
--       由于它们没有任何消费点，恢复后行为与现在完全一致（都不生效）。
-- ============================================================================

BEGIN;

-- 1. 解除角色绑定（存在则删）
DELETE FROM role_permissions
 WHERE permission_id IN (
        SELECT id FROM permissions
         WHERE code IN ('content:footer:view', 'content:footer:update')
       );

-- 2. 删除权限行
DELETE FROM permissions
 WHERE code IN ('content:footer:view', 'content:footer:update');

COMMIT;
