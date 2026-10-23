package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIEmbeddingsBillingSnapshotAndInputOnly(t *testing.T) {
	for model, price := range map[string]float64{"text-embedding-3-small": .02e-6, "text-embedding-3-large": .13e-6, "text-embedding-ada-002": .10e-6} {
		t.Run(model, func(t *testing.T) {
			billing := newTestBillingService()
			priceCard, err := billing.GetModelPricing(model)
			require.NoError(t, err)
			require.Equal(t, price, priceCard.InputPricePerToken)
			require.Zero(t, priceCard.OutputPricePerToken)
			account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-alias": model}}}
			svc := &OpenAIGatewayService{billingService: billing}
			plan, err := svc.PrepareOpenAIEmbeddingsBilling(context.Background(), account, []byte(`{"model":"gpt-alias","input":"hi"}`), nil, ChannelUsageFields{OriginalModel: "gpt-alias", BillingModelSource: BillingModelSourceRequested})
			require.NoError(t, err)
			require.Equal(t, model, plan.model, "unpriced alias must not acquire a GPT family fallback")
			billing.fallbackPrices[model] = &ModelPricing{InputPricePerToken: 100}
			usage := BuildBillableUsage(UsageTokens{InputTokens: 290000}, BillingTokenPolicy{Multiplier: 1})
			cost, err := svc.calculateOpenAIEmbeddingsCost(context.Background(), plan, &OpenAIForwardResult{}, usage, 2, time.Now())
			require.NoError(t, err)
			require.InDelta(t, 290000*price, cost.TotalCost, 1e-12)
			require.InDelta(t, 290000*price*2, cost.ActualCost, 1e-12)
		})
	}
}

func TestOpenAIEmbeddingsBillingExplicitPricingAndMissingUsage(t *testing.T) {
	for _, mode := range []BillingMode{BillingModeToken, BillingModePerRequest} {
		t.Run(string(mode), func(t *testing.T) {
			billing := newTestBillingService()
			price := 0.5
			groupID := int64(1)
			key := &APIKey{GroupID: &groupID, Group: &Group{ID: 1, LongContextPricingEnabled: true, ModelPricing: []ChannelModelPricing{{Models: []string{"alias"}, BillingMode: mode, InputPrice: &price, PerRequestPrice: &price}}}}
			svc := &OpenAIGatewayService{billingService: billing, resolver: NewModelPricingResolver(nil, billing)}
			account := &Account{Credentials: map[string]any{"model_mapping": map[string]any{"alias": "text-embedding-3-small"}}}
			plan, err := svc.PrepareOpenAIEmbeddingsBilling(context.Background(), account, []byte(`{"model":"alias","input":"hi"}`), key, ChannelUsageFields{OriginalModel: "alias", BillingModelSource: BillingModelSourceRequested})
			require.NoError(t, err)
			require.Equal(t, "alias", plan.model)
			usage := BuildBillableUsage(UsageTokens{}, BillingTokenPolicy{Multiplier: 1})
			cost, err := svc.calculateOpenAIEmbeddingsCost(context.Background(), plan, &OpenAIForwardResult{}, usage, 1, time.Now())
			require.NoError(t, err)
			if mode == BillingModePerRequest {
				require.Equal(t, .5, cost.TotalCost)
			} else {
				require.Zero(t, cost.TotalCost)
			}
			require.True(t, key.Group.LongContextPricingEnabled)
		})
	}
}

func TestOpenAIEmbeddingsBillingMissingPriceAndCompatibility(t *testing.T) {
	svc := &OpenAIGatewayService{billingService: &BillingService{fallbackPrices: map[string]*ModelPricing{}}}
	_, err := svc.PrepareOpenAIEmbeddingsBilling(context.Background(), nil, []byte(`{"model":"text-embedding-3-small","input":"hi"}`), nil, ChannelUsageFields{})
	require.ErrorIs(t, err, ErrModelPricingUnavailable)
	plan, err := svc.PrepareOpenAIEmbeddingsBilling(context.Background(), nil, []byte(`{"model":"jina-custom","input":"hi"}`), nil, ChannelUsageFields{})
	require.NoError(t, err)
	require.Nil(t, plan)
	outputOnly := .01
	key := &APIKey{Group: &Group{ModelPricing: []ChannelModelPricing{{Models: []string{"text-embedding-3-small"}, BillingMode: BillingModeToken, OutputPrice: &outputOnly}}}}
	_, err = svc.PrepareOpenAIEmbeddingsBilling(context.Background(), nil, []byte(`{"model":"text-embedding-3-small","input":"hi"}`), key, ChannelUsageFields{})
	require.ErrorIs(t, err, ErrModelPricingUnavailable, "output-only cards do not make missing input pricing free")
}

func TestOpenAIEmbeddingsRecordUsageBalanceAndSubscription(t *testing.T) {
	for _, subscription := range []bool{false, true} {
		usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
		userRepo := &openAIRecordUsageUserRepoStub{}
		subRepo := &openAIRecordUsageSubRepoStub{}
		svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, subRepo, nil)
		groupID := int64(1)
		key := &APIKey{ID: 2, GroupID: &groupID, Group: &Group{ID: 1, RateMultiplier: 2, LongContextPricingEnabled: true}}
		account := &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
		plan, err := svc.PrepareOpenAIEmbeddingsBilling(context.Background(), account, []byte(`{"model":"text-embedding-3-small","input":"hi"}`), key, ChannelUsageFields{})
		require.NoError(t, err)
		input := &OpenAIRecordUsageInput{APIKey: key, User: &User{ID: 1}, Account: account, EmbeddingsBilling: plan, Result: &OpenAIForwardResult{RequestID: "emb-billing", Model: "text-embedding-3-small", UpstreamModel: "text-embedding-3-small", Usage: OpenAIUsage{InputTokens: 290000}}}
		if subscription {
			key.Group.SubscriptionType = SubscriptionTypeSubscription
			input.Subscription = &UserSubscription{ID: 4}
		}
		require.NoError(t, svc.RecordUsage(context.Background(), input))
		require.Equal(t, 1, usageRepo.calls)
		require.Equal(t, 290000, usageRepo.lastLog.InputTokens)
		require.Zero(t, usageRepo.lastLog.OutputTokens)
		require.InDelta(t, 290000*.02e-6*2, usageRepo.lastLog.ActualCost, 1e-12)
		if subscription {
			require.Equal(t, 1, subRepo.incrementCalls)
			require.Zero(t, userRepo.deductCalls)
		} else {
			require.Equal(t, 1, userRepo.deductCalls)
			require.Zero(t, subRepo.incrementCalls)
		}
	}
}
