package tlsfingerprint

import (
	"bufio"
	"context"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProxyDialerClientHello(t *testing.T) {
	if os.Getenv("SUB2API_TEST_PROXY_ROOTS") != "1" {
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProxyDialerClientHello$", "-test.v")
		cmd.Env = append(os.Environ(), "SUB2API_TEST_PROXY_ROOTS=1")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", out)
		t.Logf("%s", out)
		return
	}
	ca := httptest.NewTLSServer(http.NotFoundHandler())
	roots := x509.NewCertPool()
	roots.AddCert(ca.Certificate())
	ca.Close()
	t.Setenv("GODEBUG", os.Getenv("GODEBUG")+",x509usefallbackroots=1")
	x509.SetFallbackRoots(roots)
	for _, profile := range []*Profile{{}, {CipherSuites: []uint16{4867, 4865, 4866}, Curves: []uint16{29, 23}}} {
		direct := captureSampleHello(t, func(ctx context.Context, address string) error {
			conn, err := NewDialer(profile, nil).DialTLSContext(ctx, "tcp", address)
			if conn != nil {
				conn.Close()
			}
			return err
		})
		for _, secure := range []bool{false, true} {
			name := "http"
			if secure {
				name = "https"
			}
			t.Run(name, func(t *testing.T) {
				type result struct {
					hello sampleClientHello
					err   error
				}
				captured := make(chan result, 1)
				proxy := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodConnect || r.Host != "target.invalid:443" || r.Header.Get("Proxy-Authorization") != "Basic dXNlcjpwYXNz" {
						t.Error("invalid CONNECT")
						w.WriteHeader(407)
						return
					}
					c, br, err := w.(http.Hijacker).Hijack()
					if err != nil {
						captured <- result{err: err}
						return
					}
					defer c.Close()
					_ = c.SetDeadline(time.Now().Add(5 * time.Second))
					_, _ = io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
					h, err := readSampleClientHello(br.Reader)
					captured <- result{h, err}
				}))
				if secure {
					proxy.StartTLS()
				} else {
					proxy.Start()
				}
				defer proxy.Close()
				parsed, err := url.Parse(proxy.URL)
				require.NoError(t, err)
				parsed.User = url.UserPassword("user", "pass")
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				_, err = NewHTTPProxyDialer(profile, parsed).DialTLSContext(ctx, "tcp", "target.invalid:443")
				require.Error(t, err, "probe stops before target certificate exchange")
				select {
				case got := <-captured:
					require.NoError(t, got.err)
					require.True(t, reflect.DeepEqual(direct, got.hello), "direct=%+v tunnel=%+v", direct, got.hello)
				default:
					t.Fatal("no target ClientHello captured")
				}
			})
		}
	}
}

func TestHTTPSProxyRejectsUntrustedCertificate(t *testing.T) {
	proxy := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("CONNECT sent before verifying proxy certificate")
	}))
	defer proxy.Close()
	parsed, err := url.Parse(proxy.URL)
	require.NoError(t, err)
	parsed.User = url.UserPassword("user", "pass")
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	conn, err := NewHTTPProxyDialer(nil, parsed).DialTLSContext(ctx, "tcp", "target.invalid:443")
	require.Nil(t, conn)
	require.ErrorContains(t, err, "proxy TLS handshake")
	var unknown x509.UnknownAuthorityError
	require.ErrorAs(t, err, &unknown)
}

func TestProxyDialerCancellation(t *testing.T) {
	for _, stage := range []string{"http_connect", "https_handshake", "socks5_negotiation", "target_handshake"} {
		for _, deadline := range []bool{false, true} {
			name := stage + "/cancel"
			if deadline {
				name = stage + "/deadline"
			}
			t.Run(name, func(t *testing.T) {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				defer listener.Close()
				ready := make(chan struct{})
				closed := make(chan struct{})
				go func() {
					c, err := listener.Accept()
					if err != nil {
						return
					}
					defer c.Close()
					defer close(closed)
					_ = c.SetDeadline(time.Now().Add(4 * time.Second))
					reader := bufio.NewReader(c)
					if stage == "http_connect" {
						_, _ = http.ReadRequest(reader)
					}
					if stage == "https_handshake" || stage == "target_handshake" {
						_, _ = readSampleClientHello(reader)
					}
					if stage == "socks5_negotiation" {
						greeting := make([]byte, 3)
						_, _ = io.ReadFull(reader, greeting)
					}
					close(ready)
					_, _ = io.Copy(io.Discard, reader)
				}()
				var ctx context.Context
				var cancel context.CancelFunc
				if deadline {
					ctx, cancel = context.WithTimeout(t.Context(), 150*time.Millisecond)
				} else {
					ctx, cancel = context.WithCancel(t.Context())
				}
				defer cancel()
				done := make(chan error, 1)
				go func() {
					var c net.Conn
					var err error
					if stage == "target_handshake" {
						c, err = NewDialer(nil, nil).DialTLSContext(ctx, "tcp", listener.Addr().String())
					} else {
						scheme := "http"
						if stage == "https_handshake" {
							scheme = "https"
						}
						if stage == "socks5_negotiation" {
							scheme = "socks5h"
						}
						parsed, _ := url.Parse(scheme + "://" + listener.Addr().String())
						if stage == "socks5_negotiation" {
							c, err = NewSOCKS5ProxyDialer(nil, parsed).DialTLSContext(ctx, "tcp", "target.invalid:443")
						} else {
							c, err = NewHTTPProxyDialer(nil, parsed).DialTLSContext(ctx, "tcp", "target.invalid:443")
						}
					}
					if c != nil {
						c.Close()
					}
					done <- err
				}()
				select {
				case <-ready:
				case <-time.After(2 * time.Second):
					t.Fatal("dialer never reached stalled stage")
				}
				if !deadline {
					cancel()
				}
				select {
				case err := <-done:
					require.Error(t, err)
				case <-time.After(time.Second):
					t.Fatal("cancellation did not stop connection setup")
				}
				select {
				case <-closed:
				case <-time.After(time.Second):
					t.Fatal("failed connection leaked")
				}
			})
		}
	}
}

func TestHTTPProxyDialerRejectsCONNECTFailure(t *testing.T) {
	for _, status := range []int{407, 502} {
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		parsed, _ := url.Parse(proxy.URL)
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		conn, err := NewHTTPProxyDialer(nil, parsed).DialTLSContext(ctx, "tcp", "target.invalid:443")
		cancel()
		proxy.Close()
		require.Nil(t, conn)
		require.ErrorContains(t, err, "proxy CONNECT failed")
	}
}

func TestHTTPProxyDialerPreservesBufferedBytes(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close()
		_, _ = io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\nready")
	}))
	defer proxy.Close()
	parsed, _ := url.Parse(proxy.URL)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	conn, err := NewHTTPProxyDialer(nil, parsed).DialContext(ctx, "tcp", "target.invalid:80")
	require.NoError(t, err)
	defer conn.Close()
	bytes, err := io.ReadAll(conn)
	require.NoError(t, err)
	require.Equal(t, "ready", string(bytes))
}
