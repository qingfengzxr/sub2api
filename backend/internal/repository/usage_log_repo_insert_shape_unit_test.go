//go:build unit

package repository

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

var (
	usageLogStaticInsertShapeRe = regexp.MustCompile(`(?s)INSERT INTO usage_logs \((.*?)\) VALUES \((.*?)\)`)
	usageLogPlaceholderRe       = regexp.MustCompile(`\$(\d+)`)
)

// newSQLCapturingMock 返回把实际下发 SQL 记录到 captured 的 sqlmock；语句一律视为匹配，
// 参数仍由 WithArgs 校验。
func newSQLCapturingMock(t *testing.T, captured *[]string) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	matcher := sqlmock.QueryMatcherFunc(func(_, actualSQL string) error {
		*captured = append(*captured, actualSQL)
		return nil
	})
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

// requireStaticInsertMatchesArgTypes 断言手写的 INSERT：列清单长度与 VALUES 占位符数量
// 都等于 usageLogInsertArgTypes，且占位符恰为 $1..$N 各出现一次。
func requireStaticInsertMatchesArgTypes(t *testing.T, query string) {
	t.Helper()
	m := usageLogStaticInsertShapeRe.FindStringSubmatch(query)
	require.Len(t, m, 3, "unrecognised INSERT shape:\n%s", query)

	want := len(usageLogInsertArgTypes)
	columns := 0
	var columnNames []string
	for _, col := range strings.Split(m[1], ",") {
		if strings.TrimSpace(col) != "" {
			columns++
			columnNames = append(columnNames, strings.TrimSpace(col))
		}
	}
	require.Equal(t, want, columns, "INSERT column list must match usageLogInsertArgTypes")
	require.Equal(t, strings.Split(usageLogSelectColumns, ", ")[1:], columnNames,
		"INSERT and SELECT must use the same field order, excluding the generated id")

	seen := make(map[int]struct{}, want)
	for _, ph := range usageLogPlaceholderRe.FindAllStringSubmatch(m[2], -1) {
		n, err := strconv.Atoi(ph[1])
		require.NoError(t, err)
		_, dup := seen[n]
		require.False(t, dup, "duplicate placeholder $%d", n)
		seen[n] = struct{}{}
	}
	require.Len(t, seen, want, "VALUES placeholder count must match usageLogInsertArgTypes")
	for i := 1; i <= want; i++ {
		_, ok := seen[i]
		require.True(t, ok, "missing placeholder $%d", i)
	}
}

// TestUsageLogStaticInsertShape_PlaceholdersMatchArgTypes 覆盖两条不经占位符生成器、
// 直接手写 $1..$N 的 INSERT 路径，防止加列后漏补占位符只在集成测试才暴露。
func TestUsageLogStaticInsertShape_PlaceholdersMatchArgTypes(t *testing.T) {
	upstreamRequestID := "20260902080329-oneapi"
	log := &service.UsageLog{
		UserID:            1,
		APIKeyID:          2,
		AccountID:         3,
		RequestID:         "client:insert-shape",
		UpstreamRequestID: &upstreamRequestID,
		Model:             "claude-3",
		InputTokens:       10,
		CreatedAt:         time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC),
	}
	prepared := prepareUsageLogInsert(log)
	args := anySliceToDriverValues(prepared.args)

	t.Run("createSingle", func(t *testing.T) {
		var captured []string
		db, mock := newSQLCapturingMock(t, &captured)
		repo := &usageLogRepository{sql: db}

		mock.ExpectQuery("INSERT INTO usage_logs").
			WithArgs(args...).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(1), log.CreatedAt))

		inserted, err := repo.Create(context.Background(), log)
		require.NoError(t, err)
		require.True(t, inserted)
		require.NoError(t, mock.ExpectationsWereMet())
		require.Len(t, captured, 1)
		requireStaticInsertMatchesArgTypes(t, captured[0])
	})

	t.Run("execUsageLogInsertNoResult", func(t *testing.T) {
		var captured []string
		db, mock := newSQLCapturingMock(t, &captured)

		mock.ExpectExec("INSERT INTO usage_logs").
			WithArgs(args...).
			WillReturnResult(sqlmock.NewResult(0, 1))

		require.NoError(t, execUsageLogInsertNoResult(context.Background(), db, prepared))
		require.NoError(t, mock.ExpectationsWereMet())
		require.Len(t, captured, 1)
		requireStaticInsertMatchesArgTypes(t, captured[0])
	})
}

// TestPrepareUsageLogInsert_UpstreamRequestIDArgWiring 把 upstream_request_id 钉在
// session_id 之前，与参数类型表保持同位；缺失时落 NULL 而不是空串。
func TestPrepareUsageLogInsert_UpstreamRequestIDArgWiring(t *testing.T) {
	upstreamRequestID := "req_upstream_123"
	prepared := prepareUsageLogInsert(&service.UsageLog{
		UserID:            1,
		APIKeyID:          2,
		RequestID:         "client:wiring",
		Model:             "gpt-5",
		UpstreamRequestID: &upstreamRequestID,
		CreatedAt:         time.Now().UTC(),
	})
	require.Len(t, prepared.args, len(usageLogInsertArgTypes))

	idx := len(prepared.args) - 4
	arg, ok := prepared.args[idx].(sql.NullString)
	require.True(t, ok, "upstream_request_id arg should be sql.NullString, got %T", prepared.args[idx])
	require.True(t, arg.Valid)
	require.Equal(t, upstreamRequestID, arg.String)
	require.Equal(t, "text", usageLogInsertArgTypes[idx])

	absent := prepareUsageLogInsert(&service.UsageLog{UserID: 1, APIKeyID: 2, RequestID: "client:absent", Model: "gpt-5", CreatedAt: time.Now().UTC()})
	nullArg, ok := absent.args[idx].(sql.NullString)
	require.True(t, ok)
	require.False(t, nullArg.Valid, "absent upstream request id must be NULL")

	require.Contains(t, usageLogSelectColumns, "upstream_request_id")
}

func TestUsageLogInsertAndScan_PreserveBillableUsageAndUpstreamRequestID(t *testing.T) {
	upstreamID := "upstream:merge-roundtrip"
	sessionID := "session:merge-roundtrip"
	source := &service.UsageLog{
		UserID: 1, APIKeyID: 2, AccountID: 3, RequestID: "client:merge-roundtrip", Model: "gpt-5",
		InputTokens: 10, OutputTokens: 5, CacheReadTokens: 3,
		BillableInputTokens: 20, BillableOutputTokens: 10, BillableCacheReadTokens: 6,
		BillingTokenMultiplier: 2,
		UpstreamRequestID:      &upstreamID, SessionID: &sessionID, NativeCompactionV2: true,
		CreatedAt: time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC),
	}
	prepared := prepareUsageLogInsert(source)
	values := append([]any{int64(42)}, prepared.args...)
	db, mock := newSQLMock(t)
	mock.ExpectQuery("SELECT .* FROM usage_logs WHERE id = \\$1").WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows(strings.Split(usageLogSelectColumns, ", ")).AddRow(anySliceToDriverValues(values)...))
	got, err := scanUsageLog(db.QueryRow("SELECT "+usageLogSelectColumns+" FROM usage_logs WHERE id = $1", int64(42)))
	require.NoError(t, err)
	require.Equal(t, source.InputTokens, got.InputTokens)
	require.Equal(t, source.OutputTokens, got.OutputTokens)
	require.Equal(t, source.BillableInputTokens, got.BillableInputTokens)
	require.Equal(t, source.BillableOutputTokens, got.BillableOutputTokens)
	require.Equal(t, source.BillableCacheReadTokens, got.BillableCacheReadTokens)
	require.Equal(t, source.BillingTokenMultiplier, got.BillingTokenMultiplier)
	require.Equal(t, source.UpstreamRequestID, got.UpstreamRequestID)
	require.Equal(t, source.SessionID, got.SessionID)
	require.True(t, got.NativeCompactionV2)
	require.Equal(t, source.CreatedAt, got.CreatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}
