package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claudehttp"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Sanitized envelope observed using Claude Code 2.1.301 against a loopback
// server. No real authentication, machine identity or user content is retained.
func admittedClaudeFixture(t *testing.T) (*http.Request, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/v1/messages?beta=true", nil)
	for k, v := range map[string]string{
		"User-Agent": "claude-cli/2.1.301 (external, sdk-cli)",
		"X-App":      "cli", "Anthropic-Version": "2023-06-01",
		"Anthropic-Beta": "claude-code-20250219,interleaved-thinking-2025-05-14",
		"Content-Type":   "application/json", "Accept": "application/json",
		"X-Stainless-Lang": "js", "X-Stainless-Package-Version": "0.128.0",
		"X-Stainless-OS": "MacOS", "X-Stainless-Arch": "arm64",
		"X-Stainless-Runtime": "node", "X-Stainless-Runtime-Version": "v26.3.0",
		"Anthropic-Dangerous-Direct-Browser-Access": "true",
	} {
		r.Header.Set(k, v)
	}
	body := map[string]any{
		"model": "claude-sonnet-4-6", "max_tokens": float64(4096), "stream": true,
		"messages": []any{map[string]any{"role": "user", "content": "Reply OK"}},
		"system": []any{
			map[string]any{"type": "text", "text": "x-anthropic-billing-header: cc_version=2.1.301.a3d; cc_entrypoint=sdk-cli;"},
			map[string]any{"type": "text", "text": "You are a Claude agent, built on Anthropic's Claude Agent SDK."},
		},
		"metadata": map[string]any{"user_id": claudeCodeMetadataUserIDJSON},
	}
	return r, body
}

func claudeFixtureFirstBlock(t *testing.T, body map[string]any, key string) map[string]any {
	t.Helper()
	items, ok := body[key].([]any)
	require.True(t, ok)
	require.NotEmpty(t, items)
	block, ok := items[0].(map[string]any)
	require.True(t, ok)
	return block
}

func TestClaudeCodeAdmission(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*http.Request, map[string]any)
		want bool
	}{
		{"observed_cli", func(*http.Request, map[string]any) {}, true},
		{"ua_only", func(r *http.Request, _ map[string]any) { r.Header = http.Header{"User-Agent": {r.UserAgent()}} }, false},
		{"version_suffix", func(r *http.Request, _ map[string]any) { r.Header.Set("User-Agent", "claude-cli/2.1.301-evil") }, false},
		{"wrong_app", func(r *http.Request, _ map[string]any) { r.Header.Set("X-App", "anything") }, false},
		{"duplicate_ua", func(r *http.Request, _ map[string]any) { r.Header.Add("User-Agent", "curl/8.0") }, false},
		{"runtime_missing", func(r *http.Request, _ map[string]any) { r.Header.Del("X-Stainless-Runtime") }, false},
		{"wrong_beta", func(r *http.Request, _ map[string]any) { r.Header.Set("Anthropic-Beta", "not-claude-code-20250219") }, false},
		{"no_messages", func(_ *http.Request, b map[string]any) { delete(b, "messages") }, false},
		{"bad_metadata", func(_ *http.Request, b map[string]any) {
			b["metadata"] = map[string]any{"user_id": `{"device_id":"x","session_id":"y"}`}
		}, false},
		{"wrong_session", func(r *http.Request, _ map[string]any) {
			r.Header.Set("X-Claude-Code-Session-Id", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
		}, false},
		{"billing_mismatch", func(_ *http.Request, b map[string]any) {
			claudeFixtureFirstBlock(t, b, "system")["text"] = "x-anthropic-billing-header: cc_version=2.1.300.abc; cc_entrypoint=cli;"
		}, false},
		{"no_prompt", func(_ *http.Request, b map[string]any) { delete(b, "system") }, false},
		{"unknown_endpoint", func(r *http.Request, _ map[string]any) { r.URL.Path = "/v1/chat/completions" }, false},
		{"method", func(r *http.Request, _ map[string]any) { r.Method = http.MethodGet }, false},
		{"probe", func(_ *http.Request, b map[string]any) {
			b["max_tokens"] = float64(1)
			delete(b, "system")
			delete(b, "metadata")
		}, true},
		{"probe_with_tools", func(_ *http.Request, b map[string]any) {
			b["max_tokens"] = float64(1)
			delete(b, "system")
			b["tools"] = []any{map[string]any{"name": "Bash"}}
		}, false},
		{"count_tokens", func(r *http.Request, b map[string]any) {
			r.URL.Path = "/v1/messages/count_tokens"
			delete(b, "system")
			delete(b, "metadata")
			delete(b, "max_tokens")
		}, true},
		{"count_tokens_ua_only", func(r *http.Request, _ map[string]any) {
			r.URL.Path = "/messages/count_tokens"
			r.Header = http.Header{"User-Agent": {r.UserAgent()}}
		}, false},
		{"count_tokens_billing_mismatch", func(r *http.Request, b map[string]any) {
			r.URL.Path = "/v1/messages/count_tokens"
			claudeFixtureFirstBlock(t, b, "system")["text"] = "x-anthropic-billing-header: cc_version=2.1.300.abc; cc_entrypoint=cli;"
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, body := admittedClaudeFixture(t)
			tc.edit(r, body)
			require.Equal(t, tc.want, NewClaudeCodeValidator().ValidateIngress(r, body))
		})
	}
}

func TestClaudeCodeAdmissionRejectsDuplicateJSONKeys(t *testing.T) {
	r, envelope := admittedClaudeFixture(t)
	body, err := json.Marshal(envelope)
	require.NoError(t, err)
	v := NewClaudeCodeValidator()
	require.True(t, v.ValidateIngressJSON(r, body))
	for _, ambiguous := range []string{
		`{"model":"unexpected",` + string(body[1:]),
		strings.Replace(string(body), `"role":"user"`, `"role":"assistant","role":"user"`, 1),
		strings.Replace(string(body), `"max_tokens":4096`, `"max_tokens":1,"max_tokens":4096`, 1),
	} {
		require.False(t, v.ValidateIngressJSON(r, []byte(ambiguous)))
	}
}

func TestClaudeCodeAdmissionRejectsMalformedEncoding(t *testing.T) {
	r, envelope := admittedClaudeFixture(t)
	body, err := json.Marshal(envelope)
	require.NoError(t, err)
	for _, malformed := range [][]byte{
		[]byte("null"), []byte("[]"), []byte("{"),
		append(append([]byte{}, body...), []byte(` {"extra":true}`)...),
		[]byte(strings.Replace(string(body), "Reply OK", "Reply \xff", 1)),
		[]byte(strings.Replace(string(body), "Reply OK", "Reply \xc0\xaf", 1)),
		[]byte(strings.Replace(string(body), "Reply OK", "Reply \xed\xa0\x80", 1)),
	} {
		require.False(t, NewClaudeCodeValidator().ValidateIngressJSON(r, malformed), "malformed JSON/UTF-8 must not be admitted")
	}
}

func TestClaudeCodeAdmissionHeaderMutations(t *testing.T) {
	for _, name := range []string{
		"User-Agent", "X-App", "Anthropic-Version", "Anthropic-Beta", "Content-Type",
		"X-Stainless-Lang", "X-Stainless-Package-Version", "X-Stainless-OS",
		"X-Stainless-Arch", "X-Stainless-Runtime", "X-Stainless-Runtime-Version",
	} {
		for _, mutation := range []string{"missing", "invalid", "duplicate"} {
			t.Run(name+"/"+mutation, func(t *testing.T) {
				r, body := admittedClaudeFixture(t)
				switch mutation {
				case "missing":
					r.Header.Del(name)
				case "invalid":
					r.Header.Set(name, "not-a-cli-value")
				case "duplicate":
					r.Header.Add(name, r.Header.Get(name))
				}
				require.False(t, NewClaudeCodeValidator().ValidateIngress(r, body))
			})
		}
	}
}

func TestAdmittedClaudeBillingPreservesInjectedReminders(t *testing.T) {
	// The normal CLI captured fc8 from the original prompt, then prepended
	// reminder blocks to the API message. Re-hashing that message gives 70e.
	for _, countTokens := range []bool{false, true} {
		r, envelope := admittedClaudeFixture(t)
		claudeFixtureFirstBlock(t, envelope, "messages")["content"] = []any{
			map[string]any{"type": "text", "text": "<system-reminder>Environment</system-reminder>"},
			map[string]any{"type": "text", "text": "0123456789ABCDEFGHIJKLMNO"},
		}
		billing := "x-anthropic-billing-header: cc_version=2.1.301.fc8; cc_entrypoint=sdk-cli;"
		claudeFixtureFirstBlock(t, envelope, "system")["text"] = billing
		body, err := json.Marshal(envelope)
		require.NoError(t, err)
		require.True(t, NewClaudeCodeValidator().ValidateIngressJSON(r, body))
		ctx := WithClaudeCodeAdmission(context.Background())
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = r.WithContext(ctx)
		svc := &GatewayService{cfg: &config.Config{}, identityService: NewIdentityService(&stubIdentityCache{})}
		account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
		var wire []byte
		if countTokens {
			_, wire, err = svc.buildCountTokensRequest(ctx, c, account, body, "synthetic", "oauth", "claude-sonnet-4-6", false)
		} else {
			_, wire, err = svc.buildUpstreamRequest(ctx, c, account, body, "synthetic", "oauth", "claude-sonnet-4-6", true, false)
		}
		require.NoError(t, err)
		require.Equal(t, billing, gjson.GetBytes(wire, "system.0.text").String())
	}
}

func TestAdmittedClaudeProfilePreservedOnWire(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, sample := range observedClaudeBillingSamples {
		t.Run(sample.name, func(t *testing.T) {
			for _, countTokens := range []bool{false, true} {
				for _, identityEnabled := range []bool{false, true} {
					r, envelope := admittedClaudeFixture(t)
					claudeFixtureFirstBlock(t, envelope, "messages")["content"] = sample.text
					billing := "x-anthropic-billing-header: cc_version=2.1.301." + sample.suffix + "; cc_entrypoint=sdk-cli;"
					claudeFixtureFirstBlock(t, envelope, "system")["text"] = billing
					body, err := json.Marshal(envelope)
					require.NoError(t, err)
					ctx := WithClaudeCodeAdmission(context.Background())
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = r.WithContext(ctx)
					svc := &GatewayService{cfg: &config.Config{}}
					if identityEnabled {
						svc.identityService = NewIdentityService(&stubIdentityCache{fingerprint: &Fingerprint{
							UserAgent: "claude-cli/2.9.0 (external, cli)", ClientID: "stable-account-device",
							StainlessOS: "Linux", StainlessPackageVersion: "0.94.0", UpdatedAt: time.Now().Unix(),
						}})
					}
					account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
					var upstream *http.Request
					var wire []byte
					if countTokens {
						upstream, wire, err = svc.buildCountTokensRequest(ctx, c, account, body, "test-token", "oauth", "claude-sonnet-4-6", false)
					} else {
						upstream, wire, err = svc.buildUpstreamRequest(ctx, c, account, body, "test-token", "oauth", "claude-sonnet-4-6", true, false)
					}
					require.NoError(t, err)
					for k, values := range r.Header {
						if k == "Anthropic-Beta" { // OAuth adds its required authorization beta.
							continue
						}
						require.Equal(t, values[0], getHeaderRaw(upstream.Header, k), k)
					}
					require.Equal(t, "true", getHeaderRaw(upstream.Header, "Anthropic-Dangerous-Direct-Browser-Access"))
					require.Equal(t, billing, gjson.GetBytes(wire, "system.0.text").String())
					require.NotContains(t, string(wire), "cc_version=2.9.0")
					actual, err := io.ReadAll(upstream.Body)
					require.NoError(t, err)
					require.Equal(t, wire, actual)
					require.NoError(t, upstream.Body.Close())
				}
			}
		})
	}
}

func TestClaudeCodeAdmissionBlocksAnthropicFallback(t *testing.T) {
	svc := &GatewayService{}
	account := &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	ctx := WithClaudeCodeAdmissionPolicy(context.Background())
	for _, endpoint := range []string{"messages", "count_tokens", "responses", "chat/completions"} {
		t.Run(endpoint, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+endpoint, nil).WithContext(ctx)
			var err error
			switch endpoint {
			case "messages":
				_, err = svc.Forward(ctx, c, account, nil)
			case "count_tokens":
				err = svc.ForwardCountTokens(ctx, c, account, nil)
			case "responses":
				_, err = svc.ForwardAsResponses(ctx, c, account, nil, nil)
			case "chat/completions":
				_, err = svc.ForwardAsChatCompletions(ctx, c, account, nil, nil)
			}
			require.ErrorIs(t, err, ErrClaudeCodeOnly)
			require.Equal(t, http.StatusForbidden, w.Code)
			require.JSONEq(t, `{"type":"error","error":{"type":"permission_error","message":"Request not permitted"}}`, w.Body.String())
		})
	}
}

func TestAdmittedClaudeHTTP1ProfileSelection(t *testing.T) {
	for _, admitted := range []bool{false, true} {
		for _, version := range []string{"2.1.301", "2.1.302"} {
			for _, route := range []string{"messages", "count_tokens", "passthrough", "count_tokens_passthrough"} {
				t.Run(fmt.Sprintf("%s/%s/admitted=%t", route, version, admitted), func(t *testing.T) {
					incoming, envelope := admittedClaudeFixture(t)
					incoming.Header.Set("User-Agent", "claude-cli/"+version+" (external, cli)")
					body, err := json.Marshal(envelope)
					require.NoError(t, err)
					ctx := context.Background()
					if admitted {
						ctx = WithClaudeCodeAdmission(ctx)
					}
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = incoming.WithContext(ctx)
					svc := &GatewayService{cfg: &config.Config{}}
					account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
					var req *http.Request
					switch route {
					case "messages":
						req, _, err = svc.buildUpstreamRequest(ctx, c, account, body, "synthetic", "apikey", "claude-sonnet-4-6", true, false)
					case "count_tokens":
						req, _, err = svc.buildCountTokensRequest(ctx, c, account, body, "synthetic", "apikey", "claude-sonnet-4-6", false)
					case "passthrough":
						req, _, err = svc.buildUpstreamRequestAnthropicAPIKeyPassthrough(ctx, c, account, body, "synthetic")
					case "count_tokens_passthrough":
						req, err = svc.buildCountTokensRequestAnthropicAPIKeyPassthrough(ctx, c, account, body, "synthetic")
					}
					require.NoError(t, err)
					defer req.Body.Close()
					require.Equal(t, admitted && version == "2.1.301", claudehttp.Enabled(req))
					if claudehttp.Enabled(req) {
						seen := map[string]bool{}
						for key := range req.Header {
							lower := strings.ToLower(key)
							require.False(t, seen[lower], "duplicate casing for %s", key)
							seen[lower] = true
						}
					}
				})
			}
		}
	}
}
