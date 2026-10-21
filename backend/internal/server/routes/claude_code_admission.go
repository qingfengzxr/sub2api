package routes

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Anthropic inference is CLI-only, including composite routes and compatibility
// aliases. Local discovery/usage GETs do not expose an upstream request identity.
func claudeCodeAdmissionMiddleware() gin.HandlerFunc {
	validator := service.NewClaudeCodeValidator()
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(service.WithClaudeCodeAdmissionPolicy(c.Request.Context()))
		platform := getGroupPlatform(c)
		if forced, _ := c.Request.Context().Value(ctxkey.ForcePlatform).(string); forced != "" {
			platform = forced
		}
		// An ungrouped key, when allowed by settings, defaults to Anthropic.
		requireAdmission := platform == service.PlatformAnthropic || platform == ""
		if c.Request.Method == http.MethodGet && !strings.Contains(c.Request.URL.Path, "/responses") {
			c.Next()
			return
		}
		// Other platforms keep accepting their normal clients. For a possible
		// Anthropic fallback, validate CLI envelopes now, before any conversion,
		// and carry the result through the original request context.
		if !requireAdmission && (!service.IsClaudeCodeIngressRequest(c.Request) ||
			!validator.ValidateUserAgent(c.GetHeader("User-Agent"))) {
			c.Next()
			return
		}
		reject := func(status int) {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalPolicyDenied)
			c.AbortWithStatusJSON(status, gin.H{"type": "error", "error": gin.H{
				"type": "permission_error", "message": "Request not permitted",
			}})
		}
		body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				reject(http.StatusRequestEntityTooLarge)
			} else if !requireAdmission {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"type": "error", "error": gin.H{
					"type": "invalid_request_error", "message": "Failed to read request body",
				}})
			} else {
				reject(http.StatusForbidden)
			}
			return
		}
		requestmodel.ResetRequestBody(c.Request, body)
		if !validator.ValidateIngressJSON(c.Request, body) {
			// Claude Code 2.1.304+ changed its bootstrap envelope. Keep the
			// request on the CLI-only route and let the gateway perform the
			// remaining body/account checks instead of returning a false 403.
			if service.IsClaudeCodeIngressRequest(c.Request) &&
				validator.IsNewerOrEqualVersion(c.GetHeader("User-Agent"), "2.1.304") {
				ctx := service.WithClaudeCodeAdmission(c.Request.Context())
				ctx = service.SetClaudeCodeVersion(ctx, validator.ExtractVersion(c.GetHeader("User-Agent")))
				c.Request = c.Request.WithContext(ctx)
				c.Next()
				return
			}
			if requireAdmission {
				reject(http.StatusForbidden)
			} else {
				c.Next()
			}
			return
		}
		ctx := service.WithClaudeCodeAdmission(c.Request.Context())
		ctx = service.SetClaudeCodeVersion(ctx, validator.ExtractVersion(c.GetHeader("User-Agent")))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
