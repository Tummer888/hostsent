ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_ip VARCHAR(64);
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_ip_region VARCHAR(128);

UPDATE users
SET last_login_ip_region = region
WHERE (last_login_ip_region IS NULL OR last_login_ip_region = '')
  AND region IS NOT NULL
  AND region <> '';

CREATE INDEX IF NOT EXISTS idx_users_last_login_ip ON users (last_login_ip);
CREATE INDEX IF NOT EXISTS idx_users_last_login_ip_region ON users (last_login_ip_region);
