package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claudehttp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

// Admission is deliberately separate from compatibility detection: a matching
// prompt or User-Agent alone must never authorize access to Anthropic forwarding.
// These are observable protocol checks, not cryptographic client attestation.
var (
	claudeIngressUA      = regexp.MustCompile(`^claude-cli/([0-9]+\.[0-9]+\.[0-9]+)(?: \([^\r\n()]+\))?$`)
	claudeSDKVersion     = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	claudeRuntimeVersion = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
	claudeDeviceID       = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
	claudeBillingVersion = regexp.MustCompile(`^x-anthropic-billing-header: cc_version=([0-9]+\.[0-9]+\.[0-9]+)(?:\.[a-fA-F0-9]+)?; cc_entrypoint=[a-zA-Z0-9_-]+;`)
)

type claudeCodeAdmissionKey struct{}
type claudeCodeAdmissionPolicyKey struct{}

// The route installs the policy even for other platforms: a later fallback to
// an Anthropic account must not bypass ingress admission. Internal account
// health checks are outside this public gateway policy.
func WithClaudeCodeAdmissionPolicy(ctx context.Context) context.Context {
	return context.WithValue(ctx, claudeCodeAdmissionPolicyKey{}, true)
}

func enforceClaudeCodeAdmission(ctx context.Context, c *gin.Context, account *Account) error {
	policy, _ := ctx.Value(claudeCodeAdmissionPolicyKey{}).(bool)
	if !policy || account == nil || account.Platform != PlatformAnthropic || HasClaudeCodeAdmission(ctx) {
		return nil
	}
	MarkOpsClientBusinessLimited(c, OpsClientBusinessLimitedReasonLocalPolicyDenied)
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"type": "error", "error": gin.H{
		"type": "permission_error", "message": "Request not permitted",
	}})
	return ErrClaudeCodeOnly
}

// WithClaudeCodeAdmission is set only after validating the original ingress body.
func WithClaudeCodeAdmission(ctx context.Context) context.Context {
	return context.WithValue(SetClaudeCodeClient(ctx, true), claudeCodeAdmissionKey{}, true)
}

func HasClaudeCodeAdmission(ctx context.Context) bool {
	admitted, _ := ctx.Value(claudeCodeAdmissionKey{}).(bool)
	return admitted
}

// ValidateIngressJSON rejects duplicate keys, since encoding/json (validation)
// and gjson (forwarding) otherwise disagree about which value takes effect.
func (v *ClaudeCodeValidator) ValidateIngressJSON(r *http.Request, body []byte) bool {
	var envelope map[string]any
	// encoding/json replaces malformed UTF-8; forwarding must not interpret
	// different raw text after the admission decision.
	if !utf8.Valid(body) || json.Unmarshal(body, &envelope) != nil || !claudeUniqueJSONKeys(gjson.ParseBytes(body)) {
		return false
	}
	return v.ValidateIngress(r, envelope)
}

func claudeUniqueJSONKeys(value gjson.Result) bool {
	if !value.IsObject() && !value.IsArray() {
		return true
	}
	seen := make(map[string]bool)
	valid := true
	value.ForEach(func(key, child gjson.Result) bool {
		if value.IsObject() {
			name := key.String()
			if seen[name] {
				valid = false
				return false
			}
			seen[name] = true
		}
		valid = claudeUniqueJSONKeys(child)
		return valid
	})
	return valid
}

// IsClaudeCodeIngressRequest recognizes only registered Messages API endpoints.
// It checks the method and path, not the client's identity or request envelope.
func IsClaudeCodeIngressRequest(r *http.Request) bool {
	if r == nil || r.URL == nil || r.Method != http.MethodPost {
		return false
	}
	switch r.URL.Path {
	case "/v1/messages", "/v1/messages/count_tokens", "/messages/count_tokens",
		"/antigravity/v1/messages", "/antigravity/v1/messages/count_tokens":
		return true
	default:
		return false
	}
}

// ValidateIngress checks the CLI envelope before any prompt injection, identity
// rewriting or protocol conversion. Auxiliary calls may omit the system prompt,
// but still require the complete SDK envelope and a valid messages body.
func (v *ClaudeCodeValidator) ValidateIngress(r *http.Request, body map[string]any) bool {
	if !IsClaudeCodeIngressRequest(r) {
		return false
	}
	countTokens := strings.HasSuffix(r.URL.Path, "/messages/count_tokens")
	// Reject ambiguous duplicate identity headers rather than validating one
	// value and forwarding another. Header names are case insensitive.
	seen := make(map[string]bool)
	for name, values := range r.Header {
		name = strings.ToLower(name)
		if allowedHeaders[name] {
			if seen[name] || len(values) != 1 {
				return false
			}
			seen[name] = true
		}
	}
	ua := claudeIngressUA.FindStringSubmatch(r.Header.Get("User-Agent"))
	if ua == nil || len(r.Header.Get("User-Agent")) > maxFingerprintUserAgentLength ||
		!isAcceptableFingerprintUserAgent(r.Header.Get("User-Agent")) ||
		r.Header.Get("X-App") != "cli" || r.Header.Get("Anthropic-Version") != "2023-06-01" ||
		r.Header.Get("X-Stainless-Lang") != "js" ||
		!claudeSDKVersion.MatchString(r.Header.Get("X-Stainless-Package-Version")) ||
		!claudeRuntimeVersion.MatchString(r.Header.Get("X-Stainless-Runtime-Version")) {
		return false
	}
	switch r.Header.Get("X-Stainless-OS") {
	case "Linux", "MacOS", "Windows":
	default:
		return false
	}
	switch r.Header.Get("X-Stainless-Arch") {
	case "arm64", "x64":
	default:
		return false
	}
	switch r.Header.Get("X-Stainless-Runtime") {
	case "node", "bun":
	default:
		return false
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" || strings.TrimSpace(r.Header.Get("Anthropic-Beta")) == "" {
		return false
	}
	model, _ := body["model"].(string)
	messages, ok := body["messages"].([]any)
	if strings.TrimSpace(model) == "" || !ok || len(messages) == 0 {
		return false
	}
	for _, raw := range messages {
		message, ok := raw.(map[string]any)
		if !ok || (message["role"] != "user" && message["role"] != "assistant") || message["content"] == nil {
			return false
		}
		switch content := message["content"].(type) {
		case string:
			if content == "" {
				return false
			}
		case []any:
			if len(content) == 0 {
				return false
			}
		default:
			return false
		}
	}
	// If supplied, metadata must be well formed even on auxiliary requests.
	metadata, hasMetadata := body["metadata"]
	var identity *ParsedUserID
	if hasMetadata {
		m, ok := metadata.(map[string]any)
		if !ok {
			return false
		}
		userID, _ := m["user_id"].(string)
		identity = ParseMetadataUserID(userID)
		if identity == nil || !claudeDeviceID.MatchString(identity.DeviceID) || !isClaudeUUID(identity.SessionID) ||
			(identity.AccountUUID != "" && !isClaudeUUID(identity.AccountUUID)) {
			return false
		}
	}
	if session := r.Header.Get("X-Claude-Code-Session-Id"); session != "" && (identity == nil || session != identity.SessionID) {
		return false
	}
	if id := r.Header.Get("X-Client-Request-Id"); id != "" && !isClaudeUUID(id) {
		return false
	}
	entries, ok := body["system"].([]any)
	if body["system"] != nil && !ok {
		return false
	}
	matched := isClaudeCodeSecurityMonitorPrompt(entries)
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok || entry["type"] != "text" {
			return false
		}
		text, _ := entry["text"].(string)
		if strings.HasPrefix(text, claudeCodeBillingHeaderPrefix) {
			billing := claudeBillingVersion.FindStringSubmatch(text)
			if billing == nil || billing[1] != ua[1] {
				return false
			}
			matched = true
		}
		for _, template := range claudeCodeSystemPrompts {
			if strings.HasPrefix(text, template) {
				matched = true
			}
		}
	}
	if countTokens {
		return true
	}
	maxTokens, ok := body["max_tokens"].(float64)
	if !ok || maxTokens <= 0 || math.Trunc(maxTokens) != maxTokens {
		return false
	}
	// A connectivity probe has one user message and no tools/system prompt.
	// max_tokens=1 on an arbitrary conversation is not a validation bypass.
	firstMessage, ok := messages[0].(map[string]any)
	if isMaxTokensOneBody(body) && len(messages) == 1 && ok && firstMessage["role"] == "user" &&
		claudeEmptyArray(body["system"]) && claudeEmptyArray(body["tools"]) {
		return true
	}
	if identity == nil {
		return false
	}
	hasCLIBeta := false
	for _, beta := range strings.Split(r.Header.Get("Anthropic-Beta"), ",") {
		if strings.TrimSpace(beta) == claude.BetaClaudeCode {
			hasCLIBeta = true
		}
	}
	if !hasCLIBeta {
		return false
	}

	// Claude Code 2.1.304+ changed the system prompt payload while retaining
	// the authenticated CLI envelope. Keep the legacy prompt match for older
	// versions, but accept newer official CLI versions after all structural,
	// identity, billing/session and beta checks above have passed.
	if matched || isNewerClaudeCLI(ua[1], "2.1.304") {
		return true
	}
	return false
}

func isNewerClaudeCLI(version, minimum string) bool {
	var a, b, c, x, y, z int
	if _, err := fmt.Sscanf(version, "%d.%d.%d", &a, &b, &c); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(minimum, "%d.%d.%d", &x, &y, &z); err != nil {
		return false
	}
	return a > x || (a == x && (b > y || (b == y && c >= z)))
}

func isClaudeUUID(value string) bool {
	_, err := uuid.Parse(value)
	return len(value) == 36 && err == nil
}

func claudeEmptyArray(value any) bool {
	if value == nil {
		return true
	}
	items, ok := value.([]any)
	return ok && len(items) == 0
}

// Select the measured wire profile only after admission and final header overrides.
func withAdmittedClaudeHTTP1Profile(req *http.Request) *http.Request {
	if HasClaudeCodeAdmission(req.Context()) {
		return claudehttp.WithProfile(req)
	}
	return req
}
