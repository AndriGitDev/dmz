#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Local fixture test of the DMZ background flow with the real installed clients.

The DMZ companion controller (``internal/companion/backgroundtesthost``) starts
with background routing enabled and no window, folder picker or client launch.
The harness then starts the installed Codex and Claude Code command-line
clients *directly and independently*, exactly as a user would, with their
normal configuration loading: no ``--ignore-user-config``, no empty
``--setting-sources`` and no provider, base URL, model or token overrides on
the command line. The only difference from a normal machine is that every
client home is a per-run temporary directory holding a synthetic fixture user
configuration, model requests reach loopback fake providers, and every key,
prompt, file and answer is synthetic (example.test addresses).

This is NOT provider, account, model or detector validation. Fake provider
keys reach each client only through its own native environment, never the
DMZ host configuration or environment. The refusing proxy observes
cooperating clients; it is not a system-wide network jail.

Output contract:
  * stdout: one JSON document with versions and aggregate counts/booleans only.
  * stderr: fixed reason codes only (``agent-background-smoke: <code>``).
  * Request bodies, client output and generated settings are held in memory
    only for the duration of a check and never printed or written outside the
    per-run temporary directory, which is removed at exit.

Python standard library only (reuses agent-client-smoke.py and
agent-manual-smoke.py fixtures).
"""

from __future__ import annotations

import argparse
import hashlib
import importlib.util
import json
import os
import platform
import re
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time
import tomllib
import urllib.error
import urllib.request

PROG = "agent-background-smoke"
_HERE = os.path.dirname(os.path.abspath(__file__))


def _load(name, filename):
    sys.dont_write_bytecode = True
    spec = importlib.util.spec_from_file_location(name, os.path.join(_HERE, filename))
    if spec is None or spec.loader is None:
        raise ImportError(filename)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


manual = _load("dmz_agent_manual_smoke", "agent-manual-smoke.py")
base = manual.base

HEX64_FULL = re.compile(r"[0-9a-f]{64}")
# Synthetic cached account (never a real account): unsigned JWT-shaped tokens
# that only the loopback fake provider ever sees.
SYNTH_ACCOUNT_ID = "acct-dmz-synthetic-0000"


def _b64(obj) -> str:
    import base64
    return base64.urlsafe_b64encode(json.dumps(obj).encode()).decode().rstrip("=")


def synthetic_account_auth() -> tuple[bytes, str]:
    far = int(time.time()) + 30 * 86400
    claims = {"https://api.openai.com/auth": {"chatgpt_plan_type": "plus", "chatgpt_account_id": SYNTH_ACCOUNT_ID,
                                             "chatgpt_user_id": "user-dmz-synthetic"},
              "email": "account.owner@example.test", "exp": far, "iat": far - 60 * 86400}
    jwt = f"{_b64({'alg': 'none', 'typ': 'JWT'})}.{_b64(claims)}.c3ludGhldGlj"
    doc = {"auth_mode": "chatgpt", "OPENAI_API_KEY": None,
           "tokens": {"id_token": jwt, "access_token": jwt, "refresh_token": "rt-dmz-synthetic-refresh",
                      "account_id": SYNTH_ACCOUNT_ID},
           "last_refresh": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}
    return json.dumps(doc).encode(), jwt


class _AccountHandler(manual._LeakCheckHandler):
    def _common(self):
        super()._common()
        srv = self.server
        if getattr(srv, "account_token", None) and self.headers.get("Authorization") == f"Bearer {srv.account_token}":
            with srv.obs.lock:
                srv.account_auth_requests += 1
                if self.headers.get("ChatGPT-Account-ID") == SYNTH_ACCOUNT_ID:
                    srv.account_header_requests += 1
# Synthetic user setting that the fixture user adds while DMZ routing is active.
USER_EDIT_KEY = "dmzSmokeUserEdit"


def diag(code: str) -> None:
    if not re.fullmatch(r"[a-z0-9_.:\-]{1,96}", code):
        code = "invalid_diagnostic_code"
    sys.stderr.write(f"{PROG}: {code}\n")
    sys.stderr.flush()


def sha(data: bytes | None) -> str | None:
    return None if data is None else hashlib.sha256(data).hexdigest()


def read_bytes(path):
    try:
        with open(path, "rb") as fh:
            return fh.read()
    except FileNotFoundError:
        return None


# --- Synthetic fixture user configuration ------------------------------------
def codex_original() -> bytes:
    features = "\n".join(f"{f} = false" for f in base.CODEX_DISABLED_FEATURES)
    return (
        "# Synthetic fixture user configuration for the dmz background smoke test.\n"
        f'model = "{base.CODEX_MODEL}"\n'
        'approval_policy = "never"\n'
        'sandbox_mode = "read-only"\n'
        "check_for_update_on_startup = false\n"
        'web_search = "disabled"\n'
        "\n[analytics]\nenabled = false\n"
        "\n[feedback]\nenabled = false\n"
        f"\n[features]\n{features}\n"
    ).encode()


def claude_original() -> bytes:
    data = {
        "model": base.CLAUDE_MODEL,
        "permissions": {"allow": ["Read"], "defaultMode": "default"},
        "env": {"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1", "DISABLE_TELEMETRY": "1",
                "DISABLE_ERROR_REPORTING": "1", "DISABLE_AUTOUPDATER": "1"},
    }
    return (json.dumps(data, indent=2) + "\n").encode()


# --- Process/HTTP helpers ------------------------------------------------------
def host_env(tmp):
    """The host gets no provider key, endpoint or route variable."""
    env = {k: os.environ[k] for k in ("PATH", "HOME", "USER", "LOGNAME", "LANG", "LC_ALL", "TZ") if k in os.environ}
    env["TMPDIR"] = os.path.join(tmp, "tmp")
    return env


def client_env(tmp, sink_port, client, codex_auth="openai_env"):
    """Native client environment: private homes and the fake key only."""
    env = base.child_env(tmp, sink_port, client)
    env["HOME"] = os.path.join(tmp, "home")
    if client == "codex":
        env["CODEX_HOME"] = os.path.join(tmp, "home", ".codex")
        if codex_auth not in ("openai_env",):
            env.pop("OPENAI_API_KEY", None)
        if codex_auth == "codex_env":
            env["CODEX_API_KEY"] = base.FAKE_OPENAI_KEY
    else:
        env["CLAUDE_CONFIG_DIR"] = os.path.join(tmp, "home", ".claude")
    # child_env sets telemetry switches for Claude; those now live in the
    # fixture user settings, so the environment carries only what a user
    # shell would: home, proxies and the native key.
    for k in ("CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC", "DISABLE_TELEMETRY", "DISABLE_ERROR_REPORTING",
              "DISABLE_AUTOUPDATER", "DISABLE_BUG_COMMAND", "CLAUDE_CODE_DISABLE_TERMINAL_TITLE"):
        env.pop(k, None)
    for k in ("OPENAI_BASE_URL", "ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_MODEL"):
        assert k not in env
    return env


def codex_argv(codex):
    return [codex, "exec", "--skip-git-repo-check", "--sandbox", "read-only", "--json", "--", None]


def claude_argv(claude):
    return [claude, "--bare", "-p", "--output-format", "json", "--", None]


def http(method, url, headers=None, body=None, timeout=5.0):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method, headers=dict(headers or {}))
    if data is not None:
        req.add_header("Content-Type", "application/json")
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        with opener.open(req, timeout=timeout) as resp:
            return resp.status, resp.read(1 << 20)
    except urllib.error.HTTPError as e:
        return e.code, b""
    except OSError:
        return 0, b""


def port_open(port):
    try:
        with socket.create_connection(("127.0.0.1", port), timeout=0.5):
            return True
    except OSError:
        return False


class Host:
    """The background testhost in its own process group; output in memory."""

    def __init__(self, binary, tmp, run, cfg):
        self.dir = os.path.join(tmp, f"host-{run}")
        os.makedirs(self.dir, mode=0o700)
        self.control = os.path.join(self.dir, "control.json")
        self.state = os.path.join(self.dir, "state.json")
        cfg = {**cfg, "control_file": self.control, "state_file": self.state}
        cfgpath = os.path.join(self.dir, "config.json")
        fd = os.open(cfgpath, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "w") as fh:
            json.dump(cfg, fh)
        self.proc = subprocess.Popen([binary, "--config", cfgpath], env=host_env(tmp), cwd=self.dir,
                                     stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                     start_new_session=True)
        self.out = bytearray()
        threading.Thread(target=self._drain, args=(self.proc.stdout,), daemon=True).start()
        threading.Thread(target=self._drain, args=(self.proc.stderr,), daemon=True).start()
        self.origin = self.token = None

    def _drain(self, stream):
        for chunk in iter(lambda: stream.read(4096), b""):
            if len(self.out) < (1 << 20):
                self.out.extend(chunk)

    def wait_ready(self, timeout=20.0):
        end = time.monotonic() + timeout
        while time.monotonic() < end and self.proc.poll() is None:
            try:
                with open(self.control, encoding="utf-8") as fh:
                    d = json.load(fh)
                self.origin, self.token = d["origin"], d["token"]
                return True
            except (OSError, ValueError, KeyError):
                time.sleep(0.1)
        return False

    def api(self, method, path, body=None):
        return http(method, self.origin + path, {"Authorization": f"Bearer {self.token}"}, body)

    def state_doc(self):
        code, raw = self.api("GET", "/api/state")
        if code != 200:
            return None
        try:
            return json.loads(raw)
        except ValueError:
            return None

    def views(self):
        st = self.state_doc() or {}
        return {v.get("client"): v for v in st.get("background") or []}

    def wait_views(self, want, timeout=15.0):
        end = time.monotonic() + timeout
        v = {}
        while time.monotonic() < end:
            v = self.views()
            if all(v.get(c, {}).get("state") == s for c, s in want.items()):
                return v
            time.sleep(0.2)
        return v

    def quit(self, timeout=20.0):
        self.api("POST", "/api/quit")
        try:
            return self.proc.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            base.kill_tree(self.proc)
            return None

    def kill9(self):
        try:
            os.killpg(self.proc.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        self.proc.wait(timeout=10)

    def final_state(self):
        try:
            with open(self.state, encoding="utf-8") as fh:
                return json.load(fh)
        except (OSError, ValueError):
            return None

    def output_leaks(self, secrets_list):
        text = bytes(self.out).decode("utf-8", "replace")
        return any(s and s in text for s in secrets_list) or bool(base.HEX64_RE.search(text))


# --- Route inspection (fixture-owned temp files only) ---------------------------
def route_of(client, data: bytes):
    """Return (origin_port, token, preserved_keys_ok) from generated settings."""
    try:
        if client == "codex":
            doc = tomllib.loads(data.decode())
            prov = doc["model_providers"]["dmz_background"]
            m = re.fullmatch(r"http://127\.0\.0\.1:(\d+)/v1", prov["base_url"])
            tok = prov["http_headers"]["X-Dmz-Token"]
            orig = tomllib.loads(codex_original().decode())
            keep = all(doc.get(k) == orig[k] for k in orig) and doc.get("model_provider") == "dmz_background" \
                and prov.get("requires_openai_auth") is True and "env_key" not in prov
        else:
            doc = json.loads(data)
            env = doc["env"]
            m = re.fullmatch(r"http://127\.0\.0\.1:(\d+)", env["ANTHROPIC_BASE_URL"])
            hdr = [l.split(":", 1)[1].strip() for l in env["ANTHROPIC_CUSTOM_HEADERS"].split("\n")
                   if l.split(":", 1)[0].strip().lower() == "x-dmz-token"]
            tok = hdr[0] if len(hdr) == 1 else ""
            orig = json.loads(claude_original())
            keep = doc.get("model") == orig["model"] and doc.get("permissions") == orig["permissions"] \
                and all(env.get(k) == v for k, v in orig["env"].items()) \
                and "ANTHROPIC_API_KEY" not in env and "ANTHROPIC_AUTH_TOKEN" not in env and "apiKeyHelper" not in doc
        if not m or not HEX64_FULL.fullmatch(tok):
            return None
        return int(m.group(1)), tok, keep
    except (KeyError, ValueError, TypeError, IndexError, UnicodeDecodeError, tomllib.TOMLDecodeError):
        return None


def gateway_probes(port, token):
    g = f"http://127.0.0.1:{port}"
    return {
        "status_with_token": http("GET", g + "/_dmz/status", {"X-Dmz-Token": token})[0],
        "status_wrong_token": http("GET", g + "/_dmz/status", {"X-Dmz-Token": "0" * 64})[0],
        "status_no_token": http("GET", g + "/_dmz/status")[0],
        "status_browser_origin": http("GET", g + "/_dmz/status", {"X-Dmz-Token": token, "Origin": "http://127.0.0.1"})[0],
    }


# --- Scenario ------------------------------------------------------------------
class Run:
    def __init__(self, args, clients, deadline):
        self.args = args
        self.clients = clients  # {"codex": path, "claude": path}
        self.deadline = deadline
        self.failures: list[str] = []
        self.incomplete: list[str] = []
        self.report: dict = {"phases": {}}
        self.tmp = tempfile.mkdtemp(prefix="dmz-background-smoke-")
        os.chmod(self.tmp, 0o700)
        for sub in ("tmp", "home", "home/.codex", "home/.claude"):
            os.makedirs(os.path.join(self.tmp, sub), mode=0o700, exist_ok=True)
        self.settings = {"codex": os.path.join(self.tmp, "home", ".codex", "config.toml"),
                         "claude": os.path.join(self.tmp, "home", ".claude", "settings.json")}
        self.originals = {"codex": codex_original(), "claude": claude_original()}
        self.sink = base.EgressSink()
        threading.Thread(target=self.sink.serve_forever, daemon=True).start()
        self.providers = {}
        for c in clients:
            proto = "responses" if c == "codex" else "messages"
            model = base.CODEX_MODEL if c == "codex" else base.CLAUDE_MODEL
            p = manual.Provider(proto, "basic", self.tmp, model, None)
            p.RequestHandlerClass = _AccountHandler
            p.account_token = None
            p.account_auth_requests = p.account_header_requests = 0
            threading.Thread(target=p.serve_forever, daemon=True).start()
            self.providers[c] = p
        self.host_cfg = {
            "background_state_dir": os.path.join(self.tmp, "background-state"),
            "clients": [{"id": c, "path": p} for c, p in clients.items()],
            "settings": {c: self.settings[c] for c in clients},
            "upstreams": {c: f"http://127.0.0.1:{self.providers[c].server_address[1]}/v1" for c in clients},
        }
        self.routes: dict[str, tuple[int, str]] = {}
        self.hosts: list[Host] = []
        self.run_index = 0

    def fail(self, code):
        if code not in self.failures:
            self.failures.append(code)

    def write_originals(self):
        for c in self.clients:
            fd = os.open(self.settings[c], os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
            with os.fdopen(fd, "wb") as fh:
                fh.write(self.originals[c])
        if "codex" in self.clients and self.args.codex_auth == "login":
            # Native API-key login into the private CODEX_HOME; the key goes
            # to the client on stdin, never into argv or the host.
            env = client_env(self.tmp, self.sink.server_address[1], "codex", "login")
            proc = subprocess.run([self.clients["codex"], "login", "--with-api-key"], input=base.FAKE_OPENAI_KEY.encode(),
                                  env=env, cwd=self.tmp, capture_output=True, timeout=30, start_new_session=True)
            self.report["codex_native_login_exit_code"] = proc.returncode
            if proc.returncode != 0:
                self.fail("codex_native_login_failed")
        if "codex" in self.clients and self.args.codex_auth == "account":
            data, self.account_token = synthetic_account_auth()
            fd = os.open(os.path.join(self.tmp, "home", ".codex", "auth.json"), os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
            with os.fdopen(fd, "wb") as fh:
                fh.write(data)
            p = self.providers["codex"]
            p.account_token = self.account_token
        self.auth_store = {"codex": read_bytes(os.path.join(self.tmp, "home", ".codex", "auth.json"))}

    def start_host(self, phase):
        self.run_index += 1
        h = Host(self.args.testhost, self.tmp, self.run_index, self.host_cfg)
        self.hosts.append(h)
        if not h.wait_ready():
            self.fail(f"{phase}.host_not_ready")
            return None
        if self.args.highlight_restored:
            code, _ = h.api("POST", "/api/preferences", {"highlight_restored": True})
            if code != 204 or not (h.state_doc() or {}).get("highlight_restored"):
                self.fail(f"{phase}.highlight_preference_not_applied")
        return h

    def secrets(self):
        return [t for _, t in self.routes.values()] + [base.FAKE_OPENAI_KEY, base.FAKE_ANTHROPIC_KEY] + \
            [h.token for h in self.hosts if h.token]

    def check_routed(self, phase, h, expect_stable):
        """After startup: every client routed, restart_required, settings generated."""
        r = {}
        views = h.wait_views({c: "restart_required" for c in self.clients})
        st = h.state_doc() or {}
        r["background_enabled"] = st.get("background_enabled")
        r["sessions"] = len(st.get("sessions") or [])
        if st.get("background_enabled") is not True:
            self.fail(f"{phase}.background_not_enabled")
        if r["sessions"]:
            self.fail(f"{phase}.launcher_session_created")
        for c in self.clients:
            v = views.get(c, {})
            cr = {"state": v.get("state"), "issue": v.get("issue"), "requests": v.get("requests"), "detected": v.get("detected")}
            if v.get("state") != "restart_required":
                self.fail(f"{phase}.{c}.state_not_restart_required")
            data = read_bytes(self.settings[c])
            route = route_of(c, data) if data else None
            cr["settings_changed"] = data != self.originals[c]
            if route is None:
                self.fail(f"{phase}.{c}.route_not_generated")
                r[c] = cr
                continue
            port, tok, keep = route
            cr["user_model_and_permissions_preserved"] = keep
            if not keep:
                self.fail(f"{phase}.{c}.user_settings_not_preserved")
            try:
                mode = os.stat(self.settings[c]).st_mode & 0o777
                cr["settings_private"] = mode & 0o077 == 0
            except OSError:
                cr["settings_private"] = False
            probes = gateway_probes(port, tok)
            cr["gateway_probes"] = probes
            if probes != {"status_with_token": 200, "status_wrong_token": 401, "status_no_token": 401, "status_browser_origin": 403}:
                self.fail(f"{phase}.{c}.gateway_auth_unexpected")
            if expect_stable and c in self.routes:
                cr["port_stable"] = self.routes[c][0] == port
                cr["token_stable"] = self.routes[c][1] == tok
                if not (cr["port_stable"] and cr["token_stable"]):
                    self.fail(f"{phase}.{c}.route_not_stable_across_restart")
            self.routes[c] = (port, tok)
            self.providers[c].local_token = tok
            # Model discovery is not traffic: state stays restart_required.
            key = {"Authorization": f"Bearer {base.FAKE_OPENAI_KEY}"} if c == "codex" else {"x-api-key": base.FAKE_ANTHROPIC_KEY}
            code, _ = http("GET", f"http://127.0.0.1:{port}/v1/models", {"X-Dmz-Token": tok, **key})
            cr["models_get_status"] = code
            after = h.views().get(c, {})
            cr["state_after_models_get"] = after.get("state")
            if after.get("state") != "restart_required":
                self.fail(f"{phase}.{c}.models_get_counted_as_traffic")
            r[c] = cr
        return r

    def run_client(self, phase, h, client, scenario):
        self.run_index += 1
        work = os.path.join(self.tmp, f"work-{self.run_index}-{client}-{scenario}")
        os.makedirs(work, mode=0o700)
        with open(os.path.join(work, base.FIXTURE_NAME), "w", encoding="utf-8") as fh:
            fh.write(base.FIXTURE_TEXT)
        p = self.providers[client]
        p.reset_scenario(scenario, work)
        p.account_auth_requests = p.account_header_requests = 0
        before = self.sink.attempts
        argv = codex_argv(self.clients[client]) if client == "codex" else claude_argv(self.clients[client])
        argv[-1] = base.build_prompt(scenario)
        env = client_env(self.tmp, self.sink.server_address[1], client, self.args.codex_auth)
        rc, out, err, to = base.run_captured(argv, env, work, self.deadline.remaining(self.args.scenario_timeout))
        res = manual.evaluate(client, scenario, p, rc, out, err, to, self.routes[client][1], self.sink)
        if self.args.highlight_restored:
            texts = []
            for line in out.splitlines():
                try:
                    event = json.loads(line)
                except (ValueError, TypeError):
                    continue
                if not isinstance(event, dict):
                    continue
                if client == "codex":
                    item = event.get("item") or {}
                    if event.get("type") == "item.completed" and item.get("type") == "agent_message":
                        texts.append(item.get("text", ""))
                elif isinstance(event.get("result"), str):
                    texts.append(event["result"])
            bold = any("**" + base.PROMPT_EMAIL + "**" in text for text in texts)
            res["client_output"]["restored_values_bold"] = bold
            if not bold:
                res["failures"].append("restored_value_not_bold_in_assistant_output")
        if client == "codex" and self.args.codex_auth == "account":
            u = res["upstream"]
            u["synthetic_account_auth_requests"] = p.account_auth_requests
            u["synthetic_account_header_requests"] = p.account_header_requests
            ok = u["requests"] and p.account_auth_requests == u["requests"] == p.account_header_requests
            if "upstream_credential_not_service_key" in res["failures"] and ok and u["fake_service_key_requests"] == 0:
                res["failures"].remove("upstream_credential_not_service_key")
            if not ok:
                res["failures"].append("native_account_auth_not_preserved")
        combined = out + "\n" + err
        res["client_output"]["dmz_host_token_in_output"] = any(x.token and x.token in combined for x in self.hosts)
        if res["client_output"]["dmz_host_token_in_output"]:
            res["failures"].append("credential_like_value_in_client_output")
        res["egress_attempts_blocked"] = self.sink.attempts - before
        if res["egress_attempts_blocked"]:
            # Counted and refused; reported as incomplete isolation evidence.
            res["incomplete"].append("client_attempted_non_loopback_egress")
        del out, err, combined
        if res["failures"]:
            res["status"] = "fail"
        elif res["incomplete"]:
            res["status"] = "incomplete"
        v = h.views().get(client, {}) if h else {}
        res["dmz_view_after"] = {"state": v.get("state"), "requests": v.get("requests"),
                                 "changed_parts": v.get("changed_parts"), "warnings": v.get("warnings")}
        if h is not None and res["upstream"]["main_requests"]:
            if v.get("state") != "traffic_observed":
                res["failures"].append("dmz_state_not_traffic_observed")
                res["status"] = "fail"
            if not v.get("requests") or not v.get("changed_parts"):
                res["failures"].append("dmz_counts_not_updated")
                res["status"] = "fail"
        for f in res["failures"]:
            self.fail(f"{phase}.{client}.{scenario}.{f}")
        for f in res["incomplete"]:
            code = f"{phase}.{client}.{scenario}.{f}"
            if code not in self.incomplete:
                self.incomplete.append(code)
        return res

    def check_restored(self, phase, exact=True):
        r = {}
        for c in self.clients:
            data = read_bytes(self.settings[c])
            same = data == self.originals[c]
            r[c] = {"byte_exact_original": same}
            if exact and not same:
                self.fail(f"{phase}.{c}.settings_not_byte_exact_after_restore")
        return r

    def check_closed(self, phase):
        r = {}
        for c, (port, _) in self.routes.items():
            r[c] = {"gateway_port_closed": not port_open(port)}
            if port_open(port):
                self.fail(f"{phase}.{c}.gateway_still_listening")
        return r

    def finish_host(self, phase, h):
        rc = h.quit()
        st = h.final_state()
        res = {"exit_code": rc, "state": st}
        if rc != 0:
            self.fail(f"{phase}.host_exit_nonzero")
        if not st or st.get("serve_error"):
            self.fail(f"{phase}.host_serve_error")
        elif st.get("folder_picker_calls") or st.get("window_open_calls") or st.get("terminal_launch_calls"):
            self.fail(f"{phase}.host_used_picker_window_or_launcher")
        res["control_file_removed"] = not os.path.exists(h.control)
        if not res["control_file_removed"]:
            self.fail(f"{phase}.control_file_left")
        res["secret_in_host_output"] = h.output_leaks(self.secrets())
        if res["secret_in_host_output"]:
            self.fail(f"{phase}.secret_in_host_output")
        return res

    def scenarios(self):
        return ["basic", "roundtrip"] if self.args.roundtrip else ["basic"]

    # ---- Phases
    def execute(self):
        P = self.report["phases"]
        self.write_originals()
        P["originals"] = {c: {"sha256_prefix": sha(self.originals[c])[:12]} for c in self.clients}

        # 1. First startup: auto-detect and route; clients launched independently.
        h = self.start_host("first_start")
        if h is None:
            return
        P["first_start"] = {"routed": self.check_routed("first_start", h, False), "clients": []}
        for c in self.clients:
            for s in self.scenarios():
                P["first_start"]["clients"].append(self.run_client("first_start", h, c, s))
        markers = {}
        for c in self.clients:
            markers[c] = self.providers[c].obs.first_marker

        # 2. Disconnect restores, reconnect writes the same owned route.
        dis = {}
        for c in self.clients:
            code, _ = h.api("POST", "/api/background", {"client": c, "connected": False})
            v = h.views().get(c, {})
            restored = read_bytes(self.settings[c]) == self.originals[c]
            closed = not port_open(self.routes[c][0])
            dis[c] = {"disconnect_status": code, "state": v.get("state"), "byte_exact_original": restored, "gateway_port_closed": closed}
            if code != 204 or v.get("state") != "disconnected" or not restored or not closed:
                self.fail(f"disconnect.{c}.not_restored")
        P["disconnect"] = dis
        rec = {}
        for c in self.clients:
            old = self.routes[c]
            code, _ = h.api("POST", "/api/background", {"client": c, "connected": True})
            v = h.views().get(c, {})
            route = route_of(c, read_bytes(self.settings[c]) or b"")
            rec[c] = {"reconnect_status": code, "state": v.get("state"),
                      "same_port": bool(route) and route[0] == old[0], "same_token": bool(route) and route[1] == old[1]}
            if code != 204 or v.get("state") != "restart_required" or not route:
                self.fail(f"reconnect.{c}.not_routed")
            elif not (rec[c]["same_port"] and rec[c]["same_token"]):
                self.fail(f"reconnect.{c}.route_not_stable")
        P["reconnect"] = {"routes": rec, "clients": [self.run_client("reconnect", h, c, "basic") for c in self.clients]}

        # 3. Quit restores every file byte-exactly and closes gateways.
        P["first_quit"] = self.finish_host("first_quit", h)
        P["first_quit"]["restored"] = self.check_restored("first_quit")
        P["first_quit"]["closed"] = self.check_closed("first_quit")

        # 4. Restart: same port/token; normal clients work again.
        h = self.start_host("restart")
        if h is None:
            return
        P["restart"] = {"routed": self.check_routed("restart", h, True),
                        "clients": [self.run_client("restart", h, c, "basic") for c in self.clients]}
        P["restart"]["marker_same_as_first_run"] = {c: self.providers[c].obs.first_marker == markers[c] for c in self.clients}

        # 5. Abrupt host loss leaves the route; next start recovers from the journal.
        h.kill9()
        P["crash"] = {"settings_still_routed": {c: route_of(c, read_bytes(self.settings[c]) or b"") is not None for c in self.clients}}
        h = self.start_host("recover")
        if h is None:
            return
        P["recover"] = {"routed": self.check_routed("recover", h, True)}

        # 6. A user edit while routed survives quit; only the owned route is removed.
        self.user_edit()
        P["user_edit_quit"] = self.finish_host("user_edit_quit", h)
        P["user_edit_quit"]["result"] = self.check_user_edit()
        P["user_edit_quit"]["closed"] = self.check_closed("user_edit_quit")
        if "codex" in self.clients:
            now = read_bytes(os.path.join(self.tmp, "home", ".codex", "auth.json"))
            P["native_auth_store_unchanged"] = now == self.auth_store["codex"]
            if now != self.auth_store["codex"]:
                self.fail("codex_native_auth_store_changed")

    def user_edit(self):
        c_path = self.settings["claude"] if "claude" in self.clients else None
        if c_path:
            doc = json.loads(read_bytes(c_path))
            doc[USER_EDIT_KEY] = True
            doc["env"]["DMZ_SMOKE_USER_VAR"] = "1"
            with open(c_path, "w", encoding="utf-8") as fh:
                fh.write(json.dumps(doc, indent=2) + "\n")
        x_path = self.settings["codex"] if "codex" in self.clients else None
        if x_path:
            data = read_bytes(x_path).decode()
            with open(x_path, "w", encoding="utf-8") as fh:
                fh.write('model_reasoning_effort = "low"\n' + data)

    def check_user_edit(self):
        r = {}
        if "claude" in self.clients:
            doc = json.loads(read_bytes(self.settings["claude"]))
            want = json.loads(self.originals["claude"])
            want[USER_EDIT_KEY] = True
            want["env"]["DMZ_SMOKE_USER_VAR"] = "1"
            ok = doc == want
            r["claude"] = {"user_edit_preserved_route_removed": ok}
            if not ok:
                self.fail("user_edit_quit.claude.merge_unexpected")
        if "codex" in self.clients:
            doc = tomllib.loads(read_bytes(self.settings["codex"]).decode())
            want = tomllib.loads(self.originals["codex"].decode())
            want["model_reasoning_effort"] = "low"
            ok = doc == want
            r["codex"] = {"user_edit_preserved_route_removed": ok}
            if not ok:
                self.fail("user_edit_quit.codex.merge_unexpected")
        return r

    def cleanup(self):
        for h in self.hosts:
            if h.proc.poll() is None:
                self.fail("host_left_running")
                base.kill_tree(h.proc)
        orphans = 0
        for pid in os.listdir("/proc"):
            if not pid.isdigit() or int(pid) == os.getpid():
                continue
            try:
                with open(f"/proc/{pid}/cmdline", "rb") as fh:
                    cmd = fh.read()
                with open(f"/proc/{pid}/environ", "rb") as fh:
                    envb = fh.read()
            except OSError:
                continue
            if self.tmp.encode() in cmd or self.tmp.encode() in envb:
                orphans += 1
                try:
                    os.kill(int(pid), signal.SIGKILL)
                except OSError:
                    pass
        self.report["orphan_owned_processes"] = orphans
        if orphans:
            self.fail("orphan_owned_processes")
        for p in list(self.providers.values()) + [self.sink]:
            p.shutdown()
            p.server_close()
        self.report["egress_attempts_blocked_total"] = self.sink.attempts
        shutil.rmtree(self.tmp, ignore_errors=True)
        self.report["temp_removed"] = not os.path.exists(self.tmp)


def main(argv=None):
    ap = argparse.ArgumentParser(description="Synthetic loopback test of the DMZ background flow with real clients.")
    ap.add_argument("--testhost", required=True, help="built internal/companion/backgroundtesthost binary")
    ap.add_argument("--codex", default=shutil.which("codex"))
    ap.add_argument("--claude", default=shutil.which("claude"))
    ap.add_argument("--clients", default="codex,claude")
    ap.add_argument("--no-roundtrip", dest="roundtrip", action="store_false")
    ap.add_argument("--highlight-restored", action="store_true", help="Enable bold restored values through the real preferences API")
    ap.add_argument("--codex-auth", choices=("openai_env", "codex_env", "login", "account"), default="login",
                    help="native Codex authentication: OPENAI_API_KEY, CODEX_API_KEY, a cached API-key login "
                         "or a cached synthetic account")
    ap.add_argument("--scenario-timeout", type=float, default=150.0)
    ap.add_argument("--total-timeout", type=float, default=1500.0)
    args = ap.parse_args(argv)
    if platform.system() != "Linux":
        diag("linux_only")
        return 2
    if not (os.path.isabs(args.testhost) and os.access(args.testhost, os.X_OK)):
        diag("testhost_unavailable")
        return 2
    wanted = [c for c in args.clients.split(",") if c]
    clients = {}
    for c in wanted:
        p = args.codex if c == "codex" else args.claude if c == "claude" else None
        if not p or not os.access(p, os.X_OK):
            diag(f"client_unavailable.{c}")
            return 2
        clients[c] = os.path.realpath(p)
    fingerprint = base.config_fingerprint()
    deadline = base.Deadline(args.total_timeout)
    run = Run(args, clients, deadline)
    run.report["versions"] = {
        "codex": base.version_of([clients["codex"], "--version"], r"\d+\.\d+\.\d+") if "codex" in clients else None,
        "claude": base.version_of([clients["claude"], "--version"], r"\d+\.\d+\.\d+") if "claude" in clients else None,
        "python": platform.python_version(),
    }
    try:
        run.execute()
    except Exception as e:  # never print exception text: it may contain content
        run.fail("harness_exception." + type(e).__name__.lower())
    finally:
        run.cleanup()
    unchanged = base.config_fingerprint() == fingerprint
    run.report["caller_client_config_metadata_unchanged"] = unchanged
    if not unchanged:
        run.fail("caller_client_config_metadata_changed")
    run.report["failures"] = run.failures
    run.report["incomplete"] = run.incomplete
    run.report["status"] = "fail" if run.failures else ("incomplete" if run.incomplete else "pass")
    run.report["scope"] = ("synthetic loopback fixture with real installed clients; "
                           "not provider, account, model or detector validation")
    json.dump(run.report, sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    for f in run.failures:
        diag(f)
    return 1 if run.failures else (3 if run.incomplete else 0)


if __name__ == "__main__":
    sys.exit(main())
