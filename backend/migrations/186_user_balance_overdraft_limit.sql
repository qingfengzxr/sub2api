ALTER TABLE users
    ADD COLUMN IF NOT EXISTS overdraft_limit DECIMAL(20,8) NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'users_overdraft_limit_nonnegative'
          AND conrelid = 'users'::regclass
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_overdraft_limit_nonnegative
            CHECK (overdraft_limit >= 0);
    END IF;
END
$$;
