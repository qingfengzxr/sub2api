package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildBillableUsage_DisabledKeepsRawCompatible(t *testing.T) {
	raw := UsageTokens{
		InputTokens:         100,
		OutputTokens:        50,
		CacheCreationTokens: 7,
		CacheReadTokens:     3,
		ImageOutputTokens:   2,
	}

	got := BuildBillableUsage(raw, BillingTokenPolicy{Enabled: false, Multiplier: 2.5})

	require.Equal(t, 1.0, got.BillingTokenMultiplier)
	require.Equal(t, 100, got.InputTokens)
	require.Equal(t, 50, got.OutputTokens)
	require.Equal(t, 7, got.CacheCreationTokens)
	require.Equal(t, 3, got.CacheReadTokens)
	require.Equal(t, 2, got.ImageOutputTokens)
}

func TestBuildBillableUsage_EnabledAppliesMultiplierAndCeil(t *testing.T) {
	raw := UsageTokens{
		InputTokens:            101,
		OutputTokens:           50,
		CacheCreationTokens:    7,
		CacheReadTokens:        3,
		CacheCreation5mTokens:  2,
		CacheCreation1hTokens:  1,
		ImageOutputTokens:      2,
		TextInputTokens:        9,
		CachedTextInputTokens:  4,
		ImageInputTokens:       5,
		CachedImageInputTokens: 1,
	}

	got := BuildBillableUsage(raw, BillingTokenPolicy{Enabled: true, Multiplier: 2.5})

	require.Equal(t, 2.5, got.BillingTokenMultiplier)
	require.Equal(t, 253, got.InputTokens)
	require.Equal(t, 125, got.OutputTokens)
	require.Equal(t, 18, got.CacheCreationTokens)
	require.Equal(t, 8, got.CacheReadTokens)
	require.Equal(t, 5, got.CacheCreation5mTokens)
	require.Equal(t, 3, got.CacheCreation1hTokens)
	require.Equal(t, 5, got.ImageOutputTokens)
	require.Equal(t, 23, got.TextInputTokens)
	require.Equal(t, 10, got.CachedTextInputTokens)
	require.Equal(t, 13, got.ImageInputTokens)
	require.Equal(t, 3, got.CachedImageInputTokens)
}

func TestBuildBillableUsage_TotalCostCanBeRecomputedFromBillableTokens(t *testing.T) {
	raw := UsageTokens{InputTokens: 100, OutputTokens: 50}
	billable := BuildBillableUsage(raw, BillingTokenPolicy{Enabled: true, Multiplier: 2.5})

	cost := (&BillingService{}).computeTokenBreakdown(&ModelPricing{
		InputPricePerToken:  0.000001,
		OutputPricePerToken: 0.000002,
	}, billable.UsageTokens(), 1.2, "", LongContextPricingPolicy{})

	require.InDelta(t, 0.0005, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.0006, cost.ActualCost, 1e-12)
}

func TestBuildBillableUsage_ZeroUsageKeepsZerosAndEnabledMultiplierSnapshot(t *testing.T) {
	got := BuildBillableUsage(UsageTokens{}, BillingTokenPolicy{Enabled: true, Multiplier: 2.5})

	require.Equal(t, 2.5, got.BillingTokenMultiplier)
	require.Zero(t, got.InputTokens)
	require.Zero(t, got.OutputTokens)
	require.Zero(t, got.CacheCreationTokens)
	require.Zero(t, got.CacheReadTokens)
	require.Zero(t, got.ImageOutputTokens)
	require.Zero(t, got.TextInputTokens)
	require.Zero(t, got.CachedTextInputTokens)
	require.Zero(t, got.ImageInputTokens)
	require.Zero(t, got.CachedImageInputTokens)
}
