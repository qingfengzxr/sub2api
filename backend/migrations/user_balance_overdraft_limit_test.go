package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserBalanceOverdraftLimitMigration(t *testing.T) {
	content, err := FS.ReadFile("186_user_balance_overdraft_limit.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS overdraft_limit DECIMAL(20,8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "CONSTRAINT users_overdraft_limit_nonnegative")
	require.Contains(t, sql, "CHECK (overdraft_limit >= 0)")
}

func TestUserOverdraftAuthCacheInvalidationMigration(t *testing.T) {
	content, err := FS.ReadFile("187_user_overdraft_auth_cache_invalidation.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE OR REPLACE FUNCTION enqueue_user_auth_cache_invalidation()")
	require.Contains(t, sql, "OLD.overdraft_limit IS NOT DISTINCT FROM NEW.overdraft_limit")
	require.Contains(t, sql, "INSERT INTO auth_cache_invalidation_outbox (cache_key)")
	require.Contains(t, sql, "encode(sha256(convert_to(k.key, 'UTF8')), 'hex')")
}
