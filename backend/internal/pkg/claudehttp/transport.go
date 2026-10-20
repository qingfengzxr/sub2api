package claudehttp

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"sort"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"
	ftrace "github.com/bogdanfinn/fhttp/httptrace"
	"golang.org/x/net/http/httpguts"
)

// Transport adapts a pinned ordered HTTP/1 transport to net/http while leaving
// redirects, account tracking and response decompression with the caller.
// Proxy tunneling and verified target TLS are supplied by the base dialers.
type Transport struct{ inner *fhttp.Transport }

func NewTransport(base *http.Transport) *Transport {
	return &Transport{inner: &fhttp.Transport{
		DialContext: base.DialContext, DialTLSContext: base.DialTLSContext,
		ForceAttemptHTTP2: false, DisableCompression: true,
		MaxIdleConns: base.MaxIdleConns, MaxIdleConnsPerHost: base.MaxIdleConnsPerHost,
		MaxConnsPerHost: base.MaxConnsPerHost, IdleConnTimeout: base.IdleConnTimeout,
		ResponseHeaderTimeout:  base.ResponseHeaderTimeout,
		ExpectContinueTimeout:  base.ExpectContinueTimeout,
		MaxResponseHeaderBytes: base.MaxResponseHeaderBytes,
	}}
}

func (t *Transport) CloseIdleConnections() { t.inner.CloseIdleConnections() }

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	headers, err := orderedHeaders(req)
	if err != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, err
	}
	ctx := req.Context()
	var tlsState *tls.ConnectionState
	trace := httptrace.ContextClientTrace(ctx)
	ft := &ftrace.ClientTrace{GotConn: func(i ftrace.GotConnInfo) {
		if c, ok := i.Conn.(*tls.Conn); ok {
			state := c.ConnectionState()
			tlsState = &state
		}
		if trace != nil && trace.GotConn != nil {
			trace.GotConn(httptrace.GotConnInfo{Conn: i.Conn, Reused: i.Reused, WasIdle: i.WasIdle, IdleTime: i.IdleTime})
		}
	}}
	// Preserve the standard trace hooks used by connection reuse diagnostics.
	// DNS/connect hooks remain on the context consumed by the standard dialer.
	if trace != nil {
		ftCopy := &ftrace.ClientTrace{GetConn: trace.GetConn, PutIdleConn: trace.PutIdleConn,
			GotFirstResponseByte: trace.GotFirstResponseByte, Got100Continue: trace.Got100Continue,
			Got1xxResponse: trace.Got1xxResponse, WroteHeaderField: trace.WroteHeaderField,
			WroteHeaders: trace.WroteHeaders, Wait100Continue: trace.Wait100Continue}
		ftCopy.GotConn = ft.GotConn
		ft = ftCopy
		if trace.WroteRequest != nil {
			ft.WroteRequest = func(i ftrace.WroteRequestInfo) { trace.WroteRequest(httptrace.WroteRequestInfo{Err: i.Err}) }
		}
	}
	ctx = ftrace.WithClientTrace(ctx, ft)
	fr, err := fhttp.NewRequestWithContext(ctx, req.Method, req.URL.String(), nil)
	if err != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, err
	}
	fr.Header = headers
	fr.Host = req.Host
	fr.ContentLength = req.ContentLength
	fr.Close = req.Close
	fr.Body = req.Body
	if req.Body == http.NoBody {
		fr.Body = fhttp.NoBody
	}
	if req.GetBody != nil {
		fr.GetBody = func() (io.ReadCloser, error) {
			b, e := req.GetBody()
			if b == http.NoBody {
				return fhttp.NoBody, e
			}
			return b, e
		}
	}
	fr.Cancel = req.Cancel
	resp, err := t.inner.RoundTrip(fr)
	if err != nil {
		return nil, err
	}
	return &http.Response{Status: resp.Status, StatusCode: resp.StatusCode, Proto: resp.Proto,
		ProtoMajor: resp.ProtoMajor, ProtoMinor: resp.ProtoMinor, Header: http.Header(resp.Header),
		Body: resp.Body, ContentLength: resp.ContentLength, TransferEncoding: resp.TransferEncoding,
		Close: resp.Close, Uncompressed: resp.Uncompressed, Trailer: http.Header(resp.Trailer), Request: req, TLS: tlsState}, nil
}

var measuredCasing = map[string]string{
	"accept": "Accept", "authorization": "Authorization", "content-type": "Content-Type",
	"user-agent": "User-Agent", "x-claude-code-session-id": "X-Claude-Code-Session-Id",
	"x-stainless-arch": "X-Stainless-Arch", "x-stainless-lang": "X-Stainless-Lang",
	"x-stainless-os": "X-Stainless-OS", "x-stainless-package-version": "X-Stainless-Package-Version",
	"x-stainless-retry-count": "X-Stainless-Retry-Count", "x-stainless-runtime": "X-Stainless-Runtime",
	"x-stainless-runtime-version": "X-Stainless-Runtime-Version", "x-stainless-timeout": "X-Stainless-Timeout",
	"x-stainless-helper-method": "X-Stainless-Helper-Method",
	"anthropic-beta":            "anthropic-beta", "anthropic-dangerous-direct-browser-access": "anthropic-dangerous-direct-browser-access",
	"anthropic-version": "anthropic-version", "x-api-key": "x-api-key", "x-app": "x-app",
}

// The captured native CLI sorts the application headers by their wire spelling,
// then appends Connection, Host, Accept-Encoding, Content-Length. Auth and retry
// layouts are captured using synthetic credentials in testdata; OAuth lifecycle
// and unknown optional header casing are not inferred from those samples.
func orderedHeaders(req *http.Request) (fhttp.Header, error) {
	if len(req.TransferEncoding) != 0 || len(req.Trailer) != 0 || req.ContentLength < 0 ||
		(req.Body != nil && req.Body != http.NoBody && req.ContentLength == 0) {
		return nil, fmt.Errorf("%s requires a known-length body without request trailers", ProfileID)
	}
	host := req.Host
	if host == "" && req.URL != nil {
		host = req.URL.Host
	}
	if !httpguts.ValidHostHeader(host) {
		return nil, fmt.Errorf("invalid HTTP Host")
	}
	headers := make(fhttp.Header, len(req.Header)+5)
	seen := map[string]bool{}
	var names []string
	var acceptEncoding []string
	for key, values := range req.Header {
		lower := strings.ToLower(key)
		if !httpguts.ValidHeaderFieldName(key) {
			return nil, fmt.Errorf("invalid HTTP header name")
		}
		if seen[lower] {
			return nil, fmt.Errorf("ambiguous case variants for HTTP header %q", lower)
		}
		seen[lower] = true
		for _, value := range values {
			if !httpguts.ValidHeaderFieldValue(value) {
				return nil, fmt.Errorf("invalid HTTP header value for %q", lower)
			}
		}
		if len(values) == 0 {
			continue
		}
		switch lower {
		case "host", "content-length", "connection", "proxy-authorization", "proxy-connection":
			continue
		case "transfer-encoding", "trailer", "upgrade":
			return nil, fmt.Errorf("unsupported HTTP framing header %q", lower)
		case "accept-encoding":
			acceptEncoding = append([]string(nil), values...)
			continue
		}
		wire := key
		if measured, ok := measuredCasing[lower]; ok {
			wire = measured
		}
		headers[wire] = append([]string(nil), values...)
		names = append(names, wire)
	}
	if len(headers["User-Agent"]) != 1 || headers["User-Agent"][0] == "" {
		return nil, fmt.Errorf("%s requires User-Agent", ProfileID)
	}
	sort.Strings(names)
	order := make([]string, 0, len(names)+4)
	for _, name := range names {
		order = append(order, strings.ToLower(name))
	}
	headers["Connection"] = []string{"keep-alive"}
	if req.Close {
		headers["Connection"] = []string{"close"}
	}
	if len(acceptEncoding) == 0 {
		acceptEncoding = []string{"gzip, deflate, br, zstd"}
	}
	headers["Accept-Encoding"] = acceptEncoding
	// Host and Content-Length are generated by the writer from URL/Host and
	// the actual body length. Caller-supplied framing headers cannot override them.
	headers[fhttp.HeaderOrderKey] = append(order, "connection", "host", "accept-encoding", "content-length")
	return headers, nil
}
