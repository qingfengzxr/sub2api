package claudehttp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type capturedRequest struct {
	Headers [][]string `json:"headers"`
	Body    string     `json:"body"`
	Method  string     `json:"method"`
	Path    string     `json:"path"`
}
type captureFile struct {
	Requests []capturedRequest `json:"requests"`
}
type namesCapture struct {
	Cases []struct {
		Auth     string `json:"auth"`
		Extra    bool   `json:"extra"`
		Retry    int    `json:"retry"`
		HTTPS    bool   `json:"https"`
		Requests []struct {
			Names  []string `json:"header_names"`
			Status int      `json:"status"`
		} `json:"requests"`
	} `json:"cases"`
}
type wireRequest struct {
	names      []string
	headers    http.Header
	body       string
	connection int64
}

// Reads raw header lines before parsing the request. Header maps alone cannot
// establish wire order or capitalization. Bodies are read by Content-Length.
func wireServer(t *testing.T) (string, <-chan wireRequest) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })
	records := make(chan wireRequest, 200)
	var count atomic.Int64
	go func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			id := count.Add(1)
			go func() {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(15 * time.Second))
				br := bufio.NewReader(c)
				for {
					var block strings.Builder
					var names []string
					line, err := br.ReadString('\n')
					if err != nil {
						return
					}
					block.WriteString(line)
					for {
						line, err = br.ReadString('\n')
						if err != nil {
							return
						}
						block.WriteString(line)
						if line == "\r\n" {
							break
						}
						key, _, _ := strings.Cut(line, ":")
						names = append(names, key)
					}
					req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(block.String())))
					if err != nil {
						t.Error(err)
						return
					}
					if req.ContentLength < 0 || len(req.TransferEncoding) > 0 {
						t.Error("unexpected request framing")
						return
					}
					body := make([]byte, req.ContentLength)
					if _, err := io.ReadFull(br, body); err != nil {
						t.Error(err)
						return
					}
					records <- wireRequest{names, req.Header, string(body), id}
					_, err = io.WriteString(c, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK")
					if err != nil {
						return
					}
				}
			}()
		}
	}()
	return "http://" + listener.Addr().String(), records
}

func testClient(t *testing.T) *http.Client {
	t.Helper()
	tr := NewTransport(&http.Transport{DialContext: (&net.Dialer{Timeout: time.Second}).DialContext, MaxConnsPerHost: 1, MaxIdleConnsPerHost: 1})
	t.Cleanup(tr.CloseIdleConnections)
	return &http.Client{Transport: tr, Timeout: 5 * time.Second}
}

func TestRecordedHeaderOrderAndBodyOnWire(t *testing.T) {
	endpoint, records := wireServer(t)
	client := testClient(t)
	files, err := filepath.Glob("../../service/testdata/claude_cli/*/capture.json")
	require.NoError(t, err)
	require.Len(t, files, 23)
	var requests int
	var reused atomic.Int64
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		var fixture captureFile
		require.NoError(t, json.Unmarshal(data, &fixture))
		for _, sample := range fixture.Requests {
			req, err := http.NewRequest(sample.Method, endpoint+sample.Path, strings.NewReader(sample.Body))
			require.NoError(t, err)
			var expected []string
			for _, pair := range sample.Headers {
				req.Header[pair[0]] = []string{pair[1]}
				expected = append(expected, pair[0])
			}
			req = req.WithContext(httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{GotConn: func(i httptrace.GotConnInfo) {
				if i.Reused {
					reused.Add(1)
				}
			}}))
			before := req.Header.Clone()
			resp, err := client.Do(req)
			require.NoError(t, err)
			_, err = io.Copy(io.Discard, resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			got := <-records
			require.Equal(t, expected, got.names, file)
			require.Equal(t, sample.Body, got.body, file)
			require.Equal(t, int64(1), got.connection)
			require.Equal(t, before, req.Header, "serializer must not mutate caller headers")
			requests++
		}
	}
	require.Equal(t, 40, requests)
	require.Equal(t, int64(39), reused.Load())
}

func TestAuthOptionalHeadersAndRetryOrder(t *testing.T) {
	data, err := os.ReadFile("testdata/cli-2.1.301-header-order.json")
	require.NoError(t, err)
	var fixture namesCapture
	require.NoError(t, json.Unmarshal(data, &fixture))
	for i, scenario := range fixture.Cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			endpoint, records := wireServer(t)
			client := testClient(t)
			for _, sample := range scenario.Requests {
				req, err := http.NewRequest("POST", endpoint+"/v1/messages?beta=true", strings.NewReader("{}"))
				require.NoError(t, err)
				for _, name := range sample.Names {
					if name == "Host" || name == "Content-Length" || name == "Connection" || name == "Accept-Encoding" {
						continue
					}
					// Exercise normalization of known headers arriving with different casing.
					key := strings.ToLower(name)
					if name == "X-Synthetic-Extra" {
						key = name
					}
					req.Header[key] = []string{"synthetic"}
				}
				resp, err := client.Do(req)
				require.NoError(t, err)
				_, err = io.Copy(io.Discard, resp.Body)
				require.NoError(t, err)
				resp.Body.Close()
				got := <-records
				require.Equal(t, sample.Names, got.names)
				require.Equal(t, "{}", got.body)
			}
		})
	}
}

func TestOrderedTransportReturnsRetryStatusesToCaller(t *testing.T) {
	for _, status := range []int{429, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					w.WriteHeader(status)
				} else {
					w.WriteHeader(200)
				}
			}))
			defer server.Close()
			client := testClient(t)
			for _, want := range []int{status, 200} {
				req, _ := http.NewRequest("POST", server.URL, strings.NewReader("{}"))
				req.Header.Set("User-Agent", "claude-cli/2.1.301")
				resp, err := client.Do(req)
				require.NoError(t, err)
				require.Equal(t, want, resp.StatusCode)
				resp.Body.Close()
			}
			require.Equal(t, int64(2), calls.Load())
		})
	}
}

func TestOrderedBodyLifecycle(t *testing.T) {
	var connections atomic.Int64
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stream":
			w.WriteHeader(200)
			_, _ = io.WriteString(w, "data: first\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		case "/trailers":
			w.Header().Set("Trailer", "X-Final")
			w.WriteHeader(200)
			_, _ = io.WriteString(w, "OK")
			w.Header().Set("X-Final", "done")
		default:
			_, _ = io.WriteString(w, "OK")
		}
	}))
	server.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			connections.Add(1)
		}
	}
	server.Start()
	defer server.Close()
	client := testClient(t)
	makeReq := func(path string) *http.Request {
		r, _ := http.NewRequest("POST", server.URL+path, strings.NewReader("{}"))
		r.Header.Set("User-Agent", "claude-cli/2.1.301")
		return r
	}
	first, err := client.Do(makeReq("/stream"))
	require.NoError(t, err)
	data := make([]byte, len("data: first\n\n"))
	_, err = io.ReadFull(first.Body, data)
	require.NoError(t, err)
	// With the only connection occupied, cancelling a queued request must not
	// close the active stream or strand the connection limit.
	queuedCtx, queuedCancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	_, queuedErr := client.Do(makeReq("/ok").WithContext(queuedCtx))
	queuedCancel()
	require.ErrorIs(t, queuedErr, context.DeadlineExceeded)
	require.Equal(t, int64(1), connections.Load())
	require.NoError(t, first.Body.Close())
	second, err := client.Do(makeReq("/trailers"))
	require.NoError(t, err)
	body, err := io.ReadAll(second.Body)
	require.NoError(t, err)
	require.Equal(t, "OK", string(body))
	require.NoError(t, second.Body.Close())
	require.Equal(t, "done", second.Trailer.Get("X-Final"))
	require.Equal(t, int64(2), connections.Load(), "early-close connection must be discarded")
	ctx, cancel := context.WithCancel(t.Context())
	third, err := client.Do(makeReq("/stream").WithContext(ctx))
	require.NoError(t, err)
	_, err = io.ReadFull(third.Body, data)
	require.NoError(t, err)
	cancel()
	_, err = io.ReadAll(third.Body)
	require.Error(t, err)
	third.Body.Close()
	fourth, err := client.Do(makeReq("/ok"))
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, fourth.Body)
	require.NoError(t, err)
	fourth.Body.Close()
}

type closeTrackingBody struct {
	io.Reader
	closed bool
}

func (b *closeTrackingBody) Close() error { b.closed = true; return nil }
func TestRejectAmbiguousFramingBeforeDial(t *testing.T) {
	for _, mutate := range []func(*http.Request){
		func(r *http.Request) {
			r.Header["content-type"] = []string{"text/plain"}
			r.Header["Content-Type"] = []string{"application/json"}
		},
		func(r *http.Request) { r.Header["X-Bad"] = []string{"a\r\nInjected: b"} },
		func(r *http.Request) { r.Header["Header-Order:"] = []string{"host"} },
		func(r *http.Request) { r.TransferEncoding = []string{"chunked"} },
		func(r *http.Request) { r.ContentLength = -1 },
		func(r *http.Request) { r.Trailer = http.Header{"X-Trailer": []string{"value"}} },
	} {
		req, _ := http.NewRequest("POST", "http://unused.invalid", strings.NewReader("{}"))
		body := &closeTrackingBody{Reader: bytes.NewBufferString("{}")}
		req.Body = body
		req.Header.Set("User-Agent", "claude-cli/2.1.301")
		mutate(req)
		tr := NewTransport(&http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
			t.Error("invalid request reached dialer")
			return nil, fmt.Errorf("unexpected dial")
		}})
		_, err := tr.RoundTrip(req)
		require.Error(t, err)
		require.True(t, body.closed)
	}
}

func TestProfileSelection(t *testing.T) {
	for _, ua := range []string{"claude-cli/2.1.301 (external, cli)", "claude-cli/2.1.301", "claude-cli/2.1.302", "claude-cli/2.1.3010", "other"} {
		req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", strings.NewReader("{}"))
		req.Header.Set("User-Agent", ua)
		require.False(t, Enabled(req))
		marked := WithProfile(req)
		require.Equal(t, ua == "claude-cli/2.1.301 (external, cli)" || ua == "claude-cli/2.1.301", Enabled(marked))
	}
}

func TestConcurrentOrderedRequests(t *testing.T) {
	endpoint, records := wireServer(t)
	transport := NewTransport(&http.Transport{DialContext: (&net.Dialer{Timeout: time.Second}).DialContext, MaxConnsPerHost: 8, MaxIdleConnsPerHost: 8})
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	common := http.Header{"accept": []string{"application/json"}, "content-type": []string{"application/json"}, "user-agent": []string{"claude-cli/2.1.301"}}
	t.Run("parallel", func(t *testing.T) {
		for i := 0; i < 40; i++ {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				t.Parallel()
				req, err := http.NewRequest("POST", endpoint+"/v1/messages", strings.NewReader(strconv.Itoa(i)))
				require.NoError(t, err)
				req.Header = common.Clone()
				if i%2 == 0 {
					req.Header.Set("X-Api-Key", "synthetic")
				} else {
					req.Header.Set("Authorization", "Bearer synthetic")
				}
				response, err := client.Do(req)
				require.NoError(t, err)
				_, err = io.Copy(io.Discard, response.Body)
				require.NoError(t, err)
				require.NoError(t, response.Body.Close())
			})
		}
	})
	for i := 0; i < 40; i++ {
		got := <-records
		id, err := strconv.Atoi(got.body)
		require.NoError(t, err)
		names := []string{"Accept", "Content-Type", "User-Agent", "x-api-key", "Connection", "Host", "Accept-Encoding", "Content-Length"}
		if id%2 != 0 {
			names = []string{"Accept", "Authorization", "Content-Type", "User-Agent", "Connection", "Host", "Accept-Encoding", "Content-Length"}
		}
		require.Equal(t, names, got.names)
	}
	require.Len(t, common, 3, "concurrent serialization mutated shared headers")
}
