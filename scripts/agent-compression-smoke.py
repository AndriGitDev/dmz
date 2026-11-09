#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Synthetic end-to-end check of compressed request bodies through ``dmz serve``.

Path under test (Linux development fixture, loopback only):

  installed Codex CLI (``codex exec``), isolated CODEX_HOME with a SYNTHETIC
  cached ChatGPT sign-in, a manual custom provider with
  ``requires_openai_auth=true`` and ``features.enable_request_compression=true``
  -> owned ingress counting proxy (loopback; headers and lengths only)
  -> ``dmz serve --protocol responses --local-auth header`` (built from this
     checkout) -> loopback fake Responses provider (agent-client-smoke.py).

The local DMZ credential travels in the ``X-Dmz-Token`` header (header mode);
the client's own synthetic sign-in token is the only upstream credential.
This is a deliberately enabled-compression variant of the desktop-like route.
The production launcher keeps ``enable_request_compression=false``; this
harness does not change it.

The ingress proxy keeps request bytes in memory only long enough to forward
them unchanged. It records the Content-Encoding category, body length, a
zstd-magic boolean and the zstd frame window size; never content.

Owned synthetic probes additionally send a gzip-encoded and a corrupt
zstd-encoded body directly to the service.

NOT evidence of: provider, account, model, desktop-application or detector
behaviour. Every credential, prompt and address is synthetic.

Output contract: stdout is one JSON document of versions, counts, booleans
and fixed codes; stderr carries fixed reason codes only. Request bodies,
client output and service output are never printed or stored.

Exit: 0 pass, 3 incomplete (e.g. the client did not compress), 1 fail,
2 dmz executable unusable. Python standard library plus the ``zstd`` command
(used only by the fake upstream to decode in memory) and ``codex``.
"""

from __future__ import annotations

import argparse
import base64
import gzip
import http.client
import importlib.util
import io
import json
import os
import platform
import re
import shutil
import socket
import subprocess
import sys
import tempfile
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PROG = "agent-compression-smoke"
HERE = os.path.dirname(os.path.abspath(__file__))
ZSTD_MAGIC = b"\x28\xb5\x2f\xfd"
WINDOW_LIMIT = 8 << 20  # dmz default MaxBodyBytes; decoder window bound
ZSTD_DECODE_CAP = 16 << 20


def _load(name, file):
    sys.dont_write_bytecode = True
    spec = importlib.util.spec_from_file_location(name, os.path.join(HERE, file))
    if spec is None or spec.loader is None:
        raise ImportError(file)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


manual = _load("dmz_agent_manual_smoke", "agent-manual-smoke.py")
base = manual.base


def diag(code: str) -> None:
    if not re.fullmatch(r"[a-z0-9_.:\-]{1,120}", code):
        code = "invalid_diagnostic_code"
    sys.stderr.write(f"{PROG}: {code}\n")
    sys.stderr.flush()


def encoding_category(values):
    if not values:
        return "absent"
    if len(values) > 1:
        return "multiple"
    v = values[0].strip().lower()
    return v if v in ("zstd", "gzip", "x-gzip", "identity", "br", "deflate", "") else "other"


def zstd_window(buf: bytes):
    """Window size of the first zstd frame (RFC 8878 frame header), or None."""
    if len(buf) < 6 or buf[:4] != ZSTD_MAGIC:
        return None
    fhd = buf[4]
    fcs_flag, single, dict_flag = fhd >> 6, (fhd >> 5) & 1, fhd & 3
    pos = 5
    if not single:
        wd = buf[pos]
        log = 10 + (wd >> 3)
        wbase = 1 << log
        return wbase + (wbase >> 3) * (wd & 7)
    pos += (0, 1, 2, 4)[dict_flag]
    size = (1, 2, 4, 8)[fcs_flag]
    if len(buf) < pos + size:
        return None
    v = int.from_bytes(buf[pos:pos + size], "little")
    return v + 256 if size == 2 else v


def zstd_encode(buf: bytes):
    """Owned probe encoder (system zstd tool, content checksum on)."""
    p = subprocess.run(["zstd", "-q", "-c", "--check", "-3"], input=buf, capture_output=True, timeout=20)
    if p.returncode != 0:
        raise OSError("zstd encode failed")
    return p.stdout


def zstd_decode(buf: bytes):
    """Decode in memory with the system zstd tool; bounded output."""
    try:
        p = subprocess.run(["zstd", "-q", "-d", "-c", "--memory=8MB"], input=buf, capture_output=True, timeout=20)
    except (OSError, subprocess.TimeoutExpired):
        return None
    if p.returncode != 0 or len(p.stdout) > ZSTD_DECODE_CAP:
        return None
    return p.stdout


def read_body(rfile, headers):
    if (headers.get("Transfer-Encoding") or "").lower() == "chunked":
        out = bytearray()
        while True:
            line = rfile.readline(1024)
            n = int(line.split(b";")[0].strip() or b"0", 16)
            if n == 0:
                while rfile.readline(1024) not in (b"\r\n", b"\n", b""):
                    pass
                return bytes(out)
            out += rfile.read(n)
            rfile.readline(8)
            if len(out) > ZSTD_DECODE_CAP:
                raise ValueError("body too large")
    n = int(headers.get("Content-Length") or 0)
    return rfile.read(n) if n > 0 else b""


# --- Ingress counting proxy (client -> proxy -> dmz serve) -------------------
class IngressStats:
    def __init__(self):
        self.lock = threading.Lock()
        self.requests = 0
        self.post_requests = 0
        self.encodings: dict[str, int] = {}
        self.zstd_magic = 0
        self.zstd_magic_with_header = 0
        self.max_window = 0
        self.windows_over_limit = 0
        self.windows_unparsed = 0
        self.body_bytes_total = 0
        self.max_body_bytes = 0
        self.chunked = 0
        self.local_header_present = 0
        self.authorization_present = 0
        self.statuses: dict[str, int] = {}
        self.routes: dict[str, int] = {}
        self.forward_errors = 0


class _IngressHandler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *_):
        pass

    def _proxy(self):
        st: IngressStats = self.server.stats  # type: ignore[attr-defined]
        try:
            body = read_body(self.rfile, self.headers) if self.command == "POST" else b""
        except (ValueError, OSError):
            self.close_connection = True
            return
        enc_values = self.headers.get_all("Content-Encoding") or []
        cat = encoding_category(enc_values)
        magic = body[:4] == ZSTD_MAGIC
        with st.lock:
            st.requests += 1
            if self.command == "POST":
                st.post_requests += 1
                st.encodings[cat] = st.encodings.get(cat, 0) + 1
                st.body_bytes_total += len(body)
                st.max_body_bytes = max(st.max_body_bytes, len(body))
                if (self.headers.get("Transfer-Encoding") or "").lower() == "chunked":
                    st.chunked += 1
                if magic:
                    st.zstd_magic += 1
                    if cat == "zstd":
                        st.zstd_magic_with_header += 1
                    w = zstd_window(body)
                    if w is None:
                        st.windows_unparsed += 1
                    else:
                        st.max_window = max(st.max_window, w)
                        if w > WINDOW_LIMIT:
                            st.windows_over_limit += 1
            if self.headers.get("X-Dmz-Token"):
                st.local_header_present += 1
            if self.headers.get("Authorization"):
                st.authorization_present += 1
        # Forward unchanged (headers and original wire bytes).
        hop = {"connection", "keep-alive", "transfer-encoding", "content-length", "host", "proxy-connection"}
        conn = http.client.HTTPConnection("127.0.0.1", self.server.target_port, timeout=60)  # type: ignore[attr-defined]
        try:
            conn.putrequest(self.command, self.path, skip_host=True, skip_accept_encoding=True)
            conn.putheader("Host", f"127.0.0.1:{self.server.target_port}")  # type: ignore[attr-defined]
            for k, v in self.headers.items():
                if k.lower() not in hop:
                    conn.putheader(k, v)
            inject = self.server.inject_token  # type: ignore[attr-defined]
            if inject and not self.headers.get("X-Dmz-Token"):
                conn.putheader("X-Dmz-Token", inject)  # diagnostic variant only
            if self.command == "POST":
                conn.putheader("Content-Length", str(len(body)))
            conn.endheaders(body if self.command == "POST" else None)
            resp = conn.getresponse()
            path = self.path.split("?")[0]
            key = f"{self.command} {path if re.fullmatch(r'/[a-z0-9/_.-]{0,64}', path) else 'other'} {resp.status}"
            with st.lock:
                st.statuses[str(resp.status)] = st.statuses.get(str(resp.status), 0) + 1
                st.routes[key] = st.routes.get(key, 0) + 1
            self.send_response_only(resp.status)
            for k, v in resp.getheaders():
                if k.lower() not in hop:
                    self.send_header(k, v)
            self.send_header("Transfer-Encoding", "chunked")
            self.send_header("Connection", "close")
            self.end_headers()
            while True:
                chunk = resp.read1(65536)
                if not chunk:
                    break
                self.wfile.write(b"%x\r\n" % len(chunk) + chunk + b"\r\n")
                self.wfile.flush()
            self.wfile.write(b"0\r\n\r\n")
        except OSError:
            with st.lock:
                st.forward_errors += 1
        finally:
            conn.close()
            self.close_connection = True
            del body

    do_GET = _proxy
    do_POST = _proxy


class IngressProxy(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, target_port, inject_token=None):
        super().__init__(("127.0.0.1", 0), _IngressHandler)
        self.target_port = target_port
        self.inject_token = inject_token
        self.stats = IngressStats()


# --- Fake upstream that decodes encoded pass-through bodies in memory ----------
class _DecodingHandler(manual._LeakCheckHandler):
    def do_POST(self):
        raw = read_body(self.rfile, self.headers)
        cat = encoding_category(self.headers.get_all("Content-Encoding") or [])
        srv = self.server
        with srv.obs.lock:  # type: ignore[attr-defined]
            srv.upstream_encodings[cat] = srv.upstream_encodings.get(cat, 0) + 1  # type: ignore[attr-defined]
            if self.headers.get("ChatGPT-Account-Id"):
                srv.account_header_requests += 1  # type: ignore[attr-defined]
        if cat == "zstd":
            dec = zstd_decode(raw)
            if dec is None:
                with srv.obs.lock:  # type: ignore[attr-defined]
                    srv.upstream_decode_failures += 1  # type: ignore[attr-defined]
            else:
                raw = dec
        elif cat in ("gzip", "x-gzip"):
            try:
                raw = gzip.decompress(raw)
            except (OSError, EOFError):
                with srv.obs.lock:  # type: ignore[attr-defined]
                    srv.upstream_decode_failures += 1  # type: ignore[attr-defined]
        if "Content-Length" in self.headers:
            self.headers.replace_header("Content-Length", str(len(raw)))
        else:
            self.headers["Content-Length"] = str(len(raw))
        if "Transfer-Encoding" in self.headers:
            del self.headers["Transfer-Encoding"]
        self.rfile = io.BytesIO(raw)
        del raw
        super().do_POST()


class Provider(manual.Provider):
    def __init__(self, workdir, token):
        super().__init__("responses", "basic", workdir, base.CODEX_MODEL, token)
        self.RequestHandlerClass = _DecodingHandler
        self._reset_extra()

    def _reset_extra(self):
        self.upstream_encodings: dict[str, int] = {}
        self.upstream_decode_failures = 0
        self.account_header_requests = 0

    def reset_scenario(self, scenario, workdir):
        super().reset_scenario(scenario, workdir)
        self._reset_extra()


# --- Synthetic cached ChatGPT sign-in (agent-companion-smoke.py pattern) ------
def seed_codex_auth(codex_home):
    now = int(time.time())

    def b64(d):
        return base64.urlsafe_b64encode(json.dumps(d).encode()).rstrip(b"=").decode()

    claims = {"email": "account.synthetic@example.test", "exp": now + 86400, "iat": now,
              "https://api.openai.com/auth": {"chatgpt_plan_type": "plus", "chatgpt_account_id": "acct-synthetic-0001",
                                              "chatgpt_user_id": "user-synthetic-0001"}}
    jwt = b64({"alg": "none", "typ": "JWT"}) + "." + b64(claims) + ".c3ludGhldGlj"
    data = {"OPENAI_API_KEY": None, "auth_mode": "chatgpt",
            "tokens": {"id_token": jwt, "access_token": base.FAKE_OPENAI_KEY, "refresh_token": "synthetic-refresh-not-used",
                       "account_id": "acct-synthetic-0001"},
            "last_refresh": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(now))}
    path = os.path.join(codex_home, "auth.json")
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w") as fh:
        json.dump(data, fh)
    return path


def codex_argv(codex, base_url, workdir, variant="custom"):
    if variant == "builtin-diagnostic":
        provider = ["-c", "openai_base_url=" + json.dumps(base_url), "-c", "features.enable_request_compression=true"]
    else:
        provider = [
            "-c", 'model_provider="dmz_local"',
            "-c", 'model_providers.dmz_local.name="dmz local compression fixture"',
            "-c", "model_providers.dmz_local.base_url=" + json.dumps(base_url),
            "-c", 'model_providers.dmz_local.wire_api="responses"',
            "-c", "model_providers.dmz_local.supports_websockets=false",
            "-c", "model_providers.dmz_local.requires_openai_auth=true",
            "-c", 'model_providers.dmz_local.env_http_headers={"X-Dmz-Token"="DMZ_LOCAL_TOKEN"}',
            "-c", "features.enable_request_compression=true"]
    return [codex, "exec", "--ignore-user-config", "--ignore-rules", "--ephemeral", "--sandbox", "read-only",
            "--skip-git-repo-check", "--color", "never", "-C", workdir, "-m", base.CODEX_MODEL,
            *provider,
            "-c", 'approval_policy="never"', "-c", "mcp_servers={}",
            "-c", "check_for_update_on_startup=false", "-c", "analytics.enabled=false",
            "-c", "feedback.enabled=false", "-c", 'web_search="disabled"',
            *base.CODEX_FEATURE_ARGS, base.build_prompt("basic")]


def post_direct(port, token, body, encoding):
    conn = http.client.HTTPConnection("127.0.0.1", port, timeout=30)
    try:
        conn.request("POST", "/v1/responses", body=body, headers={
            "Content-Type": "application/json", "Content-Encoding": encoding, "X-Dmz-Token": token,
            "Authorization": f"Bearer {base.FAKE_OPENAI_KEY}", "Accept": "text/event-stream"})
        resp = conn.getresponse()
        data = resp.read()
        return resp.status, base.FINAL_TEXT in data.decode("utf-8", "replace")
    except OSError:
        return 0, False
    finally:
        conn.close()


def synthetic_responses_body():
    return json.dumps({"model": base.CODEX_MODEL, "stream": True, "instructions": "synthetic",
                       "input": [{"type": "message", "role": "user",
                                  "content": [{"type": "input_text", "text": base.build_prompt("basic")}]}]}).encode()


def snapshot_warnings(service):
    return dict(sorted(service.warnings().items()))


def run(args, deadline):
    rep = {"codex": {}, "probes": {}, "service": {}, "control": {}, "failures": [], "incomplete": []}
    f, inc = rep["failures"], rep["incomplete"]
    tmp = tempfile.mkdtemp(prefix="dmz-compression-smoke-")
    sink = provider = service = ingress = None
    token = manual.new_local_token()
    seeded = []
    try:
        for sub in ("tmp", "codex-home"):
            os.makedirs(os.path.join(tmp, sub), mode=0o700)
        seeded.append(seed_codex_auth(os.path.join(tmp, "codex-home")))
        sink = base.EgressSink()
        threading.Thread(target=sink.serve_forever, daemon=True).start()
        workdir = os.path.join(tmp, "work")
        os.makedirs(workdir, mode=0o700)
        provider = Provider(workdir, token)
        threading.Thread(target=provider.serve_forever, daemon=True).start()
        port = manual.free_port()
        senv = manual.base_env(tmp, sink.server_address[1])
        senv["DMZ_LOCAL_TOKEN"] = token  # header mode: no provider key in the service
        service = manual.Service(args.dmz, "responses", "header", port, provider.server_address[1], senv, tmp)
        if not manual.wait_listen(port, deadline):
            f.append("service_not_listening")
            return rep
        ingress = IngressProxy(port, token if args.variant == "builtin-diagnostic" else None)
        threading.Thread(target=ingress.serve_forever, daemon=True).start()

        # --- real Codex client
        env = base.child_env(tmp, sink.server_address[1], "codex")
        env.pop("OPENAI_API_KEY", None)
        env["DMZ_LOCAL_TOKEN"] = token
        argv = codex_argv(args.codex, f"http://127.0.0.1:{ingress.server_address[1]}/v1", workdir, args.variant)
        rc, out, err, to = base.run_captured(argv, env, workdir, deadline.remaining(args.scenario_timeout))
        r = manual.evaluate("codex", "basic", provider, rc, out, err, to, token, sink)
        del out, err
        st = ingress.stats
        r["ingress"] = {
            "requests": st.requests, "post_requests": st.post_requests,
            "content_encoding": dict(sorted(st.encodings.items())),
            "zstd_magic_bodies": st.zstd_magic, "zstd_magic_with_header": st.zstd_magic_with_header,
            "zstd_max_window_bytes": st.max_window, "zstd_window_limit_bytes": WINDOW_LIMIT,
            "zstd_windows_over_limit": st.windows_over_limit, "zstd_windows_unparsed": st.windows_unparsed,
            "body_bytes_total": st.body_bytes_total, "max_body_bytes": st.max_body_bytes,
            "chunked_requests": st.chunked, "local_header_requests": st.local_header_present,
            "authorization_requests": st.authorization_present,
            "response_statuses": dict(sorted(st.statuses.items())), "routes": dict(sorted(st.routes.items())),
            "forward_errors": st.forward_errors, "local_header_injected_by_harness": args.variant == "builtin-diagnostic",
        }
        r["upstream"]["content_encoding"] = dict(sorted(provider.upstream_encodings.items()))
        r["upstream"]["decode_failures"] = provider.upstream_decode_failures
        r["upstream"]["account_header_requests"] = provider.account_header_requests
        r["service_warnings_after_client"] = snapshot_warnings(service)
        rep["codex"] = r
        for code in r["failures"]:
            f.append("codex." + code)
        r["zstd_accepted_end_to_end"] = bool(
            st.zstd_magic_with_header and r["status"] == "pass" and r["upstream"]["main_requests"]
            and not provider.upstream_encodings.get("zstd")
            and not any(k.startswith(("encoded_", "decoded_")) for k in snapshot_warnings(service)))
        if st.zstd_magic_with_header == 0:
            inc.append("codex_did_not_send_zstd_body")
        elif not r["zstd_accepted_end_to_end"]:
            f.append("codex_zstd_body_not_accepted_end_to_end")
        elif st.windows_over_limit or st.windows_unparsed:
            f.append("codex_zstd_window_outside_limit")
        if st.forward_errors:
            f.append("ingress_forward_error")
        if args.variant == "custom" and st.post_requests and st.local_header_present < st.requests:
            f.append("local_header_missing_on_ingress")
        if provider.upstream_decode_failures:
            f.append("upstream_encoded_body_undecodable")
        client_warn = r["service_warnings_after_client"]
        if any(k.startswith("encoded_") or k.startswith("decoded_") for k in client_warn):
            f.append("service_decode_warning_on_client_body")

        # --- owned synthetic probes (gzip eligible; corrupt zstd)
        provider.reset_scenario("basic", workdir)
        gz_status, gz_final = post_direct(port, token, gzip.compress(synthetic_responses_body()), "gzip")
        obs = provider.obs
        rep["probes"]["gzip_eligible"] = {
            "status": gz_status, "final_answer_streamed": gz_final,
            "upstream_content_encoding": dict(sorted(provider.upstream_encodings.items())),
            "marker_in_eligible_field": obs.marker_in_eligible, "original_in_eligible_field": obs.original_in_eligible,
            "upstream_fake_credential_requests": obs.fake_credential_requests,
            "x_dmz_header_requests": obs.dmz_header_requests,
        }
        if gz_status != 200 or not gz_final or obs.marker_in_eligible != 1 or obs.original_in_eligible \
                or provider.upstream_encodings.get("absent") != 1 or obs.dmz_header_requests:
            f.append("probe_gzip_eligible_unexpected")
        provider.reset_scenario("basic", workdir)
        good = zstd_encode(synthetic_responses_body())
        zs_status, zs_final = post_direct(port, token, good, "zstd")
        obs = provider.obs
        rep["probes"]["zstd_eligible"] = {
            "status": zs_status, "final_answer_streamed": zs_final, "window_bytes": zstd_window(good),
            "upstream_content_encoding": dict(sorted(provider.upstream_encodings.items())),
            "marker_in_eligible_field": obs.marker_in_eligible, "original_in_eligible_field": obs.original_in_eligible,
        }
        if zs_status != 200 or not zs_final or obs.marker_in_eligible != 1 or obs.original_in_eligible \
                or provider.upstream_encodings.get("absent") != 1:
            f.append("probe_zstd_eligible_unexpected")
        before = snapshot_warnings(service)
        provider.reset_scenario("basic", workdir)
        bad = bytearray(good)
        bad[-1] ^= 0xFF  # content checksum mismatch: deterministic integrity failure
        z_status, _ = post_direct(port, token, bytes(bad), "zstd")
        time.sleep(0.2)
        after = snapshot_warnings(service)
        delta = {k: after.get(k, 0) - before.get(k, 0) for k in after if after.get(k, 0) != before.get(k, 0)}
        rep["probes"]["zstd_checksum_corrupt"] = {
            "status": z_status, "warning_delta": dict(sorted(delta.items())),
            "upstream_content_encoding": dict(sorted(provider.upstream_encodings.items())),
            "upstream_original_email_in_eligible_field": provider.obs.original_in_eligible,
        }
        if delta != {"encoded_body_invalid_forwarded_unchanged": 1} or provider.upstream_encodings.get("zstd") != 1:
            f.append("probe_zstd_corrupt_unexpected")

        # --- status, reset
        rc1, out1, _, _ = manual.control(args.dmz, "status", port, token, tmp, deadline)
        st1 = manual.parse_status(out1) if rc1 == 0 else None
        rc2, _, _, _ = manual.control(args.dmz, "reset", port, token, tmp, deadline)
        rc3, out3, _, _ = manual.control(args.dmz, "status", port, token, tmp, deadline)
        st3 = manual.parse_status(out3) if rc3 == 0 else None
        leaked = token in out1 or token in out3
        del out1, out3
        rep["control"] = {"status_before_reset": st1, "reset_exit_code": rc2, "status_after_reset": st3,
                          "token_in_control_output": leaked}
        if st1 is None or not st1.get("changed_parts"):
            f.append("status_changed_parts_missing")
        if rc2 != 0 or st3 is None or st3.get("sessions") != 0:
            f.append("reset_failed")
        if leaked:
            f.append("token_in_control_output")
    except OSError:
        f.append("harness_os_error")
    finally:
        if ingress is not None:
            ingress.shutdown()
            ingress.server_close()
        if service is not None:
            rc = service.stop()
            text = service.text()
            rep["service"] = {
                "warnings": snapshot_warnings(service), "graceful_exit_code": rc,
                "secret_in_service_output": (token in text or base.FAKE_OPENAI_KEY in text
                                             or base.PROMPT_EMAIL in text or bool(base.HEX64_RE.search(text))),
            }
            if rep["service"]["secret_in_service_output"]:
                f.append("secret_in_service_output")
            del text
        for srv in (provider, sink):
            if srv is not None:
                srv.shutdown()
                srv.server_close()
        if sink is not None:
            rep["egress_attempts_blocked_total"] = sink.attempts
        # Artifact scan of the isolated tree, excluding the owned synthetic seed.
        hits = {"prompt_email": 0, "local_token": 0, "fake_key": 0}
        if os.path.isdir(tmp):
            hits["prompt_email"] = manual_scan(tmp, [base.PROMPT_EMAIL], seeded)
            hits["local_token"] = manual_scan(tmp, [token], seeded)
            hits["fake_key"] = manual_scan(tmp, [base.FAKE_OPENAI_KEY], seeded)
        rep["artifact_scan_files_excluding_seed"] = hits
        if any(hits.values()):
            f.append("synthetic_secret_or_prompt_persisted")
        lingering = marked_pids(tmp)
        rep["lingering_processes"] = len(lingering)
        if lingering:
            f.append("lingering_processes")
        shutil.rmtree(tmp, ignore_errors=True)
        rep["temporary_tree_removed"] = not os.path.exists(tmp)
    return rep


def manual_scan(root, needles, skip):
    hits = 0
    for dirpath, _, files in os.walk(root):
        for name in files:
            p = os.path.join(dirpath, name)
            if p in skip or os.path.islink(p):
                continue
            try:
                if os.path.getsize(p) > 32 << 20:
                    continue
                with open(p, "rb") as fh:
                    data = fh.read()
            except OSError:
                continue
            if any(n.encode() in data for n in needles):
                hits += 1
    return hits


def marked_pids(tmp):
    """Own processes whose command line or cwd references the owned temp tree."""
    pids = []
    needle = tmp.encode()
    for name in os.listdir("/proc"):
        if not name.isdigit() or int(name) == os.getpid():
            continue
        try:
            with open(f"/proc/{name}/cmdline", "rb") as fh:
                cmd = fh.read()
            cwd = os.readlink(f"/proc/{name}/cwd").encode()
        except OSError:
            continue
        if needle in cmd or cwd.startswith(needle):
            pids.append(int(name))
    return pids


def main(argv=None):
    ap = argparse.ArgumentParser(description="Synthetic loopback check of compressed Codex requests through dmz serve.")
    ap.add_argument("--dmz", required=True, help="dmz executable built from this checkout")
    ap.add_argument("--codex", default=shutil.which("codex"))
    ap.add_argument("--variant", choices=("custom", "builtin-diagnostic"), default="custom",
                    help="custom: manual custom provider (the route under test). builtin-diagnostic: built-in "
                         "provider with openai_base_url; the owned ingress proxy adds the local header, so it "
                         "only diagnoses whether the client compresses and is never route evidence.")
    ap.add_argument("--total-timeout", type=float, default=240.0)
    ap.add_argument("--scenario-timeout", type=float, default=120.0)
    args = ap.parse_args(argv)
    deadline = base.Deadline(args.total_timeout)
    report = {
        "harness": PROG,
        "evidence_kind": "real-codex-cli-compressed-requests-via-manual-dmz-serve-header-auth-synthetic-loopback",
        "route": ("manual-custom-provider-requires-openai-auth-compression-enabled-fixture" if args.variant == "custom"
                  else "DIAGNOSTIC-builtin-provider-openai-base-url-harness-injected-local-header-not-route-evidence"),
        "production_launcher_compression": "unchanged-disabled",
        "not_evidence_of": ["desktop-application-compatibility", "provider-compatibility",
                            "account-or-auth-validation", "model-behavior", "detector-recall"],
        "versions": {"python": platform.python_version(),
                     "dmz": base.version_of([args.dmz, "version"], r"dmz local gateway [^\n]{1,80}"),
                     "codex": base.version_of([args.codex, "--version"], r"codex-cli \d+\.\d+\.\d+") if args.codex else None,
                     "zstd_tool": shutil.which("zstd") is not None},
    }
    if not report["versions"]["dmz"]:
        diag("dmz_binary_unavailable")
        report["overall"] = "fail"
        print(json.dumps(report, indent=2))
        return 2
    if not report["versions"]["codex"] or not report["versions"]["zstd_tool"]:
        diag("codex_or_zstd_unavailable")
        report["overall"] = "incomplete"
        print(json.dumps(report, indent=2))
        return 3
    before = base.config_fingerprint()
    rep = run(args, deadline)
    report.update(rep)
    report["normal_config_unchanged"] = before == base.config_fingerprint()
    if not report["normal_config_unchanged"]:
        rep["failures"].append("normal_client_config_changed")
    for code in rep["failures"]:
        diag(code)
    for code in rep["incomplete"]:
        diag("incomplete." + code)
    if rep["failures"]:
        report["overall"], code = "fail", 1
    elif rep["incomplete"] or args.variant != "custom":
        report["overall"], code = "incomplete", 3
    else:
        report["overall"], code = "pass", 0
    print(json.dumps(report, indent=2))
    return code


if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        diag("interrupted")
        sys.exit(130)
