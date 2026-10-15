package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type billingCacheWorkerStub struct {
	balanceUpdates      int64
	subscriptionUpdates int64
}

func (b *billingCacheWorkerStub) GetUserBalance(ctx context.Context, userID int64) (float64, error) {
	return 0, errors.New("not implemented")
}

func (b *billingCacheWorkerStub) SetUserBalance(ctx context.Context, userID int64, balance float64) error {
	atomic.AddInt64(&b.balanceUpdates, 1)
	return nil
}

func (b *billingCacheWorkerStub) DeductUserBalance(ctx context.Context, userID int64, amount float64) error {
	atomic.AddInt64(&b.balanceUpdates, 1)
	return nil
}

func (b *billingCacheWorkerStub) InvalidateUserBalance(ctx context.Context, userID int64) error {
	return nil
}

func (b *billingCacheWorkerStub) GetSubscriptionCache(ctx context.Context, userID, groupID int64) (*SubscriptionCacheData, error) {
	return nil, errors.New("not implemented")
}

func (b *billingCacheWorkerStub) SetSubscriptionCache(ctx context.Context, userID, groupID int64, data *SubscriptionCacheData) error {
	atomic.AddInt64(&b.subscriptionUpdates, 1)
	return nil
}

func (b *billingCacheWorkerStub) UpdateSubscriptionUsage(ctx context.Context, userID, groupID int64, cost float64) error {
	atomic.AddInt64(&b.subscriptionUpdates, 1)
	return nil
}

func (b *billingCacheWorkerStub) InvalidateSubscriptionCache(ctx context.Context, userID, groupID int64) error {
	return nil
}

func (b *billingCacheWorkerStub) GetAPIKeyRateLimit(ctx context.Context, keyID int64) (*APIKeyRateLimitCacheData, error) {
	return nil, errors.New("not implemented")
}

func (b *billingCacheWorkerStub) SetAPIKeyRateLimit(ctx context.Context, keyID int64, data *APIKeyRateLimitCacheData) error {
	return nil
}

func (b *billingCacheWorkerStub) UpdateAPIKeyRateLimitUsage(ctx context.Context, keyID int64, cost float64) error {
	return nil
}

func (b *billingCacheWorkerStub) InvalidateAPIKeyRateLimit(ctx context.Context, keyID int64) error {
	return nil
}

func (b *billingCacheWorkerStub) GetUserPlatformQuotaCache(ctx context.Context, userID int64, platform string) (*UserPlatformQuotaCacheEntry, bool, error) {
	return nil, false, nil
}

func (b *billingCacheWorkerStub) SetUserPlatformQuotaCache(ctx context.Context, userID int64, platform string, entry *UserPlatformQuotaCacheEntry, ttl time.Duration) error {
	return nil
}

func (b *billingCacheWorkerStub) DeleteUserPlatformQuotaCache(ctx context.Context, userID int64, platform string) error {
	return nil
}

func (b *billingCacheWorkerStub) IncrUserPlatformQuotaUsageCache(ctx context.Context, userID int64, platform string, cost float64, ttl time.Duration, markDirty bool) error {
	return nil
}

func (b *billingCacheWorkerStub) PopDirtyUserPlatformQuotaKeys(ctx context.Context, n int) ([]UserPlatformQuotaKey, error) {
	return nil, nil
}

func (b *billingCacheWorkerStub) ReaddDirtyUserPlatformQuotaKeys(ctx context.Context, keys []UserPlatformQuotaKey) error {
	return nil
}

func (b *billingCacheWorkerStub) BatchGetUserPlatformQuotaCache(ctx context.Context, keys []UserPlatformQuotaKey) ([]*UserPlatformQuotaCacheEntry, error) {
	return nil, nil
}

func TestBillingCacheServiceQueueHighLoad(t *testing.T) {
	cache := &billingCacheWorkerStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	start := time.Now()
	for i := 0; i < cacheWriteBufferSize*2; i++ {
		svc.QueueDeductBalance(1, 1)
	}
	require.Less(t, time.Since(start), 2*time.Second)

	svc.QueueUpdateSubscriptionUsage(1, 2, 1.5)

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&cache.balanceUpdates) > 0
	}, 2*time.Second, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&cache.subscriptionUpdates) > 0
	}, 2*time.Second, 10*time.Millisecond)
}

func TestBillingCacheServiceEnqueueAfterStopReturnsFalse(t *testing.T) {
	cache := &billingCacheWorkerStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	svc.Stop()

	enqueued := svc.enqueueCacheWrite(cacheWriteTask{
		kind:   cacheWriteDeductBalance,
		userID: 1,
		amount: 1,
	})
	require.False(t, enqueued)
}

func TestBillingCacheServiceEvaluateRateLimits30d(t *testing.T) {
	svc := &BillingCacheService{}
	activeStart := time.Now().Add(-29 * 24 * time.Hour)
	key := &APIKey{ID: 1, RateLimit30d: 100}

	err := svc.evaluateRateLimits(context.Background(), key, 0, 0, 0, 100, nil, nil, nil, &activeStart, 0)
	require.ErrorIs(t, err, ErrAPIKeyRateLimit30dExceeded)

	expiredStart := time.Now().Add(-RateLimitWindow30d)
	err = svc.evaluateRateLimits(context.Background(), key, 0, 0, 0, 100, nil, nil, nil, &expiredStart, 0)
	require.NoError(t, err)
}

func TestBillingCacheServiceEvaluateRateLimits30dWithPending(t *testing.T) {
	svc := &BillingCacheService{}
	active := time.Now()
	expired := active.Add(-RateLimitWindow30d)
	key := &APIKey{ID: 1, RateLimit30d: 100}
	for _, tc := range []struct {
		name           string
		usage, pending float64
		start          *time.Time
		wantErr        bool
	}{
		{"below", 90, 9, &active, false},
		{"exact", 90, 10, &active, false},
		{"pending exceeds", 90, 11, &active, true},
		{"already exhausted", 100, 0, &active, true},
		{"expired usage", 100, 1, &expired, false},
		{"pending alone exceeds", 100, 101, &expired, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.evaluateRateLimits(context.Background(), key, 0, 0, 0, tc.usage, &active, &active, &active, tc.start, tc.pending)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrAPIKeyRateLimit30dExceeded)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestFixedChargeAPIKey30dUsesDatabaseUsage(t *testing.T) {
	start := time.Now()
	data := &APIKeyRateLimitData{QuotaUsed: 50, Usage30d: 90, Window30dStart: &start}
	svc := &BillingCacheService{apiKeyRateLimitLoader: &simpleModeRateLimitLoaderStub{data: data}}
	key := &APIKey{ID: 1, Quota: 200, RateLimit30d: 100}
	require.NoError(t, svc.checkFixedChargeAPIKey(context.Background(), key, 10))
	require.ErrorIs(t, svc.checkFixedChargeAPIKey(context.Background(), key, 11), ErrAPIKeyRateLimit30dExceeded)
	start = start.Add(-RateLimitWindow30d)
	require.NoError(t, svc.checkFixedChargeAPIKey(context.Background(), key, 11))
	require.ErrorIs(t, svc.checkFixedChargeAPIKey(context.Background(), key, 101), ErrAPIKeyRateLimit30dExceeded)
	require.ErrorIs(t, svc.checkFixedChargeAPIKey(context.Background(), key, 151), ErrAPIKeyQuotaExhausted)
}
