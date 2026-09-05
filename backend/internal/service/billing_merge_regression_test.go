//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// Custom long-context thresholds and max-effort pricing must compose in both
// the catalog fallback and resolver paths after the merge.
func TestCalculateCostUnified_CustomLongContextWithMaxEffort(t *testing.T) {
	for _, withResolver := range []bool{false, true} {
		name := "fallback"
		if withResolver {
			name = "resolver"
		}
		t.Run(name, func(t *testing.T) {
			bs := NewBillingService(&config.Config{}, nil)
			bs.fallbackPrices["claude-fable-5-1"] = &ModelPricing{
				InputPricePerToken: 1e-6, OutputPricePerToken: 2e-6,
				CacheReadPricePerToken:     0.1e-6,
				LongContextInputThreshold:  1000,
				LongContextInputMultiplier: 2, LongContextOutputMultiplier: 1.5,
			}
			input := CostInput{
				Model:          "claude-fable-5-1",
				Tokens:         UsageTokens{InputTokens: 150, OutputTokens: 10, CacheReadTokens: 20},
				RateMultiplier: 0.5, ReasoningEffort: "max",
				LongContextPricing: LongContextPricingPolicy{Enabled: true, ThresholdTokens: 100},
			}
			if withResolver {
				input.Resolver = &ModelPricingResolver{billingService: bs}
				input.Resolved = &ResolvedPricing{
					Mode:                      BillingModeToken,
					BasePricing:               bs.fallbackPrices[input.Model],
					longContextPricingEnabled: true,
				}
			}
			cost, err := bs.CalculateCostUnified(input)
			require.NoError(t, err)
			require.True(t, cost.LongContextBillingApplied)
			require.InDelta(t, 150*1e-6*2*3, cost.InputCost, 1e-12)
			require.InDelta(t, 10*2e-6*1.5*3, cost.OutputCost, 1e-12)
			require.InDelta(t, 20*0.1e-6*2*3, cost.CacheReadCost, 1e-12)
			require.InDelta(t, cost.TotalCost*0.5, cost.ActualCost, 1e-12)
		})
	}
}

func TestCalculateCostUnified_ExplicitLongContextDisableWithoutResolver(t *testing.T) {
	bs := NewBillingService(&config.Config{}, nil)
	bs.fallbackPrices["gpt-5.4"].LongContextInputThreshold = 272000
	bs.fallbackPrices["gpt-5.4"].LongContextInputMultiplier = 2
	baseline, err := bs.CalculateCostUnified(CostInput{
		Model: "gpt-5.4", Tokens: UsageTokens{InputTokens: 300000}, RateMultiplier: 1,
	})
	require.NoError(t, err)
	require.True(t, baseline.LongContextBillingApplied)
	require.InDelta(t, 300000*2.5e-6*2, baseline.InputCost, 1e-12)
	disabled := false
	cost, err := bs.CalculateCostUnified(CostInput{
		Model: "gpt-5.4", Tokens: UsageTokens{InputTokens: 300000},
		RateMultiplier: 1, LongContextBillingEnabled: &disabled,
	})
	require.NoError(t, err)
	require.False(t, cost.LongContextBillingApplied)
	require.InDelta(t, 300000*2.5e-6, cost.InputCost, 1e-12)
}

func TestOpenAIRecordUsageTokenCost_PreservesBillableUsageWithMaxEffort(t *testing.T) {
	for _, withResolver := range []bool{false, true} {
		name := "fallback"
		if withResolver {
			name = "channel"
		}
		t.Run(name, func(t *testing.T) {
			configuredMax := 4.0
			inputPrice, outputPrice := 1e-6, 2e-6
			bs := NewBillingService(&config.Config{}, nil)
			channelService := NewChannelService(&mergeBillingChannelRepository{pricing: []ChannelModelPricing{{
				Platform: PlatformOpenAI, Models: []string{"claude-fable-5-1"}, BillingMode: BillingModeToken,
				InputPrice: &inputPrice, OutputPrice: &outputPrice,
				MaxReasoningEffortMultiplier: &configuredMax,
			}}}, nil, nil, nil)
			resolver := NewModelPricingResolver(channelService, bs)
			bs.fallbackPrices["claude-fable-5-1"] = &ModelPricing{
				InputPricePerToken: 1e-6, OutputPricePerToken: 2e-6,
			}
			svc := &OpenAIGatewayService{billingService: bs}
			maxMultiplier := 3.0
			if withResolver {
				svc.resolver = resolver
				maxMultiplier = configuredMax
			}
			billable := BuildBillableUsage(UsageTokens{InputTokens: 100, OutputTokens: 10}, BillingTokenPolicy{Enabled: true, Multiplier: 2})
			disabled := false
			cost, err := svc.calculateOpenAIRecordUsageTokenCost(context.Background(),
				&APIKey{Group: &Group{ID: 100, Platform: PlatformOpenAI}}, "claude-fable-5-1", 0.5,
				billable, time.Now(), "", "max", &disabled)
			require.NoError(t, err)
			require.Equal(t, billable, cost.BillableUsage)
			require.InDelta(t, 200*1e-6*maxMultiplier, cost.InputCost, 1e-12)
			require.InDelta(t, 20*2e-6*maxMultiplier, cost.OutputCost, 1e-12)
			require.InDelta(t, cost.TotalCost*0.5, cost.ActualCost, 1e-12)
		})
	}
}

type mergeBillingChannelRepository struct {
	ChannelRepository
	pricing []ChannelModelPricing
}

func (r *mergeBillingChannelRepository) ListAll(context.Context) ([]Channel, error) {
	return []Channel{{ID: 1, Name: "merge-test", Status: StatusActive, GroupIDs: []int64{100}, ModelPricing: r.pricing}}, nil
}

func (r *mergeBillingChannelRepository) GetGroupPlatforms(context.Context, []int64) (map[int64]string, error) {
	return map[int64]string{100: PlatformOpenAI}, nil
}
