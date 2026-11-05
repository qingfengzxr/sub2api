package routes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClaudeCodeAdmissionDefaultCompatibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"model":"claude-sonnet-4-6","max_tokens":128,"messages":[{"role":"user","content":"ping"}]}`
	for _, platform := range []string{service.PlatformAnthropic, service.PlatformOpenAI, service.PlatformComposite, ""} {
		t.Run(platform, func(t *testing.T) {
			router := gin.New()
			router.POST("/v1/messages", func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{Platform: platform}})
				if platform == service.PlatformComposite {
					c.Request = c.Request.WithContext(service.WithResolvedTargetPlatform(c.Request.Context(), service.PlatformAnthropic))
				}
			}, claudeCodeAdmissionMiddleware(), func(c *gin.Context) {
				actual, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				require.Equal(t, body, string(actual))
				require.False(t, service.HasClaudeCodeAdmission(c.Request.Context()))
				require.False(t, service.IsClaudeCodeClient(c.Request.Context()))
				c.Status(http.StatusNoContent)
			})
			r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("User-Agent", "ordinary-sdk/1.0.0")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
		})
	}
}

func TestClaudeCodeAdmissionRegisteredCompatibilityAliases(t *testing.T) {
	router := newGatewayRoutesTestRouter(service.PlatformAnthropic)
	for _, path := range []string{
		"/v1/messages", "/v1/messages/count_tokens", "/messages/count_tokens",
		"/v1/chat/completions", "/chat/completions", "/v1/responses", "/responses",
		"/responses/compact", "/backend-api/codex/responses", "/backend-api/codex/responses/compact", "/v1/systemone",
	} {
		t.Run(path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"claude-sonnet-4-6","max_tokens":128,"messages":[{"role":"user","content":"ping"}],"input":"ping"}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("User-Agent", "ordinary-sdk/1.0.0")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			// This registered-chain fixture intentionally has no authenticated
			// user subject. Passing classification must reach that handler check,
			// not authorize a request or call a real upstream.
			require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), "User context not found")
		})
	}
}

func TestClaudeCodeAdmissionRejectsInvalidJSONIndependentlyOfClient(t *testing.T) {
	router := newGatewayRoutesTestRouter(service.PlatformAnthropic)
	for name, body := range map[string]string{
		"malformed": `{`, "null": `null`, "array": `[]`,
		"duplicate":        `{"model":"one","model":"two"}`,
		"nested_duplicate": `{"model":"one","metadata":{"a":1,"a":2}}`,
		"invalid_utf8":     "{\"model\":\"\xff\"}",
	} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("User-Agent", "ordinary-sdk/1.0.0")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}

func TestClaudeCodeAdmissionNewVersionIsNotVerifiedByUAAlone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, version := range []string{"2.1.301", "2.1.304", "2.1.305"} {
		t.Run(version, func(t *testing.T) {
			router := gin.New()
			router.POST("/v1/messages", func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{Platform: service.PlatformAnthropic}})
			}, claudeCodeAdmissionMiddleware(), func(c *gin.Context) {
				require.False(t, service.HasClaudeCodeAdmission(c.Request.Context()), "incomplete profile must use ordinary-client adaptation")
				require.False(t, service.IsClaudeCodeClient(c.Request.Context()))
				c.Status(http.StatusNoContent)
			})
			r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-6","max_tokens":128,"messages":[{"role":"user","content":"ping"}]}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("User-Agent", "claude-cli/"+version+" (external, cli)")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
		})
	}
}

func TestClaudeCodeAdmissionStillPreservesValidatedCLI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"model":"claude-sonnet-4-6","max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`
	router := gin.New()
	router.POST("/v1/messages", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{Platform: service.PlatformAnthropic}})
	}, claudeCodeAdmissionMiddleware(), func(c *gin.Context) {
		require.True(t, service.HasClaudeCodeAdmission(c.Request.Context()))
		actual, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.Equal(t, body, string(actual))
		c.Status(http.StatusNoContent)
	})
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	for k, v := range claude.DefaultHeaders() {
		r.Header.Set(k, v)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Anthropic-Version", "2023-06-01")
	r.Header.Set("Anthropic-Beta", claude.DefaultBetaHeader)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
}
