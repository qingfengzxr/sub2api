package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClaudeCodeIngressClassificationIsAuthoritative(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx := service.WithClaudeCodeAdmissionPolicy(context.Background())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
	c.Request.Header.Set("User-Agent", "claude-cli/2.1.304 (external, cli)")
	// Legacy detection accepts max_tokens=1 from the UA alone. An original
	// ingress classification must not be upgraded by that looser heuristic.
	parsed := &service.ParsedRequest{Model: "claude-sonnet-4-6", MaxTokens: 1}
	SetClaudeCodeClientContext(c, nil, parsed)
	require.False(t, service.IsClaudeCodeClient(c.Request.Context()))
	require.False(t, service.HasClaudeCodeAdmission(c.Request.Context()))
}
