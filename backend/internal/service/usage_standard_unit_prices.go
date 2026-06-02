package service

import (
	"context"
	"strings"
)

const usageStandardPricePerMillionScale = 1_000_000

// UsageStandardUnitPrices contains user-facing reference prices only.
// These values intentionally exclude billing token multipliers, user/group
// rate multipliers, service-tier pricing, long-context multipliers, and image
// rate multipliers.
type UsageStandardUnitPrices struct {
	InputPricePerMillion         *float64
	OutputPricePerMillion        *float64
	CacheCreationPricePerMillion *float64
	CacheReadPricePerMillion     *float64
	ImageOutputPricePerMillion   *float64
	UnitPrice                    *float64
}

type UsageStandardUnitPriceResolver struct {
	pricingResolver *ModelPricingResolver
	billingService  *BillingService
}

func NewUsageStandardUnitPriceResolver(pricingResolver *ModelPricingResolver, billingService *BillingService) *UsageStandardUnitPriceResolver {
	return &UsageStandardUnitPriceResolver{
		pricingResolver: pricingResolver,
		billingService:  billingService,
	}
}

func (r *UsageStandardUnitPriceResolver) Resolve(ctx context.Context, log *UsageLog) UsageStandardUnitPrices {
	if r == nil || log == nil {
		return UsageStandardUnitPrices{}
	}

	model := usageStandardPricingModel(log)
	if model == "" {
		return UsageStandardUnitPrices{}
	}

	displayMode := usageDisplayBillingMode(log)
	if displayMode == BillingModeImage {
		return r.resolveImagePrices(ctx, model, log)
	}

	resolved := r.resolveModelPricing(ctx, model, log.GroupID)
	if resolved == nil {
		return UsageStandardUnitPrices{}
	}

	switch displayMode {
	case BillingModePerRequest:
		if resolved.Mode != BillingModePerRequest {
			return UsageStandardUnitPrices{}
		}
		return UsageStandardUnitPrices{
			UnitPrice: positivePricePtr(r.resolveRequestUnitPrice(resolved, log)),
		}
	default:
		if resolved.Mode != BillingModeToken && resolved.Mode != "" {
			return UsageStandardUnitPrices{}
		}
		return r.resolveTokenPrices(resolved, log)
	}
}

func (r *UsageStandardUnitPriceResolver) Apply(ctx context.Context, log *UsageLog) {
	if log == nil {
		return
	}
	prices := r.Resolve(ctx, log)
	log.StandardInputPricePerMillion = prices.InputPricePerMillion
	log.StandardOutputPricePerMillion = prices.OutputPricePerMillion
	log.StandardCacheCreationPricePerMillion = prices.CacheCreationPricePerMillion
	log.StandardCacheReadPricePerMillion = prices.CacheReadPricePerMillion
	log.StandardImageOutputPricePerMillion = prices.ImageOutputPricePerMillion
	log.StandardUnitPrice = prices.UnitPrice
}

func (r *UsageStandardUnitPriceResolver) resolveTokenPrices(resolved *ResolvedPricing, log *UsageLog) UsageStandardUnitPrices {
	if r.pricingResolver == nil {
		return UsageStandardUnitPrices{}
	}
	pricing := r.pricingResolver.GetIntervalPricing(resolved, usageStandardContextTokens(log))
	if pricing == nil {
		return UsageStandardUnitPrices{}
	}

	return UsageStandardUnitPrices{
		InputPricePerMillion:         pricePerMillionPtr(pricing.InputPricePerToken),
		OutputPricePerMillion:        pricePerMillionPtr(pricing.OutputPricePerToken),
		CacheCreationPricePerMillion: standardCacheCreationPricePerMillion(pricing),
		CacheReadPricePerMillion:     pricePerMillionPtr(pricing.CacheReadPricePerToken),
		ImageOutputPricePerMillion:   pricePerMillionPtr(pricing.ImageOutputPricePerToken),
	}
}

func (r *UsageStandardUnitPriceResolver) resolveImagePrices(ctx context.Context, model string, log *UsageLog) UsageStandardUnitPrices {
	resolved := r.resolveModelPricing(ctx, model, log.GroupID)
	if resolved != nil && resolved.Source == PricingSourceChannel {
		if resolved.Mode == BillingModeToken {
			return r.resolveTokenPrices(resolved, log)
		}
		if resolved.Mode == BillingModeImage || resolved.Mode == BillingModePerRequest {
			return r.resolveImagePricesFromResolved(model, log, resolved)
		}
	}

	if r.billingService == nil {
		return UsageStandardUnitPrices{}
	}
	sizeTier := usageImageSizeTier(log)
	groupConfig := usageImagePriceConfig(log.Group)
	return UsageStandardUnitPrices{
		UnitPrice: positivePricePtr(r.billingService.GetImageUnitPrice(model, sizeTier, groupConfig)),
	}
}

func (r *UsageStandardUnitPriceResolver) resolveImagePricesFromResolved(model string, log *UsageLog, resolved *ResolvedPricing) UsageStandardUnitPrices {
	unitPrice := r.resolveRequestUnitPrice(resolved, log)
	if unitPrice > 0 {
		return UsageStandardUnitPrices{UnitPrice: &unitPrice}
	}

	if r.billingService == nil {
		return UsageStandardUnitPrices{}
	}
	sizeTier := usageImageSizeTier(log)
	groupConfig := usageImagePriceConfig(log.Group)
	return UsageStandardUnitPrices{
		UnitPrice: positivePricePtr(r.billingService.GetImageUnitPrice(model, sizeTier, groupConfig)),
	}
}

func (r *UsageStandardUnitPriceResolver) resolveRequestUnitPrice(resolved *ResolvedPricing, log *UsageLog) float64 {
	if r.pricingResolver == nil || resolved == nil {
		return 0
	}
	if tier := strings.TrimSpace(stringPtrValue(log.BillingTier)); tier != "" {
		if price := r.pricingResolver.GetRequestTierPrice(resolved, tier); price > 0 {
			return price
		}
	}
	if sizeTier := usageImageSizeTier(log); sizeTier != "" && log.ImageCount > 0 {
		if price := r.pricingResolver.GetRequestTierPrice(resolved, sizeTier); price > 0 {
			return price
		}
	}
	if price := r.pricingResolver.GetRequestTierPriceByContext(resolved, usageStandardContextTokens(log)); price > 0 {
		return price
	}
	return resolved.DefaultPerRequestPrice
}

func (r *UsageStandardUnitPriceResolver) resolveModelPricing(ctx context.Context, model string, groupID *int64) *ResolvedPricing {
	if r.pricingResolver == nil {
		return nil
	}
	return r.pricingResolver.Resolve(ctx, PricingInput{Model: model, GroupID: groupID})
}

func usageStandardPricingModel(log *UsageLog) string {
	if log == nil {
		return ""
	}
	if model := strings.TrimSpace(log.RequestedModel); model != "" {
		return model
	}
	return strings.TrimSpace(log.Model)
}

func usageDisplayBillingMode(log *UsageLog) BillingMode {
	if log == nil {
		return BillingModeToken
	}
	if log.ImageCount > 0 {
		return BillingModeImage
	}
	mode := BillingMode(strings.TrimSpace(stringPtrValue(log.BillingMode)))
	if !mode.IsValid() || mode == "" {
		return BillingModeToken
	}
	return mode
}

func usageStandardContextTokens(log *UsageLog) int {
	if log == nil {
		return 0
	}
	input := positiveTokenCount(log.BillableInputTokens)
	if input == 0 {
		input = positiveTokenCount(log.InputTokens)
	}
	cacheRead := positiveTokenCount(log.BillableCacheReadTokens)
	if cacheRead == 0 {
		cacheRead = positiveTokenCount(log.CacheReadTokens)
	}
	return input + cacheRead
}

func usageImageSizeTier(log *UsageLog) string {
	if log == nil {
		return ""
	}
	return NormalizeImageBillingTierOrDefault(stringPtrValue(log.ImageSize))
}

func usageImagePriceConfig(group *Group) *ImagePriceConfig {
	if group == nil {
		return nil
	}
	return &ImagePriceConfig{
		Price1K: group.ImagePrice1K,
		Price2K: group.ImagePrice2K,
		Price4K: group.ImagePrice4K,
	}
}

func standardCacheCreationPricePerMillion(pricing *ModelPricing) *float64 {
	if pricing == nil {
		return nil
	}
	if pricing.SupportsCacheBreakdown && pricing.CacheCreation5mPrice > 0 {
		return pricePerMillionPtr(pricing.CacheCreation5mPrice)
	}
	return pricePerMillionPtr(pricing.CacheCreationPricePerToken)
}

func pricePerMillionPtr(pricePerToken float64) *float64 {
	return positivePricePtr(pricePerToken * usageStandardPricePerMillionScale)
}

func positivePricePtr(price float64) *float64 {
	if price <= 0 {
		return nil
	}
	value := price
	return &value
}

func positiveTokenCount(value int) int {
	if value > 0 {
		return value
	}
	return 0
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
