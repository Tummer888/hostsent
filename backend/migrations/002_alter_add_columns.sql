-- 002_alter_add_columns.sql
-- 对已存在的库补齐 docs/项目架构.md 第6章要求、GORM model 历史缺失的字段。
-- 幂等：ADD COLUMN IF NOT EXISTS，可重复执行。
-- 适用：users/roles 表已由 GORM AutoMigrate 创建，现补齐架构文档要求的字段。

ALTER TABLE users ADD COLUMN IF NOT EXISTS real_name     VARCHAR(64);
ALTER TABLE users ADD COLUMN IF NOT EXISTS balance       DECIMAL(15,2) NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMP;

ALTER TABLE roles ADD COLUMN IF NOT EXISTS description  VARCHAR(255);
