package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type failingBillingSettingsRepo struct{ SettingRepository }

func (failingBillingSettingsRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, errors.New("settings unavailable")
}

func TestResponseBillingPolicyFailureStopsBeforeUpstream(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	svc := &OpenAIGatewayService{settingService: &SettingService{settingRepo: failingBillingSettingsRepo{}, cfg: &config.Config{}}}
	result, err := svc.Forward(context.Background(), c, nil, []byte(`{"model":"test"}`))
	require.ErrorContains(t, err, "settings unavailable")
	require.Nil(t, result)
	_, ok := BillingTokenPolicyFromContext(c.Request.Context())
	require.False(t, ok)
}

func TestClaudeConvertedResponseBillingPreservesRaw(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, force := range []bool{false, true} {
		t.Run(fmt.Sprintf("force=%t", force), func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx := WithBillingTokenPolicy(context.Background(), BillingTokenPolicy{Enabled: true, Multiplier: 2.5})
			if force {
				ctx = WithForceCacheBilling(ctx)
			}
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
			raw := ClaudeUsage{InputTokens: 80, OutputTokens: 21, CacheReadInputTokens: 20, CacheCreationInputTokens: 10}
			chat := &apicompat.ChatUsage{PromptTokens: 110, CompletionTokens: 21, TotalTokens: 131, PromptTokensDetails: &apicompat.ChatTokenDetails{CachedTokens: 20}}
			response := &apicompat.ResponsesUsage{InputTokens: 110, OutputTokens: 21, TotalTokens: 131, InputTokensDetails: &apicompat.ResponsesInputTokensDetails{CachedTokens: 20}}
			event := apicompat.ResponsesStreamEvent{Response: &apicompat.ResponsesResponse{Usage: response}}
			gotChat := billableClaudeChatUsage(c, chat, &raw)
			gotEvent := billableClaudeResponsesEvent(c, event, &raw)
			wantInput, wantCached := 230, 20
			if force {
				wantInput, wantCached = 110, 100
			}
			require.Equal(t, wantInput, gotChat.PromptTokens)
			require.Equal(t, wantCached, gotChat.PromptTokensDetails.CachedTokens)
			require.Equal(t, 53, gotChat.CompletionTokens)
			require.Equal(t, wantInput, gotEvent.Response.Usage.InputTokens)
			require.Equal(t, wantCached, gotEvent.Response.Usage.InputTokensDetails.CachedTokens)
			require.Equal(t, wantInput+53, gotEvent.Response.Usage.TotalTokens)
			require.Equal(t, 110, chat.PromptTokens)
			require.Equal(t, 20, chat.PromptTokensDetails.CachedTokens)
			require.Equal(t, 110, event.Response.Usage.InputTokens)
			require.Equal(t, 20, response.InputTokensDetails.CachedTokens)
			require.Equal(t, 80, raw.InputTokens)
		})
	}
}

func TestOpenAIChatBillableUsageDoesNotMutateStreamState(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx := WithBillingTokenPolicy(context.Background(), BillingTokenPolicy{Enabled: true, Multiplier: 2.5})
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
	original := &apicompat.ChatUsage{PromptTokens: 100, CompletionTokens: 21, CompletionTokensDetails: &apicompat.ChatTokenDetails{ReasoningTokens: 4}}
	raw := OpenAIUsage{InputTokens: 100, OutputTokens: 21, CacheReadInputTokens: 20}
	got := applyOpenAIChatBillableUsage(c, original, raw)
	require.Equal(t, 220, got.PromptTokens)
	require.Equal(t, 53, got.CompletionTokens)
	require.Equal(t, 10, got.CompletionTokensDetails.ReasoningTokens)
	require.Equal(t, 100, original.PromptTokens)
	require.Equal(t, 21, original.CompletionTokens)
	require.Equal(t, 4, original.CompletionTokensDetails.ReasoningTokens)
}

func TestAntigravityBillableUsageKeepsRaw(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, route := range []string{"gemini", "claude", "chat", "responses"} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", route, stream), func(t *testing.T) {
				svc := newAntigravityTestService(&config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}})
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				ctx := WithBillingTokenPolicy(context.Background(), BillingTokenPolicy{Enabled: true, Multiplier: 2.5})
				c.Request = httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
				body := `data: {"response":{"responseId":"r","candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":3,"totalTokenCount":11}}}` + "\n\n"
				resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
				var result *antigravityStreamResult
				var err error
				switch route {
				case "gemini":
					if stream {
						result, err = svc.handleGeminiStreamingResponse(c, resp, time.Now())
					} else {
						result, err = svc.handleGeminiStreamToNonStreaming(c, resp, time.Now())
					}
				case "claude":
					if stream {
						result, err = svc.handleClaudeStreamingResponse(c, resp, time.Now(), "model")
					} else {
						result, err = svc.handleClaudeStreamToNonStreaming(c, resp, time.Now(), "model")
					}
				case "chat":
					if stream {
						result, err = svc.handleChatCompletionsStreamingFromAntigravity(c, resp, time.Now(), "model", true)
					} else {
						result, err = svc.handleChatCompletionsNonStreamingFromAntigravity(c, resp, time.Now(), "model")
					}
				case "responses":
					if stream {
						result, err = svc.handleResponsesStreamingFromAntigravity(c, resp, time.Now(), "model")
					} else {
						result, err = svc.handleResponsesNonStreamingFromAntigravity(c, resp, time.Now(), "model")
					}
				}
				require.NoError(t, err)
				require.Equal(t, 8, result.usage.InputTokens)
				require.Equal(t, 3, result.usage.OutputTokens)
				switch route {
				case "gemini":
					require.Contains(t, rec.Body.String(), `"promptTokenCount":20`)
					require.Contains(t, rec.Body.String(), `"candidatesTokenCount":8`)
				case "chat":
					require.Contains(t, rec.Body.String(), `"prompt_tokens":20`)
					require.Contains(t, rec.Body.String(), `"completion_tokens":8`)
				default:
					require.Contains(t, rec.Body.String(), `"input_tokens":20`)
					require.Contains(t, rec.Body.String(), `"output_tokens":8`)
				}
			})
		}
	}
}

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

func TestGeminiNativeBillableUsageKeepsRaw(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":100,"cachedContentTokenCount":20,"candidatesTokenCount":3,"thoughtsTokenCount":2,"totalTokenCount":105,"candidatesTokensDetails":[{"modality":"TEXT","tokenCount":3}]}}`
	for _, stream := range []bool{false, true} {
		for _, forceCache := range []bool{false, true} {
			t.Run(fmt.Sprintf("stream=%t/force=%t", stream, forceCache), func(t *testing.T) {
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				ctx := WithBillingTokenPolicy(context.Background(), BillingTokenPolicy{Enabled: true, Multiplier: 2.5})
				if forceCache {
					ctx = WithForceCacheBilling(ctx)
				}
				c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/m:generateContent", nil).WithContext(ctx)
				responseBody := body
				if stream {
					responseBody = "data: " + body + "\n\n"
				}
				resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(responseBody))}
				svc := &GeminiMessagesCompatService{cfg: &config.Config{}}
				var usage *ClaudeUsage
				if stream {
					result, err := svc.handleNativeStreamingResponse(c, resp, time.Now(), false, &Account{ID: 1}, "")
					require.NoError(t, err)
					usage = result.usage
				} else {
					var err error
					usage, err = svc.handleNativeNonStreamingResponse(c, resp, false, &Account{ID: 1}, "")
					require.NoError(t, err)
				}
				require.Equal(t, 80, usage.InputTokens)
				require.Equal(t, 20, usage.CacheReadInputTokens)
				require.Equal(t, 5, usage.OutputTokens)
				wantPrompt, wantCached := 220, 20
				if forceCache {
					wantPrompt, wantCached = 100, 100
				}
				require.Contains(t, rec.Body.String(), fmt.Sprintf(`"promptTokenCount":%d`, wantPrompt))
				require.Contains(t, rec.Body.String(), fmt.Sprintf(`"cachedContentTokenCount":%d`, wantCached))
				require.Contains(t, rec.Body.String(), `"candidatesTokenCount":8`)
				require.Contains(t, rec.Body.String(), `"thoughtsTokenCount":5`)
				require.Contains(t, rec.Body.String(), fmt.Sprintf(`"totalTokenCount":%d`, wantPrompt+13))
			})
		}
	}
	unchanged, err := billableGeminiResponse([]byte(body), BillingTokenPolicy{Multiplier: 2.5}, false)
	require.NoError(t, err)
	require.Equal(t, body, string(unchanged))
}

func TestGeminiPromptModalityDetailsRespectCachedTokens(t *testing.T) {
	body := []byte(`{"usageMetadata":{"promptTokenCount":100,"cachedContentTokenCount":20,"candidatesTokenCount":2,"totalTokenCount":102,"promptTokensDetails":[{"modality":"TEXT","tokenCount":60},{"modality":"IMAGE","tokenCount":40}],"cacheTokensDetails":[{"modality":"TEXT","tokenCount":10},{"modality":"IMAGE","tokenCount":10}]}}`)
	policy := BillingTokenPolicy{Enabled: true, Multiplier: 2.5}
	updated, err := billableGeminiResponse(body, policy, false)
	require.NoError(t, err)
	require.EqualValues(t, 220, gjson.GetBytes(updated, "usageMetadata.promptTokenCount").Int())
	require.EqualValues(t, 135, gjson.GetBytes(updated, "usageMetadata.promptTokensDetails.0.tokenCount").Int())
	require.EqualValues(t, 85, gjson.GetBytes(updated, "usageMetadata.promptTokensDetails.1.tokenCount").Int())
	forced, err := billableGeminiResponse(body, policy, true)
	require.NoError(t, err)
	require.EqualValues(t, 60, gjson.GetBytes(forced, "usageMetadata.promptTokensDetails.0.tokenCount").Int())
	require.EqualValues(t, 40, gjson.GetBytes(forced, "usageMetadata.promptTokensDetails.1.tokenCount").Int())
	require.EqualValues(t, 60, gjson.GetBytes(forced, "usageMetadata.cacheTokensDetails.0.tokenCount").Int())
	require.EqualValues(t, 40, gjson.GetBytes(forced, "usageMetadata.cacheTokensDetails.1.tokenCount").Int())
}

func TestGeminiCompatibleBillableUsageKeepsRaw(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":100,"cachedContentTokenCount":20,"candidatesTokenCount":3,"thoughtsTokenCount":2,"totalTokenCount":105}}`
	for _, format := range []string{"claude", "chat"} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", format, stream), func(t *testing.T) {
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				ctx := WithBillingTokenPolicy(context.Background(), BillingTokenPolicy{Enabled: true, Multiplier: 2.5})
				c.Request = httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
				responseBody := body
				if stream {
					responseBody = "data: " + body + "\n\n"
				}
				resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(responseBody))}
				svc := &GeminiMessagesCompatService{cfg: &config.Config{}}
				var usage *ClaudeUsage
				if format == "claude" && stream {
					result, err := svc.handleStreamingResponse(c, resp, time.Now(), "model")
					require.NoError(t, err)
					usage = result.usage
				} else if format == "claude" {
					var err error
					usage, err = svc.handleNonStreamingResponse(c, resp, "model")
					require.NoError(t, err)
				} else if stream {
					result, err := svc.handleChatCompletionsStreamingResponseFromGemini(c, resp, time.Now(), "model", false, true)
					require.NoError(t, err)
					usage = result.usage
				} else {
					var err error
					usage, err = svc.handleChatCompletionsNonStreamingResponseFromGemini(c, resp, "model", false)
					require.NoError(t, err)
				}
				require.Equal(t, 80, usage.InputTokens)
				require.Equal(t, 20, usage.CacheReadInputTokens)
				require.Equal(t, 5, usage.OutputTokens)
				if format == "claude" {
					require.Contains(t, rec.Body.String(), `"input_tokens":200`)
					require.Contains(t, rec.Body.String(), `"output_tokens":13`)
					require.Contains(t, rec.Body.String(), `"cache_read_input_tokens":20`)
				} else {
					require.Contains(t, rec.Body.String(), `"prompt_tokens":220`)
					require.Contains(t, rec.Body.String(), `"completion_tokens":13`)
					require.Contains(t, rec.Body.String(), `"cached_tokens":20`)
				}
			})
		}
	}
}

func TestRawChatBillableUsageIncludesBridgeOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			ctx := WithBillingTokenPolicy(context.Background(), BillingTokenPolicy{Enabled: true, Multiplier: 2.5})
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)
			body := `{"id":"r","choices":[{"index":0,"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13,"prompt_tokens_details":{"cached_tokens":2}}}`
			if stream {
				body = `data: {"id":"r","choices":[{"index":0,"delta":{"content":"ok"}}]}` + "\n\n" + `data: {"id":"r","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13,"prompt_tokens_details":{"cached_tokens":2}}}` + "\n\ndata: [DONE]\n\n"
			}
			resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
			svc := &OpenAIGatewayService{cfg: &config.Config{}}
			bridge := OpenAIUsage{InputTokens: 3, OutputTokens: 1, CacheReadInputTokens: 1}
			var result *OpenAIForwardResult
			var err error
			if stream {
				result, err = svc.streamRawChatCompletions(c, resp, &Account{ID: 1}, "model", "model", "model", nil, nil, time.Now(), 100, bridge)
			} else {
				result, err = svc.bufferRawChatCompletions(c, resp, &Account{ID: 1}, "model", "model", "model", nil, nil, time.Now(), bridge)
			}
			require.NoError(t, err)
			require.Equal(t, 10, result.Usage.InputTokens)
			require.Equal(t, 3, result.Usage.OutputTokens)
			require.Contains(t, rec.Body.String(), `"prompt_tokens":28`)
			require.Contains(t, rec.Body.String(), `"completion_tokens":10`)
			require.Contains(t, rec.Body.String(), `"total_tokens":38`)
			require.Contains(t, rec.Body.String(), `"cached_tokens":3`)
		})
	}
}

func TestResponsesToChatBillableUsageKeepsRaw(t *testing.T) {
	gin.SetMode(gin.TestMode)
	event := `data: {"type":"response.completed","response":{"id":"r","object":"response","status":"completed","model":"model","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":100,"output_tokens":21,"total_tokens":121,"input_tokens_details":{"cached_tokens":20},"output_tokens_details":{"reasoning_tokens":4}}}}` + "\n\n"
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			ctx := WithBillingTokenPolicy(context.Background(), BillingTokenPolicy{Enabled: true, Multiplier: 2.5})
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)
			resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(event))}
			svc := &OpenAIGatewayService{cfg: &config.Config{}}
			var result *OpenAIForwardResult
			var err error
			if stream {
				result, err = svc.handleChatStreamingResponse(resp, c, &Account{ID: 1}, "model", "model", "model", time.Now(), 100)
			} else {
				result, err = svc.handleChatBufferedStreamingResponse(resp, c, &Account{ID: 1}, "model", "model", "model", time.Now())
			}
			require.NoError(t, err)
			require.Equal(t, 100, result.Usage.InputTokens)
			require.Equal(t, 21, result.Usage.OutputTokens)
			require.Contains(t, rec.Body.String(), `"prompt_tokens":220`)
			require.Contains(t, rec.Body.String(), `"completion_tokens":53`)
			require.Contains(t, rec.Body.String(), `"total_tokens":273`)
		})
	}
}

func TestResponsesToAnthropicBillableUsageKeepsRaw(t *testing.T) {
	gin.SetMode(gin.TestMode)
	event := `data: {"type":"response.completed","response":{"id":"r","object":"response","status":"completed","model":"model","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":100,"output_tokens":21,"total_tokens":121,"input_tokens_details":{"cached_tokens":20}}}}` + "\n\n"
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			ctx := WithBillingTokenPolicy(context.Background(), BillingTokenPolicy{Enabled: true, Multiplier: 2.5})
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
			resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(event))}
			svc := &OpenAIGatewayService{cfg: &config.Config{}}
			var result *OpenAIForwardResult
			var err error
			if stream {
				result, err = svc.handleAnthropicStreamingResponse(resp, c, &Account{ID: 1}, "model", "model", "model", time.Now())
			} else {
				result, err = svc.handleAnthropicBufferedStreamingResponse(resp, c, &Account{ID: 1}, "model", "model", "model", time.Now())
			}
			require.NoError(t, err)
			require.Equal(t, 100, result.Usage.InputTokens)
			require.Equal(t, 21, result.Usage.OutputTokens)
			require.Contains(t, rec.Body.String(), `"input_tokens":200`)
			require.Contains(t, rec.Body.String(), `"output_tokens":53`)
			require.Contains(t, rec.Body.String(), `"cache_read_input_tokens":20`)
		})
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
