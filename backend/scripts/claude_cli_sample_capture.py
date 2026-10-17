#!/usr/bin/env python3
"""Capture synthetic Claude CLI requests against an isolated loopback server.

No suffix expectations are computed here: the installed CLI is the oracle.
The output is suitable for TestClaudeCLIRecordedSamples through
CLAUDE_CLI_CAPTURE_DIR. Authentication headers are discarded before recording.
"""

import argparse
import base64
import concurrent.futures
import hashlib
import http.server
import json
import os
from pathlib import Path
import platform
import random
import signal
import struct
import subprocess
import tempfile
import threading
import time
import zlib


def sample_cases(random_count):
    cases = []

    def add(name, prompt, ids, **options):
        cases.append(dict(name=name, prompt=prompt, matrix_ids=ids, **options))

    add("ascii", "Reply OK", ["TXT-01", "CLI-01"])
    add("chinese", "请审查这段代码是否存在安全问题并给出改进建议，只回复好。", ["TXT-05"])
    add("surrogate-single", "abcd😀efghijklmnopqrstuvwxyz", ["TXT-08"])
    add("surrogate-paired", "abcd😀😀abcdefghijklmnopqrstuv", ["TXT-09"])
    text = "0123456789ABCDEFGHIJKLMNO"
    for length in range(25):
        add(f"length-{length:02}", text[:length], ["TXT-02"])
    add("positions", text, ["TXT-03"])
    for index in range(len(text)):
        add(f"mutation-{index:02}", text[:index] + "x" + text[index + 1:], ["TXT-04"])
    for index in range(23):
        add(f"chinese-at-{index:02}", "a" * index + "中文" + text, ["TXT-06"])
        add(f"emoji-at-{index:02}", "a" * index + "😀" + text, ["TXT-10"])
    for name, value, ids in [
        ("mixed", "甲a乙b丙c丁d中文标点，。！？1234567890", ["TXT-07"]),
        ("emoji-zwj", "👨‍👩‍👧‍👦👍🏽🇨🇳✈️" + text, ["TXT-11"]),
        ("nfc", "é" * 24, ["TXT-12"]),
        ("nfd", "e\u0301" * 24, ["TXT-12"]),
        ("japanese", "日本語の文章を使って入力文字列の扱いを確認します。", ["TXT-13"]),
        ("korean", "입력문자열을처리하는방식을확인하기위한테스트문장입니다", ["TXT-13"]),
        ("arabic", "هذه جملة اختبار للتحقق من معالجة النصوص العربية", ["TXT-13"]),
        ("hebrew", "זהו משפט בדיקה לטיפול בטקסט ובתווים מיוחדים", ["TXT-13"]),
        ("devanagari", "यह पाठ अक्षरों के व्यवहार की जांच के लिए है", ["TXT-13"]),
        ("whitespace", " \t" + text + "\n\n ", ["TXT-14"]),
        ("crlf", "one\r\ntwo\r\nthree\r\nfour" + text, ["TXT-14"]),
        ("invisible", "\ufeff\u00a0\u3000\u200b" + text, ["TXT-15"]),
        ("escapes", '`code` "quotes" \\ slash\n{"a":"中"}', ["TXT-16"]),
        ("long-ascii", text * 2600, ["TXT-18"]),
        ("long-unicode", "中文😀" * 3000, ["TXT-18"]),
        ("suffix-a", text + " distant A", ["TXT-19"]),
        ("suffix-b", text + " distant B", ["TXT-19"]),
        ("controls", text + "\x00\x01\x1f", ["TXT-20"]),
    ]:
        add(name, value, ids)
    add("stdin-text", "stdin synthetic 中文 😀 " + text, ["CLI-07"], stdin=True)
    add("system-replaced", text, ["MSG-14", "CLI-06"], flags=["--system-prompt", "Synthetic test system."])
    add("system-appended", text, ["MSG-14"], flags=["--append-system-prompt", "Synthetic appended context."])
    add("normal-config", text, ["CLI-06"], bare=False)
    add("tools-default", text, ["CLI-09"], tools="default")
    add("tool-roundtrip", text, ["MSG-10", "CLI-09"], tools="Read", tool=True)
    add("output-json", text, ["CLI-08"], flags=["--output-format", "json"])
    add("output-stream-json", text, ["CLI-08"], flags=["--output-format", "stream-json", "--verbose"])
    add("effort-low", text, ["CLI-15"], flags=["--effort", "low"])
    add("effort-high", text, ["CLI-15"], flags=["--effort", "high"])
    add("custom-beta", text, ["HDR-03"], flags=["--betas", "synthetic-test-2026-10-17"])
    add("haiku", text, ["ENV-08"], model="claude-haiku-4-5")
    add("retry-429", text, ["ERR-02", "HDR-04"], retry=429)
    add("retry-503", text, ["ERR-02", "HDR-04"], retry=503)
    add("stream-three-turns", text, ["CLI-03", "IDN-04"], input_messages=[text, "Second synthetic turn.", "Third synthetic turn."])
    add("stream-repeat-ten", text, ["TXT-22", "IDN-04"], input_messages=[text] * 10)
    add("stream-multiple-text", text, ["MSG-02"], input_messages=[[dict(type="text", text=text), dict(type="text", text="Second text block.")]])
    add("stream-empty-text", text, ["MSG-03"], input_messages=[[dict(type="text", text=""), dict(type="text", text=text)]])
    add("tool-error", text, ["MSG-10"], tools="Read", tool=True, tool_error=True)
    add("compact-workflow", text, ["MSG-13", "CLI-14"], input_messages=[text, "/compact", "After compact."], enable_commands=True)
    def png_chunk(kind, data):
        return struct.pack("!I", len(data)) + kind + data + struct.pack("!I", zlib.crc32(kind + data))
    png = b"\x89PNG\r\n\x1a\n" + png_chunk(b"IHDR", struct.pack("!IIBBBBB", 1, 1, 8, 2, 0, 0, 0))
    png += png_chunk(b"IDAT", zlib.compress(b"\x00\xff\xff\xff")) + png_chunk(b"IEND", b"")
    image = dict(type="image", source=dict(type="base64", media_type="image/png", data=base64.b64encode(png).decode()))
    add("stream-image-first", text, ["MSG-04", "MSG-08"], input_messages=[[image, dict(type="text", text=text)]])
    add("stream-image-last", text, ["MSG-08"], input_messages=[[dict(type="text", text=text), image]])
    add("normal-reminder-moved", text, ["MSG-07"], bare=False, flags=["--exclude-dynamic-system-prompt-sections"])
    for index in range(10):
        add(f"repeat-{index:02}", text, ["TXT-22"])
    rng = random.Random(20261017)
    alphabet = list("abcXYZ019 中文日本語é\u0301\t\n") + ["😀", "👍", "🏽", "\u200d", "\ufe0f", "🇨", "🇳"]
    for index in range(random_count):
        prompt = "".join(rng.choice(alphabet) for _ in range(rng.randint(1, 70)))
        add(f"unicode-random-{index:03}", prompt, ["TXT-23"])
    return cases


def response_message(content, stop="end_turn"):
    return dict(id="msg_local_sample", type="message", role="assistant",
                model="claude-sonnet-4-6", content=content, stop_reason=stop,
                stop_sequence=None, usage=dict(input_tokens=20, output_tokens=2))


def stream_events(message):
    initial = dict(message, content=[], stop_reason=None)
    events = [dict(type="message_start", message=initial)]
    for index, block in enumerate(message["content"]):
        if block["type"] == "text":
            start = dict(type="text", text="")
            delta = dict(type="text_delta", text=block["text"])
        else:
            start = dict(block, input={})
            delta = dict(type="input_json_delta", partial_json=json.dumps(block["input"]))
        events.extend([dict(type="content_block_start", index=index, content_block=start),
                       dict(type="content_block_delta", index=index, delta=delta),
                       dict(type="content_block_stop", index=index)])
    events.extend([dict(type="message_delta", delta=dict(stop_reason=message["stop_reason"], stop_sequence=None), usage=dict(output_tokens=2)),
                   dict(type="message_stop")])
    return "".join("event: " + event["type"] + "\ndata: " + json.dumps(event) + "\n\n" for event in events).encode()


def capture_case(case, args, binary_info):
    records = []
    record_changed = threading.Condition()
    case_dir = args.out / case["name"]
    case_dir.mkdir(parents=True, exist_ok=False)
    with tempfile.TemporaryDirectory(prefix="cli-sample-") as temp:
        work = Path(temp)
        (work / "home").mkdir()
        (work / "synthetic.txt").write_text("Synthetic tool fixture.\n", encoding="utf-8")

        class Handler(http.server.BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def log_message(self, *unused):
                pass

            def do_POST(self):
                raw = self.rfile.read(int(self.headers.get("Content-Length", "0")))
                payload = json.loads(raw)
                ordinal = len(records)
                records.append(dict(sequence=ordinal, path=self.path, method="POST",
                    headers=[[k, v] for k, v in self.headers.items() if k.lower() not in ("authorization", "x-api-key", "proxy-authorization", "cookie")],
                    body=raw.decode("utf-8"), body_sha256=hashlib.sha256(raw).hexdigest()))
                if "count_tokens" in self.path:
                    status, content_type, response = 200, "application/json", b'{"input_tokens":20}'
                elif case.get("retry") and ordinal == 0:
                    status, content_type = case["retry"], "application/json"
                    response = b'{"type":"error","error":{"type":"overloaded_error","message":"synthetic retry"}}'
                else:
                    blocks = [dict(type="text", text="OK")]
                    stop = "end_turn"
                    if case.get("tool") and ordinal == 0:
                        fixture = "missing.txt" if case.get("tool_error") else "synthetic.txt"
                        blocks = [dict(type="tool_use", id="toolu_local", name="Read", input=dict(file_path=str(work / fixture)))]
                        stop = "tool_use"
                    message = response_message(blocks, stop)
                    status = 200
                    content_type = "text/event-stream" if payload.get("stream") else "application/json"
                    response = stream_events(message) if payload.get("stream") else json.dumps(message).encode()
                self.send_response(status)
                self.send_header("Content-Type", content_type)
                self.send_header("Content-Length", str(len(response)))
                self.send_header("Request-Id", "req_local_sample")
                if status != 200:
                    self.send_header("Retry-After", "0")
                self.end_headers()
                self.wfile.write(response)
                self.wfile.flush()
                with record_changed:
                    record_changed.notify_all()

        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        server.daemon_threads = True
        thread = threading.Thread(target=server.serve_forever, kwargs=dict(poll_interval=0.02), daemon=True)
        thread.start()
        env = dict(PATH=os.defpath, HOME=str(work / "home"), LANG="en_US.UTF-8",
                   ANTHROPIC_API_KEY="local-synthetic-key", ANTHROPIC_BASE_URL=f"http://127.0.0.1:{server.server_port}",
                   CLAUDE_CONFIG_DIR=str(work / "config"), CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC="1",
                   DISABLE_AUTOUPDATER="1", NO_PROXY="127.0.0.1,localhost")
        command = [args.cli]
        if case.get("bare", True):
            command += ["--bare"]
        command += ["-p", "--no-session-persistence", "--setting-sources", "",
                    "--strict-mcp-config", "--model", case.get("model", "claude-sonnet-4-6"), "--tools", case.get("tools", "")]
        if not case.get("enable_commands"):
            command += ["--disable-slash-commands"]
        if case.get("tool"):
            command += ["--allowedTools", "Read", "--permission-prompts", "none"]
        command += case.get("flags", [])
        if case.get("input_messages"):
            command += ["--input-format", "stream-json", "--output-format", "stream-json", "--verbose"]
        stdin = case["prompt"].encode("utf-8") if case.get("stdin") or "\x00" in case["prompt"] else b""
        if not stdin and not case.get("input_messages"):
            command += ["--", case["prompt"]]
        started = time.monotonic()
        process = subprocess.Popen(command, cwd=work, env=env, stdin=subprocess.PIPE,
                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
        timed_out = False
        try:
            if case.get("input_messages"):
                for index, content in enumerate(case["input_messages"]):
                    message = dict(type="user", message=dict(role="user", content=content))
                    process.stdin.write((json.dumps(message) + "\n").encode())
                    process.stdin.flush()
                    with record_changed:
                        ready = record_changed.wait_for(lambda: len(records) > index, timeout=args.timeout)
                    if not ready:
                        timed_out = True
                        break
                process.stdin.close()
                process.stdin = None
                stdout, stderr = process.communicate(timeout=args.timeout)
            else:
                stdout, stderr = process.communicate(stdin, timeout=args.timeout)
        except (BrokenPipeError, subprocess.TimeoutExpired) as error:
            timed_out = isinstance(error, subprocess.TimeoutExpired)
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGKILL)
            process.stdin = None
            stdout, stderr = process.communicate()
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=2)
        artifact = dict(case=case, binary=binary_info, invocation=command,
                        duration_seconds=round(time.monotonic() - started, 3),
                        exit_code=process.returncode, timed_out=timed_out,
                        status="captured" if records else "blocked", requests=records,
                        stdout=stdout.decode("utf-8", errors="replace"), stderr=stderr.decode("utf-8", errors="replace"))
        (case_dir / "capture.json").write_text(json.dumps(artifact, ensure_ascii=True, indent=2) + "\n", encoding="utf-8")
    return dict(name=case["name"], matrix_ids=case["matrix_ids"], request_count=len(records),
                status=artifact["status"], exit_code=artifact["exit_code"], timed_out=timed_out,
                duration_seconds=artifact["duration_seconds"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cli", default="/opt/homebrew/bin/claude")
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--random-count", type=int, default=200)
    parser.add_argument("--workers", type=int, default=4)
    parser.add_argument("--timeout", type=int, default=25)
    parser.add_argument("--only", help="Comma-separated exact case names for bounded reruns")
    args = parser.parse_args()
    args.cli = str(Path(args.cli).resolve())
    args.out.mkdir(parents=True, exist_ok=False)
    binary = dict(version=subprocess.check_output([args.cli, "--version"], text=True).strip(),
                  sha256=hashlib.sha256(Path(args.cli).read_bytes()).hexdigest(),
                  capture_script_sha256=hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
                  system=platform.system(), release=platform.release(), machine=platform.machine(),
                  captured_at=time.strftime("%Y-%m-%dT%H:%M:%S%z"))
    cases = sample_cases(args.random_count)
    if args.only:
        wanted = set(args.only.split(","))
        cases = [case for case in cases if case["name"] in wanted]
        if {case["name"] for case in cases} != wanted:
            parser.error("--only contains an unknown case name")
    results = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.workers) as pool:
        futures = {pool.submit(capture_case, case, args, binary): case["name"] for case in cases}
        for future in concurrent.futures.as_completed(futures):
            result = future.result()
            results.append(result)
            print(json.dumps(result), flush=True)
    manifest = dict(binary=binary, case_count=len(cases), request_count=sum(row["request_count"] for row in results),
                    results=sorted(results, key=lambda row: row["name"]),
                    limitations=["Synthetic API key and loopback only; no real OAuth or live provider validation.",
                                 "Local mock success/tool/retry responses; not evidence of provider capability.",
                                 "This run is HTTP only; TLS and other operating systems are separate."])
    (args.out / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
