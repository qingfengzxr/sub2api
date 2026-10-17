package routes

import (
	"bytes"
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

func TestClaudeCodeAdmissionRegisteredRoutes(t *testing.T) {
	router := newGatewayRoutesTestRouter(service.PlatformAnthropic)
	for _, path := range []string{
		"/v1/messages", "/v1/messages/count_tokens", "/messages/count_tokens",
		"/v1/chat/completions", "/chat/completions", "/v1/responses", "/responses",
		"/responses/compact", "/backend-api/codex/responses", "/backend-api/codex/responses/compact",
		"/v1/systemone",
	} {
		t.Run(path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"claude-sonnet-4-6","max_tokens":1}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("User-Agent", "claude-cli/2.1.301 (external, cli)")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			require.Equal(t, http.StatusForbidden, w.Code)
			require.JSONEq(t, `{"type":"error","error":{"type":"permission_error","message":"Request not permitted"}}`, w.Body.String())
		})
	}
	for _, path := range []string{"/v1/responses", "/responses", "/backend-api/codex/responses"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusForbidden, w.Code)
	}
}

func TestClaudeCodeAdmissionPreservesBodyAndCompositeScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"claude-sonnet-4-6","max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`)
	for _, platform := range []string{service.PlatformAnthropic, service.PlatformOpenAI} {
		for _, validCLI := range []bool{true, false} {
			router := gin.New()
			reached := false
			router.POST("/v1/messages", func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{Platform: service.PlatformComposite}})
				c.Request = c.Request.WithContext(service.WithResolvedTargetPlatform(c.Request.Context(), platform))
			}, claudeCodeAdmissionMiddleware(), func(c *gin.Context) {
				reached = true
				actual, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				require.Equal(t, body, actual)
				require.Equal(t, validCLI, service.HasClaudeCodeAdmission(c.Request.Context()))
				c.Status(http.StatusNoContent)
			})
			r := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
			if validCLI {
				for k, v := range claude.DefaultHeaders() {
					r.Header.Set(k, v)
				}
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("Anthropic-Version", "2023-06-01")
				r.Header.Set("Anthropic-Beta", claude.DefaultBetaHeader)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			wantAllowed := validCLI || platform != service.PlatformAnthropic
			require.Equal(t, wantAllowed, reached)
			if wantAllowed {
				require.Equal(t, http.StatusNoContent, w.Code)
			} else {
				require.Equal(t, http.StatusForbidden, w.Code)
			}
		}
	}
}

func TestClaudeCodeAdmissionBodyLimit(t *testing.T) {
	router := newGatewayRoutesTestRouter(service.PlatformAnthropic)
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(strings.Repeat(" ", 1024*1024+1)))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}
