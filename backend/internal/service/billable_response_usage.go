package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func (s *GatewayService) snapshotResponseBillingTokenPolicy(ctx context.Context, c *gin.Context) BillingTokenPolicy {
	return snapshotResponseBillingTokenPolicy(ctx, c, s.billingTokenPolicy)
}

func loadResponseBillingTokenPolicy(ctx context.Context, settings *SettingService) (BillingTokenPolicy, error) {
	if policy, ok := BillingTokenPolicyFromContext(ctx); ok {
		return policy, nil
	}
	if settings == nil {
		return BillingTokenPolicy{Multiplier: 1}, nil
	}
	lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	values, err := settings.settingRepo.GetMultiple(lookupCtx, []string{SettingKeyTokenMultiplierBillingEnabled, SettingKeyBillingTokenMultiplier})
	if err != nil {
		return BillingTokenPolicy{}, fmt.Errorf("load billing token policy: %w", err)
	}
	return BillingTokenPolicy{Enabled: values[SettingKeyTokenMultiplierBillingEnabled] == "true", Multiplier: parseBillingTokenMultiplier(values[SettingKeyBillingTokenMultiplier])}, nil
}

func prepareResponseBillingTokenPolicy(ctx context.Context, c *gin.Context, settings *SettingService) (BillingTokenPolicy, error) {
	if c != nil && c.Request != nil {
		if policy, ok := BillingTokenPolicyFromContext(c.Request.Context()); ok {
			return policy, nil
		}
	}
	policy, err := loadResponseBillingTokenPolicy(ctx, settings)
	if err != nil {
		return BillingTokenPolicy{}, err
	}
	if c != nil && c.Request != nil {
		c.Request = c.Request.WithContext(WithBillingTokenPolicy(c.Request.Context(), policy))
	}
	return policy, nil
}

func (s *GatewayService) PrepareResponseBillingTokenPolicy(ctx context.Context, c *gin.Context) (BillingTokenPolicy, error) {
	return prepareResponseBillingTokenPolicy(ctx, c, s.settingService)
}

func billableAnthropicProtocolUsage(c *gin.Context, usage apicompat.AnthropicUsage) apicompat.AnthropicUsage {
	policy, _ := responseBillingPolicy(c)
	billable := BuildBillableUsage(UsageTokens{InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens}, policy)
	usage.InputTokens = billable.InputTokens
	usage.OutputTokens = billable.OutputTokens
	return usage
}

func billableAnthropicProtocolEvent(c *gin.Context, event apicompat.AnthropicStreamEvent) apicompat.AnthropicStreamEvent {
	if event.Message != nil {
		message := *event.Message
		message.Usage = billableAnthropicProtocolUsage(c, message.Usage)
		event.Message = &message
	}
	if event.Usage != nil {
		usage := billableAnthropicProtocolUsage(c, *event.Usage)
		event.Usage = &usage
	}
	return event
}

func responseBillingPolicy(c *gin.Context) (BillingTokenPolicy, bool) {
	if c == nil || c.Request == nil {
		return BillingTokenPolicy{Multiplier: 1}, false
	}
	policy, _ := BillingTokenPolicyFromContext(c.Request.Context())
	return policy, IsForceCacheBilling(c.Request.Context())
}

func billableClaudeUsageForResponse(c *gin.Context, usage *ClaudeUsage) BillableUsage {
	policy, forceCache := responseBillingPolicy(c)
	if usage == nil {
		return BuildBillableUsage(UsageTokens{}, policy)
	}
	input, cached := usage.InputTokens, usage.CacheReadInputTokens
	if forceCache {
		cached += input
		input = 0
	}
	return BuildBillableUsage(UsageTokens{
		InputTokens: input, OutputTokens: usage.OutputTokens,
		CacheReadTokens: cached, CacheCreationTokens: usage.CacheCreationInputTokens,
		ImageOutputTokens: usage.ImageOutputTokens,
	}, policy)
}

func billableClaudeChatUsage(c *gin.Context, original *apicompat.ChatUsage, raw *ClaudeUsage) *apicompat.ChatUsage {
	if original == nil || raw == nil {
		return original
	}
	usage := *original
	billable := billableClaudeUsageForResponse(c, raw)
	usage.PromptTokens = billable.InputTokens + billable.CacheReadTokens + billable.CacheCreationTokens
	usage.CompletionTokens = billable.OutputTokens
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	if usage.PromptTokensDetails != nil || billable.CacheReadTokens > 0 {
		details := apicompat.ChatTokenDetails{}
		if usage.PromptTokensDetails != nil {
			details = *usage.PromptTokensDetails
		}
		details.CachedTokens = billable.CacheReadTokens
		usage.PromptTokensDetails = &details
	}
	return &usage
}

func billableClaudeResponsesUsage(c *gin.Context, original *apicompat.ResponsesUsage, raw *ClaudeUsage) *apicompat.ResponsesUsage {
	if original == nil || raw == nil {
		return original
	}
	usage := *original
	billable := billableClaudeUsageForResponse(c, raw)
	usage.InputTokens = billable.InputTokens + billable.CacheReadTokens + billable.CacheCreationTokens
	usage.OutputTokens = billable.OutputTokens
	usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	usage.CacheCreationInputTokens = billable.CacheCreationTokens
	if usage.InputTokensDetails != nil || billable.CacheReadTokens > 0 {
		details := apicompat.ResponsesInputTokensDetails{}
		if usage.InputTokensDetails != nil {
			details = *usage.InputTokensDetails
		}
		details.CachedTokens = billable.CacheReadTokens
		usage.InputTokensDetails = &details
	}
	return &usage
}

func billableClaudeResponsesEvent(c *gin.Context, event apicompat.ResponsesStreamEvent, raw *ClaudeUsage) apicompat.ResponsesStreamEvent {
	event.Usage = billableClaudeResponsesUsage(c, event.Usage, raw)
	if event.Response != nil {
		response := *event.Response
		response.Usage = billableClaudeResponsesUsage(c, response.Usage, raw)
		event.Response = &response
	}
	return event
}

func applyGeminiClaudeResponseUsage(c *gin.Context, response map[string]any, usage *ClaudeUsage) {
	fields, ok := response["usage"].(map[string]any)
	if !ok || usage == nil {
		return
	}
	billable := billableClaudeUsageForResponse(c, usage)
	fields["input_tokens"] = billable.InputTokens
	fields["output_tokens"] = billable.OutputTokens
	if billable.CacheReadTokens > 0 {
		fields["cache_read_input_tokens"] = billable.CacheReadTokens
	}
}

func billableGeminiResponse(body []byte, policy BillingTokenPolicy, forceCache bool) ([]byte, error) {
	if !forceCache && (!policy.Enabled || normalizeBillableUsageMultiplier(policy.Multiplier) == 1) {
		return body, nil
	}
	metadata := gjson.GetBytes(body, "usageMetadata")
	if !metadata.IsObject() {
		return body, nil
	}
	usage := extractGeminiUsage(body)
	if usage == nil {
		return body, nil
	}
	if forceCache {
		usage.CacheReadInputTokens += usage.InputTokens
		usage.InputTokens = 0
	}
	billable := BuildBillableUsage(UsageTokens{
		InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens,
		CacheReadTokens: usage.CacheReadInputTokens,
	}, policy)
	prompt := billable.InputTokens + billable.CacheReadTokens
	thoughts := min(multiplyBillableTokens(int(metadata.Get("thoughtsTokenCount").Int()), billable.BillingTokenMultiplier), billable.OutputTokens)
	candidates := billable.OutputTokens - thoughts
	fields := []struct {
		name  string
		value int
	}{
		{"promptTokenCount", prompt},
		{"candidatesTokenCount", candidates},
		{"thoughtsTokenCount", thoughts},
		{"totalTokenCount", prompt + billable.OutputTokens},
		{"cachedContentTokenCount", billable.CacheReadTokens},
	}
	for _, field := range fields {
		if metadata.Get(field.name).Type != gjson.Number && !(forceCache && field.name == "cachedContentTokenCount" && billable.CacheReadTokens > 0) {
			continue
		}
		var err error
		body, err = sjson.SetBytes(body, "usageMetadata."+field.name, field.value)
		if err != nil {
			return nil, err
		}
	}
	var rawTotal, scaledTotal int
	for i, detail := range metadata.Get("candidatesTokensDetails").Array() {
		if detail.Get("tokenCount").Type != gjson.Number {
			continue
		}
		rawTotal += max(int(detail.Get("tokenCount").Int()), 0)
		next := min(multiplyBillableTokens(rawTotal, billable.BillingTokenMultiplier), candidates)
		var err error
		body, err = sjson.SetBytes(body, fmt.Sprintf("usageMetadata.candidatesTokensDetails.%d.tokenCount", i), max(next-scaledTotal, 0))
		if err != nil {
			return nil, err
		}
		scaledTotal = next
	}
	promptDetails := metadata.Get("promptTokensDetails").Array()
	if len(promptDetails) > 0 {
		cacheByModality := map[string]int{}
		for _, detail := range metadata.Get("cacheTokensDetails").Array() {
			cacheByModality[detail.Get("modality").String()] += max(int(detail.Get("tokenCount").Int()), 0)
		}
		if len(promptDetails) == 1 && len(cacheByModality) == 0 {
			cacheByModality[promptDetails[0].Get("modality").String()] = int(metadata.Get("cachedContentTokenCount").Int())
		}
		var promptTotal, cacheTotal int
		promptByModality := map[string]int{}
		detailsValid := true
		for _, detail := range promptDetails {
			if detail.Get("tokenCount").Type != gjson.Number {
				detailsValid = false
			}
			count := max(int(detail.Get("tokenCount").Int()), 0)
			promptTotal += count
			promptByModality[detail.Get("modality").String()] += count
		}
		for modality, count := range cacheByModality {
			cacheTotal += count
			if count > promptByModality[modality] {
				detailsValid = false
			}
		}
		// shortcut: retain upstream details when cache modalities are missing, upgrade when providers report the split.
		if detailsValid && promptTotal == int(metadata.Get("promptTokenCount").Int()) && (forceCache || cacheTotal == int(metadata.Get("cachedContentTokenCount").Int())) {
			if forceCache && metadata.Get("cacheTokensDetails").IsArray() {
				var err error
				body, err = sjson.SetRawBytes(body, "usageMetadata.cacheTokensDetails", []byte(metadata.Get("promptTokensDetails").Raw))
				if err != nil {
					return nil, err
				}
			}
			var rawInput, scaledInput int
			for i, detail := range promptDetails {
				count := max(int(detail.Get("tokenCount").Int()), 0)
				modality := detail.Get("modality").String()
				cached := min(cacheByModality[modality], count)
				cacheByModality[modality] -= cached
				if forceCache {
					cached = count
				}
				rawInput += count - cached
				next := multiplyBillableTokens(rawInput, billable.BillingTokenMultiplier)
				var err error
				body, err = sjson.SetBytes(body, fmt.Sprintf("usageMetadata.promptTokensDetails.%d.tokenCount", i), cached+next-scaledInput)
				if err != nil {
					return nil, err
				}
				scaledInput = next
			}
		}
	}
	return body, nil
}

func (s *OpenAIGatewayService) snapshotResponseBillingTokenPolicy(ctx context.Context, c *gin.Context) BillingTokenPolicy {
	return snapshotResponseBillingTokenPolicy(ctx, c, s.billingTokenPolicy)
}

func snapshotResponseBillingTokenPolicy(ctx context.Context, c *gin.Context, lookup func(context.Context) BillingTokenPolicy) BillingTokenPolicy {
	if c != nil && c.Request != nil {
		if policy, ok := BillingTokenPolicyFromContext(c.Request.Context()); ok {
			return policy
		}
	}
	lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	policy := lookup(lookupCtx)
	if c != nil && c.Request != nil {
		c.Request = c.Request.WithContext(WithBillingTokenPolicy(c.Request.Context(), policy))
	}
	return policy
}

func billableOpenAIResponse(body []byte, usagePath string, policy BillingTokenPolicy) ([]byte, error) {
	if !policy.Enabled || normalizeBillableUsageMultiplier(policy.Multiplier) == 1 {
		return body, nil
	}
	value := gjson.GetBytes(body, usagePath)
	raw, ok := openAIUsageFromGJSON(value)
	if !ok {
		return body, nil
	}
	return billableOpenAIResponseWithRaw(body, usagePath, policy, raw)
}

func billableOpenAIResponseWithRaw(body []byte, usagePath string, policy BillingTokenPolicy, raw OpenAIUsage) ([]byte, error) {
	value := gjson.GetBytes(body, usagePath)
	billable := BuildBillableUsage(UsageTokens{
		InputTokens:         max(raw.InputTokens-raw.CacheReadInputTokens-raw.CacheCreationInputTokens, 0),
		OutputTokens:        raw.OutputTokens,
		CacheReadTokens:     raw.CacheReadInputTokens,
		CacheCreationTokens: raw.CacheCreationInputTokens,
	}, policy)
	input := billable.InputTokens + billable.CacheReadTokens + billable.CacheCreationTokens
	output := billable.OutputTokens
	fields := []struct {
		name  string
		value int
	}{
		{"input_tokens", input}, {"prompt_tokens", input},
		{"output_tokens", output}, {"completion_tokens", output},
		{"total_tokens", input + output},
		{"input_tokens_details.image_tokens", min(multiplyBillableTokens(raw.ImageInputTokens, billable.BillingTokenMultiplier), input)},
		{"prompt_tokens_details.image_tokens", min(multiplyBillableTokens(raw.ImageInputTokens, billable.BillingTokenMultiplier), input)},
		{"output_tokens_details.image_tokens", min(multiplyBillableTokens(raw.ImageOutputTokens, billable.BillingTokenMultiplier), output)},
		{"completion_tokens_details.image_tokens", min(multiplyBillableTokens(raw.ImageOutputTokens, billable.BillingTokenMultiplier), output)},
		{"output_tokens_details.reasoning_tokens", min(multiplyBillableTokens(int(value.Get("output_tokens_details.reasoning_tokens").Int()), billable.BillingTokenMultiplier), output)},
		{"completion_tokens_details.reasoning_tokens", min(multiplyBillableTokens(int(value.Get("completion_tokens_details.reasoning_tokens").Int()), billable.BillingTokenMultiplier), output)},
	}
	for _, field := range fields {
		if value.Get(field.name).Type != gjson.Number {
			continue
		}
		var err error
		body, err = sjson.SetBytes(body, usagePath+"."+field.name, field.value)
		if err != nil {
			return nil, err
		}
	}
	return body, nil
}

func billableRawChatUsage(body []byte, policy BillingTokenPolicy, bridge OpenAIUsage) ([]byte, error) {
	value := gjson.GetBytes(body, "usage")
	raw, ok := openAIUsageFromGJSON(value)
	if !ok {
		return body, nil
	}
	addOpenAIUsage(&raw, bridge)
	if bridge.CacheReadInputTokens > 0 || bridge.CacheCreationInputTokens > 0 {
		fields := []struct {
			path   string
			amount int
		}{
			{"prompt_tokens_details.cached_tokens", raw.CacheReadInputTokens},
			{"prompt_tokens_details.cache_write_tokens", raw.CacheCreationInputTokens},
		}
		for _, field := range fields {
			if field.amount <= 0 {
				continue
			}
			var err error
			body, err = sjson.SetBytes(body, "usage."+field.path, field.amount)
			if err != nil {
				return nil, err
			}
		}
	}
	if !policy.Enabled || normalizeBillableUsageMultiplier(policy.Multiplier) == 1 {
		if !openAIUsageHasTokens(&bridge) {
			return body, nil
		}
		policy = BillingTokenPolicy{Enabled: true, Multiplier: 1}
	}
	return billableOpenAIResponseWithRaw(body, "usage", policy, raw)
}

func billableOpenAIResponseUsage(body []byte, policy BillingTokenPolicy) ([]byte, error) {
	for _, path := range []string{"usage", "response.usage", "data.usage", "data.response.usage"} {
		if gjson.GetBytes(body, path).IsObject() {
			return billableOpenAIResponse(body, path, policy)
		}
	}
	return body, nil
}

// billableAnthropicResponse rewrites only usage fields, retaining all other
// upstream response fields and the separately collected raw accounting usage.
func billableAnthropicResponse(body []byte, usagePath string, policy BillingTokenPolicy) ([]byte, error) {
	if !policy.Enabled || normalizeBillableUsageMultiplier(policy.Multiplier) == 1 {
		return body, nil
	}
	usage := gjson.GetBytes(body, usagePath)
	if !usage.IsObject() {
		return body, nil
	}
	billable := BuildBillableUsage(UsageTokens{
		InputTokens:       int(usage.Get("input_tokens").Int()),
		OutputTokens:      int(usage.Get("output_tokens").Int()),
		ImageOutputTokens: int(usage.Get("image_output_tokens").Int()),
	}, policy)
	fields := []struct {
		name  string
		value int
	}{
		{"input_tokens", billable.InputTokens},
		{"output_tokens", billable.OutputTokens},
		{"image_output_tokens", billable.ImageOutputTokens},
	}
	for _, field := range fields {
		if usage.Get(field.name).Type != gjson.Number {
			continue
		}
		var err error
		body, err = sjson.SetBytes(body, usagePath+"."+field.name, field.value)
		if err != nil {
			return nil, err
		}
	}
	return body, nil
}

func billableAnthropicStreamEvent(body []byte, policy BillingTokenPolicy, forceCache bool) ([]byte, error) {
	path := ""
	switch gjson.GetBytes(body, "type").String() {
	case "message_start":
		path = "message.usage"
	case "message_delta":
		path = "usage"
	default:
		return body, nil
	}
	usage := gjson.GetBytes(body, path)
	if forceCache && usage.IsObject() && usage.Get("input_tokens").Int() > 0 {
		var err error
		cache := usage.Get("cache_read_input_tokens").Int() + usage.Get("input_tokens").Int()
		body, err = sjson.SetBytes(body, path+".input_tokens", 0)
		if err != nil {
			return nil, err
		}
		body, err = sjson.SetBytes(body, path+".cache_read_input_tokens", cache)
		if err != nil {
			return nil, err
		}
	}
	return billableAnthropicResponse(body, path, policy)
}

func billableAnthropicResponseForClient(body []byte, c *gin.Context) ([]byte, error) {
	policy, forceCache := responseBillingPolicy(c)
	usage := gjson.GetBytes(body, "usage")
	if forceCache && usage.IsObject() && usage.Get("input_tokens").Int() > 0 {
		var err error
		body, err = sjson.SetBytes(body, "usage.cache_read_input_tokens", usage.Get("cache_read_input_tokens").Int()+usage.Get("input_tokens").Int())
		if err != nil {
			return nil, err
		}
		body, err = sjson.SetBytes(body, "usage.input_tokens", 0)
		if err != nil {
			return nil, err
		}
	}
	return billableAnthropicResponse(body, "usage", policy)
}

func billableAnthropicSSEForClient(data []byte, c *gin.Context) ([]byte, error) {
	policy, forceCache := responseBillingPolicy(c)
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		body, err := billableAnthropicStreamEvent([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), policy, forceCache)
		if err != nil {
			return nil, err
		}
		lines[i] = "data: " + string(body)
	}
	return []byte(strings.Join(lines, "\n")), nil
}
