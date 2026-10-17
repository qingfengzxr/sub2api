package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type recordedCLIRequest struct {
	Path    string     `json:"path"`
	Method  string     `json:"method"`
	Headers [][]string `json:"headers"`
	Body    string     `json:"body"`
	SHA256  string     `json:"body_sha256"`
}

type recordedCLICapture struct {
	Requests []recordedCLIRequest `json:"requests"`
	Case     struct {
		Prompt        string            `json:"prompt"`
		InputMessages []json.RawMessage `json:"input_messages"`
	} `json:"case"`
}

// The capture script records raw CLI output. These expectations must never be
// regenerated from computeClaudeCodeFingerprint or a second copy of its logic.
func TestClaudeCLIRecordedSamples(t *testing.T) {
	directory := os.Getenv("CLAUDE_CLI_CAPTURE_DIR")
	if directory == "" {
		directory = filepath.Join("testdata", "claude_cli")
	}
	paths, err := filepath.Glob(filepath.Join(directory, "*", "capture.json"))
	require.NoError(t, err)
	if len(paths) == 0 && os.Getenv("CLAUDE_CLI_CAPTURE_DIR") == "" {
		t.Skip("no recorded CLI corpus; run backend/scripts/claude_cli_sample_capture.py")
	}
	require.NotEmpty(t, paths, "capture directory must contain actual CLI requests")
	gin.SetMode(gin.TestMode)
	for _, path := range paths {
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			var capture recordedCLICapture
			require.NoError(t, json.Unmarshal(data, &capture))
			if len(capture.Requests) == 0 {
				t.Skip("CLI did not produce a request; inspect capture status")
			}
			for index, recorded := range capture.Requests {
				originalContent, err := json.Marshal(capture.Case.Prompt)
				require.NoError(t, err)
				if len(capture.Case.InputMessages) > 0 {
					originalContent = capture.Case.InputMessages[0]
				}
				checkRecordedCLIRequest(t, recorded, originalContent, index)
			}
		})
	}
}

func checkRecordedCLIRequest(t *testing.T, recorded recordedCLIRequest, originalContent json.RawMessage, index int) {
	t.Helper()
	request := httptest.NewRequest(recorded.Method, recorded.Path, strings.NewReader(recorded.Body))
	for _, pair := range recorded.Headers {
		require.Len(t, pair, 2)
		request.Header.Add(pair[0], pair[1])
	}
	body := []byte(recorded.Body)
	bodyDigest := sha256.Sum256(body)
	require.Equal(t, recorded.SHA256, hex.EncodeToString(bodyDigest[:]), "captured body was edited without preserving its source")
	require.True(t, NewClaudeCodeValidator().ValidateIngressJSON(request, body), "real CLI request %d was rejected", index)
	var billing string
	gjson.GetBytes(body, "system").ForEach(func(_, block gjson.Result) bool {
		text := block.Get("text").String()
		if strings.HasPrefix(text, "x-anthropic-billing-header:") {
			billing = text
		}
		return true
	})
	if billing != "" {
		version := ExtractCLIVersion(request.UserAgent())
		match := ccVersionWithFingerprintInBillingRe.FindString(billing)
		require.NotEmpty(t, match, "capture must contain a CLI-provided suffix")
		algorithmInput := body
		originalBody, err := json.Marshal(map[string]any{"messages": []any{map[string]any{"role": "user", "content": originalContent}}})
		require.NoError(t, err)
		emptyFirstText := gjson.GetBytes(originalBody, "messages.0.content.0.type").String() == "text" && gjson.GetBytes(originalBody, "messages.0.content.0.text").String() == ""
		if strings.HasPrefix(extractFirstUserText(body), "<system-reminder>") || emptyFirstText {
			// The original CLI invocation is recorded independently of the
			// final API body, which may include injected environment reminders.
			algorithmInput = originalBody
			t.Log("CLI changed message blocks after billing: validate suffix against captured original input; preserve billing on forwarding")
		}
		require.Equal(t, "cc_version="+version+"."+computeClaudeCodeFingerprint(algorithmInput, version), match, "request %d", index)
	}
	for _, identityEnabled := range []bool{false, true} {
		ctx := WithClaudeCodeAdmission(context.Background())
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = request.WithContext(ctx)
		svc := &GatewayService{cfg: &config.Config{}}
		if identityEnabled {
			svc.identityService = NewIdentityService(&stubIdentityCache{fingerprint: &Fingerprint{
				UserAgent: "claude-cli/2.9.0 (external, cli)", ClientID: "synthetic-device",
				StainlessOS: "Linux", StainlessPackageVersion: "0.94.0",
			}})
		}
		account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
		var upstream *http.Request
		var wire []byte
		var err error
		if strings.Contains(recorded.Path, "/count_tokens") {
			upstream, wire, err = svc.buildCountTokensRequest(ctx, c, account, body, "synthetic-token", "oauth", gjson.GetBytes(body, "model").String(), false)
		} else {
			upstream, wire, err = svc.buildUpstreamRequest(ctx, c, account, body, "synthetic-token", "oauth", gjson.GetBytes(body, "model").String(), true, false)
		}
		require.NoError(t, err)
		for key, values := range request.Header {
			if !allowedHeaders[strings.ToLower(key)] || strings.EqualFold(key, "anthropic-beta") {
				continue
			}
			require.Equal(t, values[0], getHeaderRaw(upstream.Header, key), "header %s, identity=%t", key, identityEnabled)
		}
		require.JSONEq(t, recorded.Body, string(wire), "CLI body changed with identity=%t", identityEnabled)
		if identityEnabled {
			checkRecordedCLIHTTPWire(t, upstream, recorded)
		}
	}
}

func checkRecordedCLIHTTPWire(t *testing.T, upstream *http.Request, recorded recordedCLIRequest) {
	t.Helper()
	type received struct {
		headers http.Header
		body    []byte
		err     error
		uri     string
	}
	requests := make(chan received, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		requests <- received{r.Header.Clone(), body, err, r.URL.RequestURI()}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	// Only the destination is redirected. Use the builder's actual body and
	// headers through net/http, rather than serializing a second test payload.
	upstream.URL.Scheme = "http"
	upstream.URL.Host = strings.TrimPrefix(server.URL, "http://")
	client := server.Client()
	client.Timeout = 5 * time.Second
	response, err := client.Do(upstream)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	actual := <-requests
	require.NoError(t, actual.err)
	require.Equal(t, recorded.Path, actual.uri)
	require.JSONEq(t, recorded.Body, string(actual.body))
	require.Equal(t, sha256.Sum256([]byte(recorded.Body)), sha256.Sum256(actual.body), "actual HTTP body bytes changed")
	for _, pair := range recorded.Headers {
		key := pair[0]
		if !allowedHeaders[strings.ToLower(key)] || strings.EqualFold(key, "anthropic-beta") {
			continue
		}
		require.Equal(t, pair[1], actual.headers.Get(key), "actual HTTP receiver header %s", key)
	}
	for _, beta := range strings.Split(recordedHeader(recorded, "anthropic-beta"), ",") {
		if strings.TrimSpace(beta) != "" {
			require.True(t, containsBetaToken(actual.headers.Get("anthropic-beta"), strings.TrimSpace(beta)), "CLI beta removed: %s", beta)
		}
	}
}

func recordedHeader(request recordedCLIRequest, name string) string {
	for _, pair := range request.Headers {
		if strings.EqualFold(pair[0], name) {
			return pair[1]
		}
	}
	return ""
}

// Measure raw HTTP/1.1 header order separately from the semantic assertions.
// A successful measurement may still report differences; it is not a claim of
// complete wire equivalence. Neither side's authentication fields are saved.
func TestClaudeCLIRecordedRawHTTPHeaders(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "claude_cli", "ascii", "capture.json"))
	require.NoError(t, err)
	var capture recordedCLICapture
	require.NoError(t, json.Unmarshal(data, &capture))
	require.NotEmpty(t, capture.Requests)
	original := capture.Requests[0]
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(original.Method, original.Path, strings.NewReader(original.Body))
	for _, pair := range original.Headers {
		c.Request.Header.Add(pair[0], pair[1])
	}
	svc := &GatewayService{cfg: &config.Config{}}
	req, _, err := svc.buildUpstreamRequest(WithClaudeCodeAdmission(context.Background()), c,
		&Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}, []byte(original.Body), "synthetic-token", "oauth", "claude-sonnet-4-6", true, false)
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	type result struct {
		raw string
		err error
	}
	results := make(chan result, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			results <- result{err: err}
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		var raw bytes.Buffer
		incoming, err := http.ReadRequest(bufio.NewReader(io.TeeReader(conn, &raw)))
		if err == nil {
			_, err = io.Copy(io.Discard, incoming.Body)
			_ = incoming.Body.Close()
		}
		results <- result{raw.String(), err}
		_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}")
	}()
	req.URL.Scheme, req.URL.Host = "http", listener.Addr().String()
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	actual := <-results
	require.NoError(t, actual.err)
	var headers [][]string
	for _, line := range strings.Split(strings.SplitN(actual.raw, "\r\n\r\n", 2)[0], "\r\n")[1:] {
		key, value, ok := strings.Cut(line, ":")
		if ok && !strings.EqualFold(key, "authorization") && !strings.EqualFold(key, "x-api-key") && !strings.EqualFold(key, "cookie") {
			headers = append(headers, []string{key, strings.TrimSpace(value)})
		}
	}
	comparison := map[string]any{"cli_headers": original.Headers, "forwarded_headers": headers,
		"exact_sequence_equal": reflect.DeepEqual(original.Headers, headers),
		"allowed_differences":  "destination Host/Content-Length and OAuth beta; remaining order/casing/transport-added fields are measured discrepancies"}
	encoded, err := json.MarshalIndent(comparison, "", "  ")
	require.NoError(t, err)
	if output := os.Getenv("CLAUDE_CLI_HTTP_RESULT"); output != "" {
		require.NoError(t, os.WriteFile(output, append(encoded, '\n'), 0600))
	}
	t.Log(string(encoded))
}
