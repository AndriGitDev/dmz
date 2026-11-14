# Desktop app connection plan: Codex and Claude Desktop

Research date: 9 November 2026, sources fetched 13:24–13:30 UTC. Status: **desktop acceptance unqualified**. Background Codex/Claude Code CLI routing and configuration helpers are implemented after the research below; no native vendor desktop connection is verified. This document does not change the [gateway guide](local-agent-gateway.md), the [companion guide](local-companion.md) or any public claim.

The companion currently launches the **terminal** clients (Codex CLI 0.166.0, Claude Code 2.1.318 on this host). Opening a terminal client from a graphical window is not desktop-app support, and this plan does not describe it as such. Native desktop support below means the vendor's own desktop application sends its model requests to a local dmz gateway.

dmz remains an experimental, best-effort companion to the GPL-3.0-only Chromium extension. Source licensing is separate from repository visibility, asset rights and binary distribution. Nothing here creates or announces a public download.

## Verdict

Both native apps have a **documented** custom-endpoint mechanism that a local dmz gateway can plausibly serve. Neither has been run against dmz. Each has a material limit:

| | Codex in the desktop app | Claude Desktop |
| --- | --- | --- |
| Documented route | Custom model provider in `~/.codex/config.toml`, read by the macOS and Windows apps after a restart | Third-party (3P) inference mode, provider `gateway`, configured in the app's in-app configuration window |
| Existing subscription sign-in through dmz | Documented for gateways: `requires_openai_auth = true`, with the gateway forwarding `Authorization` and `ChatGPT-Account-ID` to `chatgpt.com/backend-api/codex`. Documented in enterprise/workspace pages; **plan eligibility outside workspaces is unverified** | **Not possible as documented.** 3P mode replaces Anthropic account sign-in with local device identity. dmz would hold an API key; subscription use is outside this route |
| API key route | Documented `[model_providers.<id>.auth]` command helper (bearer) | Documented `inferenceCredentialHelper` (bearer) |
| Scope of routed traffic | Codex model-provider requests only. ChatGPT Chat/Work, voice, sign-in, MCP and plugins use other paths | All three tabs (Chat, Cowork, Code) use the configured provider; telemetry, updates, downloads, MCP and web tools use other paths |
| Settings change | Edits the user's `config.toml`, which the CLI and IDE extension also read | App-owned local configuration, written by the app itself after the user imports a file dmz generates |
| Restore | Back up the file, detect drift, restore | Choose the Anthropic sign-in option at the app's sign-in screen; the 3P configuration stays saved but inactive |

Recommended next integration: implement a **persistent desktop listener** in the existing companion host, plus a **dmz credential helper** used by both apps, then qualify Claude Desktop's API-key route and Codex desktop's API-key route on native macOS/Windows before attempting the subscription route. Desktop qualification cannot happen on this host (see [Blockers](#blockers-on-this-host)).

## Evidence labels

- **Documented**: stated in an official vendor page listed under [Sources](#sources).
- **Inferred**: follows from documented behavior or local code but is not stated for this exact case.
- **Unverified**: must be measured on a native device before any claim.
- **Local**: implemented and tested in this repository on the development host.

## Current local implementation state (Local)

- Gateway routes: Responses (`/v1/responses`, `/responses`, compaction, models) for `api.openai.com/v1` or `chatgpt.com/backend-api/codex`; Messages, token counting and models for the Anthropic API. Fixed upstreams only; redirects rejected.
- Local auth modes: `header` (`X-Dmz-Token` alongside client auth), `bearer` and `api-key` (local credential replaced by the in-memory provider key).
- Only supported user-text fields are masked; instructions, tool inputs/results, attachments and assistant text pass unchanged. Failures warn and continue. No marker restoration.
- **Updated after research:** bounded gzip/x-gzip and zstd decoding is implemented and unit/race tested. Both wire and decoded JSON use the 8 MiB default limit. Unsupported/malformed/oversized encodings still warn and pass unchanged. The CLI keeps compression disabled; actual desktop defaults remain unqualified.
- Claude mapping scope is keyed by `X-Claude-Code-Session-Id`; a missing identity warns and forwards unchanged. Codex has one shared scope per launch.
- Companion: loopback UI, one-use window code, per-launch ephemeral gateway, private launch tickets, in-memory API keys, explicit checkbox for an inherited environment key, Stop/Quit, heartbeat cleanup.
- `dmz config` prints desktop fragments only. No desktop configuration is installed. Fragments assume a long-lived manual `serve` process and a token copied by hand, which is not the target user flow.

## Claude Desktop

### Documented facts

- 3P mode routes **all** model inference to the configured provider. The app's web UI is bundled and conversations are stored locally. User identity is a local random device ID rather than an Anthropic account, unless the deployment is managed from the claude.ai admin console. [overview]
- Gateway provider requirements: `POST /v1/messages` with streaming and tool use; `GET /v1/models` is optional. The gateway should preserve `cache_control`, return provider `usage` unchanged, forward provider error status/body unchanged, and stream SSE pings without buffering. [gateway]
- Credential options for a gateway: static key `inferenceGatewayApiKey`, OIDC sign-in, or `inferenceCredentialHelper`. Auth scheme is `bearer` (default) or `x-api-key`. [gateway]
- A credential helper is an executable that prints one bare token to stdout. It receives `CLAUDE_HELPER_CONTEXT`, its output is cached for `inferenceCredentialHelperTtlSec` (default 3600), and it is re-run mid-session when the provider rejects the credential (unless that is disabled). A non-zero exit is shown to the user as a refresh failure. [credential-helper]
- Single-machine setup: enable Developer Mode (Help → Troubleshooting), open Developer → Configure Third-Party Inference, enter values, then Apply Changes → Save & Restart. On relaunch, the sign-in screen offers to start in 3P mode. "To return to standard Claude Desktop, choose the Anthropic sign-in option on the sign-in screen instead." [installation]
- The in-app window writes the per-user `configLibrary` directory (`~/Library/Application Support/Claude-3p/configLibrary/` on macOS; `%LOCALAPPDATA%\Claude-3p\configLibrary\` on Windows). API keys saved there are encrypted with OS secure storage. Any managed source that sets non-app-behavior keys takes precedence and makes local values ignored. The window can import a configuration file. [configuration-format, data-storage, in-app-configuration]
- Configuration takes effect at launch. If the gateway is unreachable at startup the app shows "Gateway was unreachable". [configuration-format, Claude Code gateway connect]
- Code sessions use the embedded Claude Code engine. Provider and credential keys are passed to it as environment/launch options that user settings cannot override. Chat and Cowork sessions also run through Claude Code. [code, gateway troubleshooting]
- Claude Code gateway traffic carries `x-claude-code-session-id`, plus agent, request-class and compaction headers. [gateway protocol]
- On gateway deployments, experimental beta features and tool search are off by default. [gateway]
- A gateway at `localhost` on the user's computer does not work for SSH sessions. [Claude Code gateway connect]
- The documented system requirements and installers cover macOS 14+ (`.dmg`) and Windows 10 19041+ (`.msix`). [installation]

### Inferred

- A loopback `http://127.0.0.1:<port>` base URL is accepted for local (non-SSH) sessions. The docs explicitly reject it only for SSH sessions. They show only HTTPS examples, so **acceptance by the in-app validator is unverified**.
- The embedded engine in Code sessions sends `x-claude-code-session-id`, so existing per-session scoping applies. Whether Chat and Cowork requests carry it is **unverified**. If they do not, dmz currently forwards them **unchanged** with a warning, which would leave Chat unmasked.
- Deterministic in-scope masking should keep cached prefixes stable because earlier turns produce the same markers. Cache hit rates through dmz are unverified.
- Whether a Cowork VM's inference reaches a host-loopback gateway is unverified. The docs say inference and telemetry connections run outside the Code sandbox, but they do not say this for Cowork's VM.

### Proposed approach (not implemented)

1. **Persistent desktop listener.** While the companion host runs, it keeps one authenticated loopback Messages gateway on a stable, per-install port recorded in dmz's private state directory. It uses bearer local auth and an in-memory Anthropic API key, and scopes mappings by session header as today. A changing port would invalidate the saved base URL.
2. **Credential helper.** A `dmz desktop-credential claude` subcommand reads dmz's owner-only state and prints the current local token. It exits non-zero when the host is not running. No provider key, prompt or mapping is printed or stored. Restarting dmz rotates the token; the app's documented mid-session re-run recovers it. No static secret is placed in the app's configuration.
3. **Generated import file.** dmz writes a JSON configuration (owner-only, in its own state directory) containing only `inferenceProvider: "gateway"`, `inferenceGatewayBaseUrl`, `inferenceGatewayAuthScheme: "bearer"`, `inferenceCredentialHelper` (absolute dmz path) and its args. Optionally it adds `inferenceModels` if discovery through dmz fails. The user imports it in the app's window. dmz does **not** write `configLibrary` itself, because the `_meta.json`/`<id>.json` format and key encryption are app-internal.
4. **Preconditions.** Before generating, dmz checks read-only for the documented managed sources: the macOS managed-preferences plists, `HKLM`/`HKCU\SOFTWARE\Policies\Claude`, or `/etc/claude-desktop/managed-settings.json`. If one is present, dmz stops with an explanation instead of chaining or overriding. It does not read app credential stores.
5. **Restore.** The documented path is the Anthropic sign-in option. dmz's Disconnect stops the listener and tells the user to do this. The saved 3P configuration remains in the app and fails closed ("Gateway was unreachable") if selected without dmz. It never silently falls back to a direct provider.
6. **Permissions.** The generated file sets no permission, sandbox, Auto mode, egress or folder keys, so the app's defaults and the user's choices are unchanged. `inferenceGatewayAutoModeServerChecks` stays unset because it would send session details (paths, user name) in request bodies.

Target user actions, with Claude Desktop installed and not signed in to an Anthropic account:

1. Download and extract dmz.
2. Open dmz, choose **Claude Desktop**, and enter an API key (or tick the inherited-environment-key checkbox).
3. Press **Prepare Claude Desktop**. dmz saves the import file and shows the next steps.
4. In Claude Desktop: Help → Troubleshooting → Enable Developer Mode.
5. Developer → Configure Third-Party Inference → Import configuration → select the dmz file.
6. Apply Changes → Save & Restart.
7. On the sign-in screen, choose the third-party option, then open the Code tab.

That is seven actions. It is a design target, not measured. Moving a macOS app, OS trust prompts, an existing Anthropic sign-in that must first be signed out (unverified) and Developer-mode re-enablement after relaunch (documented quirk) can add actions. dmz must be running whenever Claude Desktop is used in this mode.

## Codex in the desktop app

### Documented facts

- Codex runs inside the ChatGPT desktop app for macOS, Windows and Linux (the Linux app is a preview for Ubuntu 24.04/26.04, Debian 13, Fedora 43/44 and Arch, x64/ARM64). [app, linux-app]
- The macOS desktop configuration comes from `~/.codex/config.toml`; Windows uses `%USERPROFILE%\.codex\config.toml`. Restart the app to apply provider changes. Credentials supplied by environment variables must reach the desktop process; setting them in a shell does not establish that reachability. [connect-to-a-gateway]
- `model_provider` must be a top-level key before the first TOML table. General configuration precedence has higher-priority CLI/project/profile layers, but the configuration reference explicitly excludes provider/auth routing keys from project-local overrides. User-level provider settings and managed requirements still need separate compatibility checks. [connect-to-a-gateway, config-basic]
- Provider keys: `base_url`, `wire_api = "responses"`, `requires_openai_auth`, `env_key`, `env_http_headers`, `http_headers` (static), `auth.command`/`args`/`timeout_ms`/`refresh_interval_ms`, `supports_websockets`. `auth` must not be combined with `env_key` or `requires_openai_auth`. `features.enable_request_compression` (zstd) is on by default. [config-reference]
- ChatGPT sign-in through a gateway: set `requires_openai_auth = true` with `base_url` at the gateway. Send the gateway credential in a custom header (`env_http_headers`) or cookie, reserving `Authorization` for the OpenAI credential. The gateway proxies `/models` and `/responses` to `https://chatgpt.com/backend-api/codex`, forwarding `Authorization` and any `ChatGPT-Account-ID` unchanged. With ChatGPT sign-in, a command helper cannot be used for the gateway credential; it must be passed through a custom header instead. [connect-to-a-gateway]
- `CODEX_HOME` is documented for the CLI, IDE extension, app-server and installers. **The desktop app is not listed.** [environment-variables]
- Desktop deployments bundle a CLI; a model must be recognized by that bundled version. [gateway-compatibility]
- Only model-provider requests use the gateway route. Voice, workspace sign-in, MCP and plugins can use other paths. [connect-to-a-gateway]

### Inferred or unverified

- No documented private-profile mechanism exists for the desktop app. Process-only overrides, which the CLI launcher uses, are unavailable. A desktop connection therefore **must change the user's `config.toml`**, which also redirects the CLI and IDE extension for as long as it is in place (Inferred).
- dmz's `--auth chatgpt` route matches the documented forwarding shape but has **not** been run with a real account. Whether the subscription route through a non-workspace local gateway is permitted or functional is unverified.
- Desktop requests carry no documented per-chat identity header. Concurrent desktop chats would share one dmz mapping scope. This must be disclosed or replaced by a verified identity source; it is not established.
- The desktop app may prefer WebSocket transport. The documented `supports_websockets = false` should force HTTP; that is unverified on desktop builds.

### Proposed approach (not implemented)

1. **Persistent Responses listener** on a stable per-install port, as for Claude, with one of two auth modes chosen in the dmz window:
   - **API key** (first qualification target): `requires_openai_auth = false` with `[model_providers.dmz_local.auth] command = <dmz path>`, `args = ["desktop-credential", "codex"]`. The helper prints the current bearer token and dmz substitutes the in-memory `OPENAI_API_KEY`. No secret or environment dependency is placed in TOML.
   - **Existing sign-in** (second target, after the API route passes): `requires_openai_auth = true` and upstream `chatgpt.com/backend-api/codex`. The local credential travels in `X-Dmz-Token`. Because a helper is not allowed with ChatGPT sign-in and app environments are unreliable, the documented options are `env_http_headers` (environment reachability unverified) or a static `http_headers` per-install token. The static header goes against the docs' general advice but guards a listener that holds no provider secret in this mode. **This is an owner decision.**
2. **Compression.** Bounded gateway zstd/gzip decoding is now implemented with pinned dependency notices. The proposed desktop writer no longer needs to alter `[features]` just to enable masking. Verify actual desktop frame/window sizes and compression failure warnings before qualification; the CLI launcher still disables compression.
3. **Reversible edit with drift detection.** On Connect, dmz:
   - parses the whole file;
   - refuses if a project/system/managed layer or an existing custom gateway provider is active, rather than chaining;
   - copies the original to an owner-only backup and records hashes of the original and written files;
   - inserts or replaces only the top-level `model_provider` plus its own `[model_providers.dmz_local]` table (and the compression key if required);
   - writes atomically with the original file mode;
   - re-parses the result.

   On Disconnect, if the current hash equals the written hash, dmz restores the backup byte for byte. If the file drifted, dmz removes only its own keys, restoring any replaced `model_provider` value, and shows what remains. It never blindly overwrites. The companion's start and Quit also report drift.
4. **Permissions.** dmz writes no approval, sandbox, permission-profile, model, MCP or plugin key. The user's model choice is kept.
5. **Failure mode.** While connected, if dmz is not running, Codex requests fail with connection errors. There is no direct fallback. The dmz window shows "connected, gateway stopped" with a one-click restore.

Target user actions, with the desktop app already installed and signed in (or an API key ready):

1. Download and extract dmz.
2. Open dmz and choose **Codex desktop app**.
3. Choose *existing sign-in* or enter an API key.
4. Press **Connect** and review the shown change summary.
5. Quit and reopen the desktop app.
6. Start a new Codex chat.

That is six actions, seven with a key; a design target, not measured.

## Shared design constraints

- Originals and mappings stay in dmz process memory. The import/backup/state files hold no prompts, mappings or provider keys. Desktop app histories and configuration can still contain originals.
- Host-page and app-side visibility is unchanged: the app sees the draft before dmz. Only supported user-text fields are masked; tool results, file contents and attachments pass unchanged. Failures warn and continue. Responses keep markers.
- The local token is readable by same-user processes via the helper. This is not a defense against local malware.
- API-key persistence across dmz restarts (OS keychain) is new scope and an owner decision. Today a key must be re-entered after each dmz start.
- No anonymity, complete detection, compliance or provider-retention claim.

## Blockers on this host

1. No user graphical session or vendor desktop app was installed at research time. An isolated Xvfb display was subsequently added for companion browser/xterm fixtures; that does not install or qualify either vendor desktop app.
2. Claude Desktop's documented installers are macOS and Windows only. macOS and Windows hosts are unavailable here.
3. The ChatGPT desktop app has a Linux preview for Ubuntu 24.04 x64, which matches this host (Ubuntu 24.04.5, x86_64). Its documented installation path needs system installation, graphics and account sign-in. No vendor app or test account is available here; the research delegate used read-only documentation and changed no real settings.
4. No real provider credentials or accounts were supplied to these fixtures. Neither the API-key nor the subscription route has been exercised end to end against a provider. Synthetic fake upstreams can test dmz's side but not app acceptance of a loopback URL, helper behavior, headers or restore.
5. Native Windows process-tree handling and macOS signing/notarization remain open from the companion work.

Therefore no verified desktop connection can be produced in this session. Implementing the listener, helper and config writer before a native qualification run would be speculative code, so none was added.

## Operator acceptance checklist (native macOS or Windows)

Use fabricated data only, for example `Draft a reply to pilot@example.test` and a fabricated kennitala with century digit `8`. Record app version, bundled CLI version (Codex), OS build, dmz source hash, auth route and step count. Do not record prompts, tokens or request bodies.

Before:
1. Record hashes of `~/.codex/config.toml` (Codex) or a redacted diagnostic report's `managed-config.txt` and `deployment-mode.txt` (Claude: Help → Troubleshooting → Generate Diagnostic Report).
2. Confirm that no managed profile/requirements are present, or that dmz refuses with a clear message when one is.

Connect and first answer:
3. Follow the target user actions; count every click/prompt from download to first answer.
4. A send of the fabricated prompt succeeds. dmz status shows a model-route request and a masked part, and the provider (fake or real) receives a marker instead of the email.
5. Streaming arrives incrementally. Model picker or discovery works. A Read/edit tool roundtrip completes with the client's normal approvals and sandbox.
6. Claude: two concurrent Code sessions get separate scopes (status shows two scopes). Check whether Chat/Cowork requests carry a session header, and record `missing_session_identity_forwarded_unchanged` counts. Codex: record that concurrent chats share one scope.
7. Manual compaction (`/compact` or equivalent) completes, or its failure is recorded.

Failure and recovery:
8. Stop dmz. The app shows a connection/unreachable error and nothing reaches the provider directly (verify with the fake upstream or gateway logs).
9. Restart dmz (new token). Claude: the helper's mid-session re-run recovers. Codex (API route): the helper refresh recovers. Record any manual step.
10. Disconnect. Codex: the file hash equals the recorded original. Claude: after choosing the Anthropic sign-in, `deployment-mode.txt` shows standard mode.
11. Drift test (Codex): edit an unrelated key while connected, then Disconnect. dmz keeps the edit, removes only its keys, and reports the drift.
12. Confirm that approval, sandbox and permission settings are identical before and after.

Pass criteria: steps 4–5 and 8–12 pass on each OS/app/auth combination claimed. The step count is at most 7 for existing sign-in (Codex) and at most 7 with an API key (Claude), or the excess is documented. Any `*_forwarded_unchanged` warning on ordinary chat turns blocks a support claim for that surface.

## Owner decisions needed

1. Static per-install header vs environment delivery for the Codex subscription route.
2. Qualify the implemented bounded zstd decoder with actual desktop traffic. The implementation avoids needing a desktop `[features]` edit for this purpose.
3. Whether dmz may persist a provider API key in OS secure storage for desktop use.
4. Whether Claude Desktop Chat/Cowork traffic, which inevitably transits dmz in 3P mode, is presented as covered or explicitly unqualified.
5. Which native macOS/Windows machines and test accounts (fabricated data only) are used for qualification.

## Sources

All checked 9 November 2026, 13:24–13:30 UTC. `learn.chatgpt.com/codex/...` links redirect to `learn.chatgpt.com/docs/...`.

- [gateway] https://claude.com/docs/third-party/claude-desktop/gateway
- [overview] https://claude.com/docs/third-party/claude-desktop/overview
- [installation] https://claude.com/docs/third-party/claude-desktop/installation
- [in-app-configuration] https://claude.com/docs/third-party/claude-desktop/in-app-configuration
- [configuration-format] https://claude.com/docs/third-party/claude-desktop/configuration-format
- [configuration] https://claude.com/docs/third-party/claude-desktop/configuration
- [credential-helper] https://claude.com/docs/third-party/claude-desktop/credential-helper
- [code] https://claude.com/docs/third-party/claude-desktop/code
- [data-storage] https://claude.com/docs/third-party/claude-desktop/data-storage
- [bootstrap] https://claude.com/docs/third-party/claude-desktop/bootstrap
- [Claude Code gateway connect] https://code.claude.com/docs/en/llm-gateway-connect
- [gateway protocol] https://code.claude.com/docs/en/llm-gateway-protocol
- [connect-to-a-gateway] https://learn.chatgpt.com/docs/enterprise/connect-to-a-gateway
- [gateway-compatibility] https://learn.chatgpt.com/docs/enterprise/gateway-compatibility
- [roll-out-a-gateway] https://learn.chatgpt.com/docs/enterprise/roll-out-a-gateway
- [config-reference] https://learn.chatgpt.com/docs/config-file/config-reference
- [config-basic] https://learn.chatgpt.com/docs/config-file/config-basic
- [environment-variables] https://learn.chatgpt.com/docs/config-file/environment-variables
- [auth] https://learn.chatgpt.com/docs/auth
- [app] https://learn.chatgpt.com/docs/app
- [linux-app] https://learn.chatgpt.com/docs/linux/linux-app

Vendor documentation describes intended behavior for current releases. It does not show that a specific installed build, account or plan works with dmz.

The later background implementation is documented in [background-agent-integration.md](background-agent-integration.md). Its Codex writer shares the documented user config, but synthetic CLI evidence must not be presented as native desktop acceptance. Claude Desktop import/API-key integration remains a separate unfinished path.
