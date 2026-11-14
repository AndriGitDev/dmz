#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Synthetic end-to-end check of the DMZ app flow with real coding clients.

Path under test:
  app HTTP controller (internal/companion, via internal/companion/testhost)
  -> POST /api/start with a fake API key and an owned temporary project
  -> private one-use launch ticket -> real ``dmz connect --ticket PATH``
  -> installed Codex / Claude Code client -> app-owned local gateway
  -> loopback fake provider (reused from scripts/agent-client-smoke.py).

The test host replaces the native terminal launcher with a direct child
process. Native windows, terminal launchers, folder pickers, packaging,
signing and real provider accounts are NOT exercised. The API key route is
fake-only. Model requests are answered by loopback fixtures; every
credential, prompt and file is synthetic (example.test addresses).

Output contract: stdout is one JSON document with versions, counts, booleans
and fixed codes only; stderr carries fixed reason codes only. Request bodies
and client output are inspected in memory and never printed or stored.

Python standard library only.
"""

from __future__ import annotations

import argparse
import importlib.util
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import threading
import time
import urllib.error
import urllib.request
import uuid

PROG = "agent-companion-smoke"
HERE = os.path.dirname(os.path.abspath(__file__))


def _load_base():
    spec = importlib.util.spec_from_file_location("dmz_agent_client_smoke", os.path.join(HERE, "agent-client-smoke.py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)  # type: ignore[union-attr]
    return mod


base = _load_base()
HEX64_RE = base.HEX64_RE


def diag(code: str) -> None:
    if not re.fullmatch(r"[a-z0-9_.:\-]{1,120}", code):
        code = "invalid_diagnostic_code"
    sys.stderr.write(f"{PROG}: {code}\n")
    sys.stderr.flush()


# --- Fake provider that can hold the main request (stop/quit scenarios) -----
class _HoldHandler(base._FakeHandler):
    def _hold(self, raw):
        if base.NONCE.encode() in (raw or b""):
            with self.server.obs.lock:
                self.server.held += 1  # type: ignore[attr-defined]
            self.server.release.wait(90)  # type: ignore[attr-defined]

    def _responses(self, body, raw):
        self._hold(raw)
        try:
            super()._responses(body, raw)
        except OSError:
            pass

    def _messages(self, body, raw):
        self._hold(raw)
        try:
            super()._messages(body, raw)
        except OSError:
            pass


def make_provider(client, scenario, workdir):
    proto = "responses" if client == "codex" else "messages"
    model = base.CODEX_MODEL if client == "codex" else base.CLAUDE_MODEL
    provider = base.FakeProvider(proto, "roundtrip" if scenario == "roundtrip" else "basic", workdir, model)
    if scenario in ("stop", "quit", "hangup"):
        provider.RequestHandlerClass = _HoldHandler
    provider.held = 0
    provider.release = threading.Event()
    return provider


# --- Local app HTTP (never through a proxy) ---------------------------------
_OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def http(origin, method, path, token=None, body=None, headers=None, raw_body=None, timeout=10):
    data = raw_body if raw_body is not None else (json.dumps(body).encode() if body is not None else None)
    req = urllib.request.Request(origin + path, data=data, method=method)
    if token is not None:
        req.add_header("Authorization", "Bearer " + token)
    if data is not None:
        req.add_header("Content-Type", "application/json")
    for k, v in (headers or {}).items():
        req.add_header(k, v)
    try:
        with _OPENER.open(req, timeout=timeout) as resp:
            return resp.status, resp.read(), dict(resp.headers)
    except urllib.error.HTTPError as e:
        return e.code, e.read(), dict(e.headers or {})
    except OSError:
        return 0, b"", {}


def jbody(raw):
    try:
        return json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError, AttributeError):
        return None


# --- Process accounting -----------------------------------------------------
def marked_processes(marker):
    """PIDs whose environment carries this run's marker (own-user processes)."""
    needle = f"DMZ_SMOKE_RUN={marker}".encode()
    pids = []
    for name in os.listdir("/proc"):
        if not name.isdigit():
            continue
        try:
            with open(f"/proc/{name}/environ", "rb") as fh:
                if needle in fh.read().split(b"\0"):
                    pids.append(int(name))
        except OSError:
            continue
    return pids


def wait_until(pred, timeout, step=0.2):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        v = pred()
        if v:
            return v
        time.sleep(step)
    return pred()


def file_contains_any(root, needles, skip=()):
    hits = 0
    for dirpath, _, files in os.walk(root):
        for f in files:
            p = os.path.join(dirpath, f)
            if p in skip:
                continue
            try:
                if os.path.islink(p) or os.path.getsize(p) > 32 << 20:
                    continue
                with open(p, "rb") as fh:
                    data = fh.read()
            except OSError:
                continue
            if any(n.encode() in data for n in needles):
                hits += 1
    return hits


# --- Interactive terminal screens (fixed phrases, normalized: lowercase,
# whitespace and control sequences removed). Codes only are reported. -------
def tui_rules(client):
    prompt = base.build_prompt("basic") + "\r"
    if client == "claude":
        return [
            {"code": "claude_theme_onboarding", "match": "choosethetextstyle", "keys": "\r"},
            {"code": "claude_security_notes", "match": "securitynotes", "keys": "\r"},
            {"code": "claude_custom_api_key_approval", "match": "detectedacustomapikey", "keys": "\x1b[A\r"},
            {"code": "claude_folder_trust_default_no_exit", "match": "trustthisfolder", "keys": "\x1b[A\r"},
            {"code": "claude_login_method", "match": "selectloginmethod", "keys": "\x03"},
            {"code": "prompt_ready_send", "match": "shift+tab", "keys": prompt},
        ]
    return [
        {"code": "codex_folder_trust", "match": "allowcodextoworkinthisfolder", "keys": "\r"},
        {"code": "codex_folder_trust_alt", "match": "doyoutrust", "keys": "\r"},
        {"code": "codex_sign_in", "match": "signinwithchatgpt", "keys": "\x03"},
        {"code": "codex_model_notice", "match": "trynewmodel", "keys": "\r"},
        {"code": "prompt_ready_send", "match": "forshortcuts", "keys": prompt},
    ]


# Passive phrases (keys "") are only reported as seen codes, never answered.
PASSIVE = ["authconflict", "anthropic_api_key", "environment", "exit", "goodbye", "typeerror", "referenceerror", "econn", "fetch", "network", "keychain", "permission", "dangerously", "approve", "accept", "decline", "no,exit", "noexit", "interrupt", "signal", "killed", "terminated", "ttyrequired", "nottty", "isatty", "invalid", "failed", "exiting", "please", "/login", "notloggedin", "rawmode", "notsupported", "stdin", "setting", "requires", "unexpected", "cannot", "unable", "denied", "managed", "policy", "baseurl", "header", "iamtrust", "trustthisfolder", "projectyoucreated", "safetycheck", "usethisapikey", "customapikey", "detected", "forshortcuts", "esctointerrupt", "trust", "apikey", "signin", "login", "welcome", "update", "error",
           "sandbox", "approval", "model", "theme", "onboarding", "unabletoconnect", "connectionrefused",
           "notauthorized", "unauthorized", "403", "401", "proxy", "retry", "reconnecting", "streamdisconnected",
           "context", "ctrl+c", "sendmessage", "tryedit", "/init", "workingdirectory", "dmz:", "notice",
           "autoupdate", "telemetry", "doyouwant", "yes", "recommended", "press", "enter", "continue", "terminal",
           "bypass", "warning", "billing", "credit", "cost", "auth", "plan", "mcp", "ide", "keybind", "shift+tab"]


def with_passive(rules):
    return rules + [{"code": "seen:" + p, "match": p, "keys": ""} for p in PASSIVE]


# --- UI flow model ----------------------------------------------------------
# Steps a person takes in the production UI for one session. The harness
# performs the HTTP calls each step causes; it does not drive a real window.
UI_STEPS = (
    ("open_dmz_app", None),
    ("choose_project_folder", "POST /api/folder (fixture: picker unavailable -> typed path)"),
    ("optional_key_for_chosen_tool", None),
    ("open_chosen_tool_button", "POST /api/start"),
    ("work_in_opened_terminal", "dmz connect -> client"),
    ("read_status_or_stop", "GET /api/state | POST /api/stop"),
)
UI_STEPS_FORBIDDEN = ("set_environment_variable", "copy_local_token", "edit_client_config", "run_cli_command")


class Run:
    def __init__(self, args, client, client_bin, scenario):
        self.args, self.client, self.client_bin, self.scenario = args, client, client_bin, scenario
        self.tmp = tempfile.mkdtemp(prefix="dmz-companion-smoke-")
        self.marker = uuid.uuid4().hex
        self.result = {"client": client, "scenario": scenario, "status": "fail", "failures": [], "incomplete": []}
        self.proc = None
        self.sink = self.provider = None

    @property
    def fails(self):
        return self.result["failures"]

    def p(self, *parts):
        return os.path.join(self.tmp, *parts)

    def setup(self):
        for sub in ("tmp", "apptmp", "codex-home", "claude-config", "work", "private"):
            os.makedirs(self.p(sub), mode=0o700)
        with open(self.p("work", base.FIXTURE_NAME), "w", encoding="utf-8") as fh:
            fh.write(base.FIXTURE_TEXT)
        self.sink = base.EgressSink()
        threading.Thread(target=self.sink.serve_forever, daemon=True).start()
        self.provider = make_provider(self.client, self.scenario, self.p("work"))
        threading.Thread(target=self.provider.serve_forever, daemon=True).start()
        env = base.child_env(self.tmp, self.sink.server_address[1], self.client)
        # The app must supply the gateway credential; drop the fixture key the
        # direct-launcher harness used.
        env.pop("OPENAI_API_KEY", None)
        env.pop("ANTHROPIC_API_KEY", None)
        env["DMZ_SMOKE_RUN"] = self.marker
        self.seeded = []
        if self.scenario == "account":
            self.seed_account_files()
        argv = base.client_argv("/x", self.client, self.client_bin, self.provider.server_address[1],
                                "roundtrip" if self.scenario == "roundtrip" else "basic", self.p("work"))
        assert argv[1] == "run" and argv[7] == self.client
        self.client_args = argv[8:]
        if self.scenario == "account" and self.client == "claude":
            # --bare restricts Claude to API-key auth; the sign-in route needs
            # the client's normal credential lookup (private config dir only).
            self.client_args = [a for a in self.client_args if a != "--bare"]
        if self.scenario == "tui":
            # Production path: the app passes no client arguments; the person
            # types in the client's normal terminal UI.
            self.client_args = []
            env["TERM"] = "xterm-256color"
            env["SHELL"] = "/bin/sh"
            env.pop("NO_COLOR", None)
            if self.client == "claude" and self.args.tui_seed_onboarding:
                # Synthetic "already set up" client state in the private
                # config directory only (no account or credential fields).
                seed = {"hasCompletedOnboarding": True, "theme": "dark", "numStartups": 3}
                fd = os.open(self.p("claude-config", ".claude.json"), os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
                with os.fdopen(fd, "w") as fh:
                    json.dump(seed, fh)
        cfg = {
            "dmz": self.args.dmz,
            "control_file": self.p("private", "control.json"),
            "state_file": self.p("private", "state.json"),
            "clients": [{"id": self.client, "path": self.client_bin, "arguments": self.client_args}],
            "upstreams": {self.client: f"http://127.0.0.1:{self.provider.server_address[1]}/v1"},
            "environment": env,
            # Credentials must appear nowhere; the original synthetic e-mail is
            # expected in the ticket (prompt via test seam) and in the client's
            # own stderr echo, but must not come back in the client's answer.
            "probes": [base.FAKE_OPENAI_KEY, base.FAKE_ANTHROPIC_KEY],
            "stdout_probes": [base.PROMPT_EMAIL],
            "final_text": base.FINAL_TEXT,
            "replay": self.scenario == "basic",
            "connect_timeout_seconds": int(self.args.scenario_timeout) + 45,
        }
        if self.scenario == "tui":
            cfg["tui"] = {"script": shutil.which("script") or "/usr/bin/script", "rules": with_passive(tui_rules(self.client))}
        self.config_path = self.p("private", "testhost.json")
        fd = os.open(self.config_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "w") as fh:
            json.dump(cfg, fh)
        host_env = {k: os.environ[k] for k in ("PATH", "HOME", "USER", "LANG") if k in os.environ}
        sinkurl = f"http://127.0.0.1:{self.sink.server_address[1]}"
        host_env.update({"TMPDIR": self.p("apptmp"), "HTTP_PROXY": sinkurl, "HTTPS_PROXY": sinkurl,
                         "NO_PROXY": "127.0.0.1", "no_proxy": "127.0.0.1"})
        self.proc = subprocess.Popen([self.args.testhost, "--config", self.config_path], env=host_env, cwd=self.tmp,
                                     stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                     start_new_session=True)
        ctl = wait_until(lambda: os.path.exists(cfg["control_file"]) and os.path.getsize(cfg["control_file"]) > 0, 15)
        if not ctl:
            self.fails.append("control_file_missing")
            return False
        st = os.stat(cfg["control_file"])
        self.result["control_file_private"] = (st.st_mode & 0o077) == 0
        with open(cfg["control_file"], encoding="utf-8") as fh:
            c = json.load(fh)
        self.origin, self.token = c["origin"], c["token"]
        return True

    # ---- API validation and request-origin checks (fixed codes only)
    def api_checks(self):
        o, tok = self.origin, self.token
        work = self.p("work")
        checks = {}
        checks["state_no_auth_401"] = http(o, "GET", "/api/state")[0] == 401
        checks["state_wrong_token_401"] = http(o, "GET", "/api/state", token="0" * 64)[0] == 401
        checks["state_empty_bearer_401"] = http(o, "GET", "/api/state", token="")[0] == 401
        checks["cross_origin_403"] = http(o, "GET", "/api/state", token=tok, headers={"Origin": "http://evil.example.test"})[0] == 403
        checks["cross_origin_post_403"] = http(o, "POST", "/api/start", token=tok, body={}, headers={"Origin": "null"})[0] == 403
        checks["host_header_localhost_403"] = http(o, "GET", "/api/state", token=tok, headers={"Host": "localhost:" + o.rsplit(":", 1)[1]})[0] == 403
        checks["quit_no_auth_401"] = http(o, "POST", "/api/quit", body={})[0] == 401
        checks["session_control_unknown_404"] = http(o, "GET", "/api/session-control/" + "a" * 24, headers={"X-Dmz-Connect": "0" * 64})[0] == 404
        status, page, hdr = http(o, "GET", "/")
        checks["ui_page_200"] = status == 200
        csp = hdr.get("Content-Security-Policy", "")
        checks["ui_csp_self_only"] = "default-src 'none'" in csp and "connect-src 'self'" in csp
        checks["ui_no_external_asset_refs"] = not re.search(rb"(src|href)=[\"']https?://", page)
        code, raw, _ = http(o, "GET", "/api/state", token=tok)
        st = jbody(raw) or {}
        checks["state_200"] = code == 200
        clients = {c.get("id"): c for c in st.get("clients", [])}
        checks["client_discovered"] = bool(clients.get(self.client, {}).get("available"))
        checks["api_key_env_not_used"] = clients.get(self.client, {}).get("api_key_available") is False
        checks["state_has_no_client_path"] = self.client_bin.encode() not in raw
        code, raw, _ = http(o, "POST", "/api/folder", token=tok, body={})
        checks["folder_fixed_error"] = (jbody(raw) or {}).get("error") == "folder_picker_unavailable"

        def start_err(body):
            code, raw, _ = http(o, "POST", "/api/start", token=tok, body=body)
            j = jbody(raw)
            return code, (j or {}).get("error") if isinstance(j, dict) else None

        good = {"client": self.client, "project": work, "auth": "api", "api_key": self.fake_key()}
        checks["start_relative_project"] = start_err({**good, "project": "work"}) == (200, "choose_project")
        checks["start_missing_project"] = start_err({**good, "project": self.p("missing")}) == (200, "choose_project")
        checks["start_file_project"] = start_err({**good, "project": self.p("work", base.FIXTURE_NAME)}) == (200, "choose_project")
        checks["start_no_project"] = start_err({k: v for k, v in good.items() if k != "project"}) == (200, "choose_project")
        checks["start_unknown_client"] = start_err({**good, "client": "bash"}) == (200, "client_missing")
        checks["start_path_as_client"] = start_err({**good, "client": "/bin/sh"}) == (200, "client_missing")
        other = "claude" if self.client == "codex" else "codex"
        checks["start_unconfigured_client"] = start_err({**good, "client": other}) == (200, "client_missing")
        checks["start_bad_auth"] = start_err({**good, "auth": "token"}) == (200, "connection_invalid")
        checks["start_api_without_key"] = start_err({**good, "api_key": ""}) == (200, "api_key_required")
        checks["start_key_newline"] = start_err({**good, "api_key": "a\nb"}) == (200, "api_key_invalid")
        for field, value in (("upstream", "http://127.0.0.1:1/v1"), ("client_bin", "/bin/sh"), ("client_path", "/bin/sh"),
                             ("arguments", ["--version"]), ("executable", "/bin/sh"), ("environment", {"A": "B"}),
                             ("base_url", "https://example.test/v1")):
            checks[f"start_rejects_field_{field}"] = start_err({**good, field: value})[0] == 400
        checks["start_trailing_json_400"] = http(o, "POST", "/api/start", token=tok, raw_body=b'{"client":"codex"}{}')[0] == 400
        checks["start_get_404"] = http(o, "GET", "/api/start", token=tok)[0] == 404
        checks["no_session_created_by_invalid_requests"] = not (jbody(http(o, "GET", "/api/state", token=tok)[1]) or {}).get("sessions")
        self.result["api_checks"] = checks
        for k, v in checks.items():
            if not v:
                self.fails.append("api_check." + k)

    def seed_account_files(self):
        """Synthetic existing sign-in, private client directories only. The
        fake access token reuses the fixture key string so the fake provider
        can tell it apart from any other credential. Nothing is refreshed:
        expiry is in the future and the proxy sink blocks external calls."""
        import base64
        now = int(time.time())
        if self.client == "codex":
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
            path = self.p("codex-home", "auth.json")
        else:
            data = {"claudeAiOauth": {"accessToken": base.FAKE_ANTHROPIC_KEY, "refreshToken": "synthetic-refresh-not-used",
                                      "expiresAt": (now + 86400) * 1000, "scopes": ["user:inference", "user:profile"],
                                      "subscriptionType": "pro"}}
            path = self.p("claude-config", ".credentials.json")
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "w") as fh:
            json.dump(data, fh)
        self.seeded.append(path)

    def fake_key(self):
        return base.FAKE_OPENAI_KEY if self.client == "codex" else base.FAKE_ANTHROPIC_KEY

    def state(self):
        return jbody(http(self.origin, "GET", "/api/state", token=self.token)[1]) or {}

    def session(self, sid):
        for s in self.state().get("sessions", []):
            if s.get("id") == sid:
                return s
        return None

    # ---- UI flow
    def flow(self):
        steps = []
        o, tok = self.origin, self.token
        steps.append("open_dmz_app")
        st = self.state()
        if any(c.get("id") == self.client and c.get("available") for c in st.get("clients", [])):
            self.result["tools_detected_before_launch"] = True
        http(o, "POST", "/api/folder", token=tok, body={})
        steps.append("choose_project_folder")
        steps.append("optional_key_for_chosen_tool")
        body = {"client": self.client, "project": self.p("work"), "auth": "api", "api_key": self.fake_key()}
        if self.scenario == "account":
            body = {"client": self.client, "project": self.p("work"), "auth": "account"}
        code, raw, _ = http(o, "POST", "/api/start", token=tok, body=body)
        view = jbody(raw) or {}
        steps.append("open_chosen_tool_button")
        sid = view.get("id")
        start = {
            "http_status": code,
            "session_created": bool(sid),
            "initial_state": view.get("state"),
            "response_has_provider_key": self.fake_key().encode() in raw,
            "response_has_hex64": bool(HEX64_RE.search(raw.decode("utf-8", "replace"))),
            "response_has_ui_token": self.token.encode() in raw,
        }
        self.result["start"] = start
        if not sid:
            self.fails.append("start_failed:" + str(view.get("error", "unknown"))[:40])
            return
        steps.append("work_in_opened_terminal")
        seen = set()
        t0 = time.monotonic()
        stop_evidence = None
        if self.scenario in ("basic", "roundtrip", "account"):
            def finished():
                s = self.session(sid)
                if s:
                    seen.add(s.get("state"))
                return s if s and s.get("state") == "stopped" else None
            final = wait_until(finished, self.args.scenario_timeout, 0.3)
        elif self.scenario == "tui":
            def running():
                s = self.session(sid)
                if s:
                    seen.add(s.get("state"))
                return self.provider.obs.final_answers_sent > 0 or (s and s.get("state") == "stopped")
            answered = wait_until(running, self.args.scenario_timeout, 0.5)
            time.sleep(3)
            t1 = time.monotonic()
            http(o, "POST", "/api/stop", token=tok, body={"id": sid})
            final = wait_until(lambda: (lambda s: s if s and s.get("state") == "stopped" else None)(self.session(sid)), 10)
            gone = wait_until(lambda: not marked_processes(self.marker), 20, 0.2)
            stop_evidence = {"final_answer_sent_by_provider": bool(answered) and self.provider.obs.final_answers_sent > 0,
                             "client_and_connect_exited_after_stop": bool(gone),
                             "seconds_to_exit_after_stop": round(time.monotonic() - t1, 1)}
            time.sleep(1)
            stop_evidence["issue_after_user_stop"] = (self.session(sid) or {}).get("issue") or None
            self.result["stop"] = stop_evidence
            # A second session in the same app and project: shows which
            # terminal screens repeat for every session.
            for _ in range(max(0, self.args.tui_sessions - 1)):
                before = self.provider.obs.final_answers_sent
                code, raw, _ = http(o, "POST", "/api/start", token=tok,
                                    body={"client": self.client, "project": self.p("work"), "auth": "api", "api_key": self.fake_key()})
                sid2 = (jbody(raw) or {}).get("id")
                if not sid2:
                    self.fails.append("repeat_start_failed")
                    break
                wait_until(lambda: self.provider.obs.final_answers_sent > before, self.args.scenario_timeout, 0.5)
                time.sleep(3)
                http(o, "POST", "/api/stop", token=tok, body={"id": sid2})
                wait_until(lambda: not marked_processes(self.marker), 20, 0.2)
            self.result["tui_sessions_final_answers"] = self.provider.obs.final_answers_sent
        else:
            held = wait_until(lambda: self.provider.held > 0, self.args.scenario_timeout, 0.3)
            s = self.session(sid)
            if s:
                seen.add(s.get("state"))
            stop_evidence = {"main_request_held_at_provider": bool(held),
                             "marked_processes_while_running": len(marked_processes(self.marker))}
            t1 = time.monotonic()
            if self.scenario == "hangup":
                # Closing a terminal window hangs up every process attached to
                # it. Signal the launch's processes, not the app.
                for pid in marked_processes(self.marker):
                    try:
                        os.kill(pid, 1)
                    except OSError:
                        pass
                gone = wait_until(lambda: not marked_processes(self.marker), 15, 0.2)
                stop_evidence["client_and_connect_exited_after_hangup"] = bool(gone)
                time.sleep(4)
                s = self.session(sid) or {}
                stop_evidence["app_state_after_hangup"] = s.get("state")
                stop_evidence["app_noticed_hangup"] = s.get("state") == "stopped"
                final = s
                self.provider.release.set()
            elif self.scenario == "stop":
                code = http(o, "POST", "/api/stop", token=tok, body={"id": sid})[0]
                stop_evidence["stop_http_status"] = code
                final = wait_until(lambda: (lambda s: s if s and s.get("state") == "stopped" else None)(self.session(sid)), 10)
                gone = wait_until(lambda: not marked_processes(self.marker), 20, 0.2)
                stop_evidence["client_and_connect_exited_after_stop"] = bool(gone)
                stop_evidence["seconds_to_exit_after_stop"] = round(time.monotonic() - t1, 1)
                time.sleep(1)
                stop_evidence["issue_after_user_stop"] = (self.session(sid) or {}).get("issue") or None
                final = self.session(sid) or final
                self.provider.release.set()
            else:
                final = self.session(sid)
                code = http(o, "POST", "/api/quit", token=tok, body={})[0]
                stop_evidence["quit_http_status"] = code
                gone = wait_until(lambda: not marked_processes(self.marker), 30, 0.2)
                stop_evidence["client_and_connect_exited_after_quit"] = bool(gone)
                stop_evidence["seconds_to_exit_after_quit"] = round(time.monotonic() - t1, 1)
                self.provider.release.set()
            self.result["stop"] = stop_evidence
        steps.append("read_status_or_stop")
        self.result["ui_flow"] = {
            "user_steps": len(steps),
            "user_steps_under_8": len(steps) < 8,
            "steps": steps,
            "forbidden_steps_needed": [],
            "states_observed": sorted(x for x in seen if x),
            "seconds_to_stopped": round(time.monotonic() - t0, 1),
        }
        if final:
            raw = json.dumps(final)
            self.result["final_session"] = {
                "state": final.get("state"),
                "issue": final.get("issue") or None,
                "requests": final.get("requests"),
                "changed_parts": final.get("changed_parts"),
                "warnings": dict(sorted((final.get("warnings") or {}).items())),
                "has_provider_key": self.fake_key() in raw,
                "has_hex64": bool(HEX64_RE.search(raw)),
            }
        else:
            self.fails.append("session_not_stopped")
        if self.scenario != "quit":
            # Stats remain visible after stop; then quit the app.
            later = self.session(sid)
            self.result["stats_retained_after_stop"] = bool(later and later.get("requests", 0) > 0)
            if self.scenario == "basic":
                # Let the test host finish its ticket reuse/replay probes while
                # the app (and the stopped session record) is still alive.
                time.sleep(6)
            http(o, "POST", "/api/quit", token=tok, body={})

    def finish(self):
        code = None
        try:
            code = self.proc.wait(timeout=60)
        except subprocess.TimeoutExpired:
            base.kill_tree(self.proc)
            self.fails.append("testhost_did_not_exit")
        out, err = self.proc.communicate()
        self.result["testhost_exit_code"] = code
        self.result["testhost_stdout_bytes"] = len(out)
        self.result["testhost_stderr_has_secret"] = any(x.encode() in out + err for x in (base.FAKE_OPENAI_KEY, base.FAKE_ANTHROPIC_KEY, self.token)) or bool(HEX64_RE.search((out + err).decode("utf-8", "replace")))
        self.result["control_file_removed"] = not os.path.exists(self.p("private", "control.json"))
        state_path = self.p("private", "state.json")
        try:
            with open(state_path, "rb") as fh:
                raw = fh.read()
            st = json.loads(raw)
            self.result["state_file_private"] = (os.stat(state_path).st_mode & 0o077) == 0
            self.result["state_file_has_secret"] = any(x.encode() in raw for x in (base.FAKE_OPENAI_KEY, base.FAKE_ANTHROPIC_KEY, base.PROMPT_EMAIL, self.token)) or bool(HEX64_RE.search(raw.decode()))
        except (OSError, json.JSONDecodeError):
            st = {}
            self.fails.append("testhost_state_missing")
        self.result["testhost"] = st
        wait_until(lambda: not marked_processes(self.marker), 5)
        self.result["orphan_processes"] = len(marked_processes(self.marker))
        self.result["ticket_dirs_left"] = len([n for n in os.listdir(self.p("apptmp")) if n.startswith("dmz-session-")])
        self.result["dmz_temp_files_left_in_client_tmp"] = len([n for n in os.listdir(self.p("tmp")) if n.startswith("dmz")])
        cc = self.p("claude-config", ".claude.json")
        try:
            with open(cc, encoding="utf-8") as fh:
                cj = json.load(fh)
            ck = cj.get("customApiKeyResponses") or {}
            self.result["claude_private_config"] = {
                "custom_api_key_approvals_stored": len(ck.get("approved") or []),
                "custom_api_key_rejections_stored": len(ck.get("rejected") or []),
                "project_entries": len(cj.get("projects") or {}),
            }
        except (OSError, ValueError, AttributeError):
            pass
        self.result["provider_key_in_files"] = file_contains_any(self.tmp, [base.FAKE_OPENAI_KEY, base.FAKE_ANTHROPIC_KEY], skip=(self.config_path, *self.seeded))

    def observe(self):
        obs = self.provider.obs
        marker_sets = [s for s in obs.markers_by_request if s]
        stable = len(marker_sets) >= 2 and all(s[0] == marker_sets[0][0] for s in marker_sets)
        self.result["upstream"] = {
            "requests": obs.requests,
            "routes": dict(sorted(obs.routes.items())),
            "unknown_routes": obs.unknown_routes,
            "model_requests": obs.model_requests,
            "stream_requests": obs.stream_requests,
            "main_requests": obs.main_requests,
            "x_dmz_header_requests": obs.dmz_header_requests,
            "fake_provider_credential_requests": obs.fake_credential_requests,
            "other_credential_requests": obs.other_credential_requests,
            "session_header_requests": obs.session_header_requests,
            "original_email_in_eligible_field": obs.original_in_eligible,
            "original_email_anywhere_in_body": obs.original_anywhere,
            "marker_in_eligible_field": obs.marker_in_eligible,
            "marker_stable_across_requests": stable if len(marker_sets) >= 2 else None,
            "tool_calls_sent": obs.tool_calls_sent,
            "tool_result_requests": obs.tool_result_requests,
            "tool_result_fixture_unchanged": obs.tool_result_fixture_intact,
            "tool_result_fixture_masked": obs.tool_result_fixture_masked,
            "tool_result_sandbox_error": obs.tool_result_sandbox_error,
            "final_answers_sent": obs.final_answers_sent,
        }
        self.result["egress_attempts_blocked"] = self.sink.attempts
        return obs, stable

    def verdict(self):
        obs, stable = self.observe()
        r, f = self.result, self.fails
        start = r.get("start", {})
        if start.get("response_has_provider_key") or start.get("response_has_hex64") or start.get("response_has_ui_token"):
            f.append("start_response_leaks_credential")
        fs = r.get("final_session") or {}
        if fs.get("has_provider_key") or fs.get("has_hex64"):
            f.append("status_leaks_credential")
        for k in ("testhost_stderr_has_secret", "state_file_has_secret"):
            if r.get(k):
                f.append(k)
        if not r.get("control_file_private") or not r.get("control_file_removed"):
            f.append("control_file_lifecycle")
        if r.get("orphan_processes"):
            f.append("orphan_processes")
        if r.get("ticket_dirs_left") or (r.get("testhost") or {}).get("ticket_temp_dirs_left"):
            f.append("ticket_dirs_left")
        if r.get("dmz_temp_files_left_in_client_tmp"):
            f.append("dmz_temp_files_left")
        if r.get("provider_key_in_files"):
            f.append("provider_key_written_to_disk")
        th = r.get("testhost") or {}
        if th.get("environment_nonempty_returns"):
            f.append("environment_returned_value")
        if th.get("connects_killed_at_exit"):
            f.append("connect_survived_app_quit")
        launches = th.get("launches") or []
        expected = self.args.tui_sessions if self.scenario == "tui" else 1
        if len(launches) != expected:
            f.append("unexpected_launch_count")
        for L in launches:
            for k in ("ticket_private", "ticket_fields_expected", "ticket_arguments_from_test_seam", "gateway_reachable_before_connect",
                      "gateway_wrong_token_refused", "ticket_consumed", "ticket_dir_removed_after_exit", "connect_group_clean_after_exit"):
                if not L.get(k):
                    f.append("launch." + k)
            for k in ("ticket_contains_probe", "probe_in_output", "local_token_in_output", "hex64_in_output", "spawn_error",
                      "killed_by_testhost", "gateway_reachable_after_exit"):
                if L.get(k):
                    f.append("launch." + k)
            if self.scenario in ("basic", "roundtrip", "account") and not L.get("original_in_client_stdout"):
                f.append("launch.local_answer_not_restored")
            if L.get("ticket_ttl_seconds", 0) > 600:
                f.append("launch.ticket_ttl_over_10m")
            if L.get("reuse_same_path_rejected") is False:
                f.append("launch.ticket_reuse_accepted")
            rp = L.get("replay")
            if rp and rp.get("client_launched"):
                f.append("launch.replayed_ticket_launched_client_after_stop")
        if obs.other_credential_requests or (obs.model_requests and not obs.fake_credential_requests):
            f.append("unexpected_provider_credential")
        if obs.dmz_header_requests:
            f.append("x_dmz_header_leaked_upstream")
        if obs.original_in_eligible:
            f.append("original_email_in_eligible_field")
        if obs.unknown_routes:
            f.append("unsupported_route_reached_upstream")
        if self.scenario in ("basic", "roundtrip", "account"):
            L = launches[0] if launches else {}
            if L.get("exit_code") != 0:
                f.append("connect_nonzero_exit")
            if not L.get("final_answer_delivered"):
                f.append("final_answer_not_delivered")
            if not L.get("exited_before_app_quit"):
                f.append("connect_did_not_finish_before_quit")
            if obs.main_requests == 0:
                f.append("main_request_not_observed")
            elif obs.marker_in_eligible < obs.main_requests:
                f.append("marker_missing_in_eligible_field")
            if not r.get("stats_retained_after_stop"):
                f.append("stats_not_retained_after_stop")
            if fs.get("issue"):
                f.append("session_issue_" + str(fs["issue"])[:40])
            if "running" not in r.get("ui_flow", {}).get("states_observed", []) and fs.get("requests", 0) == 0:
                f.append("running_state_not_reported")
            if self.client == "claude" and obs.model_requests and obs.session_header_requests < obs.model_requests:
                f.append("claude_session_header_missing")
        if self.scenario == "roundtrip":
            if obs.tool_calls_sent == 0:
                f.append("roundtrip_tool_unavailable")
            elif obs.tool_result_requests == 0:
                f.append("roundtrip_tool_result_not_returned")
            else:
                if obs.tool_result_fixture_intact == 0 and obs.tool_result_sandbox_error:
                    r["incomplete"].append("client_sandbox_unavailable_tool_content_unverified")
                elif obs.tool_result_fixture_intact == 0:
                    f.append("roundtrip_tool_output_not_fixture")
                if obs.tool_result_fixture_masked:
                    f.append("roundtrip_tool_output_modified")
                if not stable:
                    f.append("roundtrip_marker_not_stable")
        if self.scenario == "tui":
            L = launches[0] if launches else {}
            tui = L.get("tui") or {}
            extra = [c for c in tui.get("screens_answered", []) if c != "prompt_ready_send"]
            r["tui_extra_terminal_screens"] = extra
            r["tui_extra_terminal_screens_by_session"] = [[c for c in (x.get("tui") or {}).get("screens_answered", []) if c != "prompt_ready_send"] for x in launches]
            r["tui_final_answer_rendered_by_session"] = [bool(x.get("final_answer_delivered")) for x in launches]
            r["ui_flow"]["user_steps_including_terminal_screens"] = r["ui_flow"].get("user_steps", 0) + len(extra)
            r["ui_flow"]["user_steps_under_8"] = r["ui_flow"]["user_steps_including_terminal_screens"] < 8
            if "prompt_ready_send" not in tui.get("screens_answered", []):
                f.append("tui_prompt_never_ready")
            if not all(r["tui_final_answer_rendered_by_session"]):
                f.append("tui_final_answer_not_rendered")
            if (r.get("stop") or {}).get("issue_after_user_stop"):
                f.append("user_stop_reported_as_" + str(r["stop"]["issue_after_user_stop"])[:40])
            if not (r.get("stop") or {}).get("client_and_connect_exited_after_stop"):
                f.append("tui_client_not_exited_after_stop")
            if obs.main_requests == 0:
                f.append("main_request_not_observed")
            elif obs.marker_in_eligible < obs.main_requests:
                f.append("marker_missing_in_eligible_field")
        if self.scenario == "hangup":
            se = r.get("stop") or {}
            if not se.get("app_noticed_hangup"):
                f.append("hangup_session_left_running_gateway_open")
        if self.scenario in ("stop", "quit"):
            se = r.get("stop") or {}
            if not se.get("main_request_held_at_provider"):
                f.append("held_request_not_observed")
            key = "client_and_connect_exited_after_" + self.scenario
            if not se.get(key):
                f.append(key.replace("exited", "not_exited"))
            L = launches[0] if launches else {}
            if self.scenario == "stop" and se.get("issue_after_user_stop"):
                f.append("user_stop_reported_as_" + str(se["issue_after_user_stop"])[:40])
            if self.scenario == "stop" and not L.get("exited_before_app_quit"):
                f.append("stop_did_not_cancel_before_quit")
        if not r.get("ui_flow", {}).get("user_steps_under_8"):
            f.append("ui_steps_not_under_8")
        if not f:
            r["status"] = "incomplete" if r["incomplete"] else "pass"

    def run(self, with_api_checks):
        try:
            if not self.setup():
                return self.result
            if with_api_checks:
                self.api_checks()
            self.flow()
            self.finish()
            self.verdict()
        except OSError:
            self.fails.append("harness_os_error")
        finally:
            if self.proc and self.proc.poll() is None:
                base.kill_tree(self.proc)
                self.fails.append("testhost_killed")
            for pid in marked_processes(self.marker):
                try:
                    os.kill(pid, 9)
                except OSError:
                    pass
            if self.provider:
                self.provider.release.set()
            for srv in (self.provider, self.sink):
                if srv is not None:
                    srv.shutdown()
                    srv.server_close()
            shutil.rmtree(self.tmp, ignore_errors=True)
        return self.result


def main(argv=None):
    ap = argparse.ArgumentParser(description="Synthetic app-flow smoke test: companion controller -> dmz connect -> real clients -> fake provider.")
    ap.add_argument("--dmz", required=True, help="production dmz binary (go build ./cmd/dmz)")
    ap.add_argument("--testhost", required=True, help="companion test host (go build ./internal/companion/testhost)")
    ap.add_argument("--codex", default=shutil.which("codex"))
    ap.add_argument("--claude", default=shutil.which("claude"))
    ap.add_argument("--clients", default="codex,claude")
    ap.add_argument("--scenarios", default="basic,roundtrip,stop,quit,hangup,tui,account")
    ap.add_argument("--scenario-timeout", type=float, default=90.0)
    ap.add_argument("--tui-sessions", type=int, default=2, help="tui scenario: sequential sessions in one app")
    ap.add_argument("--tui-seed-onboarding", action=argparse.BooleanOptionalAction, default=True,
                    help="tui scenario: seed a synthetic completed-onboarding Claude state in the private config dir")
    args = ap.parse_args(argv)
    args.dmz, args.testhost = os.path.abspath(args.dmz), os.path.abspath(args.testhost)
    report = {
        "harness": PROG,
        "evidence_kind": "app-controller-and-dmz-connect-with-real-cli-binaries-and-synthetic-loopback-upstream",
        "tested": ["companion HTTP controller", "launch ticket", "dmz connect", "app-owned gateway", "installed client binaries (print mode via test seam)"],
        "not_tested": ["native window/browser", "native terminal launchers", "native folder picker", "interactive TUI",
                       "packaging/signing/OS prompts", "Windows/macOS", "real provider API keys", "existing-sign-in accounts"],
        "not_evidence_of": ["provider-compatibility", "account-or-auth-validation", "model-behavior", "detector-recall"],
        "ui_flow_model": {"steps": [s for s, _ in UI_STEPS], "count": len(UI_STEPS), "forbidden": list(UI_STEPS_FORBIDDEN)},
        "versions": {"dmz": base.version_of([args.dmz, "version"], r"dmz local gateway [^\n]{1,80}")},
        "results": [], "skipped": [],
    }
    if not report["versions"]["dmz"] or not os.access(args.testhost, os.X_OK):
        diag("binary_unavailable")
        report["overall"] = "fail"
        print(json.dumps(report, indent=2))
        return 2
    before = base.config_fingerprint()
    bins = {"codex": args.codex, "claude": args.claude}
    patterns = {"codex": r"codex-cli \d+\.\d+\.\d+", "claude": r"\d+\.\d+\.\d+ \(Claude Code\)"}
    first = True
    for client in [c for c in args.clients.split(",") if c]:
        b = bins.get(client)
        ver = base.version_of([b, "--version"], patterns[client]) if b else None
        report["versions"][client] = ver
        if not ver:
            diag(f"{client}_binary_unavailable")
            report["skipped"].append({"client": client, "reason": "binary_unavailable"})
            continue
        b = os.path.realpath(b)
        for scenario in [s for s in args.scenarios.split(",") if s]:
            if scenario not in ("basic", "roundtrip", "stop", "quit", "tui", "hangup", "account"):
                diag("unknown_scenario")
                continue
            r = Run(args, client, b, scenario).run(with_api_checks=first or scenario == "basic")
            first = False
            for code in r["failures"]:
                diag(f"{client}.{scenario}.{code}")
            for code in r["incomplete"]:
                diag(f"{client}.{scenario}.incomplete.{code}")
            report["results"].append(r)
    report["normal_config_unchanged"] = before == base.config_fingerprint()
    statuses = {r["status"] for r in report["results"]}
    if not report["results"] or "fail" in statuses or not report["normal_config_unchanged"]:
        report["overall"], code = "fail", 1
    elif statuses == {"pass"} and not report["skipped"]:
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
