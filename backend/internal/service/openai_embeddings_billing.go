package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// OpenAIEmbeddingsBilling owns the prices admitted for this request, never cached account state.
type OpenAIEmbeddingsBilling struct {
	model          string
	resolved       *ResolvedPricing
	responsePrices map[string]*ResolvedPricing
}

func (s *BillingService) initOpenAIEmbeddingsPricing() {
	for model, price := range map[string]float64{
		"text-embedding-3-small": 0.02e-6,
		"text-embedding-3-large": 0.13e-6,
		"text-embedding-ada-002": 0.10e-6,
	} {
		s.fallbackPrices[model] = &ModelPricing{InputPricePerToken: price}
	}
}

// PrepareOpenAIEmbeddingsBilling runs after selection, before spending money upstream.
func (s *OpenAIGatewayService) PrepareOpenAIEmbeddingsBilling(ctx context.Context, account *Account, body []byte, apiKey *APIKey, fields ChannelUsageFields) (*OpenAIEmbeddingsBilling, error) {
	requested := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	model := normalizeOpenAIModelForUpstream(account, resolveOpenAIForwardModel(account, requested, ""))
	if openAIEmbeddingDimensions(model) == 0 {
		return nil, nil
	}
	if s.billingService == nil {
		return nil, ErrModelPricingUnavailable
	}
	preferred := model
	switch fields.BillingModelSource {
	case BillingModelSourceRequested:
		preferred = fields.OriginalModel
	case BillingModelSourceChannelMapped:
		preferred = fields.ChannelMappedModel
	}
	for _, candidate := range usageBillingModelCandidates(preferred, model) {
		resolved, err := s.resolveOpenAIEmbeddingPrice(ctx, candidate, model, apiKey)
		if err != nil {
			continue
		}
		plan := &OpenAIEmbeddingsBilling{model: candidate, resolved: resolved}
		if fields.BillingModelSource == BillingModelSourceResponse {
			plan.responsePrices = make(map[string]*ResolvedPricing)
			for _, responseModel := range []string{"text-embedding-3-small", "text-embedding-3-large", "text-embedding-ada-002"} {
				if price, err := s.resolveOpenAIEmbeddingPrice(ctx, responseModel, responseModel, apiKey); err == nil {
					plan.responsePrices[responseModel] = price
				}
			}
		}
		return plan, nil
	}
	return nil, fmt.Errorf("%w: embeddings model has no usable price", ErrModelPricingUnavailable)
}

func (s *OpenAIGatewayService) resolveOpenAIEmbeddingPrice(ctx context.Context, candidate, upstream string, apiKey *APIKey) (*ResolvedPricing, error) {
	resolver := s.resolver
	if resolver == nil {
		resolver = NewModelPricingResolver(nil, s.billingService)
	}
	input := PricingInput{Model: candidate}
	if apiKey != nil {
		input.Group, input.GroupID = apiKey.Group, apiKey.GroupID
	}
	resolved := resolver.Resolve(ctx, input)
	explicit := resolved.Source == PricingSourceGroup || resolved.Source == PricingSourceChannel
	if explicit && resolved.channelPricing != nil {
		// Alias cards inherit omitted rates from the actual embedding model, never a chat-name fallback.
		resolved = resolver.resolveConfiguredPricing(resolved.channelPricing, upstream, resolved.Source)
	}
	if !explicit && (candidate != upstream || !s.billingService.HasIdentifiedTokenPricing(candidate)) {
		return nil, ErrModelPricingUnavailable
	}
	copy := *resolved
	copy.longContextPricingEnabled = false
	if resolved.channelPricing != nil {
		config := resolved.channelPricing.Clone()
		copy.channelPricing = &config
	}
	if copy.Mode == BillingModePerRequest {
		if math.IsNaN(copy.DefaultPerRequestPrice) || math.IsInf(copy.DefaultPerRequestPrice, 0) || copy.DefaultPerRequestPrice < 0 {
			return nil, ErrModelPricingUnavailable
		}
		copy.RequestTiers = append([]PricingInterval(nil), resolved.RequestTiers...)
		return &copy, nil
	}
	if copy.Mode != "" && copy.Mode != BillingModeToken {
		return nil, ErrModelPricingUnavailable
	}
	if !s.billingService.HasIdentifiedTokenPricing(upstream) &&
		(resolved.channelPricing == nil || resolved.channelPricing.InputPrice == nil) {
		interval := FindMatchingInterval(resolved.Intervals, 1)
		if interval == nil || interval.InputPrice == nil {
			return nil, ErrModelPricingUnavailable
		}
	}
	// Freeze the first tier; aggregate batch tokens never select a chat context surcharge.
	price := resolver.GetIntervalPricing(resolved, 1)
	if price == nil || math.IsNaN(price.InputPricePerToken) || math.IsInf(price.InputPricePerToken, 0) || price.InputPricePerToken < 0 {
		return nil, ErrModelPricingUnavailable
	}
	priceCopy := *price
	priceCopy.LongContextInputThreshold = 0
	priceCopy.LongContextInputMultiplier, priceCopy.LongContextOutputMultiplier = 0, 0
	copy.BasePricing, copy.Intervals = &priceCopy, nil
	return &copy, nil
}

func (s *OpenAIGatewayService) calculateOpenAIEmbeddingsCost(ctx context.Context, plan *OpenAIEmbeddingsBilling, result *OpenAIForwardResult, usage BillableUsage, multiplier float64, pricingAt time.Time) (*CostBreakdown, error) {
	resolver := s.resolver
	if resolver == nil {
		resolver = NewModelPricingResolver(nil, s.billingService)
	}
	calculate := func(model string, price *ResolvedPricing) (*CostBreakdown, error) {
		disabled := false
		cost, err := s.billingService.CalculateCostUnified(CostInput{
			Ctx: ctx, Model: model, Tokens: usage.UsageTokens(), RequestCount: 1,
			RateMultiplier: multiplier, PricingAt: pricingAt, Resolver: resolver,
			Resolved: price, LongContextBillingEnabled: &disabled,
		})
		if cost != nil {
			cost.BillableUsage = usage
		}
		return cost, err
	}
	cost, err := calculate(plan.model, plan.resolved)
	if err != nil {
		return nil, err
	}
	if price := plan.responsePrices[result.UpstreamResponseModel]; price != nil && !result.UpstreamResponseModelConflict {
		responseCost, responseErr := calculate(result.UpstreamResponseModel, price)
		configured := func(p *ResolvedPricing) bool {
			return p.Source == PricingSourceGroup || p.Source == PricingSourceChannel
		}
		if responseErr == nil && responseModelBillingAdoptable(cost, responseCost, configured(plan.resolved), configured(price)) {
			return responseCost, nil
		}
	}
	return cost, nil
}

func isOfficialOpenAIEmbeddingResult(result *OpenAIForwardResult) bool {
	if result == nil {
		return false
	}
	model := strings.TrimSpace(result.UpstreamModel)
	if model == "" {
		model = strings.TrimSpace(result.Model)
	}
	return openAIEmbeddingDimensions(strings.ToLower(model)) > 0
}
