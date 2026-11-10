package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestDashboardTPMUsesUserBillableTokensAndAdminRawTokens(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE usage_logs (created_at DATETIME, user_id INTEGER, api_key_id INTEGER,
		input_tokens INTEGER, output_tokens INTEGER, cache_creation_tokens INTEGER, cache_read_tokens INTEGER,
		billable_input_tokens INTEGER, billable_output_tokens INTEGER, billable_cache_creation_tokens INTEGER, billable_cache_read_tokens INTEGER)`)
	require.NoError(t, err)
	now := time.Now()
	_, err = db.Exec(`INSERT INTO usage_logs VALUES
		(?, 1, 2, 100, 20, 10, 5, 250, 50, 10, 5),
		(?, 1, 2, 20, 10, 0, 0, 0, 0, 0, 0),
		(?, 9, 9, 50, 0, 0, 0, 100, 0, 0, 0)`, now, now, now)
	require.NoError(t, err)
	repo := &usageLogRepository{sql: db}

	_, adminTPM, err := repo.getPerformanceStats(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, int64(40), adminTPM)
	_, userTPM, err := repo.getPerformanceStats(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, int64(66), userTPM)
	_, keyTPM, err := repo.getPerformanceStatsByAPIKey(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, int64(69), keyTPM)
}
