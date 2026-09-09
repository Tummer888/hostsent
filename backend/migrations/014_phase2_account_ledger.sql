-- ============================================================================
-- 014_phase2_account_ledger.sql
-- Phase 2 · T2.1 账务收敛：确立 wallet_accounts 为余额唯一权威，users.balance 为派生缓存。
--
-- 现状：wallet_service 已在同一事务更新 wallet_accounts.balance 并调用 SyncUsersBalance
--      同步 users.balance（应用层同步）。本迁移在 DB 层追加等价约束（trigger）+ 对账回填，
--      形成"DB 单一真相 + 应用层幂等"的双保险，消除两处余额漂移。
-- 边界：bills 为权威账单（user_bills 已于 013 退役）；recharge/withdraw 仍走各自模块。
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1) 功能：wallet_accounts 余额变化后同步 users.balance（派生缓存，DB 强制）。
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION sync_users_balance_from_wallet() RETURNS trigger AS $$
BEGIN
    UPDATE users SET balance = NEW.balance, updated_at = now()
    WHERE id = NEW.user_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_wallet_sync_users_balance ON wallet_accounts;
CREATE TRIGGER trg_wallet_sync_users_balance
    AFTER INSERT OR UPDATE OF balance ON wallet_accounts
    FOR EACH ROW EXECUTE FUNCTION sync_users_balance_from_wallet();

-- ---------------------------------------------------------------------------
-- 2) 对账回填：以 wallet_accounts.balance 为准修正 users.balance（幂等）。
-- ---------------------------------------------------------------------------
UPDATE users u
SET balance = wa.balance, updated_at = now()
FROM wallet_accounts wa
WHERE wa.user_id = u.id AND wa.balance IS DISTINCT FROM u.balance;

-- ---------------------------------------------------------------------------
-- 3) 报告（只读，便于人工核对，不修改数据）：
--    a) 有余额但无 wallet_accounts 的用户；
--    b) users.balance 与 wallet_accounts.balance 不一致的行（回填后应为空）。
-- ---------------------------------------------------------------------------
-- 3a
SELECT u.id, u.username, u.balance
FROM users u
LEFT JOIN wallet_accounts wa ON wa.user_id = u.id
WHERE wa.id IS NULL AND u.balance <> 0;

-- 3b（对账 SQL，可留存用于持续校验）
SELECT u.id, u.username, u.balance AS users_balance, COALESCE(wa.balance, 0) AS wallet_balance
FROM users u
LEFT JOIN wallet_accounts wa ON wa.user_id = u.id
WHERE u.balance IS DISTINCT FROM COALESCE(wa.balance, 0);

COMMIT;

-- 回滚：
--   DROP TRIGGER IF EXISTS trg_wallet_sync_users_balance ON wallet_accounts;
--   DROP FUNCTION IF EXISTS sync_users_balance_from_wallet();
