package service

import "math"

type BillingTokenPolicy struct {
	Enabled    bool
	Multiplier float64
}

type BillableUsage struct {
	InputTokens            int
	OutputTokens           int
	CacheCreationTokens    int
	CacheReadTokens        int
	CacheCreation5mTokens  int
	CacheCreation1hTokens  int
	ImageOutputTokens      int
	TextInputTokens        int
	CachedTextInputTokens  int
	ImageInputTokens       int
	CachedImageInputTokens int
	BillingTokenMultiplier float64
}

func BuildBillableUsage(raw UsageTokens, policy BillingTokenPolicy) BillableUsage {
	multiplier := normalizeBillableUsageMultiplier(policy.Multiplier)
	if !policy.Enabled {
		multiplier = 1
	}

	return BillableUsage{
		InputTokens:            multiplyBillableTokens(raw.InputTokens, multiplier),
		OutputTokens:           multiplyBillableTokens(raw.OutputTokens, multiplier),
		CacheCreationTokens:    multiplyBillableTokens(raw.CacheCreationTokens, multiplier),
		CacheReadTokens:        multiplyBillableTokens(raw.CacheReadTokens, multiplier),
		CacheCreation5mTokens:  multiplyBillableTokens(raw.CacheCreation5mTokens, multiplier),
		CacheCreation1hTokens:  multiplyBillableTokens(raw.CacheCreation1hTokens, multiplier),
		ImageOutputTokens:      multiplyBillableTokens(raw.ImageOutputTokens, multiplier),
		TextInputTokens:        multiplyBillableTokens(raw.TextInputTokens, multiplier),
		CachedTextInputTokens:  multiplyBillableTokens(raw.CachedTextInputTokens, multiplier),
		ImageInputTokens:       multiplyBillableTokens(raw.ImageInputTokens, multiplier),
		CachedImageInputTokens: multiplyBillableTokens(raw.CachedImageInputTokens, multiplier),
		BillingTokenMultiplier: multiplier,
	}
}

func (u BillableUsage) UsageTokens() UsageTokens {
	return UsageTokens{
		InputTokens:           u.InputTokens,
		OutputTokens:          u.OutputTokens,
		CacheCreationTokens:   u.CacheCreationTokens,
		CacheReadTokens:       u.CacheReadTokens,
		CacheCreation5mTokens: u.CacheCreation5mTokens,
		CacheCreation1hTokens: u.CacheCreation1hTokens,
		ImageOutputTokens:     u.ImageOutputTokens,
	}
}

func (u BillableUsage) ApplyToUsageLog(log *UsageLog) {
	if log == nil {
		return
	}
	log.BillableInputTokens = u.InputTokens
	log.BillableOutputTokens = u.OutputTokens
	log.BillableCacheCreationTokens = u.CacheCreationTokens
	log.BillableCacheReadTokens = u.CacheReadTokens
	log.BillableImageOutputTokens = u.ImageOutputTokens
	log.BillableTextInputTokens = u.TextInputTokens
	log.BillableCachedTextInputTokens = u.CachedTextInputTokens
	log.BillableImageInputTokens = u.ImageInputTokens
	log.BillableCachedImageInputTokens = u.CachedImageInputTokens
	log.BillingTokenMultiplier = u.BillingTokenMultiplier
	log.NormalizeBillableUsageDefaults()
}

func normalizeBillableUsageMultiplier(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return 1
	}
	return value
}

func multiplyBillableTokens(tokens int, multiplier float64) int {
	if tokens <= 0 {
		return 0
	}
	if multiplier <= 0 {
		multiplier = 1
	}
	return int(math.Ceil(float64(tokens) * multiplier))
}
