-- ============================================================================
-- 100_instances_credential_password.sql
-- 实例登录密码的加密留存列。
--
-- 背景（实测缺陷）：
--   自营/对接两条链路开通时，**密码都是我们生成的**（dcimcloud provider.go
--   `rootpass`、zjmf provider.go `password`），生成后只发给了上游，从未落库。
--   于是实例详情页的「查看密码」永远取不到值：
--     · dcimcloud 详情虽有 rootpassword 字段，但面板不回传（实测为空）；
--     · zjmf 侧上游明确不回传密码（源码 Host::crackPass 之外无读密码接口）。
--   表现为"开通成功但客户查不到初始密码"，只能靠重置。
--
-- 存的是 **AES-256-GCM 密文**（internal/pkg/crypto，与应用同一 encrypt_key 派生），
-- 带 `enc:v1:` 前缀（pkg/credentials.EncPrefix，与渠道凭证同一套约定）。
-- 明文绝不落库、不进日志；读取只走「查看密码」接口（有 instance:credential 权限
-- 且写审计流水）。
--
-- 为什么不复用 raw_data：raw_data 是上游详情的原样快照，会被每次同步整体覆盖
-- （UpsertInstances 的 raw_data=excluded.raw_data），密码放进去会被下一次同步抹掉。
--
-- 回滚：ALTER TABLE instances DROP COLUMN IF EXISTS credential_password;
-- ============================================================================

BEGIN;

ALTER TABLE instances ADD COLUMN IF NOT EXISTS credential_password VARCHAR(512);

COMMENT ON COLUMN instances.credential_password IS
  '实例登录密码密文（enc:v1: AES-256-GCM）。开通时由本系统生成后留存，仅供有权限的运营查看';

COMMIT;
