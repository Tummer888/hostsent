-- 001_init_schema.sql
-- 宿派云控 HostSent 初始 schema（已实现模块：用户/角色/权限/菜单）
-- 对照 docs/项目架构.md 第6章。本文件为已实现6张表的权威 DDL。
-- 幂等：CREATE TABLE IF NOT EXISTS，新库可直接执行初始化。
--
-- 与架构文档的差异说明（已在实现中固化，保持不变）：
--   1. users.status / roles.status / permissions.status / menus.status
--      架构为 SMALLINT，实现为 VARCHAR('active'/'disabled')，可读性更好且已被业务代码使用。
--   2. permissions.type 架构为 SMALLINT，实现为 VARCHAR(directory/menu/button)。
--   3. users.role 架构为单字段；实现改用 user_roles 多对多关联表，支持一用户多角色。
--   4. menus 表架构文档第6章未列，为实现菜单树（管理员后台+用户中心共用，platform 区分）。

CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL    PRIMARY KEY,
    username      VARCHAR(64)  NOT NULL UNIQUE,
    email         VARCHAR(128) NOT NULL UNIQUE,
    phone         VARCHAR(32),
    password_hash VARCHAR(255) NOT NULL,
    real_name     VARCHAR(64),
    status        VARCHAR(32)  NOT NULL DEFAULT 'active',
    balance       DECIMAL(15,2) NOT NULL DEFAULT 0,
    last_login_at TIMESTAMP,
    created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS roles (
    id          BIGSERIAL    PRIMARY KEY,
    name        VARCHAR(64)  NOT NULL UNIQUE,
    code        VARCHAR(64)  NOT NULL UNIQUE,
    description VARCHAR(255),
    status      VARCHAR(32)  NOT NULL DEFAULT 'active',
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS permissions (
    id         BIGSERIAL    PRIMARY KEY,
    parent_id  BIGINT       NOT NULL DEFAULT 0,
    name       VARCHAR(64)  NOT NULL,
    code       VARCHAR(128) NOT NULL UNIQUE,
    type       VARCHAR(32)  NOT NULL,
    path       VARCHAR(255),
    component  VARCHAR(255),
    icon       VARCHAR(128),
    sort_order INT          NOT NULL DEFAULT 0,
    status     VARCHAR(32)  NOT NULL DEFAULT 'active',
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id       BIGINT NOT NULL,
    permission_id BIGINT NOT NULL,
    PRIMARY KEY (role_id, permission_id)
);

-- 架构文档原用 users.role 单字段；实现改为多对多关联表。
CREATE TABLE IF NOT EXISTS user_roles (
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    PRIMARY KEY (user_id, role_id)
);

-- 架构文档第6章未列；实现菜单树。
CREATE TABLE IF NOT EXISTS menus (
    id         BIGSERIAL    PRIMARY KEY,
    parent_id BIGINT       NOT NULL DEFAULT 0,
    platform  VARCHAR(32)  NOT NULL,
    name      VARCHAR(64)  NOT NULL,
    type      VARCHAR(32)  NOT NULL DEFAULT 'menu',
    path      VARCHAR(255),
    component VARCHAR(255),
    icon      VARCHAR(128),
    sort_order INT         NOT NULL DEFAULT 0,
    status    VARCHAR(32)  NOT NULL DEFAULT 'active',
    created_at TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_menus_platform ON menus (platform);
