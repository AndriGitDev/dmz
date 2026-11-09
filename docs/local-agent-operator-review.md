# Local coding client prototype operator review

The development branch `feat/local-agent-gateway` contains a runnable local dmz gateway and launcher for Codex and Claude Code. The existing Chromium extension and detector behavior are unchanged. This is a basic-profile prototype for fabricated-data evaluation; it is not a general release or whole-session privacy boundary.

Work began on 9 November 2026 at 11:21 UTC for the operator review window ending at 15:21 UTC. Codex implemented the service/launcher and integrated fixes; Opus 5.5 implemented the raw-preserving request transformer, both client fixture harnesses, and independently reviewed the integration. The exact source and executable identities belong to the generated review manifest, rather than an assumed release version.

## What is implemented

- Native Go reuse of the existing basic detector and best-effort marker core.
- Separate Responses and Messages adapters, plus model listing, Messages token counting and Responses compaction routes.
- Raw JSON string splicing: untouched fields retain their bytes, including opaque state and signatures. Malformed or ambiguous payloads warn and pass through unchanged.
- Authenticated loopback service, fixed provider destinations, local/provider credential separation, redirect rejection, aggregate status and reset controls.
- Temporary CLI launch routing, private Claude settings layer, bounded request parsing and masking queues, idle-scope eviction with warnings, and explicit scope/capacity warnings.
- Manual service credentials and configuration fragments for desktop evaluation. No existing desktop/client configuration is installed or overwritten.
- Graceful Linux/macOS cancellation and process-group cleanup when none of the client streams uses a terminal. Windows process-tree behavior still needs a native implementation/qualification.
- A local verification script and CI checks for unit/race/fuzz/vet and cross-compilation.

The [usage and architecture guide](local-agent-gateway.md) explains commands, limits, credential modes and recovery.

## Evidence and limits

| Target or check | Current evidence |
| --- | --- |
| Codex CLI 0.166.0 on Linux | Basic synthetic conversation passes through the launcher and through generated TOML/manual service configuration |
| Codex local tool roundtrip | Default read-only test is incomplete because this container cannot run the client sandbox; a separate explicitly unsandboxed owned-fixture diagnostic passes, which does not qualify normal sandbox behavior |
| Claude Code 2.1.318 on Linux | Basic and Read-tool synthetic roundtrips pass through both launcher and manual API-key service modes |
| Manual service credentials | Fake provider receives its own fake key; local dmz token does not reach it. Status/reset/auth probes pass |
| Desktop applications | Configuration fragments generated and parsed; native applications not run. Desktop support is unqualified |
| macOS ARM64 and Windows AMD64 | Gateway executables cross-compile; native runtime/desktop qualification not performed |
| Go checks | Full module tests, gateway/launcher/core race checks and vet pass locally |
| JSON transformer fuzzing | A bounded 30-second development run passes; this is not independent validation |
| Existing detector gates | Selected, challenge and exploratory synthetic outputs match the existing baselines byte for byte; per-type recall floors are retained |
| CI | Workflow checks added; remote CI has not been run by a push in this work session |

The fake upstreams use synthetic keys, email addresses and files. Real installed CLI binaries accept their SSE messages and complete the measured loops. This proves local request-shape and forwarding compatibility, not actual provider, account, model or subscription behavior. A refusing proxy assists fixture isolation; it is not a system-wide network firewall.

## Review artifacts

On the development host, the review bundle is generated under `/tmp/dmz-operator-review-2026-11-09`. It contains the native executable, macOS/Windows cross-builds, Go check output, existing detector summary comparisons, launcher/manual-service CLI fixture reports, applicable notices and `manifest.json` with source/artifact hashes. These are local review artifacts, not public downloads. Reports include counts and booleans only, never request bodies, originals or mappings.

Recreate the core bundle with an installed compatible Go toolchain:

```sh
GO_BIN=/path/to/go ./scripts/check-agent-gateway.sh /tmp/dmz-review
```

The script exits 3 when a fixture is incomplete on the host, preserving that status. A Go check, baseline regression or fixture failure exits unsuccessfully. Reproduce the manual configuration path separately:

```sh
python3 -I scripts/agent-manual-smoke.py --dmz /tmp/dmz-review/dmz
```

The default sandbox limitation is retained in both reports. Do not change operator sandbox policy simply to obtain a passing result.

## Review fixes already applied

Short bursts wait up to 30 seconds for a transformation slot; ordinary contention no longer immediately forwards originals. Request-body reads have a deadline. New sessions can evict the least recently used idle map, with a warning about lost history. Whole-field masking now warns about context loss. Launcher warnings are de-duplicated and routine exclusions are summarized after exit.

Cancellation signals the client before forced cleanup. Process-group handling preserves foreground terminal access when stdin, stdout or stderr is a terminal. Local authentication scrubbing handles case, whitespace, duplicate fields and common misplaced credential forms. API-key replacement cannot target the ChatGPT backend. Claude launch routing is pinned above ordinary settings, while incompatible user/project routes are rejected. Managed settings remain a qualification limit. Codex uses a dedicated non-daemon launch with compression disabled; provider/profile/transport/background overrides are outside the supported route.

The generated Codex fragment warns that `model_provider` must be placed at top level before any TOML table. Appending the whole fragment inside an existing table can otherwise leave the normal provider active. Configuration is generated for review rather than installed automatically.

## Material limitations

Only selected user-message text fields are masked. Instructions, tool outputs, tool inputs, repository files returned by tools, assistant text, attachments and other network paths remain outside coverage. Tool results can reintroduce an identifier masked earlier. Missing detections, unsupported bodies and capacity/failure fallbacks can send originals. Once history exceeds a limit, subsequent turns can repeatedly bypass masking.

Responses retain markers. There is no automatic local display or tool-input restoration. A masked filename or edit target can break an operation. Mappings are memory-only and disappear on exit/reset/eviction; resume or compaction can expose unresolved old markers or trigger conservative whole-field masking. Codex uses one explicit scope per launcher invocation/manual service, not verified per-desktop-chat isolation.

The gateway does not cover direct MCP, shell networking, web tools, cloud/SSH execution, desktop synchronization, telemetry or client histories. The local token can be inherited by agent tools. No anonymity, complete detection, compliance, certification or retention guarantee is established. Native Windows cleanup, managed-policy routing and interactive TUI behavior still need qualification. API-key modes are distinct from subscription authentication.

## Operator checks before expanding scope

1. Pin the actual desktop OS/app builds and intended auth route. Use fabricated data to confirm loopback endpoint acceptance, credential delivery, session identity, model discovery, streaming, warning visibility and restore of the previous client configuration.
2. Qualify the normal Codex sandbox on an appropriate host and verify real automatic compaction, cancellation, resume and subagents. Run each provider/account route separately, including any desired subscription path.
3. Decide the desktop mapping boundary before supporting unrelated concurrent chats. The current Responses service must not be presented as isolated per chat.
4. Design controlled local restoration before masking file/tool content. Preserve approvals; avoid automatic restoration into shell networking, remote MCP or web requests. Require a read/edit/test acceptance flow, not just a masked prompt fixture.
5. Adapt the trained scanner only as a separate runtime/asset-rights/parity milestone. Keep existing detector quality gates and honest profile distinctions.
6. Follow the existing repository history, asset rights, source-availability and release checklist before public distribution. This implementation does not change repository visibility or publish binaries.
