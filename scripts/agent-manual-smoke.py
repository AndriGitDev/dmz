#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Local synthetic smoke test of the manual ``dmz serve`` / ``dmz config`` path.

Unlike ``agent-client-smoke.py`` (which exercises ``dmz run``), this harness
starts a long-lived ``dmz serve`` process with a provider-side fake key and
points the *installed* command-line clients at it directly:

  * Codex: ``dmz config codex`` output is written to an isolated
    ``CODEX_HOME/config.toml``; ``codex exec`` reads it and sends the local
    token through the generated ``env_key`` (``--local-auth bearer``).
  * Claude Code: ``--bare`` API-key mode with the local token as
    ``ANTHROPIC_API_KEY`` and a loopback base URL (``--local-auth api-key``).
    This is wire compatibility with the alternative ``x-api-key`` scheme of a
    gateway configuration; it is NOT native desktop-application validation.
    The Claude JSON fragment from ``dmz config claude`` is only parsed.

Everything is synthetic: fake keys, example.test addresses, loopback fake
providers and a refusing proxy sink for other egress. It is not provider,
account, model or desktop validation.

Output contract: stdout is one JSON document of versions, counts and booleans;
stderr carries fixed reason codes only (``agent-manual-smoke: <code>``). Request
bodies, client output, service output and tokens are never printed or written.
Normal client configuration and authentication stores are never read (only
stat'ed to show they are unchanged).

Exit: 0 pass, 3 incomplete, 1 fail, 2 dmz executable unusable.
Python standard library only. Reuses helpers from agent-client-smoke.py.
"""

from __future__ import annotations

import argparse
import importlib.util
import io
import json
import os
import platform
import re
import secrets
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

PROG = "agent-manual-smoke"
_HERE = os.path.dirname(os.path.abspath(__file__))


def _load_base():
    sys.dont_write_bytecode = True
    spec = importlib.util.spec_from_file_location("dmz_agent_client_smoke", os.path.join(_HERE, "agent-client-smoke.py"))
    if spec is None or spec.loader is None:
        raise ImportError("agent-client-smoke.py unavailable")
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


base = _load_base()

SERVE_WARN_RE = re.compile(r"^dmz warning: ([a-z0-9_]{1,64})$", re.M)


def diag(code: str) -> None:
    if not re.fullmatch(r"[a-z0-9_.:\-]{1,96}", code):
        code = "invalid_diagnostic_code"
    sys.stderr.write(f"{PROG}: {code}\n")
    sys.stderr.flush()


def new_local_token() -> str:
    # Synthetic, >= 32 chars, deliberately not a 64-hex shape so the hex64
    # output check stays independent.
    return "dmz-local-synthetic-" + secrets.token_urlsafe(24)


# --- Fake provider with local-token leak accounting -------------------------
class _LeakCheckHandler(base._FakeHandler):
    def _common(self):
        super()._common()
        tok = self.server.local_token  # type: ignore[attr-defined]
        if tok and any(tok in v for v in self.headers.values()):
            with self.server.obs.lock:
                self.server.local_token_upstream += 1  # type: ignore[attr-defined]

    def do_POST(self):
        # Body-level leak check before delegating; body is re-fed to base handler.
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length > 0 else b""
        tok = self.server.local_token  # type: ignore[attr-defined]
        if tok and tok.encode() in raw:
            with self.server.obs.lock:
                self.server.local_token_upstream += 1  # type: ignore[attr-defined]
        self.rfile = io.BytesIO(raw)
        del raw
        super().do_POST()


class Provider(base.FakeProvider):
    def __init__(self, protocol, scenario, workdir, model, local_token):
        super().__init__(protocol, scenario, workdir, model)
        self.RequestHandlerClass = _LeakCheckHandler
        self.local_token = local_token
        self.local_token_upstream = 0

    def reset_scenario(self, scenario, workdir):
        self.scenario = scenario
        self.workdir = workdir
        self.obs = base.Observation()
        self.local_token_upstream = 0


# --- Helpers ----------------------------------------------------------------
def free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def wait_listen(port, deadline, cap=10.0) -> bool:
    end = time.monotonic() + deadline.remaining(cap)
    while time.monotonic() < end:
        try:
            with socket.create_connection(("127.0.0.1", port), timeout=0.5):
                return True
        except OSError:
            time.sleep(0.1)
    return False


def base_env(tmp, sink_port):
    env = {k: os.environ[k] for k in base.ENV_ALLOW if k in os.environ}
    proxy = f"http://127.0.0.1:{sink_port}"
    env.update({
        "TERM": "dumb", "NO_COLOR": "1", "TMPDIR": os.path.join(tmp, "tmp"),
        "HTTP_PROXY": proxy, "HTTPS_PROXY": proxy, "http_proxy": proxy, "https_proxy": proxy,
        "ALL_PROXY": proxy, "all_proxy": proxy,
        "NO_PROXY": "127.0.0.1,localhost,::1", "no_proxy": "127.0.0.1,localhost,::1",
    })
    return env


def client_env(tmp, sink_port, client, token, service_base):
    """Like the launcher harness child env, but the client holds ONLY the local token."""
    env = base.child_env(tmp, sink_port, client)
    env.pop("OPENAI_API_KEY", None)
    env.pop("ANTHROPIC_API_KEY", None)
    if client == "codex":
        env["DMZ_LOCAL_TOKEN"] = token  # consumed by the generated env_key
    else:
        env["ANTHROPIC_API_KEY"] = token  # local dmz token, not a provider key
        env["ANTHROPIC_BASE_URL"] = service_base
    return env


class Service:
    """A ``dmz serve`` process in its own process group; output kept in memory."""

    def __init__(self, dmz, protocol, local_auth, port, upstream_port, env, cwd):
        self.argv = [dmz, "serve", "--protocol", protocol, "--local-auth", local_auth,
                     "--listen", f"127.0.0.1:{port}", "--allow-loopback-upstream",
                     "--upstream", f"http://127.0.0.1:{upstream_port}/v1"]
        self.proc = subprocess.Popen(self.argv, env=env, cwd=cwd, stdin=subprocess.DEVNULL,
                                     stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
        self.err = bytearray()
        self.out = bytearray()
        self._t = [threading.Thread(target=self._drain, args=(self.proc.stderr, self.err), daemon=True),
                   threading.Thread(target=self._drain, args=(self.proc.stdout, self.out), daemon=True)]
        for t in self._t:
            t.start()

    @staticmethod
    def _drain(stream, buf):
        for chunk in iter(lambda: stream.read1(4096), b""):
            buf.extend(chunk)

    def stop(self) -> int | None:
        if self.proc.poll() is None:
            try:
                os.killpg(self.proc.pid, signal.SIGTERM)  # graceful: serve handles SIGTERM
                self.proc.wait(timeout=5)
            except (ProcessLookupError, PermissionError, subprocess.TimeoutExpired):
                pass
        base.kill_tree(self.proc)
        try:
            os.killpg(self.proc.pid, signal.SIGKILL)
        except (ProcessLookupError, PermissionError):
            pass
        for t in self._t:
            t.join(timeout=3)
        return self.proc.returncode

    def warnings(self):
        counts: dict[str, int] = {}
        for code in SERVE_WARN_RE.findall(self.err.decode("utf-8", "replace")):
            counts[code] = counts.get(code, 0) + 1
        return counts

    def text(self):
        return self.err.decode("utf-8", "replace") + self.out.decode("utf-8", "replace")


def control(dmz, action, port, token, tmp, deadline):
    env = {"PATH": os.environ.get("PATH", ""), "DMZ_LOCAL_TOKEN": token, "TMPDIR": os.path.join(tmp, "tmp"),
           "NO_PROXY": "127.0.0.1", "no_proxy": "127.0.0.1"}
    rc, out, err, to = base.run_captured([dmz, action, "--address", f"http://127.0.0.1:{port}"], env, tmp,
                                         deadline.remaining(15))
    return rc, out, err, to


def raw_status(port, headers):
    """Direct status probe used for auth checks. Returns HTTP status only."""
    req = urllib.request.Request(f"http://127.0.0.1:{port}/_dmz/status", headers=headers, method="GET")
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        with opener.open(req, timeout=5) as r:
            r.read(65536)
            return r.status
    except urllib.error.HTTPError as e:
        return e.code
    except OSError:
        return None


def parse_status(out):
    try:
        st = json.loads(out)
    except json.JSONDecodeError:
        return None
    if not isinstance(st, dict):
        return None
    warns = st.get("warnings") or {}
    recent = st.get("recent_warnings", [])
    if not isinstance(recent, list) or len(recent) > 64 or any(
        not isinstance(code, str) or not re.fullmatch(r"[a-z0-9_]{1,64}", code) or code not in warns
        for code in recent
    ):
        return None
    return {
        "requests": st.get("requests"), "text_parts": st.get("text_parts"), "changed_parts": st.get("changed_parts"),
        "sessions": st.get("sessions"),
        "warnings": {k: v for k, v in sorted(warns.items()) if re.fullmatch(r"[a-z0-9_]{1,64}", str(k)) and isinstance(v, int)},
        "keys": sorted(k for k in st.keys() if re.fullmatch(r"[a-z_]{1,32}", str(k))),
    }


# --- Config fragments ---------------------------------------------------------
def check_config(dmz, tmp, port, secrets_list, deadline):
    """Generate both fragments with secrets present in env; verify parse + non-disclosure."""
    env = {"PATH": os.environ.get("PATH", ""), "TMPDIR": os.path.join(tmp, "tmp"),
           "DMZ_LOCAL_TOKEN": secrets_list[0], "OPENAI_API_KEY": base.FAKE_OPENAI_KEY,
           "ANTHROPIC_API_KEY": base.FAKE_ANTHROPIC_KEY}
    res = {}
    codex_toml = None
    for client in ("codex", "claude"):
        rc, out, err, to = base.run_captured([dmz, "config", "--address", f"http://127.0.0.1:{port}", client],
                                             env, tmp, deadline.remaining(15))
        combined = out + err
        r = {"exit_code": rc, "secret_printed": any(s in combined for s in secrets_list),
             "hex64_printed": bool(base.HEX64_RE.search(combined))}
        if client == "codex":
            try:
                doc = tomllib.loads(out)
                prov = (doc.get("model_providers") or {}).get("dmz_local") or {}
                r.update({
                    "toml_parses": True,
                    "model_provider_selected": doc.get("model_provider") == "dmz_local",
                    "base_url_loopback_v1": prov.get("base_url") == f"http://127.0.0.1:{port}/v1",
                    "wire_api_responses": prov.get("wire_api") == "responses",
                    "env_key_local_token": prov.get("env_key") == "DMZ_LOCAL_TOKEN",
                    "requires_openai_auth_false": prov.get("requires_openai_auth") is False,
                    "websockets_disabled": prov.get("supports_websockets") is False,
                    "compression_disabled": (doc.get("features") or {}).get("enable_request_compression") is False,
                })
                codex_toml = out
            except tomllib.TOMLDecodeError:
                r["toml_parses"] = False
        else:
            try:
                doc = json.loads(out)
                r.update({
                    "json_parses": isinstance(doc, dict),
                    "base_url_loopback": doc.get("inferenceGatewayBaseUrl") == f"http://127.0.0.1:{port}",
                    "auth_scheme": doc.get("inferenceGatewayAuthScheme") if doc.get("inferenceGatewayAuthScheme") in ("bearer", "x-api-key") else "other",
                    "token_placeholder_only": doc.get("inferenceGatewayApiKey") == "REPLACE_WITH_LOCAL_DMZ_TOKEN",
                    "native_desktop_executed": False,
                })
            except json.JSONDecodeError:
                r["json_parses"] = False
        del out, err, combined
        res[client] = r
    return res, codex_toml


# --- Clients -----------------------------------------------------------------
def codex_argv(codex, scenario, workdir):
    # No --ignore-user-config: the isolated CODEX_HOME/config.toml carries the
    # generated provider. Sandbox/approval stay the launcher harness fixture values.
    return [codex, "exec", "--ignore-rules", "--ephemeral", "--sandbox", "read-only",
            "--skip-git-repo-check", "--color", "never", "-C", workdir, "-m", base.CODEX_MODEL,
            "-c", 'approval_policy="never"', "-c", "mcp_servers={}",
            "-c", "check_for_update_on_startup=false", "-c", "analytics.enabled=false",
            "-c", "feedback.enabled=false", "-c", 'web_search="disabled"',
            *base.CODEX_FEATURE_ARGS, base.build_prompt(scenario)]


def claude_argv(claude, scenario, workdir):
    tools = "Read" if scenario == "roundtrip" else ""
    argv = [claude, "--bare", "-p", "--no-session-persistence", "--setting-sources", "",
            "--strict-mcp-config", "--tools", tools, "--model", base.CLAUDE_MODEL,
            "--output-format", "text", "--max-turns", "4"]
    if scenario == "roundtrip":
        argv += ["--allowedTools", "Read", "--add-dir", workdir]
    return argv + ["--", base.build_prompt(scenario)]


def evaluate(client, scenario, provider, rc, out, err, timed_out, token, sink):
    obs = provider.obs
    combined = out + "\n" + err
    marker_sets = [s for s in obs.markers_by_request if s]
    stable = len(marker_sets) >= 2 and all(s[0] == marker_sets[0][0] for s in marker_sets)
    r = {"client": client, "scenario": scenario, "status": "fail", "failures": [], "incomplete": [],
         "exit_code": rc, "timed_out": timed_out,
         "upstream": {
             "requests": obs.requests, "routes": dict(sorted(obs.routes.items())), "unknown_routes": obs.unknown_routes,
             "model_requests": obs.model_requests, "stream_requests": obs.stream_requests,
             "main_requests": obs.main_requests, "json_parse_failures": obs.json_parse_failures,
             "x_dmz_header_requests": obs.dmz_header_requests,
             "fake_service_key_requests": obs.fake_credential_requests,
             "other_credential_requests": obs.other_credential_requests,
             "local_token_reached_upstream": provider.local_token_upstream,
             "session_header_requests": obs.session_header_requests,
             "original_email_in_eligible_field": obs.original_in_eligible,
             "original_email_anywhere_in_body": obs.original_anywhere,
             "marker_in_eligible_field": obs.marker_in_eligible,
             "marker_stable_across_requests": stable if len(marker_sets) >= 2 else None,
             "tool_calls_sent": obs.tool_calls_sent, "tool_call_unavailable": obs.tool_call_unavailable,
             "tool_result_requests": obs.tool_result_requests,
             "tool_result_fixture_unchanged": obs.tool_result_fixture_intact,
             "tool_result_fixture_masked": obs.tool_result_fixture_masked,
             "tool_result_sandbox_error": obs.tool_result_sandbox_error,
             "final_answers_sent": obs.final_answers_sent,
         },
         "client_output": {
             "final_answer_delivered": base.FINAL_TEXT in out,
             "marker_returned_unrestored": bool(obs.first_marker) and obs.first_marker in out,
             "original_email_in_output": base.PROMPT_EMAIL in out,
             "local_token_in_output": token in combined,
             "fake_key_in_output": base.FAKE_OPENAI_KEY in combined or base.FAKE_ANTHROPIC_KEY in combined,
             "hex64_token_like_in_output": bool(base.HEX64_RE.search(combined)),
         },
         "egress_attempts_blocked_total": sink.attempts}
    f = r["failures"]
    if timed_out:
        f.append("client_timeout")
    if rc != 0:
        f.append("client_nonzero_exit")
    if obs.main_requests == 0:
        f.append("main_request_not_observed")
    if obs.json_parse_failures:
        f.append("upstream_body_not_json")
    if obs.unknown_routes:
        f.append("unsupported_route_reached_upstream")
    if obs.dmz_header_requests:
        f.append("x_dmz_header_leaked_upstream")
    if provider.local_token_upstream:
        f.append("local_token_reached_upstream")
    if obs.other_credential_requests or (obs.model_requests and obs.fake_credential_requests < obs.requests):
        f.append("upstream_credential_not_service_key")
    if obs.original_in_eligible:
        f.append("original_email_in_eligible_field")
    if obs.main_requests and obs.marker_in_eligible < obs.main_requests:
        f.append("marker_missing_in_eligible_field")
    if client == "claude" and obs.model_requests and obs.session_header_requests < obs.model_requests:
        f.append("claude_session_header_missing")
    co = r["client_output"]
    if not co["final_answer_delivered"]:
        f.append("final_answer_not_delivered_via_sse")
    if co["original_email_in_output"]:
        f.append("original_email_in_client_output")
    if co["local_token_in_output"] or co["fake_key_in_output"] or co["hex64_token_like_in_output"]:
        f.append("credential_like_value_in_client_output")
    if scenario == "roundtrip":
        if obs.tool_calls_sent == 0:
            f.append("roundtrip_tool_unavailable")
        elif obs.tool_result_requests == 0:
            f.append("roundtrip_tool_result_not_returned")
        else:
            if obs.tool_result_fixture_intact == 0 and obs.tool_result_sandbox_error:
                # Default sandbox could not run on this host. Not weakened here.
                r["incomplete"].append("client_sandbox_unavailable_tool_content_unverified")
            elif obs.tool_result_fixture_intact == 0:
                f.append("roundtrip_tool_output_not_fixture")
            if obs.tool_result_fixture_masked:
                f.append("roundtrip_tool_output_modified")
            if not stable:
                f.append("roundtrip_marker_not_stable")
    if not f:
        r["status"] = "incomplete" if r["incomplete"] else "pass"
    return r


def run_client(args, client, client_bin, scenarios, deadline):
    """One service + fake provider per client; scenarios share the service."""
    section = {"client": client, "service": {}, "results": [], "control": {}, "failures": []}
    sf = section["failures"]
    tmp = tempfile.mkdtemp(prefix="dmz-manual-smoke-")
    sink = provider = service = None
    token = new_local_token()
    try:
        for sub in ("tmp", "codex-home", "claude-config"):
            os.makedirs(os.path.join(tmp, sub), mode=0o700)
        sink = base.EgressSink()
        threading.Thread(target=sink.serve_forever, daemon=True).start()
        protocol = "responses" if client == "codex" else "messages"
        local_auth = "bearer" if client == "codex" else "api-key"
        provider = Provider(protocol, "basic", tmp, base.CODEX_MODEL if client == "codex" else base.CLAUDE_MODEL, token)
        threading.Thread(target=provider.serve_forever, daemon=True).start()
        port = free_port()
        senv = base_env(tmp, sink.server_address[1])
        senv["DMZ_LOCAL_TOKEN"] = token
        senv["OPENAI_API_KEY" if client == "codex" else "ANTHROPIC_API_KEY"] = (
            base.FAKE_OPENAI_KEY if client == "codex" else base.FAKE_ANTHROPIC_KEY)
        service = Service(args.dmz, protocol, local_auth, port, provider.server_address[1], senv, tmp)
        if not wait_listen(port, deadline):
            sf.append("service_not_listening")
            return section
        service_base = f"http://127.0.0.1:{port}"

        # --- auth probes on the control endpoint (status codes only)
        probes = {
            "no_credential": raw_status(port, {}),
            "wrong_header_token": raw_status(port, {"X-Dmz-Token": "x" * 40}),
            "header_token": raw_status(port, {"X-Dmz-Token": token}),
            "browser_origin": raw_status(port, {"X-Dmz-Token": token, "Origin": "http://127.0.0.1"}),
        }
        if local_auth == "bearer":
            probes["bearer_token"] = raw_status(port, {"Authorization": f"Bearer {token}"})
            probes["bearer_lowercase_scheme"] = raw_status(port, {"Authorization": f"bearer {token}"})
            probes["bearer_wrong"] = raw_status(port, {"Authorization": "Bearer " + "y" * 40})
        else:
            probes["api_key_token"] = raw_status(port, {"x-api-key": token})
            probes["api_key_wrong"] = raw_status(port, {"x-api-key": "y" * 40})
            probes["bearer_in_api_key_mode"] = raw_status(port, {"Authorization": f"Bearer {token}"})
        section["control"]["auth_probe_status"] = probes
        expect_ok = {"header_token", "bearer_token", "bearer_lowercase_scheme", "api_key_token"}
        for name, code in probes.items():
            want = 200 if name in expect_ok else (403 if name == "browser_origin" else 401)
            if code != want:
                sf.append(f"auth_probe_unexpected.{name}")

        if client == "codex":
            _, codex_toml = check_config(args.dmz, tmp, port, [token], deadline)
            if codex_toml is None:
                sf.append("codex_config_unusable")
                return section
            with open(os.path.join(tmp, "codex-home", "config.toml"), "w", encoding="utf-8") as fh:
                fh.write(codex_toml)
            del codex_toml

        for scenario in scenarios:
            if deadline.remaining(1e9) < 10:
                sf.append(f"total_timeout_before.{scenario}")
                break
            workdir = os.path.join(tmp, f"work-{scenario}")
            os.makedirs(workdir, mode=0o700)
            with open(os.path.join(workdir, base.FIXTURE_NAME), "w", encoding="utf-8") as fh:
                fh.write(base.FIXTURE_TEXT)
            provider.reset_scenario(scenario, workdir)
            attempts_before = sink.attempts
            env = client_env(tmp, sink.server_address[1], client, token, service_base)
            argv = codex_argv(client_bin, scenario, workdir) if client == "codex" else claude_argv(client_bin, scenario, workdir)
            rc, out, err, to = base.run_captured(argv, env, workdir, deadline.remaining(args.scenario_timeout))
            r = evaluate(client, scenario, provider, rc, out, err, to, token, sink)
            r["egress_attempts_blocked"] = sink.attempts - attempts_before
            del out, err
            section["results"].append(r)

        # --- status counters, reset, status
        rc1, out1, _, _ = control(args.dmz, "status", port, token, tmp, deadline)
        st1 = parse_status(out1) if rc1 == 0 else None
        rc2, out2, _, _ = control(args.dmz, "reset", port, token, tmp, deadline)
        rc3, out3, _, _ = control(args.dmz, "status", port, token, tmp, deadline)
        st3 = parse_status(out3) if rc3 == 0 else None
        rc4, _, _, _ = control(args.dmz, "status", port, "x" * 40, tmp, deadline)
        leaked = any(token in o for o in (out1, out2, out3))
        del out1, out2, out3
        main_requests = sum(r["upstream"]["main_requests"] for r in section["results"])
        section["control"].update({
            "status_before_reset": st1, "reset_exit_code": rc2, "status_after_reset": st3,
            "status_with_wrong_token_exit_code": rc4, "token_in_control_output": leaked,
        })
        if st1 is None:
            sf.append("status_unavailable")
        else:
            if st1["requests"] is None or st1["requests"] < main_requests:
                sf.append("status_requests_undercounted")
            if not st1["changed_parts"]:
                sf.append("status_changed_parts_zero")
            if not st1["sessions"]:
                sf.append("status_sessions_zero_before_reset")
            if set(st1["keys"]) - {"profile", "coverage", "requests", "model_requests", "native_credential_missing", "text_parts", "changed_parts", "sessions", "warnings", "recent_warnings"}:
                sf.append("status_unexpected_fields")
        if rc2 != 0:
            sf.append("reset_failed")
        if st3 is None:
            sf.append("status_after_reset_unavailable")
        else:
            if st3["sessions"] != 0:
                sf.append("reset_did_not_clear_sessions")
            if st3["warnings"].get("mapping_reset_history_may_degrade") != 1:
                sf.append("reset_warning_not_counted")
            if st1 and st3["requests"] != st1["requests"]:
                sf.append("status_requests_changed_by_control")
        if rc4 == 0:
            sf.append("control_accepted_wrong_token")
        if leaked:
            sf.append("token_in_control_output")
    except OSError:
        sf.append("harness_os_error")
    finally:
        if service is not None:
            rc = service.stop()
            text = service.text()
            section["service"] = {
                "warnings": dict(sorted(service.warnings().items())),
                "graceful_exit_code": rc,
                "token_or_key_in_service_output": (token in text or base.FAKE_OPENAI_KEY in text
                                                   or base.FAKE_ANTHROPIC_KEY in text or bool(base.HEX64_RE.search(text))),
            }
            if section["service"]["token_or_key_in_service_output"]:
                sf.append("secret_in_service_output")
            del text
        for srv in (provider, sink):
            if srv is not None:
                srv.shutdown()
                srv.server_close()
        if sink is not None:
            section["egress_attempts_blocked_total"] = sink.attempts
        shutil.rmtree(tmp, ignore_errors=True)
    return section


def main(argv=None):
    ap = argparse.ArgumentParser(description="Synthetic loopback smoke test of the manual dmz serve/config path.")
    ap.add_argument("--dmz", required=True, help="dmz executable")
    ap.add_argument("--codex", default=shutil.which("codex"), help="codex executable (default: PATH)")
    ap.add_argument("--claude", default=shutil.which("claude"), help="claude executable (default: PATH)")
    ap.add_argument("--clients", default="codex,claude")
    ap.add_argument("--scenarios", default="basic,roundtrip")
    ap.add_argument("--total-timeout", type=float, default=420.0)
    ap.add_argument("--scenario-timeout", type=float, default=90.0)
    args = ap.parse_args(argv)
    deadline = base.Deadline(args.total_timeout)
    report = {
        "harness": PROG,
        "evidence_kind": "real-cli-binaries-via-manual-dmz-serve-with-synthetic-loopback-upstream",
        "not_evidence_of": ["desktop-application-compatibility", "provider-compatibility",
                            "account-or-auth-validation", "model-behavior", "detector-recall"],
        "versions": {"python": platform.python_version(),
                     "dmz": base.version_of([args.dmz, "version"], r"dmz local gateway [^\n]{1,80}")},
        "config": {}, "clients": [], "skipped": [],
    }
    if not report["versions"]["dmz"]:
        diag("dmz_binary_unavailable")
        report["overall"] = "fail"
        print(json.dumps(report, indent=2))
        return 2
    before = base.config_fingerprint()
    tmp = tempfile.mkdtemp(prefix="dmz-manual-config-")
    try:
        os.makedirs(os.path.join(tmp, "tmp"), mode=0o700)
        report["config"], _ = check_config(args.dmz, tmp, 43127, [new_local_token()], deadline)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    cfg_fail = []
    c, k = report["config"].get("codex", {}), report["config"].get("claude", {})
    for name in ("toml_parses", "model_provider_selected", "base_url_loopback_v1", "wire_api_responses",
                 "env_key_local_token", "requires_openai_auth_false", "websockets_disabled", "compression_disabled"):
        if not c.get(name):
            cfg_fail.append(f"config.codex.{name}")
    if not k.get("json_parses") or not k.get("base_url_loopback") or not k.get("token_placeholder_only"):
        cfg_fail.append("config.claude.fragment_invalid")
    for client, r in report["config"].items():
        if r.get("exit_code") != 0:
            cfg_fail.append(f"config.{client}.nonzero_exit")
        if r.get("secret_printed") or r.get("hex64_printed"):
            cfg_fail.append(f"config.{client}.secret_printed")
    for code in cfg_fail:
        diag(code)

    scenarios = [s for s in args.scenarios.split(",") if s in ("basic", "roundtrip")]
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
        sec = run_client(args, client, b, scenarios, deadline)
        for code in sec["failures"]:
            diag(f"{client}.{code}")
        for r in sec["results"]:
            for code in r["failures"]:
                diag(f"{client}.{r['scenario']}.{code}")
            for code in r["incomplete"]:
                diag(f"{client}.{r['scenario']}.incomplete.{code}")
        report["clients"].append(sec)
    report["normal_config_unchanged"] = before == base.config_fingerprint()
    if not report["normal_config_unchanged"]:
        diag("normal_client_config_changed")
    results = [r for s in report["clients"] for r in s["results"]]
    statuses = {r["status"] for r in results}
    failed = (cfg_fail or not results or "fail" in statuses or any(s["failures"] for s in report["clients"])
              or not report["normal_config_unchanged"])
    if failed:
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
