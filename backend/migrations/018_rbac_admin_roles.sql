-- 018_rbac_admin_roles.sql
-- 账号体系与权限分级重构 P1：后台员工多角色 + 角色作用域 + 员工字段
-- 依据：docs/实施计划/81-账号体系与权限分级重构设计.md §4.1、82-...执行清单.md P1-01
--
-- 注意（R1 schema 双写）：本文件仅作版本留痕，运行时不执行；
-- 实际建列/建表由 internal/pkg/db/db.go 的 AutoMigrate 完成。
-- 对应的 Go model：manager/model/admin.go、manager/model/admin_role.go、user/account/model/role.go

-- 员工多角色关联表（D2）
CREATE TABLE IF NOT EXISTS admin_roles (
    admin_id BIGINT NOT NULL,
    role_id  BIGINT NOT NULL,
    PRIMARY KEY (admin_id, role_id)
);

-- 角色作用域：隔离后台角色(admin)与客户角色(user)，后台权限树只展示 admin
ALTER TABLE roles ADD COLUMN IF NOT EXISTS scope VARCHAR(16) NOT NULL DEFAULT 'admin';

-- 员工补充字段：工单客服组、职位、首次登录强制改密
ALTER TABLE admins
  ADD COLUMN IF NOT EXISTS service_group_id BIGINT,
  ADD COLUMN IF NOT EXISTS position VARCHAR(64),
  ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT false;

-- 存量数据迁移：把 admins.role（单字符串）按 roles.code 灌入 admin_roles
INSERT INTO admin_roles (admin_id, role_id)
SELECT a.id, r.id FROM admins a JOIN roles r ON r.code = a.role
WHERE a.role <> ''
ON CONFLICT DO NOTHING;

-- 客户角色不进后台权限树；把历史 seed 的 user 角色标记为 scope=user
UPDATE roles SET scope = 'user' WHERE code = 'user';

-- P0-02 数据订正：uc 下单原写入 type='order'，统一归并为 consume（累计消费聚合口径）
UPDATE wallet_transactions SET type = 'consume' WHERE type = 'order';
