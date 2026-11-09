#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Linux virtual-display check of the production xterm launcher.

Uses real installed clients in print mode, fake providers and private client
homes. Xvfb/xdotool/xauth/xterm are operator-supplied TEST dependencies, never
bundled with DMZ. No real credentials or native desktop apps are exercised.
Only aggregate evidence is printed; terminal contents are neither captured
nor written to disk. A proxy sink is a test aid, not a network jail.
"""
import argparse
import ctypes
import importlib.util
import json
import os
from pathlib import Path
import secrets
import shlex
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time

HERE = Path(__file__).resolve().parent

def load(name, filename):
    spec = importlib.util.spec_from_file_location(name, HERE / filename)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module

base = load('native_base', 'agent-client-smoke.py')
common = load('native_common', 'agent-companion-smoke.py')


def run(command, env, **kw):
    return subprocess.run(command, env=env, stdin=subprocess.DEVNULL,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                          timeout=5, **kw)


def kill_group(proc):
    if proc is None:
        return
    try:
        os.killpg(proc.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    try:
        proc.wait(timeout=3)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        proc.wait(timeout=3)



def request_window_close(window, graphical):
    # Send the WM_DELETE_WINDOW protocol used by a window manager's close
    # button. xdotool windowclose destroys a window without terminating its
    # client, which is a different operation and can leave xterm running.
    class Data(ctypes.Union):
        _fields_ = [('bytes', ctypes.c_char * 20), ('shorts', ctypes.c_short * 10), ('longs', ctypes.c_long * 5)]
    class Message(ctypes.Structure):
        _fields_ = [('type', ctypes.c_int), ('serial', ctypes.c_ulong), ('send_event', ctypes.c_int),
                    ('display', ctypes.c_void_p), ('window', ctypes.c_ulong), ('message_type', ctypes.c_ulong),
                    ('format', ctypes.c_int), ('data', Data)]
    class Event(ctypes.Union):
        _fields_ = [('message', Message), ('pad', ctypes.c_long * 24)]
    x = ctypes.CDLL('libX11.so.6')
    x.XOpenDisplay.argtypes = [ctypes.c_char_p]; x.XOpenDisplay.restype = ctypes.c_void_p
    x.XInternAtom.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_int]; x.XInternAtom.restype = ctypes.c_ulong
    x.XSendEvent.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.c_int, ctypes.c_long, ctypes.POINTER(Event)]
    x.XFlush.argtypes = [ctypes.c_void_p]; x.XCloseDisplay.argtypes = [ctypes.c_void_p]
    prior = os.environ.get('XAUTHORITY')
    try:
        os.environ['XAUTHORITY'] = graphical['XAUTHORITY']
        display = x.XOpenDisplay(graphical['DISPLAY'].encode())
    finally:
        if prior is None: os.environ.pop('XAUTHORITY', None)
        else: os.environ['XAUTHORITY'] = prior
    if not display: return False
    try:
        event = Event(); event.message.type = 33; event.message.send_event = 1
        event.message.display = display; event.message.window = int(window)
        event.message.message_type = x.XInternAtom(display, b'WM_PROTOCOLS', 0)
        event.message.format = 32
        event.message.data.longs[0] = x.XInternAtom(display, b'WM_DELETE_WINDOW', 0)
        ok = x.XSendEvent(display, int(window), 0, 0, ctypes.byref(event))
        x.XFlush(display)
        return bool(ok)
    finally:
        x.XCloseDisplay(display)


def scenario(args, client, kind, graphical):
    failures = []
    result = {'client': client, 'scenario': kind, 'status': 'fail', 'failures': failures}
    root = Path(tempfile.mkdtemp(prefix='dmz-native-terminal-'))
    host = provider = sink = None
    marker = secrets.token_hex(12)
    child_pids = lambda: [p for p in common.marked_processes(marker) if host is None or p != host.pid]
    try:
        for name in ('tmp', 'home', 'codex-home', 'claude-config', 'private'):
            (root / name).mkdir(mode=0o700)
        project = root / "Synthetic project ' $() Í"
        project.mkdir(mode=0o700)
        dmz = root / "DMZ ' $() Í"
        shutil.copy2(args.dmz, dmz)
        sink = base.EgressSink()
        threading.Thread(target=sink.serve_forever, daemon=True).start()
        provider = common.make_provider(client, 'basic' if kind in ('basic', 'window') else 'stop', str(project))
        threading.Thread(target=provider.serve_forever, daemon=True).start()
        binary = shutil.which(client)
        if not binary:
            result['status'] = 'incomplete'
            result['reason'] = 'client_not_installed'
            return result
        env = base.child_env(str(root), sink.server_address[1], client)
        for key in ('OPENAI_API_KEY', 'ANTHROPIC_API_KEY', 'ANTHROPIC_AUTH_TOKEN', 'CLAUDE_CODE_OAUTH_TOKEN'):
            env.pop(key, None)
        env.update(graphical)
        env.update({'HOME': str(root / 'home'), 'DMZ_SMOKE_RUN': marker, 'TERM': 'xterm-256color'})
        client_args = base.client_argv('/x', client, binary, provider.server_address[1], 'basic', str(project))[8:]
        descriptor = root / 'private' / 'control.json'
        if kind == 'window':
            env['XDG_CACHE_HOME'] = str(root / 'cache')
            descriptor = root / 'cache' / 'dmz-companion' / 'instance.json'
            descriptor.parent.mkdir(parents=True, mode=0o700)
            browser_profile = root / 'browser-profile'
            browser_profile.mkdir(mode=0o700)
            wrapper = root / 'native-browser'
            command = [args.chromium, '--no-sandbox', '--disable-gpu', '--disable-background-networking',
                       '--disable-component-update', '--disable-sync', '--no-first-run', '--no-default-browser-check',
                       '--user-data-dir='+str(browser_profile), '--remote-debugging-port=0',
                       '--proxy-server=http://127.0.0.1:'+str(sink.server_address[1]),
                       '--proxy-bypass-list=127.0.0.1;localhost']
            wrapper.write_text('#!/bin/sh\nexec '+ ' '.join(shlex.quote(v) for v in command) + ' "$@"\n')
            wrapper.chmod(0o700)
            env['BROWSER'] = str(wrapper)
        cfg = {'executable': str(dmz), 'control_file': str(descriptor), 'client': client,
               'client_path': binary, 'arguments': client_args,
               'upstream': f'http://127.0.0.1:{provider.server_address[1]}/v1', 'open_window': kind == 'window'}
        config = root / 'private' / 'fixture.json'
        config.write_text(json.dumps(cfg)); config.chmod(0o600)
        host = subprocess.Popen([args.testhost, '--config', str(config)], env=env,
                                stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                                stderr=subprocess.DEVNULL, start_new_session=True)
        if not common.wait_until(lambda: descriptor.exists() and descriptor.stat().st_size, 10):
            failures.append('native_host_not_ready')
            return result
        control = json.loads(descriptor.read_text())
        origin, token = control['origin'], control['token']
        key = base.FAKE_OPENAI_KEY if client == 'codex' else base.FAKE_ANTHROPIC_KEY
        if kind == 'window':
            driver = [shutil.which('node'), str(HERE/'agent-native-window-driver.mjs'), '--playwright', args.playwright,
                      '--profile', str(browser_profile), '--origin', origin, '--client', client, '--project', str(project)]
            window_run = subprocess.run(driver, env=env, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                        stderr=subprocess.PIPE, timeout=45)
            result['browser_checks'] = (common.jbody(window_run.stdout) or {}).get('checks', 0)
            if window_run.returncode:
                failures.append('native_browser_flow_failed')
                return result
            sessions = (common.jbody(common.http(origin, 'GET', '/api/state', token=token)[1]) or {}).get('sessions', [])
            sid = sessions[0].get('id') if sessions else None
            # The normal no-argument app must reopen this isolated host,
            # retaining its private descriptor and aggregate session history.
            (browser_profile/'DevToolsActivePort').unlink(missing_ok=True)
            reopen = subprocess.run([str(dmz)], env=env, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                    stderr=subprocess.PIPE, timeout=10)
            if reopen.returncode or json.loads(descriptor.read_text()) != control:
                failures.append('native_window_reopen_created_new_host')
                return result
            window_run = subprocess.run(driver+['--reopen'], env=env, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                        stderr=subprocess.PIPE, timeout=45)
            result['reopen_browser_checks'] = (common.jbody(window_run.stdout) or {}).get('checks', 0)
            if window_run.returncode:
                failures.append('native_browser_reopen_failed')
                return result
        else:
            status, raw, _ = common.http(origin, 'POST', '/api/start', token=token,
                                        body={'client': client, 'project': str(project), 'auth': 'api', 'api_key': key})
            started = common.jbody(raw) or {}
            sid = started.get('id')
            if status != 200 or not sid:
                failures.append('native_terminal_start_failed')
                return result
        def view():
            state = common.jbody(common.http(origin, 'GET', '/api/state', token=token)[1]) or {}
            return next((s for s in state.get('sessions', []) if s.get('id') == sid), {})
        reached = common.wait_until(lambda: provider.obs.main_requests > 0 if kind in ('basic', 'window') else provider.held > 0, 30)
        result['model_request_observed'] = bool(reached)
        if not reached:
            failures.append('native_client_model_request_missing')
            return result
        if kind not in ('basic', 'window'):
            windows = run([args.xdotool, 'search', '--onlyvisible', '--class', 'XTerm'], graphical)
            ids = [i for i in windows.stdout.decode().split() if i.isdigit()]
            result['terminal_windows_observed'] = len(ids)
            if not ids:
                failures.append('native_terminal_window_missing')
            elif kind == 'close-window':
                if not request_window_close(ids[0], graphical):
                    failures.append('native_terminal_close_failed')
            else:
                common.http(origin, 'POST', '/api/stop', token=token, body={'id': sid})
        final = common.wait_until(lambda: (s if (s := view()).get('state') == 'stopped' else None), 25)
        if not final:
            failures.append('native_terminal_session_not_stopped')
        else:
            result['final_requests'] = final.get('requests')
            result['final_changed_parts'] = final.get('changed_parts')
            result['issue'] = final.get('issue')
            if final.get('issue'):
                failures.append('unexpected_native_terminal_issue')
        provider.release.set()
        clean = common.wait_until(lambda: not child_pids(), 10)
        result['native_children_clean'] = bool(clean)
        if not clean:
            failures.append('native_terminal_children_left')
        obs = provider.obs
        result.update(original_email_upstream=obs.original_anywhere,
                      original_email_in_eligible=obs.original_in_eligible,
                      marker_requests=obs.marker_in_eligible,
                      local_header_upstream=obs.dmz_header_requests,
                      other_credentials_upstream=obs.other_credential_requests,
                      blocked_background_attempts=sink.attempts)
        if obs.original_in_eligible or obs.dmz_header_requests or obs.other_credential_requests:
            failures.append('native_request_boundary_failed')
        if not obs.marker_in_eligible:
            failures.append('native_masking_not_observed')
        common.http(origin, 'POST', '/api/quit', token=token, body={})
        host.wait(timeout=10)
        result['ticket_dirs_left'] = len(list((root / 'tmp').glob('dmz-session-*')))
        result['launch_settings_left'] = len(list((root / 'tmp').glob('dmz-launch-*')))
        if result['ticket_dirs_left'] or result['launch_settings_left']:
            failures.append('native_session_files_left')
        if not failures:
            result['status'] = 'pass'
    except (OSError, ValueError, subprocess.SubprocessError):
        failures.append('native_fixture_error')
    finally:
        if provider:
            provider.release.set()
        kill_group(host)
        for pid in child_pids():
            try:
                os.kill(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        if provider:
            provider.shutdown(); provider.server_close()
        if sink:
            sink.shutdown(); sink.server_close()
        shutil.rmtree(root, ignore_errors=True)
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--dmz', required=True)
    parser.add_argument('--testhost', required=True)
    parser.add_argument('--x11-root', default='/')
    parser.add_argument('--clients', default='codex,claude')
    parser.add_argument('--playwright')
    parser.add_argument('--chromium')
    parser.add_argument('--scenarios', default='basic,stop,close-window')
    args = parser.parse_args()
    if any(c not in ('codex','claude') for c in args.clients.split(',')) or any(s not in ('basic','stop','close-window','window') for s in args.scenarios.split(',')):
        parser.error('unsupported native fixture scenario')
    if 'window' in args.scenarios.split(',') and (not args.playwright or not args.chromium):
        parser.error('window scenario requires --playwright and --chromium')
    args.dmz, args.testhost = map(os.path.abspath, (args.dmz, args.testhost))
    tools = Path(args.x11_root) / 'usr' / 'bin'
    args.xdotool = str(tools / 'xdotool')
    root = Path(tempfile.mkdtemp(prefix='dmz-x11-session-'))
    server = None
    try:
        display = next((f':{n}' for n in range(190, 240) if not Path(f'/tmp/.X11-unix/X{n}').exists() and not Path(f'/tmp/.X{n}-lock').exists()), None)
        if not display:
            raise OSError('no_display')
        auth = root / 'authority'; auth.touch(mode=0o600)
        graphical = {'PATH': str(tools) + ':' + os.environ.get('PATH', '/usr/bin:/bin'),
                     'DISPLAY': display, 'XAUTHORITY': str(auth), 'LANG': 'C.UTF-8',
                     'LD_LIBRARY_PATH': str(Path(args.x11_root) / 'usr/lib/x86_64-linux-gnu')}
        result = subprocess.run([str(tools / 'xauth'), '-f', str(auth), 'source', '-'], env=graphical,
                                input='add ' + display + ' . ' + secrets.token_hex(16) + '\n', text=True,
                                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=5)
        if result.returncode:
            raise OSError('auth_failed')
        command = [str(tools / 'Xvfb'), display, '-screen', '0', '1280x1000x24', '-nolisten', 'tcp',
                   '-auth', str(auth), '-fp', str(Path(args.x11_root) / 'usr/share/fonts/X11/misc'),
                   '-extension', 'GLX', '-noreset']
        if not Path('/usr/bin/xkbcomp').exists() and (tools / 'proot').exists():
            command = [str(tools / 'proot'), '-b', str(tools / 'xkbcomp') + ':/usr/bin/xkbcomp'] + command
        server = subprocess.Popen(command, env=graphical, stdin=subprocess.DEVNULL,
                                  stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
        if not common.wait_until(lambda: run([args.xdotool, 'getdisplaygeometry'], graphical).returncode == 0, 8):
            raise OSError('x11_failed')
        results = [scenario(args, client, kind, graphical) for client in args.clients.split(',')
                   for kind in args.scenarios.split(',')]
        print(json.dumps({'evidence_kind': 'linux_xvfb_production_xterm_launcher_synthetic',
                          'native_desktop_apps_qualified': False, 'os_download_flow_qualified': False,
                          'results': results}, sort_keys=True))
        return 1 if any(r['status'] == 'fail' for r in results) else 3 if any(r['status'] == 'incomplete' for r in results) else 0
    except (OSError, ValueError, subprocess.SubprocessError):
        print('native_terminal_dependencies_unavailable', file=sys.stderr)
        return 2
    finally:
        kill_group(server)
        shutil.rmtree(root, ignore_errors=True)

if __name__ == '__main__':
    sys.exit(main())
