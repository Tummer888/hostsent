-- ============================================================================
-- 056_staff_user_domain_split.sql
--
-- 背景（员工域 / 客户域串台，实测 2026-09-23）：
--   `admins` 与 `users` 已是两张独立的身份表（各自 JWT、各自鉴权中间件、
--   各自角色绑定表 admin_roles / user_roles），但三处收口没做，导致运营能看到
--   员工混进用户列表、用户列表能挂后台角色、员工登录被算进用户详情：
--
--   ① 残留客户账号 `users.username='admin'`
--      该行 created_at=2026-08-20，早于真正的 `admins.id=1`（2026-08-27），是
--      分表之前的历史遗留。它 status=active、口令为弱口令 `123456`，可用
--      用户端登录接口换取正常用户令牌（实测成功），且挂着
--      `user_roles → role_id=4 (super_admin)`，于是用户列表里它显示成
--      「超级管理员」，点进详情是一堆空的订单/账单/钱包面板。
--      逐表核对过：orders/bills/recharges/wallet_accounts/wallet_transactions/
--      instances/tickets/invoice_requests/notifications 全为 0，只有 3 条
--      2026-09-15 的登录日志。属于纯脏数据，无业务引用。
--
--   ② `login_logs` / `user_sessions` 的 user_id 混用两个 ID 空间
--      管理员登录成功时（admin_service.go）写的是 `user_id = admins.id`，
--      与用户登录共用一张表，仅靠 platform='admin' 区分；而用户详情页的
--      登录次数 / 有效会话数是按 `user_id` 裸查的。`admins` 与 `users` 的
--      自增 ID 有 7 组撞号，于是员工的后台登录被算到不相干的客户头上：
--        实测 users.id=16（客户 787878）login_count=10，实为
--             admins.id=16（demo_support_01）的后台登录；
--             users.id=19（客户 user）login_count=45，其中 44 条属于
--             admins.id=19（demo_tech_01）。
--      代登录（login_type='impersonate'）必须留在客户域 —— 它描述的是
--      「某客户当前有哪些登录态」，只是 platform='admin' 用于让运营一眼
--      分辨是自己代的。所以不能按 platform 过滤，必须新增域判别列。
--
--   ③ `roles.scope` 隔离失效
--      roles id=7（code='user'）在线上是 scope='admin'，与 seedRoles
--      （RoleScopeUser）和迁移 018 的 `UPDATE roles SET scope='user'` 都矛盾。
--      后果是角色列表的 scope=user 过滤形同虚设，管理端把「普通用户」当
--      后台角色返回（实测 GET /api/v1/admin/roles 含 (7,'user','admin')），
--      员工建号的角色下拉里能选中客户角色。重启后端不会自愈：seedRoles 的
--      回填条件是 `existing.Scope == ""`，非空的错值永远不被纠正。
--
-- 内容：
--   1. 纠正 roles id=7 的 scope（按 code 定位，幂等）；
--   2. login_logs / user_sessions 新增 subject_type（user/admin）+ 存量回填；
--   3. 软删除残留客户账号 users.username='admin' 并解除其 user_roles 绑定。
--
-- 幂等：全部带 IF NOT EXISTS / 存在性判断；3 的软删除条件带 deleted_at IS NULL，
--       重复执行第二次影响 0 行。
--
-- 回滚：
--   1. UPDATE roles SET scope='admin' WHERE code='user';
--   2. ALTER TABLE login_logs DROP COLUMN subject_type;
--      ALTER TABLE user_sessions DROP COLUMN subject_type;
--   3. UPDATE users SET deleted_at=NULL, deleted_by=0, delete_reason='',
--        status_before_delete='', status='active' WHERE username='admin'
--        AND email='admin@hostsent.local';
--      （user_roles 绑定需按需手工插回，但该绑定本身就是跨域污染，不建议恢复）
--
-- 注意（R1 schema 双写）：本文件仅作版本留痕；运行时的建列由
--   internal/pkg/db/db.go 的 AutoMigrate 完成（模型见 security/model/login_log.go、
--   security/model/session.go）。
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. 角色作用域：客户角色不进后台权限树
-- ---------------------------------------------------------------------------
UPDATE roles SET scope = 'user', updated_at = now()
 WHERE code = 'user' AND scope <> 'user';

-- ---------------------------------------------------------------------------
-- 2. 登录日志 / 会话的域判别列
--
--    默认 'user'：两张表的主要生产者是客户侧登录（密码/短信/邮箱/OAuth），
--    管理员登录是少数派，默认值让新增写入方无需关心本列即天然落在客户域。
--
--    回填口径（关键）：
--      platform='admin' 且 login_type<>'impersonate' → admin
--      会话侧同义：platform='admin' 且 session_id NOT LIKE 'impersonate_%' → admin
--    代登录的两条记录都留在 'user' —— 它们描述的是客户账号的登录态，
--    必须继续出现在客户详情的「安全与登录」里，否则强制下线就找不到目标。
-- ---------------------------------------------------------------------------
ALTER TABLE login_logs ADD COLUMN IF NOT EXISTS subject_type varchar(16) NOT NULL DEFAULT 'user';
ALTER TABLE user_sessions ADD COLUMN IF NOT EXISTS subject_type varchar(16) NOT NULL DEFAULT 'user';

UPDATE login_logs SET subject_type = 'admin'
 WHERE platform = 'admin' AND login_type <> 'impersonate' AND subject_type <> 'admin';

UPDATE user_sessions SET subject_type = 'admin'
 WHERE platform = 'admin' AND session_id NOT LIKE 'impersonate_%' AND subject_type <> 'admin';

-- 域内查询是最高频路径（用户详情、在线用户、会话列表），按 (域, user_id) 建索引。
CREATE INDEX IF NOT EXISTS idx_login_logs_subject_user ON login_logs (subject_type, user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_subject_user ON user_sessions (subject_type, user_id);

-- ---------------------------------------------------------------------------
-- 3. 残留客户账号 users.username='admin' 的清理
--
--    软删除而不是硬删除：注销态（deleted_at 非空）已让该账号无法登录
--    （uc/auth 的 FindByUsername 带 deleted_at IS NULL），也不再出现在用户列表
--    与详情页，同时保留行本身以备追溯 —— 硬删会一并清掉它的 3 条登录日志。
--
--    守卫：仅当该行确实是「与 admins 同名的残留账号」且没有任何业务数据时才处理。
--    若运营已自行注册过同名客户账号（有订单/账单/钱包），本语句不命中，
--    留给人工判断，避免误删真实业务数据。
-- ---------------------------------------------------------------------------
DELETE FROM user_roles
 WHERE user_id IN (
   SELECT u.id FROM users u
    WHERE u.username = 'admin'
      AND u.deleted_at IS NULL
      AND EXISTS (SELECT 1 FROM admins a WHERE a.username = u.username)
      AND NOT EXISTS (SELECT 1 FROM orders o WHERE o.user_id = u.id AND o.deleted_at IS NULL)
      AND NOT EXISTS (SELECT 1 FROM bills b WHERE b.user_id = u.id)
      AND NOT EXISTS (SELECT 1 FROM wallet_accounts w WHERE w.user_id = u.id)
      AND NOT EXISTS (SELECT 1 FROM instances i WHERE i.user_id = u.id)
      AND NOT EXISTS (SELECT 1 FROM tickets t WHERE t.user_id = u.id)
 );

UPDATE users u SET
    deleted_at           = now(),
    deleted_by           = 0,
    delete_reason        = '迁移 056：分表前遗留的客户账号，与 admins 同名同邮箱；无任何业务数据，软删除以消除跨域串台',
    status_before_delete = u.status,
    status               = 'cancelled',
    updated_at           = now()
 WHERE u.username = 'admin'
   AND u.email = 'admin@hostsent.local'
   AND u.deleted_at IS NULL
   AND EXISTS (SELECT 1 FROM admins a WHERE a.username = u.username)
   AND NOT EXISTS (SELECT 1 FROM orders o WHERE o.user_id = u.id AND o.deleted_at IS NULL)
   AND NOT EXISTS (SELECT 1 FROM bills b WHERE b.user_id = u.id)
   AND NOT EXISTS (SELECT 1 FROM wallet_accounts w WHERE w.user_id = u.id)
   AND NOT EXISTS (SELECT 1 FROM instances i WHERE i.user_id = u.id)
   AND NOT EXISTS (SELECT 1 FROM tickets t WHERE t.user_id = u.id);

COMMIT;
