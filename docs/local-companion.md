# Local DMZ companion app — private prototype

The companion opens a small device-local setup window and launches the installed Codex CLI or Claude Code in a terminal. It is an experimental companion to the Chromium extension, using the existing **basic** detector. It is not a qualified connection to the Codex desktop app or Claude Desktop, and it is not a public download or signed release.

The intended setup has six core actions with an existing sign-in, or seven with an entered API key. That is a design target, not a measured installation guarantee. Native OS trust prompts, moving a macOS app, client onboarding, sign-in and project-trust screens can add actions. A new Claude project currently requires its own trust confirmation; DMZ does not pre-approve it. Download-to-first-answer qualification remains open.

## Intended user flow

Prerequisite: the terminal tool is already installed and works with the user's normal setup. A graphical desktop, browser and terminal are needed.

1. Download the private review package for the computer.
2. Extract it.
3. Open DMZ. On macOS, first move DMZ.app to Applications; this adds an action to the private prototype flow.
4. Choose the installed tool in the local DMZ window.
5. Choose the project folder.
6. Optionally enter an API key. Otherwise use the tool's existing account sign-in. An API key inherited by DMZ is used only when the user selects its explicit checkbox; API charges are separate.
7. Press **Start with DMZ** and continue in the terminal.

Existing-sign-in users skip action 6. Users do not copy a local token, edit a routing file, run a setup command or create a DMZ account. The window offers Stop for each session and Quit DMZ for the app. Closing the browser window leaves the host running; reopening DMZ reopens its window. This prototype has no macOS Dock icon.

The tool can still ask for sign-in, onboarding or project trust. Confirm project trust only when appropriate. A failed launch has a visible issue in DMZ; an interactive terminal error stays visible until Enter or a 30-second timeout. Install missing terminal clients through their normal trusted installation process; DMZ does not install or update them.

## What the app does

- Discovers terminal executables from PATH and common installation locations, without reading client authentication stores.
- Opens a loopback browser window with bundled assets, strict content policy, no trackers and no external UI resources. Icelandic is the default, with an English toggle.
- Exchanges a one-use, one-minute window code for a control credential. The code is removed from the address bar before the exchange. The browser keeps the control credential in tab session storage until Quit. A private cache descriptor lets subsequent app launches reopen the same host; it contains a local UI credential, never a provider key, prompt or mapping.
- Creates a separate authenticated loopback gateway and mapping scope for each launch. A pasted API key lives in the parent gateway's memory. Its private launch ticket contains scoped local credentials and launch metadata only; the connector consumes it once, and its server-side claim cannot be replayed.
- Pins temporary routing above ordinary client settings, retaining the tool's own model and permission defaults. DMZ does not rewrite the user's normal settings or authentication store. Native clients can update their own history and settings as usual.
- Launches Claude API mode with a local bearer credential, avoiding repeated approval and persistence of the changing local token as a custom API key. The actual provider key is substituted by the gateway.
- Shows aggregate request/masking counts and coverage warnings. Stopped sessions retain aggregate counts while their gateway, provider-key owner and mapping references are released. Recent history is bounded.
- Handles terminal hangup on Linux/macOS and monitors connector heartbeats. A running connector that disappears for about 15 seconds is stopped by the app. A terminal that never claims its ticket is reported unavailable after 90 seconds.

Stop closes that launch's gateway and asks the connector to stop its client. Quit stops all launches. Linux/macOS pipe-mode cleanup includes owned process groups; native Windows process-tree behavior remains unqualified. The app waits briefly for connectors to collect final aggregate counts before exiting. No memory-erasure, swap or crash-dump guarantee is implied.

## Coverage and connection limits

Only supported user-message text fields pass through the basic detector. Instructions, tool inputs, tool results, repository files returned by tools, assistant messages, attachments and other network routes remain outside masking. Those paths can send original values. Failures and capacity limits warn and continue, including unchanged forwarding. Misses are possible, responses retain markers, and no local restoration is implemented here. A masked filename can break an operation. See the [gateway guide](local-agent-gateway.md) for protocol and resource limits.

Mappings stay in memory and are lost on exit, reset or eviction. Resuming history or automatic compaction can leave old markers unresolved or trigger conservative whole-field masking. Native client history can contain originals. Background updates, authentication, direct MCP, shell networking, web tools, plugins/apps and cloud/remote sessions can bypass this gateway. A synthetic proxy sink is a test aid, not a system-wide network firewall.

API keys, cached account sign-ins and subscription access are distinct routes. Synthetic account fixtures show local routing with native client authentication, not actual account entitlement, refresh, provider acceptance or plan compatibility. Alternate corporate gateways and managed-policy routing are unqualified. DMZ does not change approvals or weaken the client's sandbox.

## Development and review

Build the app with a compatible Go toolchain; no additional runtime service or GUI framework is required:

```sh
go build -o /tmp/DMZ ./cmd/dmz
/tmp/DMZ
```

Without a graphical session, use the development-only `app --no-open --control-file /private/new/path.json` mode for automated checks. The descriptor is a credential, not a user setup instruction. Do not include it in reports.

Run the app-controller/client fixture harness:

```sh
go build -o /tmp/dmz-companion ./cmd/dmz
go build -o /tmp/dmz-companion-testhost ./internal/companion/testhost
python3 -I scripts/agent-companion-smoke.py \
  --dmz /tmp/dmz-companion --testhost /tmp/dmz-companion-testhost
```

The test host replaces terminal launch with a direct owned child and uses fake loopback providers, isolated client directories and synthetic keys/files. Its optional pseudo-terminal scenarios exercise the installed client TUIs; they do not qualify native window opening, download prompts or desktop-app integration. The normal Codex sandbox cannot run in this container, so its tool roundtrip remains incomplete.

Browser checks require operator-supplied development-only Playwright and Chromium paths. They add no application dependency:

```sh
node scripts/agent-companion-ui-smoke.mjs --dmz /tmp/dmz-companion \
  --playwright /path/to/playwright-core --browser /path/to/chromium
```

Create private review archives with notices, source identity and checksums:

```sh
GO_BIN=/path/to/go scripts/package-companion.sh /tmp/new-empty-dmz-packages
```

The package script creates macOS arm64/amd64 app bundles, a Windows amd64 GUI executable and a Linux amd64 launcher archive. Cross-compilation and archive checks do not establish native runtime correctness. These packages are unsigned, unnotarized and unpublished. Public distribution requires the repository's history, rights, source-availability and release checks as well as native signing and install qualification. The existing extension's distribution approval does not qualify this companion.
