-- ============================================================================
-- 049_retire_legacy_user_orders_and_role_perms.sql
--
-- 背景（doc103 §4.8 / §4.9）：
--   用户详情页的「订单」「工单」两个 Tab 长期显示 0 条，根因不是展示而是数据源：
--   `model/user_detail.go` 里定义了 UserOrder / UserTicket 两个模型，各自
--   `TableName()` 分别指向 `user_orders` / `user_tickets` —— 这两张表早已在
--   Phase 2 被权威表 `orders` / `tickets` 取代（见 migrations 013、014 的同类
--   退役），但 AutoMigrate 里始终留着这两个模型，于是每次服务启动都会把
--   `user_tickets` 重新建成空表，覆盖 `migrateLegacyTickets` 做的归档重命名，
--   使退役表残骸永远存在。
--
--   实测（2026-09-15）：
--     orders          → 89 行        user_orders   → 3 行
--     tickets         → 18 行        user_tickets  → 0 行
--     orders WHERE user_id=2 → 21    tickets WHERE user_id=7 → 8
--   而聚合接口对同两个用户分别返回 orders=0 / tickets=0。
--
--   那 3 行 user_orders 全部属于 `user_nw_01`，该用户名在 `users` 表中**不存在**
--   （由 `seedDemoUserDetails` 每次启动写入，user_id 对不上任何活跃用户），
--   属于孤儿演示数据，不迁入 orders。
--
--   同时清理 role `user`（普通用户，id=7）身上挂着的三个**后台**权限码
--   `system:user` / `system:user:list` / `user:detail`：客户角色不登录后台，
--   不应持有任何后台权限。它们的唯一可见后果就是让客户详情页渲染出
--   「用户列表 / 用户管理 / 查看用户详情」这类后台权限节点（doc103 §1.2 P0-⑤）。
--   seed 已同步改为空集合（seed 只增不删，存量绑定必须在这里清）。
--
-- 内容：
--   1. 快照 user_orders 到 retired_user_orders_20260915（留证据），再 DROP；
--   2. DROP 归档表 user_tickets_legacy（migrateLegacyTickets 的产物，0 行）；
--   3. 解除 role `user` 的后台权限绑定。
--
-- 幂等：全部带 IF EXISTS / 存在性判断，重复执行无副作用。
--
-- 回滚：1 的快照表保留了原始 3 行，需要时 `INSERT INTO user_orders SELECT ...`
--       即可还原（但该表已无任何读路径，还原后也不会被读到）；
--       3 可通过重新执行 seedRolePermissions 恢复（新 seed 不再赋权，需手工插回）。
-- ============================================================================

BEGIN;

-- 1. 退役 user_orders：先留快照，再删除。
DO $$
BEGIN
    IF to_regclass('public.user_orders') IS NOT NULL THEN
        CREATE TABLE IF NOT EXISTS retired_user_orders_20260915 AS
            SELECT * FROM user_orders;
        RAISE NOTICE 'user_orders 已快照到 retired_user_orders_20260915（% 行）',
            (SELECT COUNT(*) FROM retired_user_orders_20260915);
        DROP TABLE user_orders;
    ELSE
        RAISE NOTICE 'user_orders 不存在，跳过';
    END IF;
END $$;

-- 2. 归档表 user_tickets_legacy：内容已由 migrateLegacyTickets 迁入 tickets，本身 0 行。
DROP TABLE IF EXISTS user_tickets;

-- 3. role `user` 不应持有后台权限码。
DELETE FROM role_permissions
 WHERE role_id IN (SELECT id FROM roles WHERE code = 'user')
   AND permission_id IN (
        SELECT id FROM permissions
         WHERE code IN ('system:user', 'system:user:list', 'user:detail')
       );

COMMIT;