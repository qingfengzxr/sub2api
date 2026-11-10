package service

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func (s *GatewayService) snapshotResponseBillingTokenPolicy(ctx context.Context, c *gin.Context) BillingTokenPolicy {
	return snapshotResponseBillingTokenPolicy(ctx, c, s.billingTokenPolicy)
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
