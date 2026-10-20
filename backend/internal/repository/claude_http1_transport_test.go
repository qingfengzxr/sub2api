package repository

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claudehttp"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type orderedOriginRecord struct {
	Names      []string
	Body       string
	Connection int64
}

func orderedTLSOrigin(t *testing.T, certificate tls.Certificate) string {
	t.Helper()
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{certificate}, NextProtos: []string{"http/1.1"}})
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })
	var id atomic.Int64
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			n := id.Add(1)
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
				br := bufio.NewReader(conn)
				for {
					var raw strings.Builder
					var names []string
					line, err := br.ReadString('\n')
					if err != nil {
						return
					}
					raw.WriteString(line)
					for {
						line, err = br.ReadString('\n')
						if err != nil {
							return
						}
						raw.WriteString(line)
						if line == "\r\n" {
							break
						}
						name, _, _ := strings.Cut(line, ":")
						names = append(names, name)
					}
					req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(raw.String())))
					if err != nil {
						t.Error(err)
						return
					}
					if req.ContentLength < 0 {
						t.Error("unexpected streaming request framing")
						return
					}
					body := make([]byte, req.ContentLength)
					if _, err := io.ReadFull(br, body); err != nil {
						t.Error(err)
						return
					}
					if req.Header.Get("Proxy-Authorization") != "" {
						t.Error("proxy credentials reached target")
					}
					result, _ := json.Marshal(orderedOriginRecord{names, string(body), n})
					_, err = fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", len(result), result)
					if err != nil {
						return
					}
				}
			}()
		}
	}()
	return "https://" + listener.Addr().String()
}

func TestClaudeOrderedHTTP1ProxyRoutes(t *testing.T) {
	if os.Getenv("SUB2API_TEST_ORDERED_ROOTS") != "1" {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestClaudeOrderedHTTP1ProxyRoutes$", "-test.v")
		cmd.Env = append(os.Environ(), "SUB2API_TEST_ORDERED_ROOTS=1")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", out)
		t.Logf("%s", out)
		return
	}
	ca := httptest.NewTLSServer(http.NotFoundHandler())
	certificate := ca.TLS.Certificates[0]
	roots := x509.NewCertPool()
	roots.AddCert(ca.Certificate())
	ca.Close()
	t.Setenv("GODEBUG", os.Getenv("GODEBUG")+",x509usefallbackroots=1")
	x509.SetFallbackRoots(roots)
	fixtureBytes, err := os.ReadFile("../service/testdata/claude_cli/ascii/capture.json")
	require.NoError(t, err)
	var fixture struct {
		Requests []struct {
			Headers [][]string
			Body    string
		}
	}
	require.NoError(t, json.Unmarshal(fixtureBytes, &fixture))
	sample := fixture.Requests[0]
	expected := map[string][]string{}
	data, err := os.ReadFile("../pkg/claudehttp/testdata/cli-2.1.301-header-order.json")
	require.NoError(t, err)
	var captured struct {
		Cases []struct {
			Auth     string
			Extra    bool
			Retry    int
			HTTPS    bool
			Requests []struct {
				Names []string `json:"header_names"`
			}
		}
	}
	require.NoError(t, json.Unmarshal(data, &captured))
	for _, c := range captured.Cases {
		if c.HTTPS && !c.Extra && c.Retry == 0 {
			expected[c.Auth] = c.Requests[0].Names
		}
	}
	for _, route := range []string{"direct", "http", "https", "socks5", "socks5h"} {
		for _, fingerprint := range []bool{false, true} {
			for _, auth := range []string{"api_key", "bearer"} {
				t.Run(fmt.Sprintf("%s/tls_profile=%t/%s", route, fingerprint, auth), func(t *testing.T) {
					endpoint := orderedTLSOrigin(t, certificate)
					target := strings.TrimPrefix(endpoint, "https://")
					proxyAddress := ""
					switch route {
					case "http", "https":
						proxyAddress, _ = fingerprintTestCONNECTProxy(t, route == "https", target)
					case "socks5", "socks5h":
						proxyAddress, _ = startTestSOCKS5Proxy(t)
						proxyAddress = strings.Replace(proxyAddress, "socks5h://", route+"://", 1)
					}
					upstream := NewHTTPUpstream(nil).(*httpUpstreamService)
					defer func() {
						for _, e := range upstream.clients {
							e.client.CloseIdleConnections()
						}
					}()
					for attempt := 0; attempt < 2; attempt++ {
						ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
						req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/messages?beta=true", strings.NewReader(sample.Body))
						require.NoError(t, err)
						for _, pair := range sample.Headers {
							req.Header.Add(pair[0], pair[1])
						}
						if auth == "api_key" {
							req.Header.Set("X-Api-Key", "synthetic-target")
						} else {
							req.Header.Set("Authorization", "Bearer synthetic-target")
						}
						req = claudehttp.WithProfile(req)
						require.True(t, claudehttp.Enabled(req))
						var resp *http.Response
						if fingerprint {
							resp, err = upstream.DoWithTLS(req, proxyAddress, 1, 1, &tlsfingerprint.Profile{})
						} else {
							resp, err = upstream.Do(req, proxyAddress, 1, 1)
						}
						require.NoError(t, err)
						if !fingerprint {
							require.NotNil(t, resp.TLS)
							require.NotEmpty(t, resp.TLS.VerifiedChains)
						}
						var got orderedOriginRecord
						require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
						_, err = io.Copy(io.Discard, resp.Body)
						require.NoError(t, err)
						require.NoError(t, resp.Body.Close())
						cancel()
						require.Equal(t, expected[auth], got.Names)
						require.Equal(t, sample.Body, got.Body)
						require.Equal(t, int64(1), got.Connection)
						requireNoUpstreamInFlight(t, upstream)
					}
				})
			}
		}
	}
}

func TestClaudeOrderedPoolIsolation(t *testing.T) {
	upstream := NewHTTPUpstream(nil).(*httpUpstreamService)
	defer func() {
		for _, e := range upstream.clients {
			e.client.CloseIdleConnections()
		}
	}()
	standard, err := upstream.getClientEntry("", 1, 1, "", false, true)
	require.NoError(t, err)
	ordered, err := upstream.getClientEntry("", 1, 1, "", false, true, true)
	require.NoError(t, err)
	require.NotSame(t, standard, ordered)
	require.IsType(t, &claudehttp.Transport{}, ordered.client.Transport)
	again, err := upstream.getClientEntry("", 1, 1, "", false, true, true)
	require.NoError(t, err)
	require.Same(t, ordered, again)
	_, err = buildClaudeHTTP1Transport(poolSettings{}, nil, &tlsfingerprint.Profile{ALPNProtocols: []string{"h2", "http/1.1"}})
	require.ErrorContains(t, err, "HTTP/1.1 ALPN")
}
