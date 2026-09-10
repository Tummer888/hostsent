-- ============================================================================
-- 028_user_groups_default_and_drop_priority.sql
-- 用户组收尾：删掉无处使用的 priority 列，把 is_default（默认用户组）真正
-- 兑现为「全库至多一个」的约束，并清理悬空的 users.user_group_id 引用。
--
-- 背景（2026-09-10 审计结论）：
--   · user_groups.priority 注释写「组间优先级，兜底用」，但全仓无任何读取方；
--     且 users.user_group_id 是单值外键，一个用户只属于一个组，
--     在单组归属模型下「组间优先级」没有意义 → 删除。
--   · is_default 前端文案承诺「新用户未指定分组时归入」，但后端从未读过它，
--     注册链路也不写 user_group_id，导致 16/17 个用户散在 NULL。
--     本迁移加唯一约束，代码侧（用户组服务）负责置默认时清掉其他组。
--   · users.user_group_id 存在指向不存在组的悬空值（如 group 5），
--     LEFT JOIN 会把它渲染成「未分组」，把脏数据藏起来 → 统一置 NULL。
--
-- 幂等（R4）：DROP COLUMN IF EXISTS / DELETE 天然幂等；
--   唯一索引用 IF NOT EXISTS + 先修数据（多条默认只保留 sort_order 最小的一条）。
-- 时序：先部署新后端（模型已移除 Priority），再执行本迁移；否则旧后端 Save 会带上
--   已删除的列而报错。AutoMigrate 不会删列，所以必须由本迁移落 DROP。
-- ============================================================================

BEGIN;

-- 1. 删除无处使用的组间优先级
ALTER TABLE user_groups DROP COLUMN IF EXISTS priority;

-- 2. 默认组唯一化：若历史数据存在多条 is_default，只保留 sort_order/id 最小的一条
UPDATE user_groups
SET is_default = false
WHERE is_default = true
  AND id <> (
    SELECT id FROM user_groups
    WHERE is_default = true
    ORDER BY sort_order ASC, id ASC
    LIMIT 1
  );

-- 3. 用部分唯一索引把「默认组唯一」变成数据库级约束（is_default=false 的行不受约束）
CREATE UNIQUE INDEX IF NOT EXISTS uk_user_groups_single_default
  ON user_groups ((is_default))
  WHERE is_default = true;

-- 4. 清理悬空引用：指向不存在用户组的 users.user_group_id 置回 NULL
UPDATE users u
SET user_group_id = NULL
WHERE u.user_group_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM user_groups g WHERE g.id = u.user_group_id);

COMMIT;
