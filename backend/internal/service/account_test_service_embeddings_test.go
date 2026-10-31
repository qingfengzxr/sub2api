//go:build unit

package service

import (
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAccountTestService_EmbeddingsRouting(t *testing.T) {
	for _, tc := range []struct {
		name, model, mapped, prompt, base string
		responses, embeddingsOnly         bool
	}{
		{"responses", "text-embedding-3-small", "", "", "https://api.example.com", true, false},
		{"chat", "text-embedding-3-large", "", "custom input", "https://api.example.com/v1", false, false},
		{"mapped", "text-embedding-3-small", "deployment-1", "", "https://api.example.com/v1/", true, false},
		{"alias", "my-model", "text-embedding-ada-002", "", "https://api.example.com", true, false},
		{"embedding-only", "custom-vector-model", "", "", "https://api.example.com", true, true},
		{"embedding-only-default", "", "", "", "https://api.example.com", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, recorder := newTestContext()
			upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(200, `{"data":[{"embedding":[0.1,-0.2,0.3]}]}`)}}
			svc := &AccountTestService{httpUpstream: upstream, cfg: &config.Config{}}
			account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"api_key": "test-key", "base_url": tc.base},
				Extra:       map[string]any{openai_compat.ExtraKeyResponsesSupported: tc.responses}}
			if tc.mapped != "" {
				account.Credentials["model_mapping"] = map[string]any{tc.model: tc.mapped}
			}
			if tc.embeddingsOnly {
				account.Credentials["openai_capabilities"] = []string{"embeddings"}
			}
			require.NoError(t, svc.testOpenAIAccountConnection(ctx, account, tc.model, tc.prompt, ""))
			require.Len(t, upstream.requests, 1)
			req := upstream.requests[0]
			require.Equal(t, "https://api.example.com/v1/embeddings", req.URL.String())
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, "Bearer test-key", req.Header.Get("Authorization"))
			require.Equal(t, "application/json", req.Header.Get("Accept"))
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			expectedModel := tc.model
			if expectedModel == "" {
				expectedModel = "text-embedding-3-small"
			}
			if tc.mapped != "" {
				expectedModel = tc.mapped
			}
			expectedInput := tc.prompt
			if expectedInput == "" {
				expectedInput = "hi"
			}
			require.Equal(t, expectedModel, gjson.GetBytes(body, "model").String())
			require.Equal(t, expectedInput, gjson.GetBytes(body, "input").String())
			require.Equal(t, "float", gjson.GetBytes(body, "encoding_format").String())
			require.False(t, gjson.GetBytes(body, "messages").Exists())
			require.False(t, gjson.GetBytes(body, "stream").Exists())
			require.Contains(t, recorder.Body.String(), "Embedding OK: 3 dimensions")
			require.Contains(t, recorder.Body.String(), `"success":true`)
		})
	}
}

func TestAccountTestService_EmbeddingsRejectsInvalidResponses(t *testing.T) {
	for _, body := range []string{`{}`, `{"data":[]}`, `{"data":[{"embedding":[]}]}`, `{"data":[{"embedding":[null]}]}`, `{"data":[{"embedding":["bad"]}]}`, `{"choices":[]}`, `not json`} {
		t.Run(body, func(t *testing.T) {
			ctx, recorder := newTestContext()
			upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(200, body)}}
			svc := &AccountTestService{httpUpstream: upstream, cfg: &config.Config{}}
			account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key"}}
			require.ErrorContains(t, svc.testOpenAIAccountConnection(ctx, account, "text-embedding-3-small", "", ""), "Invalid embeddings response")
			require.NotContains(t, recorder.Body.String(), `"success":true`)
		})
	}
}

func TestAccountTestService_EmbeddingsUpstreamError(t *testing.T) {
	ctx, recorder := newTestContext()
	upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(404, `{"error":{"code":"model_not_found"}}`)}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key"}}
	require.ErrorContains(t, svc.testOpenAIAccountConnection(ctx, account, "text-embedding-3-small", "", ""), "/v1/embeddings) returned 404")
	require.Contains(t, recorder.Body.String(), "model_not_found")
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_EmbeddingsRejectsUnsupportedAccount(t *testing.T) {
	for _, account := range []*Account{
		{Platform: PlatformOpenAI, Type: AccountTypeOAuth},
		{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"openai_capabilities": []string{"chat_completions"}}},
	} {
		ctx, _ := newTestContext()
		svc := &AccountTestService{}
		require.ErrorContains(t, svc.testOpenAIAccountConnection(ctx, account, "text-embedding-3-small", "", ""), "embeddings enabled")
	}
}
