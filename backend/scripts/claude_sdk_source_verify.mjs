#!/usr/bin/env node
// Run a pinned, separately installed SDK against loopback servers only.
// Usage: node claude_sdk_source_verify.mjs SDK_DIRECTORY CLI_CAPTURE_JSON OUTPUT_JSON
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFile, writeFile } from 'node:fs/promises';
import http from 'node:http';
import net from 'node:net';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const [sdkDirectory, capturePath, outputPath] = process.argv.slice(2);
assert(sdkDirectory && capturePath && outputPath, 'Expected SDK directory, capture JSON, output JSON');
const sdkRoot = path.resolve(sdkDirectory);
const sdkURL = (file) => pathToFileURL(path.join(sdkRoot, file)).href;
const { default: Anthropic } = await import(sdkURL('index.mjs'));
const { buildHeaders } = await import(sdkURL('internal/headers.mjs'));
const sdkPackage = JSON.parse(await readFile(path.join(sdkRoot, 'package.json'), 'utf8'));
assert.equal(sdkPackage.name, '@anthropic-ai/sdk');
const capture = JSON.parse(await readFile(capturePath, 'utf8'));
const cli = capture.requests[0];
const digest = (data) => createHash('sha256').update(data).digest('hex');
assert.equal(digest(cli.body), cli.body_sha256);
const message = { model: 'claude-sonnet-4-6', max_tokens: 1, messages: [{ role: 'user', content: 'Synthetic SDK probe' }] };
const secretHeaders = new Set(['authorization', 'x-api-key', 'cookie', 'proxy-authorization']);
const sanitized = (pairs) => pairs.filter(([name]) => !secretHeaders.has(name.toLowerCase()));
const cliSpecific = Object.fromEntries(cli.headers.filter(([name]) =>
  ['user-agent', 'x-app', 'x-claude-code-session-id', 'anthropic-beta', 'anthropic-dangerous-direct-browser-access'].includes(name.toLowerCase())));

// This process is a disposable probe. Do not inherit credential/profile or
// transport overrides from the invoking shell into the isolated experiment.
for (const name of Object.keys(process.env)) {
  if (/^ANTHROPIC_/i.test(name) || /^(https?_proxy|all_proxy|no_proxy|node_use_env_proxy)$/i.test(name)) delete process.env[name];
}

async function httpCase(name, { profile = false, retry = 0, retryStatus = 429, outerRetry = false, bearer = false } = {}) {
  const records = [];
  const server = http.createServer(async (request, response) => {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const body = Buffer.concat(chunks).toString('utf8');
    const pairs = [];
    for (let index = 0; index < request.rawHeaders.length; index += 2) pairs.push(request.rawHeaders.slice(index, index + 2));
    const status = (retry || outerRetry) && records.length === 0 ? retryStatus : 200;
    records.push({ method: request.method, path: request.url, headers: sanitized(pairs),
      body, body_sha256: digest(body), status,
      authentication: { api_key_present: 'x-api-key' in request.headers, bearer_present: /^Bearer /.test(request.headers.authorization || '') } });
    response.writeHead(status, { 'content-type': 'application/json', 'retry-after': '0' });
    response.end(JSON.stringify(status !== 200 ? { type: 'error', error: { type: 'api_error', message: 'Synthetic retry' } } : { type: 'message', content: [{ type: 'text', text: 'OK' }] }));
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    const client = new Anthropic({ apiKey: bearer ? null : 'synthetic-key', authToken: bearer ? 'synthetic-token' : null,
      baseURL: `http://127.0.0.1:${server.address().port}`, maxRetries: retry, timeout: 5000,
      ...(profile ? { defaultHeaders: cliSpecific } : {}) });
    const send = () => client.post('/v1/messages', { body: profile ? JSON.parse(cli.body) : message, query: { beta: true },
      // CLI advertises a 600s per-request timeout. Bound this local probe with
      // a separate cancellation signal while reproducing that SDK input.
      ...(profile ? { timeout: 600_000, signal: AbortSignal.timeout(5000) } : {}) });
    if (outerRetry) {
      await assert.rejects(send, (error) => error.status === retryStatus);
      await send();
    } else await send();
    const header = (row, name) => row.headers.find(([key]) => key.toLowerCase() === name)?.[1];
    assert.deepEqual(records.map((row) => header(row, 'x-stainless-retry-count')), retry ? ['0', '1'] : outerRetry ? ['0', '0'] : ['0']);
    assert(records.every((row) => row.authentication.api_key_present === !bearer && row.authentication.bearer_present === bearer));
    if (profile) assert.equal(records[0].body_sha256, cli.body_sha256, 'fixture body bytes changed');
    return { name, records };
  } finally {
    server.closeAllConnections();
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}

function parseHello(handshake) {
  assert.equal(handshake[0], 1);
  let offset = 4;
  const take = (length) => { assert(offset + length <= handshake.length); const value = handshake.subarray(offset, offset + length); offset += length; return value; };
  const uint16 = () => take(2).readUInt16BE();
  const vector16 = (buffer) => { assert.equal(buffer.length % 2, 0); return Array.from({ length: buffer.length / 2 }, (_, index) => buffer.readUInt16BE(index * 2)); };
  const result = { legacy_version: uint16(), cipher_suites: [], extensions: [], groups: [], signature_algorithms: [], alpn: [], supported_versions: [], key_shares: [], point_formats: [], psk_modes: [] };
  take(32); take(take(1)[0]);
  result.cipher_suites = vector16(take(uint16()));
  take(take(1)[0]);
  const extensionsLength = uint16();
  const end = offset + extensionsLength;
  while (offset < end) {
    const type = uint16(); const value = take(uint16()); result.extensions.push(type);
    if (type === 10) result.groups = vector16(value.subarray(2));
    if (type === 13) result.signature_algorithms = vector16(value.subarray(2));
    if (type === 43) result.supported_versions = vector16(value.subarray(1));
    if (type === 11) result.point_formats = [...value.subarray(1)];
    if (type === 45) result.psk_modes = [...value.subarray(1)];
    if (type === 16) {
      for (let index = 2; index < value.length;) { const size = value[index++]; result.alpn.push(value.subarray(index, index + size).toString()); index += size; }
    }
    if (type === 51) {
      for (let index = 2; index < value.length;) { result.key_shares.push(value.readUInt16BE(index)); index += 4 + value.readUInt16BE(index + 2); }
    }
  }
  assert.equal(offset, end);
  return result;
}

async function captureSDKHello() {
  let settle;
  const captured = new Promise((resolve) => { settle = resolve; });
  const server = net.createServer((socket) => {
    let input = Buffer.alloc(0); let handshake = Buffer.alloc(0);
    socket.on('error', (error) => settle({ error: error.message }));
    socket.setTimeout(5000, () => socket.destroy(new Error('ClientHello timeout')));
    socket.on('data', (chunk) => {
      try {
        input = Buffer.concat([input, chunk]);
        while (input.length >= 5 && input.length >= 5 + input.readUInt16BE(3)) {
          assert.equal(input[0], 22);
          const length = input.readUInt16BE(3); handshake = Buffer.concat([handshake, input.subarray(5, 5 + length)]); input = input.subarray(5 + length);
          if (handshake.length >= 4 && handshake.length >= 4 + handshake.readUIntBE(1, 3)) { settle({ hello: parseHello(handshake) }); socket.destroy(); return; }
        }
      } catch (error) { settle({ error: error.message }); socket.destroy(); }
    });
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    const client = new Anthropic({ apiKey: 'synthetic-key', authToken: null, baseURL: `https://localhost:${server.address().port}`, maxRetries: 0, timeout: 5000 });
    const request = client.post('/v1/messages', { body: message }).then(() => { throw new Error('TLS probe unexpectedly completed'); }, (error) => ({ expected_transport_error: error.name }));
    const result = await Promise.race([captured, request.then(() => ({ error: 'Transport failed before complete ClientHello' }))]);
    await request;
    assert(!result.error, result.error);
    return result.hello;
  } finally { await new Promise((resolve) => server.close(resolve)); }
}

const results = [];
for (const [name, options] of [
  ['sdk-default', {}], ['sdk-with-cli-specific-headers', { profile: true }],
  ['sdk-inner-retry', { retry: 1 }], ['sdk-inner-retry-503', { retry: 1, retryStatus: 503 }],
  ['caller-outer-retry', { outerRetry: true }],
  ['sdk-static-bearer', { bearer: true }],
]) results.push(await httpCase(name, options));

const merged = buildHeaders([{ 'X-Example': 'initial', 'X-Remove': 'remove', 'X-Stainless-Helper': 'first' },
  { 'x-example': 'override', 'x-remove': null, 'x-stainless-helper': 'second, first' }]).values;
assert.equal(merged.get('x-example'), 'override'); assert.equal(merged.has('x-remove'), false);
assert.equal(merged.get('x-stainless-helper'), 'first, second');

const platformCases = [];
const originalPlatform = Object.getOwnPropertyDescriptor(process, 'platform');
const originalArch = Object.getOwnPropertyDescriptor(process, 'arch');
try {
  for (const [platform, arch, expectedOS, expectedArch] of [
    ['darwin', 'arm64', 'MacOS', 'arm64'], ['darwin', 'x64', 'MacOS', 'x64'],
    ['linux', 'x64', 'Linux', 'x64'], ['linux', 'aarch64', 'Linux', 'arm64'],
    ['win32', 'x64', 'Windows', 'x64'], ['win32', 'arm64', 'Windows', 'arm64'],
  ]) {
    Object.defineProperty(process, 'platform', { value: platform }); Object.defineProperty(process, 'arch', { value: arch });
    const { getPlatformHeaders } = await import(sdkURL('internal/detect-platform.mjs') + `?case=${platform}-${arch}`);
    const headers = getPlatformHeaders();
    assert.equal(headers['X-Stainless-OS'], expectedOS); assert.equal(headers['X-Stainless-Arch'], expectedArch);
    platformCases.push({ simulated_platform: platform, simulated_arch: arch, headers });
  }
} finally {
  Object.defineProperty(process, 'platform', originalPlatform); Object.defineProperty(process, 'arch', originalArch);
}

const report = {
  sdk_version: sdkPackage.version,
  runtime: { node: process.versions.node, openssl: process.versions.openssl, undici: process.versions.undici, platform: process.platform, arch: process.arch },
  source_hashes: Object.fromEntries(await Promise.all(['src/client.ts', 'src/internal/headers.ts', 'src/internal/detect-platform.ts', 'src/internal/shims.ts'].map(async (file) => [file, digest(await readFile(path.join(sdkRoot, file)))]))),
  source_kind: 'Official API SDK, not CLI; simulated platform tests do not establish physical platform transport behavior',
  cli_reference: { binary: capture.binary, headers: cli.headers, body_sha256: cli.body_sha256 },
  http_cases: results, header_merge: [...merged.entries()], simulated_platform_cases: platformCases,
  client_hello: await captureSDKHello(),
};
await writeFile(outputPath, JSON.stringify(report, null, 2) + '\n', { mode: 0o600 });
console.log(JSON.stringify({ output: outputPath, sdk_version: report.sdk_version, runtime: report.runtime, http_cases: results.length, platform_cases: platformCases.length }));
