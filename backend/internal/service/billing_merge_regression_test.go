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
				ReasoningEffortMultipliers: map[string]float64{"max": 3},
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
				ReasoningEffortMultipliers: map[string]float64{"max": configuredMax},
			}}}, nil, nil, nil, nil)
			resolver := NewModelPricingResolver(channelService, bs)
			bs.fallbackPrices["claude-fable-5-1"] = &ModelPricing{
				InputPricePerToken: 1e-6, OutputPricePerToken: 2e-6,
			}
			svc := &OpenAIGatewayService{billingService: bs}
			maxMultiplier := 1.0
			if withResolver {
				svc.resolver = resolver
				maxMultiplier = configuredMax
			}
			billable := BuildBillableUsage(UsageTokens{InputTokens: 100, OutputTokens: 10}, BillingTokenPolicy{Enabled: true, Multiplier: 2})
			disabled := false
			cost, err := svc.calculateOpenAIRecordUsageTokenCost(context.Background(),
				&APIKey{Group: &Group{ID: 100, Platform: PlatformOpenAI}}, "claude-fable-5-1", 0.5,
				billable, time.Now(), "", "max", &disabled, false)
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

func TestQuoteUsageCost_CacheStoragePreservesTokenMultiplier(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := &GatewayService{
		billingService: NewBillingService(&config.Config{}, nil),
		usageLogRepo:   usageRepo,
		userRepo:       userRepo,
		settingService: NewSettingService(&openAIRecordUsageSettingRepoStub{values: map[string]string{
			SettingKeyTokenMultiplierBillingEnabled: "true",
			SettingKeyBillingTokenMultiplier:        "2.5",
		}}, &config.Config{}),
	}
	for _, storageOnly := range []bool{false, true} {
		result := &ForwardResult{
			RequestID: "cache-storage-merge", Model: "gemini-3.8-flash",
			CacheStorageTokenHours: 20000,
		}
		if !storageOnly {
			result.Usage = ClaudeUsage{InputTokens: 10000, OutputTokens: 100, CacheReadInputTokens: 1000}
		}
		input := &RecordUsageInput{
			Result: result,
			APIKey: &APIKey{ID: 1, GroupID: i64p(3), Group: &Group{ID: 3, RateMultiplier: 2}},
			User:   &User{ID: 2}, Account: &Account{ID: 3, Platform: PlatformGemini},
		}
		quote := svc.QuoteUsageCost(context.Background(), input)
		require.NoError(t, svc.RecordUsage(context.Background(), input))
		log := usageRepo.lastLog
		require.NotNil(t, log)
		require.InDelta(t, quote, log.ActualCost, 1e-12)
		require.InDelta(t, 2.5, log.BillingTokenMultiplier, 1e-12)
		if storageOnly {
			require.InDelta(t, 0.01*2, quote, 1e-12)
			require.Zero(t, log.BillableInputTokens)
		} else {
			require.Equal(t, 25000, log.BillableInputTokens)
			require.Equal(t, 250, log.BillableOutputTokens)
			require.Equal(t, 1000, log.BillableCacheReadTokens)
			require.Greater(t, quote, 0.01*2)
		}
	}
}

func TestOpenAIRecordUsageTokenCost_ExemptionPreservesBillableUsage(t *testing.T) {
	for _, withResolver := range []bool{false, true} {
		billing := NewBillingService(&config.Config{}, nil)
		svc := &OpenAIGatewayService{billingService: billing}
		if withResolver {
			svc.resolver = NewModelPricingResolver(nil, billing)
		}
		billable := BuildBillableUsage(UsageTokens{InputTokens: 200000, OutputTokens: 1000}, BillingTokenPolicy{Enabled: true, Multiplier: 2})
		cost, err := svc.calculateOpenAIRecordUsageTokenCost(context.Background(),
			&APIKey{Group: &Group{ID: 1, Platform: PlatformOpenAI, LongContextPricingEnabled: true}},
			"gpt-6.1-sol", 0.5, billable, time.Now(), "", "", boolPtr(true), true)
		require.NoError(t, err)
		require.False(t, cost.LongContextBillingApplied)
		require.Equal(t, billable, cost.BillableUsage)
		require.InDelta(t, 400000*2e-6+2000*10e-6, cost.TotalCost, 1e-12)
		require.InDelta(t, cost.TotalCost*0.5, cost.ActualCost, 1e-12)
	}
}
