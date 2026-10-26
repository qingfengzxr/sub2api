package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *AccountTestService) testOpenAIEmbeddingsConnection(c *gin.Context, account *Account, model, prompt string) error {
	ctx := c.Request.Context()
	if !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings) {
		return s.sendErrorAndEnd(c, "Embeddings testing requires an API Key account with embeddings enabled")
	}
	credentialAccount := account
	if account.IsCredentialShadow() {
		resolved, err := resolveCredentialAccount(ctx, s.accountRepo, account)
		if err != nil {
			return s.sendErrorAndEnd(c, err.Error())
		}
		credentialAccount = resolved
	}
	apiKey := credentialAccount.GetOpenAIProtocolAPIKey()
	if apiKey == "" {
		return s.sendErrorAndEnd(c, "No API key available")
	}
	baseURL := credentialAccount.GetOpenAIFormatBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	baseURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Invalid base URL: %s", err))
	}
	if strings.TrimSpace(prompt) == "" {
		prompt = "hi"
	}
	payload, _ := json.Marshal(map[string]any{"model": model, "input": prompt, "encoding_format": "float"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildOpenAIEmbeddingsURL(baseURL), bytes.NewReader(payload))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create embeddings request")
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if userAgent := account.GetOpenAIUserAgent(); userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	account.ApplyHeaderOverrides(req.Header)
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()
	s.sendEvent(c, TestEvent{Type: "test_start", Model: model})
	s.sendEvent(c, TestEvent{Type: "status", Text: "正在通过 /v1/embeddings 测试连接"})
	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Embeddings request failed: %s", sanitizeUpstreamErrorMessage(err.Error())))
	}
	defer resp.Body.Close()
	const maxResponseBytes = 2 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return s.sendErrorAndEnd(c, "Failed to read embeddings response or response too large")
	}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			s.reconcileOpenAI429State(ctx, account, resp.Header, body)
		}
		return s.sendErrorAndEnd(c, fmt.Sprintf("Embeddings API (/v1/embeddings) returned %d: %s", resp.StatusCode, sanitizeUpstreamErrorMessage(truncateString(string(body), 2000))))
	}
	var result struct {
		Data []struct {
			Embedding []*float64 `json:"embedding"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &result) != nil || len(result.Data) != 1 || len(result.Data[0].Embedding) == 0 {
		return s.sendErrorAndEnd(c, "Invalid embeddings response: expected a non-empty numeric vector")
	}
	for _, value := range result.Data[0].Embedding {
		if value == nil {
			return s.sendErrorAndEnd(c, "Invalid embeddings response: expected a non-empty numeric vector")
		}
	}
	s.sendEvent(c, TestEvent{Type: "content", Text: fmt.Sprintf("Embedding OK: %d dimensions", len(result.Data[0].Embedding))})
	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}
