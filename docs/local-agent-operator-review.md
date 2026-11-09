# DMZ background app: operator review

The review target is a macOS menu-bar app that detects installed Codex CLI and
Claude Code, prepares compatible local routes, and lets the user open those tools
normally. Windows has a tray prototype; Linux retains a browser/terminal launcher.
The existing Chromium extension and detector behavior are unchanged.

This is a basic-profile review build for fabricated data, not a general release.
The owner approved public source, Mac CI and unsigned Mac review downloads on
draft PR #27, then extended implementation beyond the original 15:21 UTC window
until operator readiness. Codex and Opus 5.5 collaborate on implementation,
synthetic verification and review. Native user acceptance remains the operator's
next step, not an implied result of CI.

## Start the Mac review

Prerequisite: Codex CLI or Claude Code already works with the operator's setup.

1. Download and extract the universal Mac review ZIP, then move `DMZ.app` to
   Applications. Quit an older DMZ build before opening its replacement.
2. Open DMZ. It should show a menu-bar icon without opening a browser or terminal.
3. Open the coding tool normally; restart it once if it was already open.
4. Send a fabricated message such as `Contact pilot@example.test` and inspect
   the connection status from the icon. Settings is optional.

Count download, relocation, Gatekeeper and vendor sign-in/trust actions during
this review. This list is a target flow, not proof of an under-eight-action
downloaded installation. The app is not Developer ID signed or notarized; an OS
block is a review result. Do not treat bypassing OS protections as finished setup.

DMZ must stay open. Quit restores its routes, and tools opened afterwards connect
directly without DMZ masking. This build has no login item: open it again after
computer login. Already-open clients may need a restart after disconnect or Quit.
A crash can instead leave the local route unavailable until DMZ is reopened.

## What is implemented

- Native Mac status icon and optional local settings, with detection, configured
  but unconfirmed, observed model traffic, warning and disconnected states.
- Automatic routing of compatible user settings, stable private loopback ports
  and access tokens, and discovery of tools installed after DMZ opens.
- Native client authentication, model, sandbox, approval and permission choices.
  Codex's own credential takes precedence; its `OPENAI_API_KEY` fallback is
  consumed locally and stripped before forwarding.
- Private recovery journals persisted before settings change. Unchanged settings
  restore exactly; vendor-specific removal preserves unrelated later edits.
  Recovery files may contain sensitive existing settings; never attach them to
  reports. Ambiguous recovery is reported instead of overwriting settings.
- Fixed-destination Responses/Messages gateways, raw JSON preservation outside
  changed strings, streaming replies and bounded gzip/x-gzip/zstd decoding.
- Lifetime warning counts plus recent attention state. Processing warnings affect
  attention for two minutes; normal mapping eviction is informational. Expiring
  an alert does not establish that previously forwarded text was masked.

Installed-tool detection and prepared settings do not prove routing. A shell can
use a different `CODEX_HOME` or `CLAUDE_CONFIG_DIR` from an app opened in Finder.
Managed/project/local settings and CLI overrides can bypass a route. One tool's
observed traffic does not prove that all its concurrent sessions use DMZ.
Custom gateways, symlinked settings and separate Codex profiles need manual review.

## Evidence and remaining qualifications

Local synthetic fixtures use real Codex 0.166.0 and Claude Code 2.1.318 with
isolated client homes, fabricated credentials and loopback providers. They
exercise ordinary background launches, masking, streaming, disconnect/reconnect,
restart, exact restoration, user-edit preservation and recovery after SIGKILL.
Codex cached API-key login, `CODEX_API_KEY`, `OPENAI_API_KEY` and synthetic account
credentials have separate fixture coverage. These do not establish real account
entitlement, refresh, provider acceptance or subscription compatibility.

Claude's Read-tool roundtrip passes locally. Codex's normal tool roundtrip remains
incomplete because this host cannot run its bwrap sandbox; DMZ does not weaken
that policy. A separate unsandboxed diagnostic does not qualify normal operation.

Focused Go tests, race checks, vet, JSON fuzzing and existing detector comparisons
provide development evidence. Browser fixtures cover local controls, focus,
warning history and Quit behavior. Apple Silicon and Intel CI compile and execute
the universal package and its CLI passthrough. The current artifact's source
revision and checksum, rather than this document, identify the reviewed build.

Not yet qualified: actual Finder/menu interaction, Gatekeeper, live provider or
account routes, direct vendor desktop apps, native Windows UI/ACLs/session-end
cleanup, real-client compaction/resume/subagents, and a normal Codex sandbox on
this host. Claude Desktop requires separate third-party inference setup and
cannot reuse its Anthropic subscription on that route. See the
[desktop plan](local-agent-desktop-plan.md).

## Coverage the operator must understand

Only selected user-message text is masked with the basic detector. Instructions,
tool arguments, repository files, attachments and other network paths remain
outside coverage. Tool results and assistant text are not generally scanned.
Exact known originals are re-masked in their supported text fields. Misses and processing or
capacity failures warn and continue, and can forward original values. Native
client histories can retain originals. Known answer markers restore locally,
and exact known originals are re-masked in supported user, assistant and tool-result
text. Unknown and whole-message fallback markers stay visible. Tool inputs are not restored; masked file names can break
operations. This basic build still lacks the trained scanner/address vocabulary.

Mappings are memory-only and disappear on restart, reset or eviction. Codex
clients currently share one connection scope. Start a fresh conversation after
DMZ restarts or mappings reset: older history can contain restored originals
that lost mappings cannot reliably re-mask. Old markers in resumed history can
be unresolved or cause conservative whole-field masking. Scope capacity can make
later requests pass unchanged; start a fresh conversation and reconnect after
that warning. Other routes such as MCP, shell networking, telemetry and remote
sessions are not intercepted. No whole-session privacy or retention guarantee
is established.

## Recovery checks on the Mac

- Disconnect each tool, restart it and confirm its previous route works. Reconnect
  and check a new fabricated request. Model, approvals and sandbox must stay as
  the operator configured them.
- Change an unrelated setting while connected (including a project trust entry),
  then disconnect. Keep the value and the operator's comments/formatting.
- Quit and reopen DMZ; restart the tools and verify the route. Force-quit the app
  in a separate fabricated-data run and confirm journal recovery after reopening.
- An occupied saved port must show a connection issue rather than silently moving
  the endpoint. A recovery conflict must remain visible and Disconnect must not
  claim success. Keep recovery files private; stop and request review rather than
  deleting a journal or guessing the original settings.
- Verify Finder reopen, relocation and app replacement. Record OS/app versions,
  auth route, action count and fixed error codes, without prompts or credentials.

## Reproduce local evidence

On the Linux development host with both clients installed:

```sh
GO_BIN=/path/to/go scripts/check-agent-gateway.sh /tmp/new-dmz-review
```

This runs Go/race/vet/fuzz, unchanged detector comparisons, launcher/manual/
companion fixtures, background routing and the environment-key fallback. Browser
checks use `DMZ_TEST_PLAYWRIGHT` and `DMZ_TEST_BROWSER`. Exit 3 means incomplete
without failures; exit 1 means failure. Counts-only reports and a source/artifact
hash manifest remain for review. Fixture proxy sinks assist isolation; they are
not a network jail. Never submit real credentials or recovery files as evidence.

On macOS, build the native menu-bar package with:

```sh
GO_BIN=/path/to/go scripts/package-companion.sh /tmp/new-empty-dmz-packages
```

Non-Mac cross-builds label their Mac archive `macos-core-only`; that is not the
menu-bar app. Public PR #27's Mac artifact job runs after all prerequisite checks
pass, pins the head source, includes notices/checksums and retains downloads for
seven days. It creates no release. Historical `private-prototype` bundle names
do not restrict public artifact access. Signing, notarization and general release
qualification remain separate.
