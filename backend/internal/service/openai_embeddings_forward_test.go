package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestForwardEmbeddingsRejectsIneligibleAccounts(t *testing.T) {
	for _, account := range []*Account{nil,
		{Platform: PlatformOpenAI, Type: AccountTypeOAuth},
		{Platform: PlatformGemini, Type: AccountTypeAPIKey},
		{Platform: PlatformAnthropic, Type: AccountTypeAPIKey},
		{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"openai_capabilities": []any{"chat_completions"}}},
		{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"allowed": "allowed"}}},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		svc := &OpenAIGatewayService{}
		_, err := svc.ForwardEmbeddings(context.Background(), c, account, []byte(`{"model":"text-embedding-3-small","input":"hi"}`), "")
		require.Error(t, err, "nil HTTP upstream must never be called")
	}
}

func TestForwardEmbeddingsRawEncodingAndMissingUsage(t *testing.T) {
	for _, encoding := range []string{"float", "base64"} {
		for _, input := range []string{`"hi"`, `["hi","你好"]`, `[1,2]`, `[[1],[2]]`} {
			body := []byte(`{"model":"text-embedding-3-small","input":` + input + `,"encoding_format":"` + encoding + `","dimensions":512,"user":"u"}`)
			vector := `[0.1,0.2]`
			if encoding == "base64" {
				vector = `"zczMPc3MTD4="`
			}
			response := `{"object":"list","model":"text-embedding-3-small","data":[{"object":"embedding","index":0,"embedding":` + vector + `}]}`
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"X-Request-Id": {"test"}}, Body: io.NopCloser(strings.NewReader(response))}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "key"}}
			result, err := svc.ForwardEmbeddings(context.Background(), c, account, body, "")
			require.NoError(t, err)
			require.Equal(t, body, upstream.lastBody)
			require.Equal(t, response, rec.Body.String())
			require.Zero(t, result.Usage.InputTokens)
			events, ok := c.Get(OpsUpstreamErrorsKey)
			require.True(t, ok)
			require.Equal(t, "embedding_usage_unavailable", events.([]*OpsUpstreamErrorEvent)[0].Kind)
		}
	}
}

type embeddingsErrorUpstream struct {
	HTTPUpstream
	err error
	ctx context.Context
}

func (u *embeddingsErrorUpstream) Do(r *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.ctx = r.Context()
	return nil, u.err
}

func TestForwardEmbeddingsCancellationAndTransportFailure(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		upstream := &embeddingsErrorUpstream{err: errors.New("connection reset")}
		if canceled {
			cancel()
			upstream.err = context.Canceled
		}
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil).WithContext(ctx)
		svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
		_, err := svc.ForwardEmbeddings(ctx, c, &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "key"}}, []byte(`{"model":"text-embedding-3-small","input":"hi"}`), "")
		if canceled {
			require.ErrorIs(t, err, context.Canceled)
			require.ErrorIs(t, upstream.ctx.Err(), context.Canceled)
		} else {
			var retry *UpstreamFailoverError
			require.ErrorAs(t, err, &retry)
		}
		require.False(t, c.Writer.Written())
		cancel()
	}
}

func TestForwardEmbeddingsInvalidSuccessfulResponse(t *testing.T) {
	for _, body := range []string{"<html>error</html>", "", `{"data":[],"usage":{"prompt_tokens":5}}`} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
		svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}}}
		result, err := svc.ForwardEmbeddings(context.Background(), c, &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "key"}}, []byte(`{"model":"text-embedding-3-small","input":"hi"}`), "")
		require.Error(t, err)
		require.Nil(t, result)
		require.Equal(t, 502, rec.Code)
	}
}

func TestForwardEmbeddingsHTTPErrorPolicy(t *testing.T) {
	for _, status := range []int{400, 401, 403, 429, 500, 502} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
		body := `{"error":{"type":"invalid_request_error","message":"provider message"}}`
		svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: &httpUpstreamRecorder{resp: &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}}}
		_, err := svc.ForwardEmbeddings(context.Background(), c, &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "key"}}, []byte(`{"model":"text-embedding-3-small","input":"hi"}`), "")
		require.Error(t, err)
		if status == 400 {
			require.Equal(t, 400, rec.Code)
			require.Equal(t, body, rec.Body.String())
		} else {
			var retry *UpstreamFailoverError
			require.ErrorAs(t, err, &retry)
			require.Equal(t, status, retry.StatusCode)
			require.False(t, c.Writer.Written())
		}
	}
}

type embeddingsBrokenBody struct{}

func (embeddingsBrokenBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (embeddingsBrokenBody) Close() error             { return nil }

func TestForwardEmbeddingsReadFailureAndSizeLimit(t *testing.T) {
	for _, body := range []io.ReadCloser{embeddingsBrokenBody{}, io.NopCloser(strings.NewReader(strings.Repeat("x", 128)))} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
		cfg := &config.Config{}
		cfg.Gateway.UpstreamResponseReadMaxBytes = 64
		svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{}, Body: body}}}
		_, err := svc.ForwardEmbeddings(context.Background(), c, &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "key"}}, []byte(`{"model":"text-embedding-3-small","input":"hi"}`), "")
		require.Error(t, err)
		var retry *UpstreamFailoverError
		require.False(t, errors.As(err, &retry))
		require.Equal(t, 502, rec.Code)
	}
}
