package repository

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claudehttp"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

func buildClaudeHTTP1Transport(settings poolSettings, proxyURL *url.URL, profile *tlsfingerprint.Profile) (http.RoundTripper, error) {
	if profile != nil {
		for _, protocol := range profile.ALPNProtocols {
			if protocol != "http/1.1" {
				return nil, fmt.Errorf("%s requires HTTP/1.1 ALPN", claudehttp.ProfileID)
			}
		}
	}
	base, err := buildUpstreamTransportWithTLSFingerprint(settings, proxyURL, profile)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		// Header ordering does not opt an account into TLS fingerprint simulation.
		// Use standard Go TLS over the same verified proxy tunnel instead.
		base.DialTLSContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			conn, err := base.DialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				conn.Close()
				return nil, err
			}
			client := tls.Client(conn, &tls.Config{ServerName: host, NextProtos: []string{"http/1.1"}})
			trace := httptrace.ContextClientTrace(ctx)
			if trace != nil && trace.TLSHandshakeStart != nil {
				trace.TLSHandshakeStart()
			}
			err = client.HandshakeContext(ctx)
			if trace != nil && trace.TLSHandshakeDone != nil {
				trace.TLSHandshakeDone(client.ConnectionState(), err)
			}
			if err != nil {
				conn.Close()
				return nil, err
			}
			return client, nil
		}
	}
	return claudehttp.NewTransport(base), nil
}
