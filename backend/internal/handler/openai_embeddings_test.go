//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type embeddingsTestHTTP struct {
	service.HTTPUpstream
	client   *http.Client
	mu       sync.Mutex
	accounts []int64
}

func (u *embeddingsTestHTTP) Do(req *http.Request, _ string, id int64, _ int) (*http.Response, error) {
	u.mu.Lock()
	u.accounts = append(u.accounts, id)
	u.mu.Unlock()
	return u.client.Do(req)
}

type embeddingsTestUsageRepo struct {
	service.UsageLogRepository
	logs chan *service.UsageLog
}

type embeddingsTestAccountRepo struct {
	openAIImagesFailoverAccountRepo
}

func (r embeddingsTestAccountRepo) ListModelAvailabilityCandidates(_ context.Context, _ *int64, _ []string, _ bool) ([]service.Account, error) {
	return r.accounts, nil
}

func (r *embeddingsTestUsageRepo) Create(_ context.Context, log *service.UsageLog) (bool, error) {
	r.logs <- log
	return true, nil
}

func newEmbeddingsHandlerForTest(t *testing.T, endpoint string, onlyOAuth bool) (*OpenAIGatewayHandler, *embeddingsTestHTTP, *embeddingsTestUsageRepo) {
	t.Helper()
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Default.RateMultiplier = 1
	accounts := []service.Account{{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"access_token": "must-not-send"}}}
	if !onlyOAuth {
		for _, id := range []int64{2, 3} {
			accounts = append(accounts, service.Account{ID: id, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Priority: int(id), Credentials: map[string]any{"api_key": "upstream-test-key", "base_url": endpoint}})
		}
	}
	upstream := &embeddingsTestHTTP{client: &http.Client{}}
	usage := &embeddingsTestUsageRepo{logs: make(chan *service.UsageLog, 8)}
	billing := service.NewBillingService(cfg, nil)
	gateway := service.NewOpenAIGatewayService(embeddingsTestAccountRepo{openAIImagesFailoverAccountRepo{accounts: accounts}}, usage, nil, nil, nil, nil, nil, cfg, nil, nil, billing, nil, nil, upstream, nil, nil, nil, service.NewModelPricingResolver(nil, billing), nil, nil, nil, nil)
	cache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(cache.Stop)
	h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), cache, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
	return h, upstream, usage
}

func embeddingsHandlerContext(body string, ctx context.Context) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewBufferString(body)).WithContext(ctx)
	c.Request.Header.Set("Authorization", "Bearer client-gateway-key")
	groupID := int64(1)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 2, GroupID: &groupID, Group: &service.Group{ID: 1, Platform: service.PlatformOpenAI, RateMultiplier: 1}, User: &service.User{ID: 1}})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 1})
	return c, rec
}

func TestEmbeddingsHandlerBatch512FailoverAndBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	vector := make([]float64, 512)
	response, err := json.Marshal(map[string]any{"object": "list", "model": "text-embedding-3-small", "usage": map[string]int{"prompt_tokens": 12, "total_tokens": 12}, "data": []any{map[string]any{"object": "embedding", "index": 0, "embedding": vector}, map[string]any{"object": "embedding", "index": 1, "embedding": vector}}})
	require.NoError(t, err)
	request := `{"model":"text-embedding-3-small","input":["Hello world","你好世界"],"dimensions":512,"encoding_format":"float"}`
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		require.Equal(t, request, string(body))
		require.Equal(t, "/v1/embeddings", r.URL.Path)
		require.Equal(t, "Bearer upstream-test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		w.Header().Set("x-request-id", "embedding-test")
		_, _ = w.Write(response)
	}))
	defer server.Close()
	h, upstream, usage := newEmbeddingsHandlerForTest(t, server.URL, false)
	c, rec := embeddingsHandlerContext(request, context.Background())
	h.Embeddings(c)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, response, rec.Body.Bytes())
	require.Len(t, upstream.accounts, 2)
	require.NotContains(t, upstream.accounts, int64(1))
	require.NotEqual(t, upstream.accounts[0], upstream.accounts[1])
	select {
	case log := <-usage.logs:
		require.Equal(t, 12, log.InputTokens)
		require.Zero(t, log.OutputTokens)
		require.InDelta(t, 12*.02e-6, log.TotalCost, 1e-12)
	case <-time.After(time.Second):
		t.Fatal("missing usage record")
	}
	require.Empty(t, usage.logs)
}

func TestEmbeddingsHandlerErrorsDoNotDoubleWrite(t *testing.T) {
	for _, test := range []struct {
		name, body           string
		upstreamStatus, want int
		onlyOAuth            bool
	}{
		{"missing model", `{"input":"hi"}`, 200, 400, false},
		{"missing input", `{"model":"text-embedding-3-small"}`, 200, 400, false},
		{"invalid dimensions", `{"model":"text-embedding-3-small","input":"hi","dimensions":0}`, 200, 400, false},
		{"upstream bad request", `{"model":"text-embedding-3-small","input":"hi"}`, 400, 400, false},
		{"upstream empty error", `{"model":"text-embedding-3-small","input":"hi"}`, 422, 422, false},
		{"oauth only", `{"model":"text-embedding-3-small","input":"hi"}`, 200, 503, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.upstreamStatus)
				if test.upstreamStatus != 422 {
					_, _ = w.Write([]byte(`{"error":{"message":"bad input"}}`))
				}
			}))
			defer server.Close()
			h, upstream, usage := newEmbeddingsHandlerForTest(t, server.URL, test.onlyOAuth)
			c, rec := embeddingsHandlerContext(test.body, context.Background())
			h.Embeddings(c)
			require.Equal(t, test.want, rec.Code, rec.Body.String())
			if test.upstreamStatus == 422 {
				require.Empty(t, rec.Body.String())
			} else {
				var result map[string]any
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result), "must contain one JSON response")
			}
			if test.onlyOAuth || test.upstreamStatus == 200 {
				require.Empty(t, upstream.accounts)
			} else {
				require.Len(t, upstream.accounts, 1)
			}
			require.Empty(t, usage.logs)
		})
	}
}

func TestEmbeddingsHandlerLocalDeadline(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	c, rec := embeddingsHandlerContext(`{}`, context.Background())
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	require.True(t, h.finishEmbeddingsCanceled(c, context.Background()))
	require.Equal(t, 504, rec.Code)
}

func TestEmbeddingsHandlerMandatoryBillingOnOverflow(t *testing.T) {
	for _, policy := range []string{"drop", "sample"} {
		t.Run(policy, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"object":"list","data":[],"model":"text-embedding-3-small","usage":{"prompt_tokens":2}}`))
			}))
			defer server.Close()
			h, _, usage := newEmbeddingsHandlerForTest(t, server.URL, false)
			pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{WorkerCount: 1, QueueSize: 1, TaskTimeout: time.Second, OverflowPolicy: policy, OverflowSamplePercent: 0})
			t.Cleanup(pool.Stop)
			h.usageRecordWorkerPool = pool
			entered, release := make(chan struct{}), make(chan struct{})
			defer close(release)
			pool.Submit(func(context.Context) { close(entered); <-release })
			<-entered
			pool.Submit(func(context.Context) {})
			c, rec := embeddingsHandlerContext(`{"model":"text-embedding-3-small","input":"hi"}`, context.Background())
			h.Embeddings(c)
			require.Equal(t, 200, rec.Code, rec.Body.String())
			select {
			case log := <-usage.logs:
				require.Equal(t, 2, log.InputTokens)
			default:
				t.Fatal("overflow lost the bill")
			}
		})
	}
}

func TestEmbeddingsHandlerCanceledUpstream(t *testing.T) {
	clientCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		cancel()
		<-r.Context().Done()
	}))
	defer server.Close()
	h, upstream, usage := newEmbeddingsHandlerForTest(t, server.URL, false)
	c, _ := embeddingsHandlerContext(`{"model":"text-embedding-3-small","input":"hi"}`, clientCtx)
	h.Embeddings(c)
	require.Equal(t, 499, c.Writer.Status())
	require.Len(t, upstream.accounts, 1)
	require.Empty(t, usage.logs)
}

func TestEmbeddingsMissingUsageIsQueuedForOpsOnSuccess(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 4)
	c, _ := embeddingsHandlerContext(`{}`, context.Background())
	c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{Kind: "embedding_usage_unavailable", UpstreamStatusCode: 200, AccountID: 2, Message: "input must not persist", Detail: "private-input"}})
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	logOpsRecoveredUpstream(c, ops, 200)
	select {
	case job := <-opsErrorLogQueue:
		require.Equal(t, 200, job.entry.StatusCode)
		require.NotNil(t, job.entry.UpstreamErrorsJSON)
		events, err := service.ParseOpsUpstreamErrors(*job.entry.UpstreamErrorsJSON)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, "embedding_usage_unavailable", events[0].Kind)
		require.NotContains(t, events[0].Message, "input must not persist")
		require.Empty(t, events[0].Detail)
	default:
		t.Fatal("successful embedding anomaly was not queued for persistence")
	}
}

func TestEmbeddingsDeferredBillingDoesNotReadReusedGinContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"object":"list","data":[],"model":"text-embedding-3-small","usage":{"prompt_tokens":2}}`))
	}))
	defer server.Close()
	h, _, usage := newEmbeddingsHandlerForTest(t, server.URL, false)
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{WorkerCount: 1, QueueSize: 2, TaskTimeout: time.Second, OverflowPolicy: "drop"})
	t.Cleanup(pool.Stop)
	h.usageRecordWorkerPool = pool
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	pool.Submit(func(context.Context) { close(entered); <-release })
	<-entered
	parent := context.WithValue(context.Background(), ctxkey.RequestedPublicModel, "public-embedding-alias")
	c, rec := embeddingsHandlerContext(`{"model":"text-embedding-3-small","input":"hi"}`, parent)
	h.Embeddings(c)
	require.Equal(t, 200, rec.Code)
	c.Request = httptest.NewRequest(http.MethodPost, "/unrelated", nil)
	once.Do(func() { close(release) })
	select {
	case log := <-usage.logs:
		require.Equal(t, "public-embedding-alias", log.RequestedModel)
	case <-time.After(time.Second):
		t.Fatal("missing deferred bill")
	}
}

type embeddingsFailedWriter struct{ gin.ResponseWriter }

func (w embeddingsFailedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestEmbeddingsHandlerWriteFailureStillBillsKnownUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"object":"list","data":[],"model":"text-embedding-3-small","usage":{"prompt_tokens":7}}`))
	}))
	defer server.Close()
	h, upstream, usage := newEmbeddingsHandlerForTest(t, server.URL, false)
	c, _ := embeddingsHandlerContext(`{"model":"text-embedding-3-small","input":"hi"}`, context.Background())
	c.Writer = embeddingsFailedWriter{c.Writer}
	h.Embeddings(c)
	require.Len(t, upstream.accounts, 1)
	select {
	case log := <-usage.logs:
		require.Equal(t, 7, log.InputTokens)
	default:
		t.Fatal("client write error lost known usage")
	}
	require.Empty(t, usage.logs)
}
