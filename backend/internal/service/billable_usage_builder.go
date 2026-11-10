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
	CacheCreation30mTokens int
	CacheCreation12hTokens int
	ImageOutputTokens      int
	TextInputTokens        int
	CachedTextInputTokens  int
	ImageInputTokens       int
	ImageCacheReadTokens   int
	CachedImageInputTokens int
	BillingTokenMultiplier float64
}

// BuildBillableUsage scales non-cached input and output only; cache usage keeps
// its original quantity, including cache creation durations and modality details.
func BuildBillableUsage(raw UsageTokens, policy BillingTokenPolicy) BillableUsage {
	multiplier := normalizeBillableUsageMultiplier(policy.Multiplier)
	if !policy.Enabled {
		multiplier = 1
	}

	return BillableUsage{
		InputTokens:            multiplyBillableTokens(raw.InputTokens, multiplier),
		OutputTokens:           multiplyBillableTokens(raw.OutputTokens, multiplier),
		CacheCreationTokens:    multiplyBillableTokens(raw.CacheCreationTokens, 1),
		CacheReadTokens:        multiplyBillableTokens(raw.CacheReadTokens, 1),
		CacheCreation5mTokens:  multiplyBillableTokens(raw.CacheCreation5mTokens, 1),
		CacheCreation1hTokens:  multiplyBillableTokens(raw.CacheCreation1hTokens, 1),
		CacheCreation30mTokens: multiplyBillableTokens(raw.CacheCreation30mTokens, 1),
		CacheCreation12hTokens: multiplyBillableTokens(raw.CacheCreation12hTokens, 1),
		ImageOutputTokens:      multiplyBillableTokens(raw.ImageOutputTokens, multiplier),
		TextInputTokens:        multiplyBillableTokens(raw.TextInputTokens, multiplier),
		CachedTextInputTokens:  multiplyBillableTokens(raw.CachedTextInputTokens, 1),
		ImageInputTokens:       multiplyBillableTokens(raw.ImageInputTokens, multiplier),
		ImageCacheReadTokens:   multiplyBillableTokens(raw.ImageCacheReadTokens, 1),
		CachedImageInputTokens: multiplyBillableTokens(raw.CachedImageInputTokens, 1),
		BillingTokenMultiplier: multiplier,
	}
}

func (u BillableUsage) UsageTokens() UsageTokens {
	return UsageTokens{
		InputTokens:            u.InputTokens,
		OutputTokens:           u.OutputTokens,
		CacheCreationTokens:    u.CacheCreationTokens,
		CacheReadTokens:        u.CacheReadTokens,
		CacheCreation5mTokens:  u.CacheCreation5mTokens,
		CacheCreation1hTokens:  u.CacheCreation1hTokens,
		CacheCreation30mTokens: u.CacheCreation30mTokens,
		CacheCreation12hTokens: u.CacheCreation12hTokens,
		ImageOutputTokens:      u.ImageOutputTokens,
		ImageInputTokens:       u.ImageInputTokens,
		ImageCacheReadTokens:   u.ImageCacheReadTokens,
		TextInputTokens:        u.TextInputTokens,
		CachedTextInputTokens:  u.CachedTextInputTokens,
		CachedImageInputTokens: u.CachedImageInputTokens,
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
