//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type admissionFallbackUpstream struct {
	calls []int64
}

func (u *admissionFallbackUpstream) Do(_ *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.calls = append(u.calls, accountID)
	body := `{"error":{"type":"invalid_request_error","message":"Synthetic terminal error"}}`
	if accountID == 9510 {
		body = `{"error":{"code":400,"message":"Prompt is too long"}}`
	}
	return &http.Response{StatusCode: 400, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
}

func (u *admissionFallbackUpstream) DoWithTLS(r *http.Request, proxy string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(r, proxy, accountID, concurrency)
}

// Exercise the actual PromptTooLongError -> regroup -> reschedule -> Forward
// path. Registered ingress middleware is covered separately in routes tests.
func TestGatewayMessagesAdmissionThroughActualAntigravityFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	data, err := os.ReadFile("../service/testdata/claude_cli/ascii/capture.json")
	require.NoError(t, err)
	var recorded struct {
		Requests []struct {
			Body    string     `json:"body"`
			Headers [][]string `json:"headers"`
		} `json:"requests"`
	}
	require.NoError(t, json.Unmarshal(data, &recorded))
	require.NotEmpty(t, recorded.Requests)
	for _, forced := range []bool{false, true} {
		for _, kind := range []string{"cli", "recorded_cli", "non_cli", "duplicate_json"} {
			name := kind + "/normal"
			if forced {
				name = kind + "/forced"
			}
			t.Run(name, func(t *testing.T) {
				fallbackID := int64(9501)
				primary := &service.Group{ID: 9500, Hydrated: true, Platform: service.PlatformAntigravity, Status: service.StatusActive, FallbackGroupIDOnInvalidRequest: &fallbackID}
				fallback := &service.Group{ID: fallbackID, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive}
				accounts := []*service.Account{
					{ID: 9510, Platform: service.PlatformAntigravity, Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true, Concurrency: 1,
						Credentials:   map[string]any{"access_token": "synthetic", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339), "project_id": "synthetic-project", "model_mapping": map[string]any{"claude-sonnet-4-5": "claude-sonnet-4-5", "claude-sonnet-4-5-thinking": "claude-sonnet-4-5-thinking", "claude-sonnet-4-6": "claude-sonnet-4-5"}},
						AccountGroups: []service.AccountGroup{{AccountID: 9510, GroupID: primary.ID}}},
					{ID: 9511, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1,
						Credentials: map[string]any{"api_key": "synthetic-upstream"}, AccountGroups: []service.AccountGroup{{AccountID: 9511, GroupID: fallback.ID}}},
				}
				scheduler := &groupScopedSchedulerCache{fakeSchedulerCache: &fakeSchedulerCache{accounts: accounts}}
				upstream := &admissionFallbackUpstream{}
				cfg := &config.Config{RunMode: config.RunModeSimple}
				settings := service.NewSettingService(&toggleSettingRepo{values: map[string]string{}}, cfg)
				gateway := service.NewGatewayService(nil, &groupMapRepo{fakeGroupRepo: &fakeGroupRepo{}, groups: map[int64]*service.Group{primary.ID: primary, fallback.ID: fallback}},
					nil, nil, nil, nil, nil, nil, cfg, service.NewSchedulerSnapshotService(scheduler, nil, nil, nil, nil),
					nil, nil, nil, nil, nil, upstream, nil, nil, nil, nil, nil, settings, nil, nil, nil, nil, nil, nil)
				billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
				t.Cleanup(billing.Stop)
				h := &GatewayHandler{gatewayService: gateway, billingCacheService: billing, cfg: cfg,
					settingService:            settings,
					antigravityGatewayService: service.NewAntigravityGatewayService(nil, nil, nil, service.NewAntigravityTokenProvider(nil, nil, nil), nil, upstream, nil, nil),
					concurrencyHelper:         NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0), maxAccountSwitches: 1}
				body := `{"model":"claude-sonnet-4-5","max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`
				if kind == "recorded_cli" {
					body = recorded.Requests[0].Body
				}
				if kind == "duplicate_json" {
					body = `{"model":"claude-sonnet-4-5",` + body[1:]
				}
				path := "/v1/messages"
				if forced {
					path = "/antigravity/v1/messages"
				}
				r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
				r.Header.Set("Content-Type", "application/json")
				if kind != "non_cli" {
					for k, v := range claude.DefaultHeaders() {
						r.Header.Set(k, v)
					}
					r.Header.Set("Anthropic-Version", "2023-06-01")
					r.Header.Set("Anthropic-Beta", claude.DefaultBetaHeader)
				}
				if kind == "recorded_cli" {
					r.Header = make(http.Header)
					for _, pair := range recorded.Requests[0].Headers {
						r.Header.Add(pair[0], pair[1])
					}
				}
				ctx := service.WithClaudeCodeAdmissionPolicy(context.WithValue(context.Background(), ctxkey.Group, primary))
				if service.NewClaudeCodeValidator().ValidateIngressJSON(r, []byte(body)) {
					ctx = service.WithClaudeCodeAdmission(ctx)
				}
				if forced {
					ctx = context.WithValue(ctx, ctxkey.ForcePlatform, service.PlatformAntigravity)
				}
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = r.WithContext(ctx)
				apiKey := &service.APIKey{ID: 9520, UserID: 9530, GroupID: &primary.ID, Group: primary, Status: service.StatusActive, User: &service.User{ID: 9530, Concurrency: 10, Balance: 100}}
				c.Set(string(middleware.ContextKeyAPIKey), apiKey)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})
				h.Messages(c)
				require.Contains(t, scheduler.queriedGroupIDs(), primary.ID)
				require.Contains(t, scheduler.queriedGroupIDs(), fallback.ID, "must actually reschedule the fallback group: %s", w.Body.String())
				if kind == "cli" || kind == "recorded_cli" {
					require.Equal(t, []int64{9510, 9511}, upstream.calls)
					require.NotEqual(t, http.StatusForbidden, w.Code)
				} else {
					require.Equal(t, []int64{9510}, upstream.calls, "rejected traffic must never reach Anthropic")
					require.Equal(t, http.StatusForbidden, w.Code)
				}
			})
		}
	}
}
