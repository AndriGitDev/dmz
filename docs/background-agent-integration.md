# Background DMZ integration direction

Owner direction, 9 November 2026, 15:05 UTC: DMZ should live in the macOS menu
bar and Windows notification area. Opening DMZ should discover the user's coding
harnesses and connect supported local model routes. Users should keep opening
their tools normally; project selection and launching a fresh terminal session
are not the target default flow. Settings remain optional.

## Required behavior

- No initial browser window. An icon provides status, settings and Quit.
- Discover installed and running Codex and Claude Code clients automatically,
  including more than one tool at once. Do not inspect process arguments or
  authentication stores to discover a running tool.
- Use official routing settings, with private recovery journals written before
  modifications. Preserve model, permissions, approval and sandbox choices.
- Preserve the user's existing authentication route and billing choice. Never
  silently choose a provider API key because it happens to be in the environment.
- A first connection may require a tool restart or explicit vendor import. Show
  this clearly; installation detection is not evidence of routed model traffic.
- On disconnect/Quit, restore owned settings byte for byte when unchanged. If
  another program or the user edited them, preserve the edit and report conflict;
  selective vendor-specific removal is still required for that case.
- Crash recovery must find an interrupted routing transaction and recover it
  without losing the original. Recovery files may include existing sensitive
  settings: keep them private, exclude them from reports, and never publish them.
- Show detected, waiting-for-restart, traffic-observed and warning states
  separately. Even traffic-observed covers only the documented model route and
  supported text fields; it is not a whole-session protection claim.

## Current implementation checkpoint

The original companion/gateway checkpoint remains available for review. The new
macOS Objective-C menu-bar host and Windows PowerShell/WinForms tray host are
prototypes. Mac CI at `93edb8a` compiled the native host for both CPUs and passed CLI
passthrough on Apple Silicon and Intel. This is separate from the current
download package and does not qualify the menu UI itself. The native host's
parent-PID supervision stops its Go core if the host crashes. Windows tray code
cross-compiles; its actual UI, ACLs and process cleanup are unqualified.

The routing package implements private write-ahead recovery, atomic settings
replacement, drift rejection and exact restoration. A Claude Code JSON planner
changes only its local routing environment keys and refuses existing custom
backends. Synthetic tests cover preserved permissions/model settings, new-file
cleanup, stale plans and conflicting edits. These primitives are not yet wired
to automatic connection or to the tray status; no real user settings were
modified while developing them. Codex TOML adaptation, selective merge after
user edits, stable background listeners, credential delivery and integrated
crash/restart fixtures remain unfinished.

Claude Desktop has a separate documented third-party inference flow; it cannot
reuse an Anthropic subscription through that route. Native Codex desktop shares
configuration with other Codex surfaces. See the dated official-source research
in [the desktop plan](local-agent-desktop-plan.md). Native desktop qualification,
provider accounts, signing/notarization and the download-to-first-answer action
count remain open. Do not describe the tray prototype as automatic hooking.

Only selected user text is masked. Instructions, tool inputs/results, files,
attachments and other network paths remain outside coverage. Misses and
processing failures can forward originals; replies retain markers. The basic
detector and its existing recall gates are unchanged.
