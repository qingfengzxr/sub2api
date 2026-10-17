package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClaudeCodeAdmissionRegisteredAntigravityFallback(t *testing.T) {
	for _, route := range []struct{ path, group string }{
		{"/v1/messages", service.PlatformAntigravity},
		{"/v1/messages/count_tokens", service.PlatformAntigravity},
		{"/antigravity/v1/messages", service.PlatformAntigravity},
		{"/antigravity/v1/messages/count_tokens", service.PlatformAntigravity},
		{"/antigravity/v1/messages", service.PlatformAnthropic},
		{"/antigravity/v1/messages/count_tokens", service.PlatformAnthropic},
	} {
		for _, kind := range []string{"cli", "non_cli", "duplicate_json"} {
			t.Run(route.path+"/"+route.group+"/"+kind, func(t *testing.T) {
				// Observe the actual registered middleware chain. The empty handler
				// stops at its missing auth subject, before any service dependencies.
				var ingress *http.Request
				router := newGatewayRoutesTestRouterWithGroup(&service.Group{Platform: route.group}, func(c *gin.Context) {
					c.Next()
					ingress = c.Request
				})
				body := `{"model":"claude-sonnet-4-6","max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`
				if kind == "duplicate_json" {
					body = `{"model":"unexpected",` + body[1:]
				}
				r := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(body))
				r.Header.Set("Content-Type", "application/json")
				if kind != "non_cli" {
					for k, v := range claude.DefaultHeaders() {
						r.Header.Set(k, v)
					}
					r.Header.Set("Anthropic-Version", "2023-06-01")
					r.Header.Set("Anthropic-Beta", claude.DefaultBetaHeader)
				}
				w := httptest.NewRecorder()
				router.ServeHTTP(w, r)
				require.NotNil(t, ingress)
				// Ordinary Antigravity traffic remains allowed, including when
				// the forced route is used with an Anthropic-group key.
				require.NotEqual(t, http.StatusForbidden, w.Code)
				require.Equal(t, kind == "cli", service.HasClaudeCodeAdmission(ingress.Context()))

				// The handler's platform fallback clears ForcePlatform but retains
				// ingress context. Exercise the real forwarding guard with that state.
				ctx := context.WithValue(ingress.Context(), ctxkey.ForcePlatform, "")
				fallbackWriter := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(fallbackWriter)
				c.Request = ingress.WithContext(ctx)
				svc := &service.GatewayService{}
				account := &service.Account{Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth}
				var err error
				if strings.HasSuffix(route.path, "/count_tokens") {
					err = svc.ForwardCountTokens(ctx, c, account, nil)
				} else {
					_, err = svc.Forward(ctx, c, account, nil)
				}
				if kind == "cli" {
					require.ErrorContains(t, err, "empty request") // reached parsing, past admission
					require.NotEqual(t, http.StatusForbidden, fallbackWriter.Code)
				} else {
					require.ErrorIs(t, err, service.ErrClaudeCodeOnly)
					require.Equal(t, http.StatusForbidden, fallbackWriter.Code)
				}
			})
		}
	}
}
