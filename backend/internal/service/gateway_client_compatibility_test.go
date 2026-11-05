package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// The production request constructors and response converters run unchanged;
// only the transport destination is replaced by a credential-free local server.
// This is not a live Anthropic acceptance test or a measured TLS-profile test.
type compatibilityLoopbackUpstream struct {
	target *url.URL
	client *http.Client
}

func (u *compatibilityLoopbackUpstream) Do(r *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	clone := r.Clone(r.Context())
	target := *r.URL
	target.Scheme, target.Host = u.target.Scheme, u.target.Host
	clone.URL = &target
	clone.Host = ""
	return u.client.Do(clone)
}
func (u *compatibilityLoopbackUpstream) DoWithTLS(r *http.Request, p string, id int64, n int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(r, p, id, n)
}

func TestAnthropicCompatibilityForwardingReachesLoopback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []string{"messages", "messages/count_tokens", "chat/completions", "responses"} {
		for _, ua := range []string{"ordinary-sdk/1.0.0", "claude-cli/2.1.300 (external, cli)"} {
			t.Run(endpoint+"/"+ua, func(t *testing.T) {
				type observed struct {
					headers http.Header
					body    []byte
					path    string
				}
				requests := make(chan observed, 1)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					requests <- observed{r.Header.Clone(), body, r.URL.Path}
					w.Header().Set("Content-Type", "application/json")
					if strings.Contains(endpoint, "count_tokens") {
						_, _ = io.WriteString(w, `{"input_tokens":4}`)
					} else if gjson.GetBytes(body, "stream").Bool() {
						// Compatibility converters request upstream SSE even for
						// buffered clients; exercise their real terminal events.
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = io.WriteString(w, strings.Join([]string{
							`event: message_start`,
							`data: {"type":"message_start","message":{"id":"msg_local_fixture","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[],"usage":{"input_tokens":4,"output_tokens":0}}}`,
							``, `event: content_block_start`,
							`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"local fixture"}}`,
							``, `event: content_block_stop`,
							`data: {"type":"content_block_stop","index":0}`,
							``, `event: message_delta`,
							`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
							``, `event: message_stop`, `data: {"type":"message_stop"}`, ``, ``,
						}, "\n"))
					} else {
						_, _ = io.WriteString(w, `{"id":"msg_local_fixture","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[{"type":"text","text":"local fixture"}],"stop_reason":"end_turn","usage":{"input_tokens":4,"output_tokens":2}}`)
					}
				}))
				defer server.Close()
				target, err := url.Parse(server.URL)
				require.NoError(t, err)
				svc := &GatewayService{cfg: &config.Config{}, httpUpstream: &compatibilityLoopbackUpstream{target, server.Client()}}
				account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth,
					Credentials: map[string]any{"access_token": "synthetic-local-only"}}
				body := `{"model":"claude-sonnet-4-6","max_tokens":128,"stream":false,"messages":[{"role":"user","content":"ping"}],"metadata":{"user_id":"` + strings.Repeat("a", 64) + `"}}`
				if endpoint == "responses" {
					body = `{"model":"claude-sonnet-4-6","input":"ping","stream":false}`
				}
				ctx := WithClaudeCodeAdmissionPolicy(context.Background())
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+endpoint, strings.NewReader(body)).WithContext(ctx)
				c.Request.Header.Set("Content-Type", "application/json")
				c.Request.Header.Set("User-Agent", ua)
				parsed, err := ParseGatewayRequest(NewRequestBodyRef([]byte(body)), "anthropic")
				require.NoError(t, err)
				switch endpoint {
				case "messages":
					_, err = svc.Forward(ctx, c, account, parsed)
				case "messages/count_tokens":
					err = svc.ForwardCountTokens(ctx, c, account, parsed)
				case "chat/completions":
					_, err = svc.ForwardAsChatCompletions(ctx, c, account, []byte(body), parsed)
				case "responses":
					_, err = svc.ForwardAsResponses(ctx, c, account, []byte(body), parsed)
				}
				require.NoError(t, err, recorder.Body.String())
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				select {
				case got := <-requests:
					require.True(t, strings.HasPrefix(got.headers.Get("User-Agent"), "claude-cli/"))
					require.Equal(t, "cli", got.headers.Get("X-App"))
					require.Equal(t, "js", got.headers.Get("X-Stainless-Lang"))
					require.Contains(t, gjson.GetBytes(got.body, "messages.0.content").Raw, `"ping"`, "original user content must survive conversion")
					if endpoint == "messages/count_tokens" {
						require.Equal(t, "/v1/messages/count_tokens", got.path)
					} else {
						require.Equal(t, "/v1/messages", got.path)
						require.Contains(t, gjson.GetBytes(got.body, "system").Raw, "cc_entrypoint=cli;", "ordinary clients must take the mimicry path, not the raw-CLI preservation path")
					}
				default:
					t.Fatal("request did not reach the real local HTTP server")
				}
			})
		}
	}
}
