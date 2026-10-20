package repository

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Fallback roots are process-global. Isolate the CA in a child process.
func TestFingerprintTransportProxyRoutes(t *testing.T) {
	if os.Getenv("SUB2API_TEST_FINGERPRINT_ROOTS") != "1" {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestFingerprintTransportProxyRoutes$", "-test.v")
		cmd.Env = append(os.Environ(), "SUB2API_TEST_FINGERPRINT_ROOTS=1")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", output)
		t.Logf("%s", output)
		return
	}
	certServer := httptest.NewTLSServer(http.NotFoundHandler())
	roots := x509.NewCertPool()
	roots.AddCert(certServer.Certificate())
	certServer.Close()
	t.Setenv("GODEBUG", os.Getenv("GODEBUG")+",x509usefallbackroots=1")
	x509.SetFallbackRoots(roots)
	for _, route := range []string{"direct", "http", "https", "socks5", "socks5h"} {
		t.Run(route, func(t *testing.T) {
			hellos := make(chan *tls.ClientHelloInfo, 4)
			var requests atomic.Int64
			target := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Header.Get("Proxy-Authorization") != "" || r.Header.Get("Authorization") != "Bearer synthetic-target" {
					t.Error("target authentication leaked or missing")
				}
				_, _ = io.WriteString(w, "data: {\"type\":\"message_stop\"}\n\n")
			}))
			target.TLS = &tls.Config{GetConfigForClient: func(h *tls.ClientHelloInfo) (*tls.Config, error) { hellos <- h; return nil, nil }}
			target.StartTLS()
			defer target.Close()
			proxyAddress := ""
			var calls *atomic.Int64
			switch route {
			case "http", "https":
				proxyAddress, calls = fingerprintTestCONNECTProxy(t, route == "https", target.Listener.Addr().String())
			case "socks5", "socks5h":
				proxyAddress, calls = startTestSOCKS5Proxy(t)
				proxyAddress = strings.Replace(proxyAddress, "socks5h://", route+"://", 1)
			}
			upstream := NewHTTPUpstream(nil).(*httpUpstreamService)
			defer func() {
				for _, entry := range upstream.clients {
					entry.client.CloseIdleConnections()
				}
			}()
			profile := &tlsfingerprint.Profile{Name: "measured-cli"}
			send := func() {
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer synthetic-target")
				resp, err := upstream.DoWithTLS(req, proxyAddress, 91, 1, profile)
				require.NoError(t, err)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
				require.Equal(t, "data: {\"type\":\"message_stop\"}\n\n", string(body))
			}
			send()
			select {
			case hello := <-hellos:
				require.Len(t, hello.CipherSuites, 17, "must not fall back to Go TLS")
				require.Equal(t, []uint16{4865, 4866, 4867}, hello.CipherSuites[:3])
				require.Equal(t, []string{"http/1.1"}, hello.SupportedProtos)
			default:
				t.Fatal("missing ClientHello")
			}
			send()
			require.Empty(t, hellos, "same profile should reuse its connection")
			if calls != nil {
				require.Equal(t, int64(1), calls.Load())
			}
			profile.CipherSuites = []uint16{4867, 4865, 4866}
			send()
			select {
			case hello := <-hellos:
				require.Equal(t, profile.CipherSuites, hello.CipherSuites)
			default:
				t.Fatal("changed profile reused old connection")
			}
			require.Equal(t, int64(3), requests.Load())
			if calls != nil {
				require.Equal(t, int64(2), calls.Load())
			}
		})
	}
	t.Run("proxy_hostname_rejected", func(t *testing.T) {
		proxyAddress, calls := fingerprintTestCONNECTProxy(t, true, "unused.invalid:443")
		parsed, err := url.Parse(proxyAddress)
		require.NoError(t, err)
		parsed.Host = net.JoinHostPort("localhost", parsed.Port()) // cert covers IP, not localhost
		tr, err := buildUpstreamTransportWithTLSFingerprint(poolSettings{}, parsed, &tlsfingerprint.Profile{})
		require.NoError(t, err)
		defer tr.CloseIdleConnections()
		_, err = (&http.Client{Transport: tr, Timeout: 3 * time.Second}).Get("https://unused.invalid")
		require.ErrorContains(t, err, "proxy TLS handshake")
		var hostnameError x509.HostnameError
		require.ErrorAs(t, err, &hostnameError)
		require.Zero(t, calls.Load(), "CONNECT credentials must follow proxy verification")
	})
	t.Run("target_hostname_rejected", func(t *testing.T) {
		target := httptest.NewTLSServer(http.NotFoundHandler())
		defer target.Close()
		_, port, _ := net.SplitHostPort(target.Listener.Addr().String())
		targetHost := net.JoinHostPort("localhost", port)
		proxyAddress, _ := fingerprintTestCONNECTProxy(t, true, targetHost)
		parsed, _ := url.Parse(proxyAddress)
		tr, err := buildUpstreamTransportWithTLSFingerprint(poolSettings{}, parsed, &tlsfingerprint.Profile{})
		require.NoError(t, err)
		defer tr.CloseIdleConnections()
		_, err = (&http.Client{Transport: tr, Timeout: 3 * time.Second}).Get("https://" + targetHost)
		var hostnameError x509.HostnameError
		require.ErrorAs(t, err, &hostnameError)
	})
	t.Run("plaintext_redirect_keeps_proxy", func(t *testing.T) {
		const destination = "redirect-target.invalid:80"
		var plainCalls atomic.Int64
		target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://"+destination+"/done", http.StatusFound)
		}))
		defer target.Close()
		proxy := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Host == destination {
				plainCalls.Add(1)
				c, buffered, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(3 * time.Second))
				_, _ = io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
				// Respond only after receiving the tunneled HTTP request.
				// Sending unsolicited HTTP data races net/http's idle reader.
				tunneled, err := http.ReadRequest(buffered.Reader)
				if err != nil {
					t.Error(err)
					return
				}
				_ = tunneled.Body.Close()
				_, _ = io.WriteString(c, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nOK")
				return
			}
			fingerprintRelayCONNECT(t, w, r, target.Listener.Addr().String())
		}))
		defer proxy.Close()
		parsed, _ := url.Parse(proxy.URL)
		tr, err := buildUpstreamTransportWithTLSFingerprint(poolSettings{}, parsed, &tlsfingerprint.Profile{})
		require.NoError(t, err)
		defer tr.CloseIdleConnections()
		resp, err := (&http.Client{Transport: tr, Timeout: 5 * time.Second}).Get(target.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "OK", string(body))
		require.Equal(t, int64(1), plainCalls.Load())
	})
}

func fingerprintTestCONNECTProxy(t *testing.T, secure bool, target string) (string, *atomic.Int64) {
	t.Helper()
	calls := &atomic.Int64{}
	proxy := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Proxy-Authorization") != "Basic dXNlcjpwYXNz" || r.Header.Get("Authorization") != "" {
			t.Error("proxy auth missing or target auth leaked")
			w.WriteHeader(407)
			return
		}
		fingerprintRelayCONNECT(t, w, r, target)
	}))
	if secure {
		proxy.StartTLS()
	} else {
		proxy.Start()
	}
	t.Cleanup(proxy.Close)
	parsed, err := url.Parse(proxy.URL)
	require.NoError(t, err)
	parsed.User = url.UserPassword("user", "pass")
	return parsed.String(), calls
}

func fingerprintRelayCONNECT(t *testing.T, w http.ResponseWriter, r *http.Request, target string) {
	t.Helper()
	if r.Method != http.MethodConnect || r.Host != target {
		t.Errorf("unexpected proxy request %s %s", r.Method, r.Host)
		w.WriteHeader(400)
		return
	}
	remote, err := net.DialTimeout("tcp", target, 3*time.Second)
	if err != nil {
		t.Error(err)
		w.WriteHeader(502)
		return
	}
	defer remote.Close()
	client, buffered, err := w.(http.Hijacker).Hijack()
	if err != nil {
		t.Error(err)
		return
	}
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(10 * time.Second))
	_ = remote.SetDeadline(time.Now().Add(10 * time.Second))
	_, _ = io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n")
	done := make(chan struct{})
	go func() { _, _ = io.Copy(remote, buffered); _ = remote.Close(); close(done) }()
	_, _ = io.Copy(client, remote)
	_ = client.Close()
	<-done
}

func TestFingerprintTransportRejectsUnsupportedProxy(t *testing.T) {
	parsed, _ := url.Parse("ftp://proxy.invalid")
	_, err := buildUpstreamTransportWithTLSFingerprint(poolSettings{}, parsed, &tlsfingerprint.Profile{})
	require.ErrorContains(t, err, "unsupported")
}

func TestFingerprintPoolProfileIsolation(t *testing.T) {
	for _, isolation := range []string{config.ConnectionPoolIsolationAccount, config.ConnectionPoolIsolationProxy, config.ConnectionPoolIsolationAccountProxy} {
		t.Run(isolation, func(t *testing.T) {
			cfg := &config.Config{Gateway: config.GatewayConfig{ConnectionPoolIsolation: isolation}}
			upstream := NewHTTPUpstream(cfg).(*httpUpstreamService)
			defer func() {
				for _, entry := range upstream.clients {
					entry.client.CloseIdleConnections()
				}
			}()
			a := &tlsfingerprint.Profile{Name: "same", Curves: []uint16{29, 23}}
			first, err := upstream.getClientEntryWithTLS("", 1, 1, a, service.HTTPUpstreamProfileDefault, false, true)
			require.NoError(t, err)
			b := &tlsfingerprint.Profile{Name: "same", Curves: []uint16{29, 23}}
			same, err := upstream.getClientEntryWithTLS("", 1, 1, b, service.HTTPUpstreamProfileDefault, false, true)
			require.NoError(t, err)
			require.Same(t, first, same)
			b.Curves[0] = 24
			changed, err := upstream.getClientEntryWithTLS("", 1, 1, b, service.HTTPUpstreamProfileDefault, false, true)
			require.NoError(t, err)
			require.NotSame(t, first, changed)
		})
	}
}
