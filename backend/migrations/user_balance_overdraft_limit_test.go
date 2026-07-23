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
