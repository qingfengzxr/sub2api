package tlsfingerprint

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// Capture the target ClientHello inside real HTTP and trusted HTTPS CONNECT
// proxies. The tunnel is closed before target authentication. This does not
// test proxy authentication or an end-to-end authenticated target response.
func TestCaptureInstalledCLIProxyClientHello(t *testing.T) {
	cli := os.Getenv("CLAUDE_CLI_BINARY")
	if cli == "" {
		t.Skip("set CLAUDE_CLI_BINARY for local proxy measurements")
	}
	directHome := t.TempDir()
	direct := captureSampleHello(t, func(ctx context.Context, address string) error {
		cmd := exec.CommandContext(ctx, cli, "--bare", "-p", "Reply OK", "--tools", "", "--no-session-persistence", "--setting-sources", "", "--disable-slash-commands", "--strict-mcp-config")
		cmd.Dir = directHome
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + directHome, "CLAUDE_CONFIG_DIR=" + filepath.Join(directHome, "config"),
			"ANTHROPIC_API_KEY=synthetic-key", "ANTHROPIC_BASE_URL=https://" + address,
			"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "DISABLE_AUTOUPDATER=1", "NO_PROXY=localhost,127.0.0.1"}
		return cmd.Run()
	})
	measurements := map[string]any{}
	for _, secure := range []bool{false, true} {
		name := "http_proxy"
		if secure {
			name = "https_proxy"
		}
		t.Run(name, func(t *testing.T) {
			type result struct {
				hello sampleClientHello
				host  string
				err   error
			}
			captured := make(chan result, 1)
			proxy := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodConnect {
					http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
					captured <- result{err: fmt.Errorf("unexpected proxy method %s", r.Method)}
					return
				}
				conn, buffered, err := w.(http.Hijacker).Hijack()
				if err != nil {
					captured <- result{err: err}
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				_, _ = io.WriteString(buffered, "HTTP/1.1 200 Connection Established\r\n\r\n")
				_ = buffered.Flush()
				hello, err := readSampleClientHello(buffered.Reader)
				captured <- result{hello: hello, host: r.Host, err: err}
			}))
			if secure {
				proxy.StartTLS()
			} else {
				proxy.Start()
			}
			defer proxy.Close()
			home := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, cli, "--bare", "-p", "Reply OK", "--tools", "", "--no-session-persistence", "--setting-sources", "", "--disable-slash-commands", "--strict-mcp-config")
			cmd.Dir = home
			cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "CLAUDE_CONFIG_DIR=" + filepath.Join(home, "config"),
				"ANTHROPIC_API_KEY=synthetic-key", "ANTHROPIC_BASE_URL=https://claude-sdk-audit.invalid",
				"HTTPS_PROXY=" + proxy.URL, "NO_PROXY=", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "DISABLE_AUTOUPDATER=1"}
			if secure {
				certPath := filepath.Join(home, "proxy-ca.pem")
				if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: proxy.Certificate().Raw}), 0600); err != nil {
					t.Fatal(err)
				}
				cmd.Env = append(cmd.Env, "NODE_EXTRA_CA_CERTS="+certPath)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Run() }()
			var measured result
			select {
			case measured = <-captured:
				cancel()
				<-done
			case err := <-done:
				t.Fatalf("CLI exited before tunnel ClientHello: %v", err)
			case <-ctx.Done():
				cancel()
				<-done
				t.Fatal("timed out waiting for CONNECT tunnel ClientHello")
			}
			if measured.err != nil {
				t.Fatal(measured.err)
			}
			if measured.host != "claude-sdk-audit.invalid:443" {
				t.Fatalf("unexpected CONNECT target %q", measured.host)
			}
			matches := reflect.DeepEqual(direct, measured.hello)
			measurements[name] = map[string]any{"target": measured.host, "proxy_certificate_verification": secure, "client_hello": measured.hello, "matches_direct_measured_fields": matches}
			if !matches {
				t.Errorf("target ClientHello through %s differs from same CLI direct capture: direct=%+v proxy=%+v", name, direct, measured.hello)
			}
		})
	}
	if output := os.Getenv("CLAUDE_CLI_PROXY_RESULT"); output != "" {
		data, err := json.MarshalIndent(measurements, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
