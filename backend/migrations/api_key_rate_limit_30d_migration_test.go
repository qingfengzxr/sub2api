package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyRateLimit30dMigration(t *testing.T) {
	content, err := FS.ReadFile("232_add_api_key_rate_limit_30d.sql")
	require.NoError(t, err)
	sql := string(content)
	require.Contains(t, sql, "rate_limit_30d DECIMAL(20,8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "usage_30d DECIMAL(20,8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "window_30d_start TIMESTAMPTZ")
}
