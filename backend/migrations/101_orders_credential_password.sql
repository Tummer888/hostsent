-- ============================================================================
-- 101_orders_credential_password.sql
-- 订单上的实例登录密码密文（开通时随单生成，重试复用，成功后在实例行留一份）。
--
-- 背景：
--   100 给 instances 加了 credential_password 密文列，但实例行不总是随开通立即产生：
--     · 开通由异步 worker 执行，worker 手里只有订单（重读 orders）；
--     · 开通重试必须复用**同一个**密码 —— 第一次尝试可能已在上游把机器建出来了
--       （只是响应丢了），重试换新密码就会与机器上的实际密码不一致；
--     · 上游幂等续跑分支（zjmf settleExistingInvoice / 回源接管）压根不带密码，
--       此时"最终留在实例上的密码"只能是首次下发的那一个。
--   因此密码必须与 downstream_token / upstream_invoice_id 同样在订单上固化，
--   开通成功后由 buildRecordedInstance 原样（已是密文）搬进 instances 行。
--
-- 存的是 **AES-256-GCM 密文**（internal/pkg/crypto，应用 encrypt_key 派生），
-- 带 `enc:v1:` 前缀。明文绝不落库、不进日志。
--
-- 幂等：IF NOT EXISTS，重复执行无副作用。
-- 顺序：列由 GORM AutoMigrate 启动时补齐对齐（Order.CredentialPassword 已建字段）。
-- 回滚：ALTER TABLE orders DROP COLUMN IF EXISTS credential_password;
-- ============================================================================

BEGIN;

ALTER TABLE orders ADD COLUMN IF NOT EXISTS credential_password VARCHAR(512);

COMMENT ON COLUMN orders.credential_password IS
  '实例登录密码密文（enc:v1: AES-256-GCM）：开通时随单生成后固化，重试复用，成功后落 instances';

COMMIT;
