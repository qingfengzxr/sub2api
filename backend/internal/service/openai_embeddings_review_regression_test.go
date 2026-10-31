package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestEmbeddingsReviewRegressionMissingInputPrice(t *testing.T) {
	for model, fallback := range map[string]float64{"text-embedding-3-small": .02e-6, "text-embedding-3-large": .13e-6, "text-embedding-ada-002": .1e-6} {
		for _, tc := range []struct {
			name, card string
			price      float64
		}{
			{"missing", `{"output_cost_per_token":0}`, fallback},
			{"null", `{"input_cost_per_token":null,"output_cost_per_token":0}`, fallback},
			{"explicit free", `{"input_cost_per_token":0,"output_cost_per_token":0}`, 0},
			{"explicit rate", `{"input_cost_per_token":0.000003}`, .000003},
		} {
			t.Run(model+"/"+tc.name, func(t *testing.T) {
				ps := &PricingService{}
				data, err := ps.parsePricingData([]byte(`{"` + model + `":` + tc.card + `}`))
				require.NoError(t, err)
				ps.pricingData = data
				svc := &OpenAIGatewayService{billingService: NewBillingService(&config.Config{}, ps)}
				plan, err := svc.PrepareOpenAIEmbeddingsBilling(context.Background(), nil, []byte(`{"model":"`+model+`","input":"hello"}`), nil, ChannelUsageFields{})
				require.NoError(t, err)
				usage := BuildBillableUsage(UsageTokens{InputTokens: 1000}, BillingTokenPolicy{Multiplier: 1})
				cost, err := svc.calculateOpenAIEmbeddingsCost(context.Background(), plan, &OpenAIForwardResult{}, usage, 1, time.Now())
				require.NoError(t, err)
				require.InDelta(t, 1000*tc.price, cost.TotalCost, 1e-12)
			})
		}
	}
}

func TestEmbeddingsReviewRegressionBatchModeration(t *testing.T) {
	single := ExtractContentModerationText("openai_embeddings", []byte(`{"input":"sensitive-text"}`))
	batch := ExtractContentModerationText("openai_embeddings", []byte(`{"input":["sensitive-text","hello"]}`))
	require.Equal(t, "sensitive-text", single)
	require.Equal(t, "sensitive-text hello", batch)
	require.Equal(t, "sensitive-text hello", extractContentModerationKeywordText("openai_embeddings", []byte(`{"input":["sensitive-text","hello"]}`)))
	for _, input := range []string{`[1,2,3]`, `[[1,2],[3]]`} {
		require.Empty(t, ExtractContentModerationText("openai_embeddings", []byte(`{"input":`+input+`}`)))
	}
}

func BenchmarkEmbeddingsResponseValidation(b *testing.B) {
	body := []byte(`{"object":"list","data":[{"embedding":[` + strings.Repeat("0.123456789,", 1000000) + `0]}],"model":"text-embedding-3-small"}`)
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !validOpenAIEmbeddingsResponse(body) {
			b.Fatal("invalid")
		}
	}
}

type reviewModerationTransport func(*http.Request) (*http.Response, error)

func (f reviewModerationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func reviewModerationService(transport http.RoundTripper) (*ContentModerationService, *ContentModerationConfig) {
	cfg := defaultContentModerationConfig()
	cfg.Enabled = true
	cfg.Mode = ContentModerationModeObserve
	cfg.SampleRate = 100
	cfg.AllGroups = true
	cfg.WorkerCount = 1
	cfg.QueueSize = 1
	cfg.APIKeys = []string{"review-key"}
	cfg.BaseURL = "https://review.invalid"
	cfg.TimeoutMS = 2000
	cfg.RecordNonHits = false
	svc := &ContentModerationService{settingRepo: &contentModerationTestSettingRepo{}, repo: &contentModerationTestRepo{}, keyHealth: make(map[string]*contentModerationKeyHealth), httpClient: &http.Client{Transport: transport}, asyncQueue: make(chan contentModerationTask, 1)}
	svc.runtimeSnapshot.Store(&contentModerationRuntimeSnapshot{riskControlEnabled: true, config: cfg, loadedAt: time.Now()})
	return svc, cfg
}

func TestEmbeddingsReviewRegressionObserveConcurrency(t *testing.T) {
	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	transport := reviewModerationTransport(func(r *http.Request) (*http.Response, error) {
		entered <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"results":[{"flagged":false,"category_scores":{}}]}`))}, nil
	})
	svc, cfg := reviewModerationService(transport)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.Check(context.Background(), ContentModerationCheckInput{Protocol: "openai_embeddings", Model: "text-embedding-3-small", Body: []byte(`{"input":"hello"}`)})
		}()
	}
	timer := time.NewTimer(time.Second)
	count := 0
loop:
	for count < 8 {
		select {
		case <-entered:
			count++
		case <-timer.C:
			break loop
		}
	}
	timer.Stop()
	close(release)
	wg.Wait()
	require.Equal(t, cfg.WorkerCount, count)
	require.Equal(t, int64(7), svc.asyncDropped.Load())
	require.Zero(t, svc.embeddingsObserveActive.Load(), "finished observations must release capacity")
	_, err := svc.Check(context.Background(), ContentModerationCheckInput{Protocol: "openai_embeddings", Body: []byte(`{"input":"next"}`)})
	require.NoError(t, err)
	require.Len(t, entered, 1, "a subsequent observation must be admitted")
}

func TestEmbeddingsReviewRegressionAuditErrorHealthLeaksInput(t *testing.T) {
	transport := reviewModerationTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 400, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":"private embedding input echoed"}`))}, nil
	})
	svc, _ := reviewModerationService(transport)
	_, err := svc.Check(context.Background(), ContentModerationCheckInput{Protocol: "openai_embeddings", Model: "text-embedding-3-small", Body: []byte(`{"input":"private embedding input"}`)})
	if err != nil {
		t.Fatal(err)
	}
	require.Len(t, svc.keyHealth, 1)
	for _, state := range svc.keyHealth {
		if strings.Contains(state.LastError, "private embedding input") {
			t.Errorf("audit key health retains user input after sanitization: %s", state.LastError)
		}
		require.NotEmpty(t, state.LastError)
	}
}

func TestEmbeddingsResponseEnvelopeTypes(t *testing.T) {
	for _, data := range []string{`[]`, `[{"embedding":[0.1]}]`} {
		require.True(t, validOpenAIEmbeddingsResponse([]byte(`{"object":"list","data":`+data+`,"model":"text-embedding-3-small"}`)))
	}
	for _, data := range []string{`null`, `0`, `"[]"`, `{}`, `{"#":0}`, `{"\u0023":2}`, `true`} {
		require.False(t, validOpenAIEmbeddingsResponse([]byte(`{"object":"list","data":`+data+`,"model":"text-embedding-3-small"}`)), data)
	}
}

func TestEmbeddingsBatchKeywordBlocking(t *testing.T) {
	svc, cfg := reviewModerationService(reviewModerationTransport(func(*http.Request) (*http.Response, error) {
		t.Error("keyword-only audit must not call the provider")
		return nil, context.Canceled
	}))
	cfg.Mode = ContentModerationModePreBlock
	cfg.KeywordBlockingMode = ContentModerationKeywordModeKeywordOnly
	cfg.BlockedKeywords = []string{"blocked-term"}
	decision, err := svc.Check(context.Background(), ContentModerationCheckInput{Protocol: "openai_embeddings", Body: []byte(`{"input":["blocked-term","innocent last item"]}`)})
	require.NoError(t, err)
	require.True(t, decision.Blocked)
	require.Equal(t, ContentModerationActionKeywordBlock, decision.Action)
}

func TestEmbeddingsObservationReleasesCapacityAfterCancel(t *testing.T) {
	entered := make(chan struct{}, 1)
	svc, _ := reviewModerationService(reviewModerationTransport(func(r *http.Request) (*http.Response, error) {
		entered <- struct{}{}
		<-r.Context().Done()
		return nil, r.Context().Err()
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = svc.Check(ctx, ContentModerationCheckInput{Protocol: "openai_embeddings", Body: []byte(`{"input":"hello"}`)})
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("audit did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled audit did not exit")
	}
	require.Zero(t, svc.embeddingsObserveActive.Load())
	require.True(t, svc.acquireEmbeddingsObservation(1))
	svc.embeddingsObserveActive.Add(-1)
}

func TestOtherProtocolsRetainModerationErrorDiagnostics(t *testing.T) {
	svc, cfg := reviewModerationService(reviewModerationTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 400, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":"provider diagnostic"}`))}, nil
	}))
	_ = svc.checkSync(context.Background(), ContentModerationCheckInput{Protocol: ContentModerationProtocolOpenAIResponses}, cfg, ContentModerationInput{Text: "hello"}, "", nil, false)
	require.Len(t, svc.keyHealth, 1)
	for _, state := range svc.keyHealth {
		require.Contains(t, state.LastError, "provider diagnostic")
	}
}
