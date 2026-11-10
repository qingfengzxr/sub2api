package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicBillingPolicySnapshotSurvivesRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	svc := &GatewayService{}
	first := BillingTokenPolicy{Enabled: true, Multiplier: 2.5}
	second := BillingTokenPolicy{Enabled: true, Multiplier: 3}
	require.Equal(t, first, svc.snapshotResponseBillingTokenPolicy(WithBillingTokenPolicy(context.Background(), first), c))
	require.Equal(t, first, svc.snapshotResponseBillingTokenPolicy(WithBillingTokenPolicy(context.Background(), second), c))
}

func TestOpenAIPassthroughBillableUsageKeepsRaw(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policy := BillingTokenPolicy{Enabled: true, Multiplier: 2.5}
	usage := `"usage":{"input_tokens":100,"output_tokens":21,"total_tokens":121,"input_tokens_details":{"cached_tokens":20,"cache_write_tokens":10,"cache_write_tokens_details":{"ttl_30m_tokens":4,"ttl_12h_tokens":6}},"output_tokens_details":{"reasoning_tokens":4}}`
	for _, route := range []string{"passthrough_json", "passthrough_sse", "gateway_json", "gateway_sse"} {
		t.Run(route, func(t *testing.T) {
			stream := strings.HasSuffix(route, "sse")
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			body := `{"id":"r",` + usage + `}`
			if stream {
				body = `data: {"type":"response.completed","response":{"id":"r",` + usage + `}}` + "\n\n"
			}
			resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
			ctx := WithBillingTokenPolicy(context.Background(), policy)
			svc := &OpenAIGatewayService{cfg: &config.Config{}, toolCorrector: NewCodexToolCorrector()}
			if route == "passthrough_sse" {
				result, err := svc.handleStreamingResponsePassthrough(ctx, resp, c, &Account{ID: 1}, time.Now(), "model", "model")
				require.NoError(t, err)
				require.Equal(t, 100, result.usage.InputTokens)
				require.Equal(t, 21, result.usage.OutputTokens)
			} else if route == "passthrough_json" {
				result, err := svc.handleNonStreamingResponsePassthrough(ctx, resp, c, &Account{ID: 1}, "model", "model")
				require.NoError(t, err)
				require.Equal(t, 100, result.usage.InputTokens)
				require.Equal(t, 21, result.usage.OutputTokens)
			} else if stream {
				result, err := svc.handleStreamingResponse(ctx, resp, c, &Account{ID: 1}, time.Now(), "model", "model")
				require.NoError(t, err)
				require.Equal(t, 100, result.usage.InputTokens)
				require.Equal(t, 21, result.usage.OutputTokens)
			} else {
				result, err := svc.handleNonStreamingResponse(ctx, resp, c, &Account{ID: 1}, "model", "model")
				require.NoError(t, err)
				require.Equal(t, 100, result.usage.InputTokens)
				require.Equal(t, 21, result.usage.OutputTokens)
			}
			require.Contains(t, rec.Body.String(), `"input_tokens":205`)
			require.Contains(t, rec.Body.String(), `"output_tokens":53`)
			require.Contains(t, rec.Body.String(), `"total_tokens":258`)
			require.Contains(t, rec.Body.String(), `"ttl_30m_tokens":4`)
			require.Contains(t, rec.Body.String(), `"ttl_12h_tokens":6`)
			require.Contains(t, rec.Body.String(), `"reasoning_tokens":10`)
		})
	}
}

func TestBillableOpenAIResponseUsagePaths(t *testing.T) {
	policy := BillingTokenPolicy{Enabled: true, Multiplier: 2.5}
	for _, body := range []string{
		`{"type":"response.incomplete","response":{"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}`,
		`{"data":{"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}`,
	} {
		got, err := billableOpenAIResponseUsage([]byte(body), policy)
		require.NoError(t, err)
		require.Contains(t, string(got), `"input_tokens":25`)
		require.Contains(t, string(got), `"output_tokens":5`)
		require.Contains(t, string(got), `"total_tokens":30`)
	}
}

func TestAnthropicPassthroughBillableResponseKeepsRawUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policy := BillingTokenPolicy{Enabled: true, Multiplier: 2.5}
	for _, forceCache := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "force cache"}[forceCache], func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			body := `{"id":"m","usage":{"input_tokens":80,"output_tokens":21,"cache_read_input_tokens":20},"extension":true}`
			resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
			ctx := WithBillingTokenPolicy(context.Background(), policy)
			if forceCache {
				ctx = WithForceCacheBilling(ctx)
			}
			usage, err := (&GatewayService{cfg: &config.Config{}}).handleNonStreamingResponseAnthropicAPIKeyPassthrough(ctx, resp, c, &Account{ID: 1})
			require.NoError(t, err)
			require.Equal(t, 80, usage.InputTokens)
			require.Equal(t, 21, usage.OutputTokens)
			require.Equal(t, 20, usage.CacheReadInputTokens)
			want := `{"id":"m","usage":{"input_tokens":200,"output_tokens":53,"cache_read_input_tokens":20},"extension":true}`
			if forceCache {
				want = `{"id":"m","usage":{"input_tokens":0,"output_tokens":53,"cache_read_input_tokens":100},"extension":true}`
			}
			require.JSONEq(t, want, rec.Body.String())
			snapshot, ok := BillingTokenPolicyFromContext(c.Request.Context())
			require.True(t, ok)
			require.Equal(t, policy, snapshot)
		})
	}
}

func TestAnthropicGatewayBillableResponseKeepsRawUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policy := BillingTokenPolicy{Enabled: true, Multiplier: 2.5}
	for _, forceCache := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "force cache"}[forceCache], func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			body := `{"id":"m","usage":{"input_tokens":80,"output_tokens":21,"cache_read_input_tokens":20},"extension":true}`
			resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
			ctx := WithBillingTokenPolicy(context.Background(), policy)
			if forceCache {
				ctx = WithForceCacheBilling(ctx)
			}
			svc := &GatewayService{cfg: &config.Config{}, rateLimitService: &RateLimitService{}}
			usage, err := svc.handleNonStreamingResponse(ctx, resp, c, &Account{ID: 1}, "model", "model")
			require.NoError(t, err)
			require.Equal(t, 80, usage.InputTokens)
			require.Equal(t, 21, usage.OutputTokens)
			require.Equal(t, 20, usage.CacheReadInputTokens)
			want := `{"id":"m","usage":{"input_tokens":200,"output_tokens":53,"cache_read_input_tokens":20},"extension":true}`
			if forceCache {
				want = `{"id":"m","usage":{"input_tokens":0,"output_tokens":53,"cache_read_input_tokens":100},"extension":true}`
			}
			require.JSONEq(t, want, rec.Body.String())
			snapshot, ok := BillingTokenPolicyFromContext(c.Request.Context())
			require.True(t, ok)
			require.Equal(t, policy, snapshot)
		})
	}
}

func TestAnthropicPassthroughStreamReportsBillableUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, forceCache := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "force cache"}[forceCache], func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			body := `data: {"type":"message_start","message":{"usage":{"input_tokens":80,"cache_read_input_tokens":20}}}

data: {"type":"message_delta","usage":{"output_tokens":21}}

data: [DONE]

`
			resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
			ctx := WithBillingTokenPolicy(context.Background(), BillingTokenPolicy{Enabled: true, Multiplier: 2.5})
			if forceCache {
				ctx = WithForceCacheBilling(ctx)
			}
			svc := &GatewayService{cfg: &config.Config{}}
			result, err := svc.handleStreamingResponseAnthropicAPIKeyPassthrough(ctx, resp, c, &Account{ID: 1}, time.Now(), "model")
			require.NoError(t, err)
			require.Equal(t, 80, result.usage.InputTokens)
			require.Equal(t, 21, result.usage.OutputTokens)
			require.Equal(t, 20, result.usage.CacheReadInputTokens)
			if forceCache {
				require.Contains(t, rec.Body.String(), `"input_tokens":0`)
				require.Contains(t, rec.Body.String(), `"cache_read_input_tokens":100`)
			} else {
				require.Contains(t, rec.Body.String(), `"input_tokens":200`)
				require.Contains(t, rec.Body.String(), `"cache_read_input_tokens":20`)
			}
			require.Contains(t, rec.Body.String(), `"output_tokens":53`)
		})
	}
}

func TestAnthropicGatewayStreamReportsBillableUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	body := `data: {"type":"message_start","message":{"usage":{"input_tokens":80,"cache_read_input_tokens":20}}}

data: {"type":"message_delta","usage":{"output_tokens":21}}

data: [DONE]

`
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
	ctx := WithBillingTokenPolicy(context.Background(), BillingTokenPolicy{Enabled: true, Multiplier: 2.5})
	svc := &GatewayService{cfg: &config.Config{}, rateLimitService: &RateLimitService{}}
	result, err := svc.handleStreamingResponse(ctx, resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	require.NoError(t, err)
	require.Equal(t, 80, result.usage.InputTokens)
	require.Equal(t, 21, result.usage.OutputTokens)
	require.Equal(t, 20, result.usage.CacheReadInputTokens)
	require.Contains(t, rec.Body.String(), `"input_tokens":200`)
	require.Contains(t, rec.Body.String(), `"cache_read_input_tokens":20`)
	require.Contains(t, rec.Body.String(), `"output_tokens":53`)
}
