package tlsfingerprint

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	utls "github.com/refraction-networking/utls"
)

type sampleClientHello struct {
	LegacyVersion       uint16   `json:"legacy_version"`
	CipherSuites        []uint16 `json:"cipher_suites"`
	Extensions          []uint16 `json:"extensions"`
	Groups              []uint16 `json:"groups"`
	SignatureAlgorithms []uint16 `json:"signature_algorithms"`
	ALPN                []string `json:"alpn"`
	Versions            []uint16 `json:"supported_versions"`
	KeyShares           []uint16 `json:"key_shares"`
	PointFormats        []uint16 `json:"point_formats"`
	PSKModes            []uint16 `json:"psk_modes"`
}

// This is an opt-in measurement, not a claim that Go or the built-in profile
// matches the installed CLI. The local listener only reads the public hello;
// no credentials are sent, and certificate verification is never disabled.
func TestCaptureInstalledCLIClientHello(t *testing.T) {
	cli := os.Getenv("CLAUDE_CLI_BINARY")
	if cli == "" {
		t.Skip("set CLAUDE_CLI_BINARY to capture local CLI and transport hellos")
	}
	cli, err := filepath.Abs(cli)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	measurements := map[string]sampleClientHello{}
	measurements["cli"] = captureSampleHello(t, func(ctx context.Context, address string) error {
		cmd := exec.CommandContext(ctx, cli, "--bare", "-p", "Reply OK", "--tools", "", "--no-session-persistence", "--setting-sources", "", "--disable-slash-commands")
		cmd.Dir = home
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "CLAUDE_CONFIG_DIR=" + filepath.Join(home, "config"),
			"ANTHROPIC_API_KEY=local-synthetic-key", "ANTHROPIC_BASE_URL=https://" + address,
			"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "DISABLE_AUTOUPDATER=1", "NO_PROXY=localhost,127.0.0.1"}
		return cmd.Run()
	})
	measurements["go_default"] = captureSampleHello(t, func(ctx context.Context, address string) error {
		dialer := tls.Dialer{Config: &tls.Config{ServerName: "localhost", NextProtos: []string{"h2", "http/1.1"}}}
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if conn != nil {
			_ = conn.Close()
		}
		return err
	})
	measurements["builtin_profile"] = captureSampleHello(t, func(ctx context.Context, address string) error {
		conn, err := NewDialer(&Profile{}, nil).DialTLSContext(ctx, "tcp", address)
		if conn != nil {
			_ = conn.Close()
		}
		return err
	})
	comparison := map[string]any{"measurement": "ClientHello only; handshakes deliberately stopped before authentication", "measurements": measurements}
	for _, name := range []string{"go_default", "builtin_profile"} {
		matches := reflect.DeepEqual(measurements["cli"], measurements[name])
		comparison[name+"_matches_cli"] = matches
		t.Logf("%s matches installed CLI public ClientHello fields: %t", name, matches)
	}
	data, err := json.MarshalIndent(comparison, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if output := os.Getenv("CLAUDE_CLI_TLS_RESULT"); output != "" {
		if err := os.WriteFile(output, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Log(string(data))
}

func TestBuiltinProfileObservedCLI301(t *testing.T) {
	// Fixed public fields captured from the CLI binary with SHA-256
	// 1f04c69e1c24e4d0a716a863af0aad8fc64d49a499334c29701f073700276b99.
	want := sampleClientHello{
		LegacyVersion:       771,
		CipherSuites:        []uint16{4865, 4866, 4867, 49195, 49199, 49196, 49200, 52393, 52392, 49161, 49171, 49162, 49172, 156, 157, 47, 53},
		Extensions:          []uint16{0, 23, 65281, 10, 11, 35, 16, 5, 13, 18, 51, 45, 43},
		Groups:              []uint16{4588, 29, 23, 24},
		SignatureAlgorithms: []uint16{1027, 2052, 1025, 1283, 2053, 1281, 2054, 1537, 513},
		ALPN:                []string{"http/1.1"},
		Versions:            []uint16{772, 771},
		KeyShares:           []uint16{4588, 29},
		PointFormats:        []uint16{0},
		PSKModes:            []uint16{1},
	}
	actual := captureSampleHello(t, func(ctx context.Context, address string) error {
		_, err := NewDialer(&Profile{}, nil).DialTLSContext(ctx, "tcp", address)
		return err
	})
	if !reflect.DeepEqual(want, actual) {
		t.Fatalf("actual ClientHello differs from recorded CLI fields: got %+v; want %+v", actual, want)
	}
}

func TestBuiltinProfileTrustedLocalTLSHandshake(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	host, _, _ := net.SplitHostPort(server.Listener.Addr().String())
	client := utls.UClient(conn, &utls.Config{RootCAs: roots, ServerName: host}, utls.HelloCustom)
	if err := client.ApplyPreset(buildClientHelloSpecFromProfile(nil)); err != nil {
		t.Fatal(err)
	}
	if err := client.Handshake(); err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := request.Write(client); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(client), request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || string(body) != "OK" {
		t.Fatalf("local verified TLS request failed: body=%q err=%v", body, err)
	}
}

func TestProfileDefaultKeySharesRespectCustomCurves(t *testing.T) {
	for _, curves := range [][]uint16{{29, 23, 24}, {23}} {
		spec := buildClientHelloSpecFromProfile(&Profile{Curves: curves})
		for _, extension := range spec.Extensions {
			shares, ok := extension.(*utls.KeyShareExtension)
			if !ok {
				continue
			}
			if len(shares.KeyShares) == 0 {
				t.Fatal("profile has no usable key share")
			}
			for _, share := range shares.KeyShares {
				found := false
				for _, curve := range curves {
					found = found || uint16(share.Group) == curve
				}
				if !found {
					t.Fatalf("default share %d not advertised by custom curves %v", share.Group, curves)
				}
			}
		}
	}
}

func captureSampleHello(t *testing.T, run func(context.Context, string) error) sampleClientHello {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	type result struct {
		hello sampleClientHello
		err   error
	}
	results := make(chan result, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			results <- result{err: err}
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(8 * time.Second))
		hello, err := readSampleClientHello(conn)
		results <- result{hello, err}
	}()
	done := make(chan error, 1)
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	go func() { done <- run(ctx, net.JoinHostPort("localhost", port)) }()
	select {
	case result := <-results:
		cancel()
		<-done
		if result.err != nil {
			t.Fatal(result.err)
		}
		return result.hello
	case err := <-done:
		cancel()
		_ = listener.Close()
		result := <-results
		if result.err != nil {
			t.Fatalf("client exited before a complete hello: %v; capture: %v", err, result.err)
		}
		return result.hello
	case <-ctx.Done():
		cancel()
		_ = listener.Close()
		<-done
		<-results
		t.Fatal("timed out waiting for a local ClientHello")
		return sampleClientHello{}
	}
}

func readSampleClientHello(conn io.Reader) (sampleClientHello, error) {
	var handshake []byte
	for {
		var header [5]byte
		if _, err := io.ReadFull(conn, header[:]); err != nil {
			return sampleClientHello{}, err
		}
		if header[0] != 22 {
			return sampleClientHello{}, fmt.Errorf("unexpected TLS record %d", header[0])
		}
		payload := make([]byte, binary.BigEndian.Uint16(header[3:]))
		if _, err := io.ReadFull(conn, payload); err != nil {
			return sampleClientHello{}, err
		}
		handshake = append(handshake, payload...)
		if len(handshake) >= 4 {
			size := int(handshake[1])<<16 | int(handshake[2])<<8 | int(handshake[3])
			if len(handshake) >= 4+size {
				break
			}
		}
	}
	if handshake[0] != 1 {
		return sampleClientHello{}, fmt.Errorf("not a ClientHello")
	}
	data := handshake[4:]
	take := func(size int) ([]byte, error) {
		if size > len(data) || size < 0 {
			return nil, io.ErrUnexpectedEOF
		}
		value := data[:size]
		data = data[size:]
		return value, nil
	}
	legacy, err := take(34)
	if err != nil {
		return sampleClientHello{}, err
	}
	readVector := func(width int) ([]byte, error) {
		length, err := take(width)
		if err != nil {
			return nil, err
		}
		size := int(length[0])
		if width == 2 {
			size = int(binary.BigEndian.Uint16(length))
		}
		return take(size)
	}
	if _, err := readVector(1); err != nil {
		return sampleClientHello{}, err
	}
	ciphers, err := readVector(2)
	if err != nil {
		return sampleClientHello{}, err
	}
	hello := sampleClientHello{LegacyVersion: binary.BigEndian.Uint16(legacy), CipherSuites: sampleUint16List(ciphers)}
	if _, err = readVector(1); err != nil {
		return hello, err
	}
	extensions, err := readVector(2)
	if err != nil {
		return hello, err
	}
	for len(extensions) >= 4 {
		kind, size := binary.BigEndian.Uint16(extensions), int(binary.BigEndian.Uint16(extensions[2:]))
		if size > len(extensions)-4 {
			return hello, io.ErrUnexpectedEOF
		}
		value := extensions[4 : 4+size]
		extensions = extensions[4+size:]
		hello.Extensions = append(hello.Extensions, kind)
		switch kind {
		case 11, 45:
			if len(value) == 0 {
				return hello, io.ErrUnexpectedEOF
			}
			var list []uint16
			for _, item := range value[1:] {
				list = append(list, uint16(item))
			}
			if kind == 11 {
				hello.PointFormats = list
			} else {
				hello.PSKModes = list
			}
		case 10, 13:
			if len(value) < 2 {
				return hello, io.ErrUnexpectedEOF
			}
			list := sampleUint16List(value[2:])
			if kind == 10 {
				hello.Groups = list
			} else {
				hello.SignatureAlgorithms = list
			}
		case 16:
			if len(value) < 2 {
				return hello, io.ErrUnexpectedEOF
			}
			for names := value[2:]; len(names) > 0; {
				size := int(names[0])
				if len(names) < size+1 {
					return hello, io.ErrUnexpectedEOF
				}
				hello.ALPN = append(hello.ALPN, string(names[1:1+size]))
				names = names[1+size:]
			}
		case 43:
			if len(value) == 0 {
				return hello, io.ErrUnexpectedEOF
			}
			hello.Versions = sampleUint16List(value[1:])
		case 51:
			if len(value) < 2 {
				return hello, io.ErrUnexpectedEOF
			}
			for shares := value[2:]; len(shares) >= 4; {
				group, size := binary.BigEndian.Uint16(shares), int(binary.BigEndian.Uint16(shares[2:]))
				if size > len(shares)-4 {
					return hello, io.ErrUnexpectedEOF
				}
				hello.KeyShares = append(hello.KeyShares, group)
				shares = shares[4+size:]
			}
		}
	}
	return hello, nil
}

func sampleUint16List(data []byte) []uint16 {
	var values []uint16
	for len(data) >= 2 {
		values = append(values, binary.BigEndian.Uint16(data))
		data = data[2:]
	}
	return values
}
