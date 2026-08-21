ALTER TABLE users ADD COLUMN IF NOT EXISTS user_group_id BIGINT;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'fk_users_user_group_id'
  ) THEN
    ALTER TABLE users
      ADD CONSTRAINT fk_users_user_group_id
      FOREIGN KEY (user_group_id) REFERENCES user_groups(id)
      ON DELETE SET NULL;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_users_user_group_id ON users(user_group_id);
