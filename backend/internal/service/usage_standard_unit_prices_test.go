//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageStandardUnitPriceResolver_UsesBaseTokenPrices(t *testing.T) {
	t.Parallel()

	billing := newTestBillingServiceForResolver()
	resolver := NewUsageStandardUnitPriceResolver(NewModelPricingResolver(&ChannelService{}, billing), billing)
	log := &UsageLog{
		Model:                  "claude-sonnet-4",
		InputTokens:            1_000,
		OutputTokens:           1_000,
		CacheReadTokens:        1_000,
		InputCost:              0.01,
		OutputCost:             0.05,
		CacheReadCost:          0.01,
		RateMultiplier:         3,
		ServiceTier:            stringPtrForStandardUnitPriceTest("priority"),
		BillingMode:            stringPtrForStandardUnitPriceTest(string(BillingModeToken)),
		BillingTokenMultiplier: 2.5,
	}

	prices := resolver.Resolve(context.Background(), log)

	require.InDelta(t, 3, *prices.InputPricePerMillion, 1e-12)
	require.InDelta(t, 15, *prices.OutputPricePerMillion, 1e-12)
	require.InDelta(t, 0.3, *prices.CacheReadPricePerMillion, 1e-12)
	require.InDelta(t, 3.75, *prices.CacheCreationPricePerMillion, 1e-12)
}

func TestUsageStandardUnitPriceResolver_ReturnsEmptyForUnknownTokenModel(t *testing.T) {
	t.Parallel()

	billing := newTestBillingServiceForResolver()
	resolver := NewUsageStandardUnitPriceResolver(NewModelPricingResolver(&ChannelService{}, billing), billing)
	log := &UsageLog{
		Model:          "unknown-model-xyz",
		InputTokens:    1_000,
		BillingMode:    stringPtrForStandardUnitPriceTest(string(BillingModeToken)),
		InputCost:      0.5,
		RateMultiplier: 5,
	}

	prices := resolver.Resolve(context.Background(), log)

	require.Nil(t, prices.InputPricePerMillion)
	require.Nil(t, prices.OutputPricePerMillion)
	require.Nil(t, prices.UnitPrice)
}

func stringPtrForStandardUnitPriceTest(value string) *string {
	return &value
}
