#!/usr/bin/env python3
"""Capture header names/order only from an isolated CLI with synthetic credentials."""
import argparse
import hashlib
import http.server
import json
import os
from pathlib import Path
import subprocess
import ssl
import tempfile
import threading
import sys
sys.dont_write_bytecode = True
from claude_cli_sample_capture import response_message, stream_events


def capture(cli, auth, retry, extra, secure):
    records = []
    class Handler(http.server.BaseHTTPRequestHandler):
        protocol_version = 'HTTP/1.1'
        def log_message(self, *args):
            pass
        def do_POST(self):
            body = self.rfile.read(int(self.headers['Content-Length']))
            payload = json.loads(body)
            status = retry if retry and not records else 200
            records.append({'method': self.command, 'path': self.path,
                            'header_names': list(self.headers.keys()), 'status': status})
            if status != 200:
                data = b'{"type":"error","error":{"type":"overloaded_error","message":"Synthetic retry"}}'
                content_type = 'application/json'
            else:
                message = response_message([{'type': 'text', 'text': 'OK'}])
                data = stream_events(message) if payload.get('stream') else json.dumps(message).encode()
                content_type = 'text/event-stream' if payload.get('stream') else 'application/json'
            self.send_response(status)
            self.send_header('Content-Type', content_type)
            self.send_header('Content-Length', str(len(data)))
            self.send_header('Retry-After', '0')
            self.end_headers()
            self.wfile.write(data)
            self.wfile.flush()
    server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
    cert_temp = tempfile.TemporaryDirectory(prefix='cli-order-cert-')
    cert_root = Path(cert_temp.name)
    cert = cert_root / 'ca.pem'
    if secure:
        cfg = cert_root / 'openssl.cnf'
        cfg.write_text('[req]\ndistinguished_name=dn\n[dn]\nCN=localhost\n[x509_ext]\nsubjectAltName=IP:127.0.0.1,DNS:localhost\nbasicConstraints=critical,CA:TRUE\nkeyUsage=critical,keyCertSign,digitalSignature,keyEncipherment\n')
        key = cert_root / 'key.pem'
        subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
                        '-keyout', str(key), '-out', str(cert), '-subj', '/CN=localhost',
                        '-config', str(cfg), '-extensions', 'x509_ext'], check=True, capture_output=True)
        tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        tls.load_cert_chain(cert, key)
        tls.set_alpn_protocols(['http/1.1'])
        server.socket = tls.wrap_socket(server.socket, server_side=True)
    server.daemon_threads = True
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        with tempfile.TemporaryDirectory(prefix='cli-header-order-') as work:
            env = {'PATH': os.defpath, 'HOME': work, 'CLAUDE_CONFIG_DIR': work + '/config',
                   'ANTHROPIC_BASE_URL': f"{'https' if secure else 'http'}://127.0.0.1:{server.server_port}",
                   'CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC': '1', 'DISABLE_AUTOUPDATER': '1',
                   'NO_PROXY': '127.0.0.1,localhost'}
            env['ANTHROPIC_API_KEY' if auth == 'api_key' else 'ANTHROPIC_AUTH_TOKEN'] = 'synthetic-local-only'
            if secure:
                env['NODE_EXTRA_CA_CERTS'] = str(cert)
            if extra:
                env['ANTHROPIC_CUSTOM_HEADERS'] = 'X-Synthetic-Extra: marker\nX-Stainless-Helper-Method: stream'
            cmd = [cli, '--bare', '-p', '--no-session-persistence', '--setting-sources', '',
                   '--strict-mcp-config', '--disable-slash-commands', '--tools', '',
                   '--model', 'claude-sonnet-4-6', '--', 'Reply OK']
            result = subprocess.run(cmd, cwd=work, env=env, capture_output=True, timeout=45)
            if result.returncode or not records:
                raise RuntimeError(f'{auth}/{retry}/{extra}: exit={result.returncode}, requests={len(records)}')
    finally:
        server.shutdown()
        server.server_close()
        thread.join()
        cert_temp.cleanup()
    expected = 'x-api-key' if auth == 'api_key' else 'authorization'
    assert all(expected in [h.lower() for h in row['header_names']] for row in records)
    if extra:
        assert all('X-Synthetic-Extra' in row['header_names'] for row in records)
    return {'auth': auth, 'retry': retry, 'extra': extra, 'https': secure, 'requests': records}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--cli', required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    cli = str(Path(args.cli).resolve())
    version = subprocess.check_output([cli, '--version'], text=True).strip()
    report = {'cli_version': version, 'binary_sha256': hashlib.sha256(Path(cli).read_bytes()).hexdigest(),
              'scope': 'Loopback HTTP/HTTPS HTTP/1.1; trusted local CA; synthetic API key/static bearer; header names only; no real OAuth',
              'cases': [capture(cli, auth, retry, extra, secure) for secure in [False, True] for auth in ['api_key', 'bearer']
                        for retry, extra in [(0, False), (429, False), (503, False), (0, True)]]}
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(report, indent=2) + '\n')
    print(f"Captured {sum(len(c['requests']) for c in report['cases'])} requests across {len(report['cases'])} cases")

if __name__ == '__main__':
    main()
