#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Restoration smoke test: installed coding CLIs -> dmz run -> fake loopback API.

This harness proves, on this machine only, that the *installed* Codex and
Claude Code clients consume assistant answers whose known markers dmz restored
locally, and that when those clients resend their conversation history, the
restored originals are re-masked before the request leaves the launcher.

Scenarios (all synthetic; example.test addresses only):

  commentary  One launcher, one client turn. The fake model answers the first
              request with an assistant commentary TEXT that contains the
              marker issued for the prompt address, followed by a tool call in
              the same reply (marker also placed in the tool arguments). The
              client runs the tool and sends a second request carrying the
              assistant history. The provider must see that commentary text
              with the exact live marker and never the prompt address; tool
              arguments must come back byte-for-byte as issued.
  resume      Two launcher processes (fresh dmz mapping each). Turn 1 receives a
              restored final answer that the client persists on disk. Turn 2
              resumes that client session with a new address-free prompt. The
              provider must see turn-1 assistant history re-masked with the
              marker of the *new* live scope (rebuilt from the replayed user
              history) and never the prompt address.
  subagent-probe  (opt-in review probe, Claude only, non-bare.) The model asks
              the Agent tool to run a sub-agent whose answer contains the live
              marker. It reports whether the restored sub-agent answer re-enters
              the parent conversation (tool result or user text) outside exact
              re-mask coverage. A synthetic bearer-style probe value, masked in
              its prompt context, must never reach upstream afterwards.
  compaction-probe  (opt-in review probe.) Fake usage forces client
              auto-compaction; the fake summary repeats the live markers. The
              same probe value must never reach upstream when the restored
              summary re-enters the conversation. Clients that do not compact
              here report incomplete, never pass.
  Probe failures are real findings, not harness errors.

It is NOT provider, account, model or native GUI validation: every credential
is a fake string, every model reply is a loopback fixture, and the refusing
proxy observes cooperating clients only; it is not a network jail.

Output contract:
  * stdout: one JSON document with versions, fixed codes, counts and booleans.
  * stderr: fixed reason codes only (``agent-restoration-smoke: <code>``).
  * Request bodies, client output and launcher output stay in memory for one
    scenario and are never printed or written to disk by this harness.

Isolation: allowlisted child environment, per-run HOME, CODEX_HOME,
CLAUDE_CONFIG_DIR and TMPDIR, HTTP(S)_PROXY pointing at a refusing local sink,
NO_PROXY for loopback. Codex keeps its read-only sandbox; if that sandbox cannot
run on this host the tool CONTENT evidence is reported incomplete while history
replay is still checked. The temporary tree is removed afterwards.

Reuses the sink, process and environment helpers of agent-client-smoke.py.
Python standard library only.
"""

from __future__ import annotations

import argparse
import importlib.util
import json
import os
import platform
import re
import shutil
import sys
import tempfile
import threading
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PROG = "agent-restoration-smoke"
HERE = os.path.dirname(os.path.abspath(__file__))


def _load_base():
    sys.dont_write_bytecode = True  # never leave caches next to the scripts
    spec = importlib.util.spec_from_file_location("dmz_agent_client_smoke", os.path.join(HERE, "agent-client-smoke.py"))
    mod = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(mod)
    return mod


base = _load_base()

# --- Synthetic data -------------------------------------------------------
NONCE1 = "DMZ-RESTORE-T1-4KQ"
NONCE2 = "DMZ-RESTORE-T2-8PZ"
SUB_NONCE = "DMZ-RESTORE-SUB-3MV"
PROMPT_EMAIL = "sigrun.synthetic@example.test"          # must never reach upstream
FIXTURE_EMAIL = "tool.fixture.owner@example.test"      # tool data: a different address
FIXTURE_NAME = "dmz-restore-fixture.txt"
FIXTURE_TEXT = f"synthetic restoration fixture; contact {FIXTURE_EMAIL}; token-free\n"
# Reply tags never occur in prompts, so an echoed prompt cannot satisfy a check.
COMMENTARY_TAG = "SYNTH-COMMENTARY-Q7"
FINAL1_TAG = "SYNTH-FINAL-ONE-R2"
FINAL2_TAG = "SYNTH-FINAL-TWO-W5"
SUB_TAG = "SYNTH-SUBAGENT-K9"
SUMMARY_TAG = "SYNTH-SUMMARY-H4"
AUX_TEXT = "SYNTH-AUX-OK"
# Probe-only synthetic credential: masked in its "Authorization: Bearer" prompt
# context; used to test re-entry of a restored value without that context.
PROBE_TOKEN = "dmzSynthProbeQx2Lm9Vt4Rk8Wp3Zn6"
FAKE_OPENAI_KEY = base.FAKE_OPENAI_KEY
FAKE_ANTHROPIC_KEY = base.FAKE_ANTHROPIC_KEY
# Exact core-issued email marker shape (namespace is 16 upper-case hex digits).
ISSUED_RE = re.compile(r"⟦EMAIL-[0-9A-F]{16}-[0-9]+: [^⟦⟧\r\n]{1,128}⟧")
ANY_MARKER_RE = re.compile(r"⟦[A-Z]+-[^⟧]{1,160}⟧")
LIVE_ANY_RE = re.compile(r"⟦[A-Z]+-[0-9A-F]{16}-[0-9]+: [^⟦⟧\r\n]{1,128}⟧")
CODEX_MODEL = base.CODEX_MODEL
CLAUDE_MODEL = base.CLAUDE_MODEL
SCENARIOS = ("commentary", "resume", "subagent-probe", "compaction-probe")


def diag(code: str) -> None:
    if not re.fullmatch(r"[a-z0-9_.:\-]{1,120}", code):
        code = "invalid_diagnostic_code"
    sys.stderr.write(f"{PROG}: {code}\n")
    sys.stderr.flush()


def prompt_for(turn: int, scenario: str) -> str:
    if turn == 2:
        return f"[{NONCE2}] Synthetic follow-up. Please repeat the confirmation line from your previous answer."
    text = f"[{NONCE1}] Synthetic test request. Please draft a one-line reply to {PROMPT_EMAIL} confirming the meeting."
    if scenario == "commentary":
        text += f" First read the local file {FIXTURE_NAME} in the working directory."
    if scenario in ("subagent-probe", "compaction-probe"):
        text += (" Delegate the drafting to a helper agent. The helper may call the staging API with "
                 f"Authorization: Bearer {PROBE_TOKEN}")
    return text


# --- Observation -----------------------------------------------------------
class Obs:
    """Aggregates only. Bodies are inspected in memory and then discarded."""

    def __init__(self):
        self.lock = threading.Lock()
        self.requests = 0
        self.model_requests = 0
        self.unknown_routes = 0
        self.routes: dict[str, int] = {}
        self.json_parse_failures = 0
        self.x_dmz_header_requests = 0
        self.fake_credential_requests = 0
        self.other_credential_requests = 0
        self.identity_accept_encoding = 0
        self.session_ids: set[str] = set()
        self.session_header_requests = 0
        # privacy
        self.prompt_email_raw_requests = 0       # raw bytes, case-insensitive
        self.prompt_email_decoded_requests = 0   # any decoded JSON string
        self.prompt_email_in_tool_result = 0
        self.probe_token_raw_requests = 0
        self.probe_token_in_user_text = 0
        self.probe_token_in_tool_result = 0
        self.probe_marker_issued = 0
        self.prompt_email_in_user_text = 0
        self.prompt_email_in_assistant_text = 0
        # markers
        self.issued: list[str] = []              # markers seen in user text, in order (kept in memory only)
        self.user_marker_requests = 0
        # stage tracking
        self.main1 = 0
        self.main1_followups = 0
        self.main2 = 0
        self.sub_requests = 0
        self.sub_same_session_header = None
        self.parent_session_id = None
        # replay evidence
        self.history_checked_requests = 0
        self.history_tagged_texts = 0
        self.history_tagged_with_live_marker = 0
        self.history_tagged_with_other_marker = 0
        self.history_tagged_without_marker = 0
        self.history_tagged_with_original = 0
        # tool evidence
        self.tool_calls_sent = 0
        self.tool_call_unavailable = 0
        self.tool_result_requests = 0
        self.tool_result_fixture_intact = 0
        self.tool_result_sandbox_error = 0
        self.tool_args_echoed = 0
        self.tool_args_identical = 0
        self.tool_args_marker_kept = 0
        self.sub_tool_result_tagged = 0
        self.sub_tool_result_marker = 0
        self.compact_requests = 0
        self.compact_route_requests = 0
        self.post_compact_requests = 0
        self.summary_user_text = 0
        self.summary_user_text_live_marker = 0
        self.sub_user_text_tagged = 0
        self.sub_user_text_marker = 0
        # reply evidence
        self.replies_with_marker = 0
        self.fragmented_replies = 0
        self.markers_t1: list[str] = []
        self.markers_t2: list[str] = []


def _strings(v):
    if isinstance(v, str):
        yield v
    elif isinstance(v, dict):
        for k, x in v.items():
            yield k
            yield from _strings(x)
    elif isinstance(v, list):
        for x in v:
            yield from _strings(x)


def _responses_parts(body):
    """Return (user_texts, assistant_texts, tool_outputs, function_calls)."""
    users, assistants, outputs, calls = [], [], [], []
    inp = body.get("input")
    if isinstance(inp, str):
        return [inp], [], [], []
    for item in inp if isinstance(inp, list) else []:
        if not isinstance(item, dict):
            continue
        typ = item.get("type", "message")
        if typ == "message" and item.get("role") in ("user", "assistant"):
            content = item.get("content")
            texts = []
            if isinstance(content, str):
                texts.append(content)
            elif isinstance(content, list):
                for part in content:
                    if isinstance(part, dict) and part.get("type") in ("input_text", "output_text") and isinstance(part.get("text"), str):
                        texts.append(part["text"])
            (users if item["role"] == "user" else assistants).extend(texts)
        elif isinstance(typ, str) and typ.endswith("_output"):
            o = item.get("output")
            outputs.append(o if isinstance(o, str) else json.dumps(o, ensure_ascii=False))
        elif typ == "function_call":
            calls.append((item.get("call_id"), item.get("arguments")))
    return users, assistants, outputs, calls


def _messages_parts(body):
    users, assistants, outputs, calls = [], [], [], []
    for msg in body.get("messages") or []:
        if not isinstance(msg, dict) or msg.get("role") not in ("user", "assistant"):
            continue
        content = msg.get("content")
        bucket = users if msg["role"] == "user" else assistants
        if isinstance(content, str):
            bucket.append(content)
            continue
        for block in content if isinstance(content, list) else []:
            if not isinstance(block, dict):
                continue
            if block.get("type") == "text" and isinstance(block.get("text"), str):
                bucket.append(block["text"])
            elif block.get("type") == "tool_result":
                outputs.append(json.dumps(block.get("content"), ensure_ascii=False))
            elif block.get("type") == "tool_use":
                calls.append((block.get("id"), block.get("input")))
    return users, assistants, outputs, calls


def _last_user_text(body, proto):
    """Text of the final input item if it is a user message, else None."""
    items = body.get("input") if proto == "responses" else body.get("messages")
    if not isinstance(items, list) or not items or not isinstance(items[-1], dict):
        return None
    last = items[-1]
    if last.get("role") != "user":
        return None
    content = last.get("content")
    if isinstance(content, str):
        return content
    texts = [p.get("text") for p in content or [] if isinstance(p, dict) and isinstance(p.get("text"), str)]
    return "\n".join(texts) if texts else None


def _sse_frames(events):
    return "".join(f"event: {n}\ndata: {json.dumps(d, ensure_ascii=False)}\n\n" for n, d in events).encode("utf-8")


def _split_text(text: str, marker: str | None):
    """Split reply text into deltas: the marker opener alone, then the marker
    body split in half, exercising the engine's held-partial-marker path."""
    if not marker or marker not in text:
        return [text[: len(text) // 2], text[len(text) // 2:]]
    i = text.index(marker)
    j = i + 1 + (len(marker) - 1) // 2
    return [p for p in (text[:i], text[i:i + 1], text[i + 1:j], text[j:]) if p]


class Provider(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, protocol, scenario, workdir, model):
        super().__init__(("127.0.0.1", 0), Handler)
        self.protocol = protocol
        self.scenario = scenario
        self.workdir = workdir
        self.model = model
        self.turn = 1
        self.obs = Obs()
        self.sent_args: dict[str, object] = {}   # call id -> issued arguments (in memory only)
        self.sub_call_ids: set[str] = set()


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    server: Provider

    def log_message(self, *_):
        pass

    def _send(self, status, ctype, payload, fragment=False):
        self.send_response(status)
        if self.server.generic_reply_content_type and ctype == "text/event-stream":
            ctype = "application/octet-stream"
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(payload)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        if not fragment:
            self.wfile.write(payload)
            return
        # Small, uneven TCP writes split frames, lines and UTF-8 sequences.
        at, step = 0, 29
        while at < len(payload):
            self.wfile.write(payload[at:at + step])
            self.wfile.flush()
            at += step
            step = 17 if step == 29 else 29
            time.sleep(0.002)

    def _common(self):
        obs = self.server.obs
        with obs.lock:
            obs.requests += 1
            key = f"{self.command} {self.path.split('?')[0]}"
            obs.routes[key] = obs.routes.get(key, 0) + 1
            if any(k.lower().startswith("x-dmz-") for k in self.headers.keys()):
                obs.x_dmz_header_requests += 1
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
            if (self.headers.get("Accept-Encoding") or "").strip().lower() == "identity":
                obs.identity_accept_encoding += 1
            sid = self.headers.get("X-Claude-Code-Session-Id")
            if sid:
                obs.session_header_requests += 1
                obs.session_ids.add(sid)
            return sid

    def do_GET(self):
        self._common()
        if self.path.split("?")[0] == "/v1/models":
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
        sid = self._common()
        obs = self.server.obs
        path = self.path.split("?")[0]
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length > 0 else b""
        try:
            body = json.loads(raw.decode("utf-8")) if raw else None
        except (UnicodeDecodeError, json.JSONDecodeError):
            body = None
        proto = self.server.protocol
        if proto == "messages" and path == "/v1/messages/count_tokens":
            self._privacy(raw, body, None)
            self._send(200, "application/json", b'{"input_tokens":12}')
        elif proto == "responses" and path == "/v1/responses/compact":
            with obs.lock:
                obs.compact_route_requests += 1
            self._privacy(raw, body, _responses_parts(body) if isinstance(body, dict) else None)
            self._send(404, "application/json", b'{"error":{"type":"not_found"}}')
        elif (proto == "responses" and path == "/v1/responses") or (proto == "messages" and path == "/v1/messages"):
            with obs.lock:
                obs.model_requests += 1
            if not isinstance(body, dict):
                with obs.lock:
                    obs.json_parse_failures += 1
                self._send(400, "application/json", b'{"error":{"type":"invalid_request_error"}}')
            else:
                parts = (_responses_parts if proto == "responses" else _messages_parts)(body)
                self._privacy(raw, body, parts)
                self._model(body, raw, parts, sid)
        else:
            with obs.lock:
                obs.unknown_routes += 1
            self._send(404, "application/json", b'{"error":{"type":"not_found"}}')
        del raw, body

    # ---- privacy counters for every request
    def _privacy(self, raw, body, parts):
        obs = self.server.obs
        with obs.lock:
            if PROMPT_EMAIL.encode().lower() in raw.lower():
                obs.prompt_email_raw_requests += 1
            if PROBE_TOKEN.encode() in raw:
                obs.probe_token_raw_requests += 1
            if body is not None and any(PROMPT_EMAIL.lower() in s.lower() for s in _strings(body)):
                obs.prompt_email_decoded_requests += 1
            if parts:
                users, assistants, outputs, _ = parts
                if any(PROMPT_EMAIL in t for t in users):
                    obs.prompt_email_in_user_text += 1
                if any(PROMPT_EMAIL in t for t in assistants):
                    obs.prompt_email_in_assistant_text += 1
                if any(PROMPT_EMAIL in t for t in outputs):
                    obs.prompt_email_in_tool_result += 1
                if any(PROBE_TOKEN in t for t in users):
                    obs.probe_token_in_user_text += 1
                if any(PROBE_TOKEN in t for t in outputs):
                    obs.probe_token_in_tool_result += 1

    # ---- request classification and replies
    def _model(self, body, raw, parts, sid):
        obs = self.server.obs
        users, assistants, outputs, calls = parts
        text = raw.decode("utf-8", "replace")
        joined_users = "\n".join(users)
        live = ISSUED_RE.findall(joined_users)
        live_marker = live[0] if live else None
        with obs.lock:
            if live:
                obs.user_marker_requests += 1
                for m in live:
                    if m not in obs.issued:
                        obs.issued.append(m)
        scenario = self.server.scenario
        last_user = _last_user_text(body, proto=self.server.protocol)
        if any(SUMMARY_TAG in t for t in users):
            with obs.lock:
                obs.summary_user_text += 1
                if any(live_marker and live_marker in t for t in users if SUMMARY_TAG in t):
                    obs.summary_user_text_live_marker += 1
        if scenario == "compaction-probe" and last_user is not None and SUMMARY_TAG in last_user:
            stage = "post_compact"
        elif scenario == "compaction-probe" and NONCE1 in text and last_user is not None and NONCE1 not in last_user:
            stage = "compact"
        elif NONCE2 in text:
            stage = "main2"
        elif SUB_NONCE in joined_users and NONCE1 not in joined_users:
            stage = "sub"
        elif NONCE1 in text:
            stage = "main1"
        else:
            stage = "aux"
        if stage == "aux":
            return self._reply_text(body, AUX_TEXT, None, fragment=False)
        if stage == "compact":
            every = list(dict.fromkeys(LIVE_ANY_RE.findall(joined_users)))
            with obs.lock:
                obs.compact_requests += 1
            return self._reply_text(body, f"{SUMMARY_TAG} user asked about " + " and ".join(every or ["no-marker"]) + ".", live_marker, fragment=True)
        if stage == "post_compact":
            with obs.lock:
                obs.post_compact_requests += 1
            return self._reply_text(body, f"{FINAL1_TAG} Reply drafted for {live_marker or 'no-marker'}.", live_marker, fragment=True)
        if stage == "sub":
            every = list(dict.fromkeys(LIVE_ANY_RE.findall(joined_users)))
            with obs.lock:
                obs.sub_requests += 1
                obs.sub_same_session_header = (sid is not None and sid == obs.parent_session_id)
            extra = "".join(f" ref {m}" for m in every if m != live_marker)
            return self._reply_text(body, f"{SUB_TAG} contact {live_marker or 'no-marker'} done.{extra}", live_marker, fragment=True)
        if stage == "main1" and obs.parent_session_id is None:
            with obs.lock:
                obs.parent_session_id = sid
        if any(SUB_TAG in t for t in users):
            with obs.lock:
                obs.sub_user_text_tagged += 1
                if any(live_marker and live_marker in t for t in users if SUB_TAG in t):
                    obs.sub_user_text_marker += 1
        # History replay evidence: assistant texts that carry one of our reply tags.
        tags = (COMMENTARY_TAG, FINAL1_TAG, SUB_TAG)
        tagged = [t for t in assistants if any(tag in t for tag in tags)]
        if tagged:
            with obs.lock:
                obs.history_checked_requests += 1
                for t in tagged:
                    obs.history_tagged_texts += 1
                    found = ISSUED_RE.findall(t)
                    if PROMPT_EMAIL.lower() in t.lower():
                        obs.history_tagged_with_original += 1
                    if live_marker and live_marker in found:
                        obs.history_tagged_with_live_marker += 1
                    elif found:
                        obs.history_tagged_with_other_marker += 1
                    else:
                        obs.history_tagged_without_marker += 1
        if stage == "main2":
            with obs.lock:
                obs.main2 += 1
                if live_marker:
                    obs.markers_t2.append(live_marker)
            return self._reply_text(body, f"{FINAL2_TAG} confirmed again for {live_marker or 'no-marker'}.", live_marker, fragment=True)
        # main1
        with obs.lock:
            if live_marker:
                obs.markers_t1.append(live_marker)
            if self.server.scenario in ("subagent-probe", "compaction-probe"):
                others = {m for m in LIVE_ANY_RE.findall(joined_users) if not m.startswith("⟦EMAIL-")}
                obs.probe_marker_issued = max(obs.probe_marker_issued, len(others))
        if outputs:
            self._check_tool_echo(calls, outputs, live_marker)
            with obs.lock:
                obs.main1_followups += 1
            return self._reply_text(body, f"{FINAL1_TAG} Reply drafted for {live_marker or 'no-marker'}.", live_marker, fragment=True)
        with obs.lock:
            obs.main1 += 1
        if scenario in ("commentary", "compaction-probe"):
            return self._reply_commentary_tool(body, live_marker)
        if scenario == "subagent-probe":
            return self._reply_subagent_call(body, live_marker)
        return self._reply_text(body, f"{FINAL1_TAG} Reply drafted for {live_marker or 'no-marker'}.", live_marker, fragment=True)

    def _check_tool_echo(self, calls, outputs, live_marker):
        obs = self.server.obs
        sent = self.server.sent_args
        with obs.lock:
            obs.tool_result_requests += 1
            joined = "\n".join(outputs)
            if FIXTURE_EMAIL in joined:
                obs.tool_result_fixture_intact += 1
            elif base.SANDBOX_ERROR_RE.search(joined):
                obs.tool_result_sandbox_error += 1
            if any(SUB_TAG in o for o in outputs):
                obs.sub_tool_result_tagged += 1
                if any(ISSUED_RE.search(o) for o in outputs):
                    obs.sub_tool_result_marker += 1
            for cid, args in calls:
                if cid not in sent:
                    continue
                obs.tool_args_echoed += 1
                issued = sent[cid]
                got = args
                if isinstance(got, str):
                    try:
                        got = json.loads(got)
                    except json.JSONDecodeError:
                        got = None
                if got == issued:
                    obs.tool_args_identical += 1
                if live_marker and live_marker in json.dumps(got, ensure_ascii=False) and PROMPT_EMAIL not in json.dumps(got, ensure_ascii=False):
                    obs.tool_args_marker_kept += 1

    # ---- reply builders
    def _reply_text(self, body, text, marker, fragment):
        obs = self.server.obs
        with obs.lock:
            if marker and marker in text:
                obs.replies_with_marker += 1
            if fragment:
                obs.fragmented_replies += 1
        deltas = _split_text(text, marker) if fragment else [text]
        if self.server.protocol == "responses":
            self._responses_reply(body, [("text", text, deltas)])
        else:
            self._messages_reply(body, [("text", text, deltas)], "end_turn")

    def _reply_commentary_tool(self, body, marker):
        obs = self.server.obs
        text = f"{COMMENTARY_TAG} I will read the fixture before replying to {marker or 'no-marker'}."
        with obs.lock:
            if marker:
                obs.replies_with_marker += 1
            obs.fragmented_replies += 1
        deltas = _split_text(text, marker)
        if self.server.protocol == "responses":
            call = base._codex_tool_call(body.get("tools"), self.server.workdir)
            if not call:
                with obs.lock:
                    obs.tool_call_unavailable += 1
                return self._responses_reply(body, [("text", text, deltas)])
            name, args = call
            # A shell comment carries the marker: tool arguments must stay untouched.
            if "cmd" in args:
                args["cmd"] = f"{args['cmd']} # {marker or 'no-marker'}"
            elif isinstance(args.get("command"), str):
                args["command"] = f"{args['command']} # {marker or 'no-marker'}"
            cid = "call_" + uuid.uuid4().hex[:12]
            self.server.sent_args[cid] = args
            with obs.lock:
                obs.tool_calls_sent += 1
            return self._responses_reply(body, [("text", text, deltas), ("call", name, cid, args)])
        tools = body.get("tools") or []
        if not any(isinstance(t, dict) and t.get("name") == "Read" for t in tools):
            with obs.lock:
                obs.tool_call_unavailable += 1
            return self._messages_reply(body, [("text", text, deltas)], "end_turn")
        # The marker is carried in the file name: arguments must stay untouched
        # (a restored path would name a non-existent file and leak upstream).
        fname = f"dmz-restore-arg {marker or 'no-marker'}.txt"
        try:
            with open(os.path.join(self.server.workdir, fname), "w", encoding="utf-8") as fh:
                fh.write(FIXTURE_TEXT)
        except OSError:
            fname = FIXTURE_NAME
        args = {"file_path": os.path.join(self.server.workdir, fname)}
        cid = "toolu_" + uuid.uuid4().hex[:16]
        self.server.sent_args[cid] = args
        with obs.lock:
            obs.tool_calls_sent += 1
        return self._messages_reply(body, [("text", text, deltas), ("tool", "Read", cid, args)], "tool_use")

    def _reply_subagent_call(self, body, marker):
        obs = self.server.obs
        tools = body.get("tools") or []
        agent = next((t for t in tools if isinstance(t, dict) and t.get("name") in ("Agent", "Task")), None)
        if self.server.protocol != "messages" or agent is None:
            with obs.lock:
                obs.tool_call_unavailable += 1
            return self._reply_text(body, f"{FINAL1_TAG} no helper for {marker or 'no-marker'}.", marker, fragment=True)
        props = ((agent.get("input_schema") or {}).get("properties") or {})
        every = list(dict.fromkeys(LIVE_ANY_RE.findall("\n".join(_messages_parts(body)[0]))))
        refs = " ".join(m for m in every if m != marker)
        args = {"description": "Synthetic helper",
                "prompt": f"[{SUB_NONCE}] Reply with one line naming the contact {marker or 'no-marker'}. Context: {refs}"}
        if "subagent_type" in props:
            args["subagent_type"] = "general-purpose"
        cid = "toolu_" + uuid.uuid4().hex[:16]
        self.server.sent_args[cid] = args
        with obs.lock:
            obs.tool_calls_sent += 1
        return self._messages_reply(body, [("tool", agent["name"], cid, args)], "tool_use")

    def _responses_reply(self, body, items):
        rid = "resp_" + uuid.uuid4().hex[:16]
        model = body.get("model") or self.server.model
        created = int(time.time())
        events, output = [], []
        for idx, it in enumerate(items):
            if it[0] == "text":
                _, text, deltas = it
                mid = "msg_" + uuid.uuid4().hex[:12]
                item = {"type": "message", "id": mid, "status": "completed", "role": "assistant",
                        "content": [{"type": "output_text", "text": text, "annotations": []}]}
                events.append(("response.output_item.added", {"type": "response.output_item.added", "output_index": idx,
                                                              "item": {**item, "status": "in_progress", "content": []}}))
                events.append(("response.content_part.added", {"type": "response.content_part.added", "item_id": mid, "output_index": idx,
                                                               "content_index": 0, "part": {"type": "output_text", "text": "", "annotations": []}}))
                for d in deltas:
                    events.append(("response.output_text.delta", {"type": "response.output_text.delta", "item_id": mid, "output_index": idx,
                                                                  "content_index": 0, "delta": d}))
                events.append(("response.output_text.done", {"type": "response.output_text.done", "item_id": mid, "output_index": idx,
                                                             "content_index": 0, "text": text}))
                events.append(("response.content_part.done", {"type": "response.content_part.done", "item_id": mid, "output_index": idx,
                                                              "content_index": 0, "part": {"type": "output_text", "text": text, "annotations": []}}))
            else:
                _, name, cid, args = it
                item = {"type": "function_call", "id": "fc_" + uuid.uuid4().hex[:12], "call_id": cid, "name": name,
                        "arguments": json.dumps(args), "status": "completed"}
                events.append(("response.output_item.added", {"type": "response.output_item.added", "output_index": idx, "item": item}))
            events.append(("response.output_item.done", {"type": "response.output_item.done", "output_index": idx, "item": item}))
            output.append(item)
        head = {"id": rid, "object": "response", "created_at": created, "model": model, "status": "in_progress", "output": []}
        big = 180000 if self.server.scenario == "compaction-probe" else 12
        usage = {"input_tokens": big, "input_tokens_details": {"cached_tokens": 0}, "output_tokens": 6,
                 "output_tokens_details": {"reasoning_tokens": 0}, "total_tokens": big + 6}
        events = [("response.created", {"type": "response.created", "response": head}),
                  ("response.in_progress", {"type": "response.in_progress", "response": head})] + events
        events.append(("response.completed", {"type": "response.completed",
                                              "response": {**head, "status": "completed", "output": output, "usage": usage}}))
        self._send(200, "text/event-stream", _sse_frames(events), fragment=True)

    def _messages_reply(self, body, blocks, stop):
        mid = "msg_" + uuid.uuid4().hex[:16]
        model = body.get("model") or self.server.model
        big = 190000 if self.server.scenario == "compaction-probe" else 12
        usage = {"input_tokens": big, "output_tokens": 6, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}
        if body.get("stream") is not True:
            content = []
            for b in blocks:
                content.append({"type": "text", "text": b[1]} if b[0] == "text" else {"type": "tool_use", "id": b[2], "name": b[1], "input": b[3]})
            payload = {"id": mid, "type": "message", "role": "assistant", "model": model, "content": content,
                       "stop_reason": stop, "stop_sequence": None, "usage": usage}
            self._send(200, "application/json", json.dumps(payload, ensure_ascii=False).encode("utf-8"))
            return
        events = [("message_start", {"type": "message_start", "message": {"id": mid, "type": "message", "role": "assistant", "model": model,
                                                                          "content": [], "stop_reason": None, "stop_sequence": None,
                                                                          "usage": {**usage, "output_tokens": 1}}})]
        for idx, b in enumerate(blocks):
            if b[0] == "text":
                events.append(("content_block_start", {"type": "content_block_start", "index": idx, "content_block": {"type": "text", "text": ""}}))
                for d in b[2]:
                    events.append(("content_block_delta", {"type": "content_block_delta", "index": idx, "delta": {"type": "text_delta", "text": d}}))
            else:
                _, name, cid, args = b
                events.append(("content_block_start", {"type": "content_block_start", "index": idx,
                                                       "content_block": {"type": "tool_use", "id": cid, "name": name, "input": {}}}))
                events.append(("content_block_delta", {"type": "content_block_delta", "index": idx,
                                                       "delta": {"type": "input_json_delta", "partial_json": json.dumps(args)}}))
            events.append(("content_block_stop", {"type": "content_block_stop", "index": idx}))
        events += [("message_delta", {"type": "message_delta", "delta": {"stop_reason": stop, "stop_sequence": None}, "usage": {"output_tokens": 6}}),
                   ("message_stop", {"type": "message_stop"})]
        self._send(200, "text/event-stream", _sse_frames(events), fragment=True)


# --- Client output parsing ----------------------------------------------------
def assistant_texts(client, stdout):
    """Assistant-authored text only. User prompt echoes are never included."""
    texts, events = [], 0
    for line in stdout.splitlines():
        line = line.strip()
        if not line.startswith("{"):
            continue
        try:
            ev = json.loads(line)
        except json.JSONDecodeError:
            continue
        if not isinstance(ev, dict):
            continue
        events += 1
        if client == "codex":
            item = ev.get("item") if ev.get("type") == "item.completed" else None
            if isinstance(item, dict) and item.get("type") == "agent_message" and isinstance(item.get("text"), str):
                texts.append(item["text"])
        else:
            msg = ev.get("message") if ev.get("type") == "assistant" else None
            if isinstance(msg, dict) and msg.get("role", "assistant") == "assistant":
                for block in msg.get("content") or []:
                    if isinstance(block, dict) and block.get("type") == "text" and isinstance(block.get("text"), str):
                        texts.append(block["text"])
    return texts, events


def tag_restored(texts, tag, issued):
    with_tag = [t for t in texts if tag in t]
    restored = [t for t in with_tag if PROMPT_EMAIL in t and not any(m in t for m in issued) and not ANY_MARKER_RE.search(t)]
    return {"seen": bool(with_tag), "restored": bool(restored)}


# --- Scenario runner -------------------------------------------------------
def child_env(tmp, sink_port, client):
    env = base.child_env(tmp, sink_port, client)
    env["HOME"] = os.path.join(tmp, "home")
    return env


def launcher(dmz, client_bin, port, client):
    return [dmz, "run", "--client-bin", client_bin, "--allow-loopback-upstream",
            "--upstream", f"http://127.0.0.1:{port}/v1", client]


def codex_common():
    return ["--ignore-user-config", "--ignore-rules", "--skip-git-repo-check", "--json", "-m", CODEX_MODEL,
            "-c", 'approval_policy="never"', "-c", "mcp_servers={}", "-c", "check_for_update_on_startup=false",
            "-c", "analytics.enabled=false", "-c", "feedback.enabled=false", "-c", 'web_search="disabled"',
            *base.CODEX_FEATURE_ARGS]


def argv_for(dmz, client, client_bin, port, scenario, workdir, turn, session_id):
    pre = launcher(dmz, client_bin, port, client)
    prompt = prompt_for(turn, scenario)
    if client == "codex":
        if turn == 2:
            return pre + ["exec", "resume", "--last", *codex_common(), "-c", 'sandbox_mode="read-only"', prompt]
        persist = [] if scenario == "resume" else ["--ephemeral"]
        if scenario == "compaction-probe":
            persist += ["-c", "model_auto_compact_token_limit=20000"]
        return pre + ["exec", *persist, "--sandbox", "read-only", "--color", "never", "-C", workdir, *codex_common(), prompt]
    argv = pre + ["-p", "--setting-sources", "", "--strict-mcp-config", "--model", CLAUDE_MODEL,
                  "--output-format", "stream-json", "--verbose", "--max-turns", "6"]
    if scenario != "subagent-probe":
        argv.insert(len(pre), "--bare")
    if scenario in ("commentary", "compaction-probe"):
        argv += ["--tools", "Read", "--allowedTools", "Read", "--add-dir", workdir, "--no-session-persistence"]
    elif scenario == "subagent-probe":
        argv += ["--tools", "Read,Agent", "--allowedTools", "Read,Agent", "--no-session-persistence"]
    else:
        argv += ["--tools", ""]
        argv += ["--session-id", session_id] if turn == 1 else ["--resume", session_id]
    return argv + ["--", prompt]


def launch_summary(err):
    m = base.LAUNCH_OBS_RE.search(err)
    warnings: dict[str, int] = {}
    for code in base.LAUNCH_WARN_RE.findall(err):
        warnings[code] = warnings.get(code, 0) + 1
    for code, count in base.LAUNCH_WARN_COUNT_RE.findall(err):
        warnings[code] = int(count)
    return {"model_route_requests": int(m.group(1)) if m else None, "text_parts": int(m.group(2)) if m else None,
            "changed_parts": int(m.group(3)) if m else None, "warnings": dict(sorted(warnings.items())),
            "summary_present": m is not None}


def run_scenario(args, client, client_bin, scenario, deadline):
    result = {"client": client, "scenario": scenario, "status": "fail", "failures": [], "incomplete": []}
    fails, incomplete = result["failures"], result["incomplete"]
    tmp = tempfile.mkdtemp(prefix="dmz-restore-smoke-")
    sink = provider = None
    try:
        for sub in ("tmp", "home", "codex-home", "claude-config", "work"):
            os.makedirs(os.path.join(tmp, sub), mode=0o700)
        workdir = os.path.join(tmp, "work")
        with open(os.path.join(workdir, FIXTURE_NAME), "w", encoding="utf-8") as fh:
            fh.write(FIXTURE_TEXT)
        sink = base.EgressSink()
        threading.Thread(target=sink.serve_forever, daemon=True).start()
        provider = Provider("responses" if client == "codex" else "messages", scenario, workdir,
                            CODEX_MODEL if client == "codex" else CLAUDE_MODEL)
        provider.generic_reply_content_type = args.generic_reply_content_type
        threading.Thread(target=provider.serve_forever, daemon=True).start()
        env = child_env(tmp, sink.server_address[1], client)
        session_id = str(uuid.uuid4())
        turns = [1, 2] if scenario == "resume" else [1]
        launches, texts_by_turn, combined = [], {}, ""
        for turn in turns:
            timeout = deadline.remaining(args.scenario_timeout)
            if timeout < 5:
                fails.append(f"total_timeout_before_turn{turn}")
                return result
            provider.turn = turn
            argv = argv_for(args.dmz, client, client_bin, provider.server_address[1], scenario, workdir, turn, session_id)
            rc, out, err, timed_out = base.run_captured(argv, env, workdir, timeout)
            texts, events = assistant_texts(client, out)
            texts_by_turn[turn] = texts
            combined += out + "\n" + err
            summary = launch_summary(err)
            launches.append({"turn": turn, "exit_code": rc, "timed_out": timed_out, "client_json_events": events,
                             "assistant_text_items": len(texts), "launcher": summary})
            if timed_out:
                fails.append(f"turn{turn}_client_timeout")
            if rc != 0:
                fails.append(f"turn{turn}_client_nonzero_exit")
            if not summary["summary_present"]:
                fails.append(f"turn{turn}_launcher_summary_missing")
            inferred = {"reply_content_type_inferred_sse", "reply_content_type_inferred_json"}
            if any(code.startswith("reply_") and code not in inferred for code in summary["warnings"]):
                fails.append(f"turn{turn}_reply_warning")
            if args.generic_reply_content_type and not any(code in inferred for code in summary["warnings"]):
                fails.append(f"turn{turn}_content_type_inference_unverified")
            del out, err
        obs = provider.obs
        issued = list(obs.issued)
        all_texts = [t for ts in texts_by_turn.values() for t in ts]
        client_out = {
            "commentary": tag_restored(all_texts, COMMENTARY_TAG, issued),
            "final_turn1": tag_restored(texts_by_turn.get(1, []), FINAL1_TAG, issued),
            "final_turn2": tag_restored(texts_by_turn.get(2, []), FINAL2_TAG, issued),
            "subagent": tag_restored(all_texts, SUB_TAG, issued),
            "original_email_in_assistant_output": any(PROMPT_EMAIL in t for t in all_texts),
            "marker_returned_unrestored": any(m in t for t in all_texts for m in issued),
            "fake_key_in_output": FAKE_OPENAI_KEY in combined or FAKE_ANTHROPIC_KEY in combined,
            "hex64_token_like_in_output": bool(base.HEX64_RE.search(combined)),
        }
        del combined, texts_by_turn, all_texts
        t1, t2 = obs.markers_t1, obs.markers_t2
        upstream = {
            "requests": obs.requests, "routes": dict(sorted(obs.routes.items())), "model_requests": obs.model_requests,
            "unknown_routes": obs.unknown_routes, "json_parse_failures": obs.json_parse_failures,
            "x_dmz_header_requests": obs.x_dmz_header_requests, "fake_credential_requests": obs.fake_credential_requests,
            "other_credential_requests": obs.other_credential_requests,
            "identity_accept_encoding_requests": obs.identity_accept_encoding,
            "session_header_requests": obs.session_header_requests, "distinct_session_ids": len(obs.session_ids),
            "prompt_email_raw_requests": obs.prompt_email_raw_requests,
            "prompt_email_decoded_requests": obs.prompt_email_decoded_requests,
            "prompt_email_in_user_text": obs.prompt_email_in_user_text,
            "prompt_email_in_assistant_text": obs.prompt_email_in_assistant_text,
            "prompt_email_in_tool_result": obs.prompt_email_in_tool_result,
            "user_marker_requests": obs.user_marker_requests, "distinct_issued_markers": len(issued),
            "turn1_first_requests": obs.main1, "turn1_followup_requests": obs.main1_followups, "turn2_requests": obs.main2,
            "history_checked_requests": obs.history_checked_requests, "history_tagged_assistant_texts": obs.history_tagged_texts,
            "history_tagged_with_live_marker": obs.history_tagged_with_live_marker,
            "history_tagged_with_other_marker": obs.history_tagged_with_other_marker,
            "history_tagged_without_marker": obs.history_tagged_without_marker,
            "history_tagged_with_original": obs.history_tagged_with_original,
            "replies_with_marker": obs.replies_with_marker, "fragmented_replies": obs.fragmented_replies,
            "tool_calls_sent": obs.tool_calls_sent, "tool_call_unavailable": obs.tool_call_unavailable,
            "tool_result_requests": obs.tool_result_requests, "tool_result_fixture_unchanged": obs.tool_result_fixture_intact,
            "tool_result_sandbox_error": obs.tool_result_sandbox_error, "tool_args_echoed": obs.tool_args_echoed,
            "tool_args_identical": obs.tool_args_identical, "tool_args_marker_kept": obs.tool_args_marker_kept,
            "subagent_requests": obs.sub_requests, "subagent_same_session_header": obs.sub_same_session_header,
            "subagent_tool_result_tagged": obs.sub_tool_result_tagged, "subagent_tool_result_has_marker": obs.sub_tool_result_marker,
            "probe_token_marker_issued": obs.probe_marker_issued, "probe_token_raw_requests": obs.probe_token_raw_requests,
            "probe_token_in_user_text": obs.probe_token_in_user_text, "probe_token_in_tool_result": obs.probe_token_in_tool_result,
            "compact_route_requests": obs.compact_route_requests, "compaction_requests": obs.compact_requests,
            "post_compaction_requests": obs.post_compact_requests, "summary_in_user_text_requests": obs.summary_user_text,
            "summary_user_text_with_live_marker": obs.summary_user_text_live_marker,
            "subagent_user_text_tagged": obs.sub_user_text_tagged, "subagent_user_text_has_live_marker": obs.sub_user_text_marker,
            "turn1_marker_stable": (len(set(t1)) == 1) if t1 else None,
            "turn2_marker_differs_from_turn1": (bool(t1) and bool(t2) and t2[0] != t1[0]) if scenario == "resume" else None,
        }
        result.update({"launches": launches, "upstream": upstream, "client_output": client_out,
                       "egress_attempts_blocked": sink.attempts})
        # --- verdict: privacy first
        if obs.prompt_email_raw_requests or obs.prompt_email_decoded_requests:
            fails.append("prompt_email_reached_upstream")
        if obs.prompt_email_in_tool_result:
            fails.append("prompt_email_in_upstream_tool_result")
        if obs.prompt_email_in_assistant_text or obs.history_tagged_with_original:
            fails.append("restored_original_in_upstream_assistant_history")
        if obs.probe_token_raw_requests:
            # Masked in the prompt; any later appearance is a restored value re-entering.
            fails.append("restored_probe_credential_reached_upstream")
        if obs.unknown_routes - obs.compact_route_requests:
            fails.append("unsupported_route_reached_upstream")
        if obs.json_parse_failures:
            fails.append("upstream_body_not_json")
        if obs.x_dmz_header_requests:
            fails.append("x_dmz_header_leaked_upstream")
        if obs.other_credential_requests or (obs.model_requests and obs.fake_credential_requests == 0):
            fails.append("unexpected_provider_credential")
        if client == "claude" and obs.model_requests and obs.session_header_requests < obs.model_requests:
            fails.append("claude_session_header_missing")
        if client_out["fake_key_in_output"] or client_out["hex64_token_like_in_output"]:
            fails.append("credential_like_value_in_client_output")
        if not obs.user_marker_requests:
            fails.append("prompt_email_not_masked_in_user_text")
        if client_out["marker_returned_unrestored"]:
            fails.append("known_reply_marker_left_unrestored")
        if obs.main1 == 0:
            fails.append("turn1_request_not_observed")
        # --- scenario evidence (never silently skipped)
        if scenario == "commentary":
            if not client_out["commentary"]["restored"]:
                fails.append("commentary_not_restored_in_client_output")
            if obs.tool_calls_sent == 0:
                fails.append("tool_call_unavailable")
            elif obs.tool_result_requests == 0:
                fails.append("tool_followup_not_sent_replay_unverified")
            else:
                if obs.history_tagged_texts == 0:
                    fails.append("commentary_missing_from_upstream_assistant_history")
                elif obs.history_tagged_with_live_marker < obs.history_tagged_texts:
                    fails.append("commentary_history_without_live_marker")
                if obs.tool_args_echoed == 0:
                    fails.append("tool_call_missing_from_history")
                elif obs.tool_args_identical < obs.tool_args_echoed or obs.tool_args_marker_kept < obs.tool_args_echoed:
                    fails.append("tool_arguments_modified")
                if obs.tool_result_fixture_intact == 0 and obs.tool_result_sandbox_error:
                    incomplete.append("client_sandbox_unavailable_tool_content_unverified")
                elif obs.tool_result_fixture_intact == 0:
                    fails.append("tool_output_not_fixture")
            if not client_out["final_turn1"]["restored"]:
                fails.append("final_answer_not_restored_in_client_output")
        elif scenario == "resume":
            if not client_out["final_turn1"]["restored"]:
                fails.append("turn1_answer_not_restored_in_client_output")
            if obs.main2 == 0:
                fails.append("turn2_request_not_observed_replay_unverified")
            else:
                if obs.history_tagged_texts == 0:
                    fails.append("turn1_answer_missing_from_resumed_history")
                elif obs.history_tagged_with_live_marker < obs.history_tagged_texts:
                    fails.append("resumed_history_without_live_marker")
                if not client_out["final_turn2"]["restored"]:
                    fails.append("turn2_answer_not_restored_in_client_output")
                if upstream["turn2_marker_differs_from_turn1"] is not True:
                    incomplete.append("new_scope_marker_change_unverified")
        elif scenario == "subagent-probe":
            if obs.tool_calls_sent == 0:
                fails.append("agent_tool_unavailable")
            elif obs.sub_requests == 0:
                fails.append("subagent_request_not_observed")
            elif obs.sub_tool_result_tagged == 0 and obs.sub_user_text_tagged == 0:
                fails.append("subagent_result_not_returned_to_parent")
            else:
                if obs.sub_tool_result_tagged and obs.sub_tool_result_marker < obs.sub_tool_result_tagged:
                    fails.append("subagent_tool_result_without_live_marker")
                if obs.sub_user_text_tagged and obs.sub_user_text_marker < obs.sub_user_text_tagged:
                    fails.append("subagent_user_text_without_live_marker")
        elif scenario == "compaction-probe":
            if obs.compact_requests == 0 and obs.compact_route_requests == 0:
                incomplete.append("compaction_not_triggered_probe_unverified")
            elif obs.compact_requests == 0:
                incomplete.append("remote_compaction_route_only")
            elif obs.post_compact_requests == 0:
                fails.append("post_compaction_request_not_observed")
        if scenario in ("subagent-probe", "compaction-probe") and obs.probe_marker_issued == 0:
            incomplete.append("probe_credential_not_masked_in_prompt")
        if not fails:
            result["status"] = "incomplete" if incomplete else "pass"
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


def main(argv=None):
    ap = argparse.ArgumentParser(description="Synthetic loopback restoration/replay smoke of installed coding CLIs through dmz run.")
    ap.add_argument("--dmz", required=True, help="dmz launcher binary built from the tree under test")
    ap.add_argument("--codex", default=shutil.which("codex"))
    ap.add_argument("--claude", default=shutil.which("claude"))
    ap.add_argument("--clients", default="codex,claude")
    ap.add_argument("--scenarios", default="commentary,resume",
                    help="comma list of commentary, resume and the opt-in review probes subagent-probe (Claude only) "
                         "and compaction-probe")
    ap.add_argument("--total-timeout", type=float, default=600.0)
    ap.add_argument("--scenario-timeout", type=float, default=120.0)
    ap.add_argument("--generic-reply-content-type", action="store_true",
                    help="Label synthetic SSE replies application/octet-stream to verify format inference")
    args = ap.parse_args(argv)
    deadline = base.Deadline(args.total_timeout)
    report = {
        "harness": PROG,
        "generic_reply_content_type": args.generic_reply_content_type,
        "evidence_kind": "installed-cli-binaries-with-synthetic-loopback-upstream",
        "not_evidence_of": ["provider-compatibility", "account-or-auth-validation", "model-behavior",
                            "native-gui-behavior", "detector-recall", "restart-replay-without-user-history"],
        "versions": {"python": platform.python_version(),
                     "dmz": base.version_of([args.dmz, "version"], r"dmz local gateway [^\n]{1,80}")},
        "results": [], "skipped": [],
    }
    if not report["versions"]["dmz"]:
        diag("dmz_binary_unavailable")
        report["overall"] = "fail"
        print(json.dumps(report, indent=2))
        return 2
    before = base.config_fingerprint()
    bins = {"codex": args.codex, "claude": args.claude}
    patterns = {"codex": r"codex-cli \d+\.\d+\.\d+", "claude": r"\d+\.\d+\.\d+ \(Claude Code\)"}
    for client in [c for c in args.clients.split(",") if c]:
        if client not in bins:
            diag("unknown_client")
            continue
        b = bins[client]
        ver = base.version_of([b, "--version"], patterns[client]) if b else None
        report["versions"][client] = ver
        if not ver:
            diag(f"{client}_binary_unavailable")
            report["skipped"].append({"client": client, "reason": "binary_unavailable"})
            continue
        for scenario in [s for s in args.scenarios.split(",") if s]:
            if scenario not in SCENARIOS:
                diag("unknown_scenario")
                report["skipped"].append({"client": client, "scenario": "unknown", "reason": "unknown_scenario"})
                continue
            if scenario == "subagent-probe" and client != "claude":
                report["skipped"].append({"client": client, "scenario": scenario, "reason": "not_applicable"})
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
    report["normal_config_unchanged"] = before == base.config_fingerprint()
    if not report["normal_config_unchanged"]:
        diag("normal_client_config_changed")
    statuses = {r["status"] for r in report["results"]}
    hard_skips = [s for s in report["skipped"] if s.get("reason") != "not_applicable"]
    if not report["results"] or "fail" in statuses or not report["normal_config_unchanged"]:
        report["overall"], code = "fail", 1
    elif statuses == {"pass"} and not hard_skips:
        report["overall"], code = "pass", 0
    else:
        report["overall"], code = "incomplete", 3
    print(json.dumps(report, indent=2))
    return code


if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        diag("interrupted")
        sys.exit(130)
