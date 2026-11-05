package routes

import (
	"errors"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Classify original CLI profiles before protocol conversion. Classification is
// not authorization: ordinary clients use the existing server-side adapters,
// while optional claude_code_only restrictions remain owned by group scheduling.
func claudeCodeAdmissionMiddleware() gin.HandlerFunc {
	validator := service.NewClaudeCodeValidator()
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(service.WithClaudeCodeAdmissionPolicy(c.Request.Context()))
		platform := getGroupPlatform(c)
		if forced, _ := c.Request.Context().Value(ctxkey.ForcePlatform).(string); forced != "" {
			platform = forced
		}
		if c.Request.Method == http.MethodGet {
			c.Next()
			return
		}
		// Do not interpret other platforms' multipart/binary endpoints as JSON.
		// Messages requests may later fall back to Anthropic, so validate their
		// original JSON and classify their profile regardless of initial platform.
		if platform != service.PlatformAnthropic && platform != "" && !service.IsClaudeCodeIngressRequest(c.Request) {
			c.Next()
			return
		}
		reject := func(status int) {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalPolicyDenied)
			c.AbortWithStatusJSON(status, gin.H{"type": "error", "error": gin.H{
				"type": "invalid_request_error", "message": "Invalid request body",
			}})
		}
		body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				reject(http.StatusRequestEntityTooLarge)
			} else {
				reject(http.StatusBadRequest)
			}
			return
		}
		requestmodel.ResetRequestBody(c.Request, body)
		if !service.ValidateGatewayJSONBody(body) {
			reject(http.StatusBadRequest)
			return
		}
		if validator.ValidateIngressJSON(c.Request, body) {
			ctx := service.WithClaudeCodeAdmission(c.Request.Context())
			ctx = service.SetClaudeCodeVersion(ctx, validator.ExtractVersion(c.GetHeader("User-Agent")))
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}
}
