#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Local integration smoke test: real coding CLIs -> dmz launcher -> fake API.

This harness runs the *installed* Codex and Claude Code command-line clients
through ``dmz run`` against synthetic, loopback-only fake provider servers. It
proves request-shape compatibility and masking of synthetic user text on this
machine only. It is NOT provider, account or model validation: model requests
are answered by loopback fixtures, every credential is a fake string and every
prompt, file and answer is synthetic (example.test addresses). The refusing
proxy observes cooperating clients; it is not a system-wide network jail.

Output contract:
  * stdout: one JSON document with versions and aggregate counts/booleans only.
  * stderr: fixed reason codes only (``agent-client-smoke: <code>``).
  * Request bodies, client output and launcher output are held in memory for
    the duration of a scenario and never printed or written to disk.

Isolation:
  * Child processes receive an allowlisted environment: provider credentials,
    endpoints, telemetry and session variables of the caller are dropped.
  * CODEX_HOME and CLAUDE_CONFIG_DIR point at per-run temporary directories
    (documented client isolation for the child only). The caller's HOME and
    shell environment are not modified.
  * HTTP(S)_PROXY points at a local refusing sink so a client that tries to
    reach any non-loopback host is counted and blocked; NO_PROXY keeps the
    loopback launcher and fake servers direct.
  * Tool use is limited to reading one owned synthetic fixture file in a
    temporary working directory (Codex read-only sandbox, Claude Read tool).

Python standard library only.
"""

from __future__ import annotations

import argparse
import json
import os
import platform
import re
import shutil
import signal
import socket
import socketserver
import subprocess
import sys
import tempfile
import threading
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PROG = "agent-client-smoke"

# --- Synthetic data -------------------------------------------------------
NONCE = "DMZ-SMOKE-7F3Q"
PROMPT_EMAIL = "rakel.synthetic@example.test"
FIXTURE_EMAIL = "fixture.owner@example.test"
FIXTURE_NAME = "dmz-smoke-fixture.txt"
FIXTURE_TEXT = f"synthetic fixture line; contact {FIXTURE_EMAIL}; token-free\n"
FINAL_TEXT = "SYNTHETIC-FINAL-ANSWER-91K"
AUX_TEXT = "SYNTHETIC-AUX-OK"
FAKE_OPENAI_KEY = "sk-dmz-synthetic-fixture-openai-000000000000"
FAKE_ANTHROPIC_KEY = "sk-ant-dmz-synthetic-fixture-000000000000"
MARKER_RE = re.compile(r"⟦EMAIL-[^⟧]{1,80}⟧")
SANDBOX_ERROR_RE = re.compile(r"bwrap|landlock|seccomp|sandbox|operation not permitted", re.I)
HEX64_RE = re.compile(r"(?<![0-9a-f])[0-9a-f]{64}(?![0-9a-f])")
# Fixture models: names the clients already know, used only inside the
# isolated configuration of this harness and only answered by the fake server.
CODEX_MODEL = "gpt-5.1-codex"
CLAUDE_MODEL = "claude-sonnet-4-5"

# Background features that contact plugin, app or model-discovery services are
# disabled for this synthetic run; any remaining attempt is counted by the sink.
CODEX_DISABLED_FEATURES = ("plugins", "remote_plugin", "plugin_discovery_prefetch", "apps", "api_key_model_discovery",
                           "tool_suggest", "skill_mcp_dependency_install", "in_app_updates", "browser_use",
                           "computer_use", "image_generation", "multi_agent", "system_proxy_fallback")
CODEX_FEATURE_ARGS = tuple(a for f in CODEX_DISABLED_FEATURES for a in ("--disable", f))

ENV_ALLOW = ("PATH", "HOME", "USER", "LOGNAME", "LANG", "LC_ALL", "LC_CTYPE", "TZ", "SHELL")


def diag(code: str) -> None:
    """Emit a fixed reason code. Never pass request or client content."""
    if not re.fullmatch(r"[a-z0-9_.:\-]{1,96}", code):
        code = "invalid_diagnostic_code"
    sys.stderr.write(f"{PROG}: {code}\n")
    sys.stderr.flush()


# --- Process control -------------------------------------------------------
class Deadline:
    def __init__(self, seconds: float) -> None:
        self.end = time.monotonic() + seconds

    def remaining(self, cap: float) -> float:
        return max(0.0, min(cap, self.end - time.monotonic()))


def kill_tree(proc: subprocess.Popen) -> None:
    if proc.poll() is not None:
        return
    try:
        os.killpg(proc.pid, signal.SIGTERM)
    except (ProcessLookupError, PermissionError):
        return
    try:
        proc.wait(timeout=3)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except (ProcessLookupError, PermissionError):
            pass
        try:
            proc.wait(timeout=3)
        except subprocess.TimeoutExpired:
            pass


def run_captured(argv, env, cwd, timeout):
    """Run in a new process group; capture output in memory only."""
    proc = subprocess.Popen(argv, env=env, cwd=cwd, stdin=subprocess.DEVNULL,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                            start_new_session=True)
    try:
        out, err = proc.communicate(timeout=timeout)
        timed_out = False
    except subprocess.TimeoutExpired:
        kill_tree(proc)
        out, err = proc.communicate()
        timed_out = True
    finally:
        # The client may have spawned helpers in its group; leave none behind.
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except (ProcessLookupError, PermissionError):
            pass
    return proc.returncode, out.decode("utf-8", "replace"), err.decode("utf-8", "replace"), timed_out


def version_of(argv, pattern, timeout=15):
    try:
        rc, out, err, to = run_captured(argv, {"PATH": os.environ.get("PATH", "")}, None, timeout)
    except OSError:
        return None
    m = re.search(pattern, out + "\n" + err)
    return m.group(0) if m and not to else None


# --- Egress sink -----------------------------------------------------------
class _SinkHandler(socketserver.BaseRequestHandler):
    def handle(self):
        self.server.attempts += 1  # type: ignore[attr-defined]
        try:
            self.request.settimeout(2)
            self.request.recv(4096)
            self.request.sendall(b"HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
        except OSError:
            pass


class EgressSink(socketserver.ThreadingTCPServer):
    daemon_threads = True
    allow_reuse_address = True

    def __init__(self):
        super().__init__(("127.0.0.1", 0), _SinkHandler)
        self.attempts = 0


# --- Fake provider ---------------------------------------------------------
class Observation:
    """Aggregates only. Bodies are inspected in memory and then discarded."""

    def __init__(self):
        self.lock = threading.Lock()
        self.requests = 0
        self.routes: dict[str, int] = {}
        self.unknown_routes = 0
        self.dmz_header_requests = 0
        self.fake_credential_requests = 0
        self.other_credential_requests = 0
        self.session_header_requests = 0
        self.session_ids: set[str] = set()
        self.model_requests = 0
        self.main_requests = 0
        self.stream_requests = 0
        self.json_parse_failures = 0
        self.shape_supported = 0
        self.original_in_eligible = 0
        self.original_anywhere = 0
        self.marker_in_eligible = 0
        self.nonce_seen = 0
        self.markers_by_request: list[tuple[str, ...]] = []
        self.tool_result_requests = 0
        self.tool_result_fixture_intact = 0
        self.tool_result_fixture_masked = 0
        self.tool_result_sandbox_error = 0
        self.tool_calls_sent = 0
        self.final_answers_sent = 0
        self.tool_names: set[str] = set()
        self.tool_call_unavailable = 0
        self.instructions_original_kept = 0
        self.first_marker: str | None = None

    def bump(self, attr, n=1):
        setattr(self, attr, getattr(self, attr) + n)


def _texts_responses_user(body):
    """Eligible fields for Responses: user message text (string or input_text)."""
    out = []
    inp = body.get("input")
    if isinstance(inp, str):
        return [inp]
    if not isinstance(inp, list):
        return out
    for item in inp:
        if not isinstance(item, dict) or item.get("role") != "user" or item.get("type", "message") != "message":
            continue
        content = item.get("content")
        if isinstance(content, str):
            out.append(content)
        elif isinstance(content, list):
            for part in content:
                if isinstance(part, dict) and part.get("type") == "input_text" and isinstance(part.get("text"), str):
                    out.append(part["text"])
    return out


def _tool_outputs_responses(body):
    out = []
    for item in body.get("input") or []:
        if isinstance(item, dict) and str(item.get("type", "")).endswith("_output"):
            o = item.get("output")
            out.append(o if isinstance(o, str) else json.dumps(o))
    return out


def _texts_messages_user(body):
    out = []
    for msg in body.get("messages") or []:
        if not isinstance(msg, dict) or msg.get("role") != "user":
            continue
        content = msg.get("content")
        if isinstance(content, str):
            out.append(content)
        elif isinstance(content, list):
            for block in content:
                if isinstance(block, dict) and block.get("type") == "text" and isinstance(block.get("text"), str):
                    out.append(block["text"])
    return out


def _tool_outputs_messages(body):
    out = []
    for msg in body.get("messages") or []:
        if not isinstance(msg, dict) or not isinstance(msg.get("content"), list):
            continue
        for block in msg["content"]:
            if isinstance(block, dict) and block.get("type") == "tool_result":
                out.append(json.dumps(block.get("content")))
    return out


def _sse(events):
    chunks = []
    for name, data in events:
        chunks.append(f"event: {name}\ndata: {json.dumps(data, ensure_ascii=False)}\n\n")
    return "".join(chunks).encode("utf-8")


def _codex_tool_call(tools, workdir):
    """Pick a read-only shell tool from the client's own schema; read the fixture."""
    by_name = {}
    for t in tools or []:
        if isinstance(t, dict) and t.get("type") == "function" and isinstance(t.get("name"), str):
            by_name[t["name"]] = t
    for name in ("exec_command", "shell_command", "shell", "local_shell"):
        tool = by_name.get(name)
        if not tool:
            continue
        props = ((tool.get("parameters") or {}).get("properties") or {})
        args = {}
        if "cmd" in props:
            args["cmd"] = f"cat {FIXTURE_NAME}"
        elif "command" in props:
            if (props["command"] or {}).get("type") == "array":
                args["command"] = ["cat", FIXTURE_NAME]
            else:
                args["command"] = f"cat {FIXTURE_NAME}"
        else:
            continue
        if "workdir" in props:
            args["workdir"] = workdir
        return name, args
    return None


class FakeProvider(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, protocol, scenario, workdir, model):
        super().__init__(("127.0.0.1", 0), _FakeHandler)
        self.protocol = protocol  # "responses" | "messages"
        self.scenario = scenario  # "basic" | "roundtrip"
        self.workdir = workdir
        self.model = model
        self.obs = Observation()


class _FakeHandler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    server: FakeProvider

    def log_message(self, *_):  # never log request lines or bodies
        pass

    # ---- helpers
    def _send(self, status, ctype, payload: bytes):
        self.send_response(status)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(payload)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(payload)

    def _common(self):
        obs = self.server.obs
        with obs.lock:
            obs.requests += 1
            key = f"{self.command} {self.path.split('?')[0]}"
            obs.routes[key] = obs.routes.get(key, 0) + 1
            if any(k.lower().startswith("x-dmz-") for k in self.headers.keys()):
                obs.dmz_header_requests += 1
            if self.server.protocol == "responses":
                cred = self.headers.get("Authorization", "")
                fake = cred == f"Bearer {FAKE_OPENAI_KEY}"
            else:
                cred = self.headers.get("x-api-key", "") or self.headers.get("Authorization", "")
                fake = cred in (FAKE_ANTHROPIC_KEY, f"Bearer {FAKE_ANTHROPIC_KEY}")
            if fake:
                obs.fake_credential_requests += 1
            elif cred:
                obs.other_credential_requests += 1
            sid = self.headers.get("X-Claude-Code-Session-Id")
            if sid:
                obs.session_header_requests += 1
                obs.session_ids.add(sid)

    def do_GET(self):
        self._common()
        path = self.path.split("?")[0]
        if path == "/v1/models":
            if self.server.protocol == "responses":
                payload = {"object": "list", "data": [{"id": self.server.model, "object": "model", "owned_by": "synthetic"}]}
            else:
                payload = {"data": [{"id": self.server.model, "type": "model", "display_name": "synthetic fixture"}], "has_more": False}
            self._send(200, "application/json", json.dumps(payload).encode())
            return
        with self.server.obs.lock:
            self.server.obs.unknown_routes += 1
        self._send(404, "application/json", b'{"error":{"type":"not_found"}}')

    def do_POST(self):
        self._common()
        obs = self.server.obs
        path = self.path.split("?")[0]
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length > 0 else b""
        try:
            body = json.loads(raw.decode("utf-8")) if raw else {}
        except (UnicodeDecodeError, json.JSONDecodeError):
            body = None
        proto = self.server.protocol
        if proto == "responses" and path == "/v1/responses":
            self._responses(body, raw)
        elif proto == "messages" and path == "/v1/messages":
            self._messages(body, raw)
        elif proto == "messages" and path == "/v1/messages/count_tokens":
            self._send(200, "application/json", b'{"input_tokens":12}')
        else:
            with obs.lock:
                obs.unknown_routes += 1
            self._send(404, "application/json", b'{"error":{"type":"not_found"}}')
        del raw, body  # bodies never outlive the handler

    # ---- body inspection shared by both protocols
    def _inspect(self, body, raw, user_texts, tool_outputs, instructions_text):
        obs = self.server.obs
        with obs.lock:
            obs.model_requests += 1
            if body is None or not isinstance(body, dict):
                obs.json_parse_failures += 1
                return False, False
            if body.get("stream") is True:
                obs.stream_requests += 1
            joined = "\n".join(user_texts)
            is_main = NONCE in raw.decode("utf-8", "replace")
            if not is_main:
                return False, bool(tool_outputs)
            obs.main_requests += 1
            obs.shape_supported += 1
            if NONCE in joined:
                obs.nonce_seen += 1
            if PROMPT_EMAIL in joined:
                obs.original_in_eligible += 1
            if PROMPT_EMAIL.encode() in raw:
                obs.original_anywhere += 1
            markers = tuple(MARKER_RE.findall(joined))
            if markers:
                obs.marker_in_eligible += 1
                if obs.first_marker is None:
                    obs.first_marker = markers[0]
            obs.markers_by_request.append(markers)
            if instructions_text and FIXTURE_EMAIL in instructions_text:
                obs.instructions_original_kept += 1
            if tool_outputs:
                obs.tool_result_requests += 1
                joined_out = "\n".join(tool_outputs)
                if FIXTURE_EMAIL in joined_out:
                    obs.tool_result_fixture_intact += 1
                if MARKER_RE.search(joined_out):
                    obs.tool_result_fixture_masked += 1
                if FIXTURE_EMAIL not in joined_out and SANDBOX_ERROR_RE.search(joined_out):
                    obs.tool_result_sandbox_error += 1
            return True, bool(tool_outputs)

    def _final_text(self):
        marker = self.server.obs.first_marker or "no-marker"
        return f"{FINAL_TEXT} {marker}"

    # ---- Responses (Codex)
    def _responses(self, body, raw):
        user_texts = _texts_responses_user(body) if isinstance(body, dict) else []
        tool_outputs = _tool_outputs_responses(body) if isinstance(body, dict) else []
        instr = body.get("instructions") if isinstance(body, dict) and isinstance(body.get("instructions"), str) else ""
        is_main, has_tool_out = self._inspect(body, raw, user_texts, tool_outputs, instr)
        obs = self.server.obs
        if isinstance(body, dict):
            with obs.lock:
                for t in body.get("tools") or []:
                    if isinstance(t, dict) and isinstance(t.get("name"), str):
                        obs.tool_names.add(t["name"])
        rid = "resp_" + uuid.uuid4().hex[:16]
        model = (body or {}).get("model") if isinstance(body, dict) else self.server.model
        item = None
        if is_main and self.server.scenario == "roundtrip" and not has_tool_out:
            call = _codex_tool_call(body.get("tools"), self.server.workdir)
            if call:
                name, args = call
                item = {"type": "function_call", "id": "fc_" + uuid.uuid4().hex[:12], "call_id": "call_" + uuid.uuid4().hex[:12],
                        "name": name, "arguments": json.dumps(args), "status": "completed"}
                with obs.lock:
                    obs.tool_calls_sent += 1
            else:
                with obs.lock:
                    obs.tool_call_unavailable += 1
        if item is None:
            text = self._final_text() if is_main else AUX_TEXT
            if is_main:
                with obs.lock:
                    obs.final_answers_sent += 1
            mid = "msg_" + uuid.uuid4().hex[:12]
            item = {"type": "message", "id": mid, "status": "completed", "role": "assistant",
                    "content": [{"type": "output_text", "text": text, "annotations": []}]}
            events_mid = [
                ("response.output_item.added", {"type": "response.output_item.added", "output_index": 0,
                                                "item": {**item, "status": "in_progress", "content": []}}),
                ("response.content_part.added", {"type": "response.content_part.added", "item_id": mid, "output_index": 0,
                                                 "content_index": 0, "part": {"type": "output_text", "text": "", "annotations": []}}),
                ("response.output_text.delta", {"type": "response.output_text.delta", "item_id": mid, "output_index": 0,
                                                "content_index": 0, "delta": text}),
                ("response.output_text.done", {"type": "response.output_text.done", "item_id": mid, "output_index": 0,
                                               "content_index": 0, "text": text}),
            ]
        else:
            events_mid = [("response.output_item.added", {"type": "response.output_item.added", "output_index": 0, "item": item})]
        created = int(time.time())
        base = {"id": rid, "object": "response", "created_at": created, "model": model, "status": "in_progress", "output": []}
        usage = {"input_tokens": 12, "input_tokens_details": {"cached_tokens": 0}, "output_tokens": 6,
                 "output_tokens_details": {"reasoning_tokens": 0}, "total_tokens": 18}
        events = [("response.created", {"type": "response.created", "response": base}),
                  ("response.in_progress", {"type": "response.in_progress", "response": base})]
        events += events_mid
        events.append(("response.output_item.done", {"type": "response.output_item.done", "output_index": 0, "item": item}))
        events.append(("response.completed", {"type": "response.completed",
                                              "response": {**base, "status": "completed", "output": [item], "usage": usage}}))
        self._send(200, "text/event-stream", _sse(events))

    # ---- Messages (Claude)
    def _messages(self, body, raw):
        user_texts = _texts_messages_user(body) if isinstance(body, dict) else []
        tool_outputs = _tool_outputs_messages(body) if isinstance(body, dict) else []
        system = body.get("system") if isinstance(body, dict) else None
        instr = json.dumps(system) if system is not None else ""
        is_main, has_tool_out = self._inspect(body, raw, user_texts, tool_outputs, instr)
        obs = self.server.obs
        tools = body.get("tools") if isinstance(body, dict) else None
        if isinstance(tools, list):
            with obs.lock:
                for t in tools:
                    if isinstance(t, dict) and isinstance(t.get("name"), str):
                        obs.tool_names.add(t["name"])
        model = body.get("model") if isinstance(body, dict) else self.server.model
        block = None
        stop = "end_turn"
        if is_main and self.server.scenario == "roundtrip" and not has_tool_out:
            if isinstance(tools, list) and any(isinstance(t, dict) and t.get("name") == "Read" for t in tools):
                block = {"type": "tool_use", "id": "toolu_" + uuid.uuid4().hex[:16], "name": "Read",
                         "input": {"file_path": os.path.join(self.server.workdir, FIXTURE_NAME)}}
                stop = "tool_use"
                with obs.lock:
                    obs.tool_calls_sent += 1
            else:
                with obs.lock:
                    obs.tool_call_unavailable += 1
        if block is None:
            text = self._final_text() if is_main else AUX_TEXT
            if is_main:
                with obs.lock:
                    obs.final_answers_sent += 1
            block = {"type": "text", "text": text}
        mid = "msg_" + uuid.uuid4().hex[:16]
        usage = {"input_tokens": 12, "output_tokens": 6, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}
        if not (isinstance(body, dict) and body.get("stream") is True):
            payload = {"id": mid, "type": "message", "role": "assistant", "model": model, "content": [block],
                       "stop_reason": stop, "stop_sequence": None, "usage": usage}
            self._send(200, "application/json", json.dumps(payload).encode())
            return
        events = [("message_start", {"type": "message_start", "message": {"id": mid, "type": "message", "role": "assistant",
                                                                          "model": model, "content": [], "stop_reason": None,
                                                                          "stop_sequence": None, "usage": {**usage, "output_tokens": 1}}})]
        if block["type"] == "text":
            events += [("content_block_start", {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}),
                       ("content_block_delta", {"type": "content_block_delta", "index": 0,
                                                "delta": {"type": "text_delta", "text": block["text"]}})]
        else:
            events += [("content_block_start", {"type": "content_block_start", "index": 0,
                                                "content_block": {**block, "input": {}}}),
                       ("content_block_delta", {"type": "content_block_delta", "index": 0,
                                                "delta": {"type": "input_json_delta", "partial_json": json.dumps(block["input"])}})]
        events += [("content_block_stop", {"type": "content_block_stop", "index": 0}),
                   ("message_delta", {"type": "message_delta", "delta": {"stop_reason": stop, "stop_sequence": None},
                                      "usage": {"output_tokens": 6}}),
                   ("message_stop", {"type": "message_stop"})]
        self._send(200, "text/event-stream", _sse(events))


# --- Scenario runner -------------------------------------------------------
LAUNCH_OBS_RE = re.compile(r"^dmz: observed (\d+) model-route requests, (\d+) text parts, (\d+) changed parts\.$", re.M)
LAUNCH_WARN_RE = re.compile(r"^dmz warning: ([a-z0-9_]{1,64})$", re.M)
LAUNCH_WARN_COUNT_RE = re.compile(r"^dmz: warning-count ([a-z0-9_]{1,64})=(\d+)$", re.M)


def child_env(tmp, sink_port, client):
    env = {k: os.environ[k] for k in ENV_ALLOW if k in os.environ}
    env.update({
        "TERM": "dumb", "NO_COLOR": "1", "TMPDIR": os.path.join(tmp, "tmp"),
        "HTTP_PROXY": f"http://127.0.0.1:{sink_port}", "HTTPS_PROXY": f"http://127.0.0.1:{sink_port}",
        "http_proxy": f"http://127.0.0.1:{sink_port}", "https_proxy": f"http://127.0.0.1:{sink_port}",
        "ALL_PROXY": f"http://127.0.0.1:{sink_port}", "all_proxy": f"http://127.0.0.1:{sink_port}",
        "NO_PROXY": "127.0.0.1,localhost,::1", "no_proxy": "127.0.0.1,localhost,::1",
    })
    if client == "codex":
        env["CODEX_HOME"] = os.path.join(tmp, "codex-home")
        env["OPENAI_API_KEY"] = FAKE_OPENAI_KEY
        env["CODEX_DISABLE_UPDATE_CHECK"] = "1"
    else:
        env["CLAUDE_CONFIG_DIR"] = os.path.join(tmp, "claude-config")
        env["ANTHROPIC_API_KEY"] = FAKE_ANTHROPIC_KEY
        env["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] = "1"
        env["DISABLE_TELEMETRY"] = "1"
        env["DISABLE_ERROR_REPORTING"] = "1"
        env["DISABLE_AUTOUPDATER"] = "1"
        env["DISABLE_BUG_COMMAND"] = "1"
        env["CLAUDE_CODE_DISABLE_TERMINAL_TITLE"] = "1"
    return env


def build_prompt(scenario):
    base = (f"[{NONCE}] Synthetic test request. Please draft a one-line reply to {PROMPT_EMAIL} "
            f"confirming the meeting.")
    if scenario == "roundtrip":
        base += f" First read the local file {FIXTURE_NAME} in the working directory."
    return base


def client_argv(dmz, client, client_bin, upstream_port, scenario, workdir, codex_sandbox="read-only"):
    launcher = [dmz, "run", "--client-bin", client_bin, "--allow-loopback-upstream",
                "--upstream", f"http://127.0.0.1:{upstream_port}/v1", client]
    prompt = build_prompt(scenario)
    if client == "codex":
        return launcher + ["exec", "--ignore-user-config", "--ignore-rules", "--ephemeral", "--sandbox", codex_sandbox,
                           "--skip-git-repo-check", "--color", "never", "-C", workdir, "-m", CODEX_MODEL,
                           "-c", 'approval_policy="never"', "-c", "mcp_servers={}",
                           "-c", "check_for_update_on_startup=false", "-c", "analytics.enabled=false",
                           "-c", "feedback.enabled=false", "-c", "web_search=\"disabled\"",
                           *CODEX_FEATURE_ARGS, prompt]
    tools = "Read" if scenario == "roundtrip" else ""
    argv = launcher + ["--bare", "-p", "--no-session-persistence", "--setting-sources", "",
                       "--strict-mcp-config", "--tools", tools, "--model", CLAUDE_MODEL,
                       "--output-format", "text", "--max-turns", "4"]
    if scenario == "roundtrip":
        argv += ["--allowedTools", "Read", "--add-dir", workdir]
    return argv + ["--", prompt]


def run_scenario(args, client, client_bin, scenario, deadline):
    result = {"client": client, "scenario": scenario, "status": "fail", "failures": [], "incomplete": []}
    fails = result["failures"]
    tmp = tempfile.mkdtemp(prefix="dmz-agent-smoke-")
    sink = provider = None
    try:
        for sub in ("tmp", "codex-home", "claude-config", "work"):
            os.makedirs(os.path.join(tmp, sub), mode=0o700)
        workdir = os.path.join(tmp, "work")
        with open(os.path.join(workdir, FIXTURE_NAME), "w", encoding="utf-8") as fh:
            fh.write(FIXTURE_TEXT)
        sink = EgressSink()
        threading.Thread(target=sink.serve_forever, daemon=True).start()
        provider = FakeProvider("responses" if client == "codex" else "messages", scenario, workdir,
                                CODEX_MODEL if client == "codex" else CLAUDE_MODEL)
        threading.Thread(target=provider.serve_forever, daemon=True).start()
        env = child_env(tmp, sink.server_address[1], client)
        argv = client_argv(args.dmz, client, client_bin, provider.server_address[1], scenario, workdir,
                           args.codex_roundtrip_sandbox if scenario == "roundtrip" else "read-only")
        timeout = deadline.remaining(args.scenario_timeout)
        if timeout < 5:
            fails.append("total_timeout_before_start")
            return result
        rc, out, err, timed_out = run_captured(argv, env, workdir, timeout)
        obs = provider.obs
        # --- launcher fixed lines (aggregate only)
        m = LAUNCH_OBS_RE.search(err)
        warnings: dict[str, int] = {}
        for code in LAUNCH_WARN_RE.findall(err):
            warnings[code] = warnings.get(code, 0) + 1
        for code, count in LAUNCH_WARN_COUNT_RE.findall(err):
            warnings[code] = int(count)
        marker_sets = [s for s in obs.markers_by_request if s]
        stable = len(marker_sets) >= 2 and all(s[0] == marker_sets[0][0] for s in marker_sets)
        combined = out + "\n" + err
        result.update({
            "exit_code": rc,
            "timed_out": timed_out,
            "launcher": {"model_route_requests": int(m.group(1)) if m else None,
                         "text_parts": int(m.group(2)) if m else None,
                         "changed_parts": int(m.group(3)) if m else None,
                         "warnings": dict(sorted(warnings.items()))},
            "upstream": {
                "requests": obs.requests,
                "routes": dict(sorted(obs.routes.items())),
                "unknown_routes": obs.unknown_routes,
                "model_requests": obs.model_requests,
                "stream_requests": obs.stream_requests,
                "main_requests": obs.main_requests,
                "json_parse_failures": obs.json_parse_failures,
                "x_dmz_header_requests": obs.dmz_header_requests,
                "fake_credential_requests": obs.fake_credential_requests,
                "other_credential_requests": obs.other_credential_requests,
                "session_header_requests": obs.session_header_requests,
                "distinct_session_ids": len(obs.session_ids),
                "original_email_in_eligible_field": obs.original_in_eligible,
                "original_email_anywhere_in_body": obs.original_anywhere,
                "marker_in_eligible_field": obs.marker_in_eligible,
                "nonce_in_eligible_field": obs.nonce_seen,
                "marker_stable_across_requests": stable if len(marker_sets) >= 2 else None,
                "tool_calls_sent": obs.tool_calls_sent,
                "tool_call_unavailable": obs.tool_call_unavailable,
                "tool_result_requests": obs.tool_result_requests,
                "tool_result_fixture_unchanged": obs.tool_result_fixture_intact,
                "tool_result_fixture_masked": obs.tool_result_fixture_masked,
                "tool_result_sandbox_error": obs.tool_result_sandbox_error,
                "final_answers_sent": obs.final_answers_sent,
                "client_tool_count": len(obs.tool_names),
            },
            "client_output": {
                "final_answer_delivered": FINAL_TEXT in out,
                "marker_returned_unrestored": bool(obs.first_marker) and obs.first_marker in out,
                "original_email_in_output": PROMPT_EMAIL in out,
                "fake_key_in_output": FAKE_OPENAI_KEY in combined or FAKE_ANTHROPIC_KEY in combined,
                "hex64_token_like_in_output": bool(HEX64_RE.search(combined)),
            },
            "egress_attempts_blocked": sink.attempts,
        })
        # --- verdict
        if timed_out:
            fails.append("client_timeout")
        if rc != 0:
            fails.append("client_nonzero_exit")
        if obs.requests == 0:
            fails.append("no_upstream_traffic")
        if obs.main_requests == 0:
            fails.append("main_request_not_observed")
        if obs.json_parse_failures:
            fails.append("upstream_body_not_json")
        if obs.unknown_routes:
            fails.append("unsupported_route_reached_upstream")
        if obs.dmz_header_requests:
            fails.append("x_dmz_header_leaked_upstream")
        if obs.other_credential_requests or (obs.model_requests and obs.fake_credential_requests == 0):
            fails.append("unexpected_provider_credential")
        if obs.original_in_eligible:
            fails.append("original_email_in_eligible_field")
        if obs.main_requests and obs.marker_in_eligible < obs.main_requests:
            fails.append("marker_missing_in_eligible_field")
        if client == "claude":
            if obs.model_requests and obs.session_header_requests < obs.model_requests:
                fails.append("claude_session_header_missing")
            if warnings.get("missing_session_identity_forwarded_unchanged"):
                fails.append("launcher_missing_session_identity")
        if not result["client_output"]["final_answer_delivered"]:
            fails.append("final_answer_not_delivered")
        if not result["client_output"]["original_email_in_output"]:
            fails.append("local_answer_not_restored")
        if result["client_output"]["marker_returned_unrestored"]:
            fails.append("known_reply_marker_left_unrestored")
        if result["client_output"]["fake_key_in_output"] or result["client_output"]["hex64_token_like_in_output"]:
            fails.append("credential_like_value_in_client_output")
        if m is None:
            fails.append("launcher_summary_missing")
        if scenario == "roundtrip":
            if obs.tool_calls_sent == 0:
                fails.append("roundtrip_tool_unavailable")
            elif obs.tool_result_requests == 0:
                fails.append("roundtrip_tool_result_not_returned")
            else:
                if obs.tool_result_fixture_intact == 0 and obs.tool_result_sandbox_error:
                    # The client's own sandbox cannot run here (e.g. container
                    # without user namespaces). The tool result still made the
                    # round trip; content integrity is not established.
                    result["incomplete"].append("client_sandbox_unavailable_tool_content_unverified")
                elif obs.tool_result_fixture_intact == 0:
                    fails.append("roundtrip_tool_output_not_fixture")
                if obs.tool_result_fixture_masked:
                    fails.append("roundtrip_tool_output_modified")
                if not stable:
                    fails.append("roundtrip_marker_not_stable")
        if not fails:
            result["status"] = "incomplete" if result["incomplete"] else "pass"
        del out, err, combined
        return result
    except OSError:
        fails.append("harness_os_error")
        return result
    finally:
        for srv in (provider, sink):
            if srv is not None:
                srv.shutdown()
                srv.server_close()
        shutil.rmtree(tmp, ignore_errors=True)


def config_fingerprint():
    """Stat (never read) the caller's normal client config to prove it is unchanged."""
    home = os.path.expanduser("~")
    paths = [os.path.join(home, ".codex", "config.toml"), os.path.join(home, ".codex", "auth.json"),
             os.path.join(home, ".claude.json"), os.path.join(home, ".claude", "settings.json"),
             os.path.join(home, ".claude", ".credentials.json")]
    fp = []
    for p in paths:
        try:
            st = os.stat(p)
            fp.append((p, st.st_mtime_ns, st.st_size))
        except OSError:
            fp.append((p, None, None))
    return fp


def main(argv=None):
    ap = argparse.ArgumentParser(description="Synthetic loopback smoke test of real coding CLIs through dmz run.")
    ap.add_argument("--dmz", required=True, help="dmz launcher binary")
    ap.add_argument("--codex", default=shutil.which("codex"), help="codex binary (default: PATH)")
    ap.add_argument("--claude", default=shutil.which("claude"), help="claude binary (default: PATH)")
    ap.add_argument("--clients", default="codex,claude")
    ap.add_argument("--scenarios", default="basic,roundtrip")
    ap.add_argument("--total-timeout", type=float, default=300.0)
    ap.add_argument("--scenario-timeout", type=float, default=90.0)
    ap.add_argument("--codex-roundtrip-sandbox", choices=("read-only", "danger-full-access"), default="read-only",
                    help="Codex sandbox for the roundtrip scenario. danger-full-access is opt-in for hosts that are "
                         "already externally sandboxed and cannot run Codex's own sandbox; the fake server only ever "
                         "requests `cat` of the owned synthetic fixture.")
    args = ap.parse_args(argv)
    deadline = Deadline(args.total_timeout)
    report = {
        "harness": PROG,
        "evidence_kind": "real-cli-binaries-with-synthetic-loopback-upstream",
        "not_evidence_of": ["provider-compatibility", "account-or-auth-validation", "model-behavior", "detector-recall"],
        "versions": {
            "python": platform.python_version(),
            "dmz": version_of([args.dmz, "version"], r"dmz local gateway [^\n]{1,80}"),
        },
        "results": [],
        "skipped": [],
    }
    if not report["versions"]["dmz"]:
        diag("dmz_binary_unavailable")
        report["overall"] = "fail"
        print(json.dumps(report, indent=2))
        return 2
    before = config_fingerprint()
    bins = {"codex": args.codex, "claude": args.claude}
    patterns = {"codex": r"codex-cli \d+\.\d+\.\d+", "claude": r"\d+\.\d+\.\d+ \(Claude Code\)"}
    for client in [c for c in args.clients.split(",") if c]:
        if client not in bins:
            diag("unknown_client")
            continue
        b = bins[client]
        ver = version_of([b, "--version"], patterns[client]) if b else None
        report["versions"][client] = ver
        if not ver:
            diag(f"{client}_binary_unavailable")
            report["skipped"].append({"client": client, "reason": "binary_unavailable"})
            continue
        for scenario in [s for s in args.scenarios.split(",") if s]:
            if scenario not in ("basic", "roundtrip"):
                diag("unknown_scenario")
                continue
            if deadline.remaining(1e9) < 5:
                report["skipped"].append({"client": client, "scenario": scenario, "reason": "total_timeout"})
                diag("total_timeout")
                continue
            r = run_scenario(args, client, b, scenario, deadline)
            for code in r["failures"]:
                diag(f"{client}.{scenario}.{code}")
            for code in r["incomplete"]:
                diag(f"{client}.{scenario}.incomplete.{code}")
            report["results"].append(r)
    report["normal_config_unchanged"] = before == config_fingerprint()
    if not report["normal_config_unchanged"]:
        diag("normal_client_config_changed")
    statuses = {r["status"] for r in report["results"]}
    if not report["results"] or "fail" in statuses or not report["normal_config_unchanged"]:
        report["overall"], code = "fail", 1
    elif statuses == {"pass"} and not report["skipped"]:
        report["overall"], code = "pass", 0
    else:
        report["overall"], code = "incomplete", 3
    report["codex_roundtrip_sandbox"] = args.codex_roundtrip_sandbox
    print(json.dumps(report, indent=2, sort_keys=False))
    return code


if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        diag("interrupted")
        sys.exit(130)
