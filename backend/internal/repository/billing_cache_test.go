//go:build unit

package repository

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestBillingBalanceKey(t *testing.T) {
	tests := []struct {
		name     string
		userID   int64
		expected string
	}{
		{
			name:     "normal_user_id",
			userID:   123,
			expected: "billing:balance:123",
		},
		{
			name:     "zero_user_id",
			userID:   0,
			expected: "billing:balance:0",
		},
		{
			name:     "negative_user_id",
			userID:   -1,
			expected: "billing:balance:-1",
		},
		{
			name:     "max_int64",
			userID:   math.MaxInt64,
			expected: "billing:balance:9223372036854775807",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := billingBalanceKey(tc.userID)
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestBillingCacheRateLimit30dAccumulatesAndRestartsExpiredWindow(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	cache := &billingCache{rdb: redis.NewClient(&redis.Options{Addr: server.Addr()})}
	t.Cleanup(func() { require.NoError(t, cache.rdb.Close()) })

	activeStart := time.Now().Add(-29 * 24 * time.Hour).Unix()
	require.NoError(t, cache.SetAPIKeyRateLimit(ctx, 7, &service.APIKeyRateLimitCacheData{
		Usage30d:  10,
		Window30d: activeStart,
	}))
	require.NoError(t, cache.UpdateAPIKeyRateLimitUsage(ctx, 7, 2.5))
	got, err := cache.GetAPIKeyRateLimit(ctx, 7)
	require.NoError(t, err)
	require.InDelta(t, 12.5, got.Usage30d, 1e-9)
	require.Equal(t, activeStart, got.Window30d)

	expiredStart := time.Now().Add(-31 * 24 * time.Hour).Unix()
	require.NoError(t, cache.SetAPIKeyRateLimit(ctx, 7, &service.APIKeyRateLimitCacheData{
		Usage30d:  90,
		Window30d: expiredStart,
	}))
	require.NoError(t, cache.UpdateAPIKeyRateLimitUsage(ctx, 7, 3))
	got, err = cache.GetAPIKeyRateLimit(ctx, 7)
	require.NoError(t, err)
	require.InDelta(t, 3, got.Usage30d, 1e-9)
	require.Greater(t, got.Window30d, expiredStart)
}

func TestBillingSubKey(t *testing.T) {
	tests := []struct {
		name     string
		userID   int64
		groupID  int64
		expected string
	}{
		{
			name:     "normal_ids",
			userID:   123,
			groupID:  456,
			expected: "billing:sub:123:456",
		},
		{
			name:     "zero_ids",
			userID:   0,
			groupID:  0,
			expected: "billing:sub:0:0",
		},
		{
			name:     "negative_ids",
			userID:   -1,
			groupID:  -2,
			expected: "billing:sub:-1:-2",
		},
		{
			name:     "max_int64_ids",
			userID:   math.MaxInt64,
			groupID:  math.MaxInt64,
			expected: "billing:sub:9223372036854775807:9223372036854775807",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := billingSubKey(tc.userID, tc.groupID)
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestJitteredTTL(t *testing.T) {
	const (
		minTTL = 4*time.Minute + 30*time.Second // 270s = 5min - 30s
		maxTTL = 5*time.Minute + 30*time.Second // 330s = 5min + 30s
	)

	for i := 0; i < 200; i++ {
		ttl := jitteredTTL()
		require.GreaterOrEqual(t, ttl, minTTL, "jitteredTTL() 返回值低于下限: %v", ttl)
		require.LessOrEqual(t, ttl, maxTTL, "jitteredTTL() 返回值超过上限: %v", ttl)
	}
}

func TestJitteredTTL_HasVariation(t *testing.T) {
	// 多次调用应该产生不同的值（验证抖动存在）
	seen := make(map[time.Duration]struct{}, 50)
	for i := 0; i < 50; i++ {
		seen[jitteredTTL()] = struct{}{}
	}
	// 50 次调用中应该至少有 2 个不同的值
	require.Greater(t, len(seen), 1, "jitteredTTL() 应产生不同的 TTL 值")
}
