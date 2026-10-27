# Temporary disable: active-tab pause

Requested after publishing the renamed test release. Implementation plan and acceptance scope, 27 October 2026:

1. Add **Pause DMZ for 15 minutes / Resume now** to the extension settings for the active supported chat tab, matching the menu's existing tab-scoped status/reset controls. Clearly explain that text is sent without masking while paused.
2. Keep pause state in the content script only. Reload restores the default enabled state; expiry is checked on sends/status as well as a timer. Add no permissions, account, server request or persistent preference.
3. Suspend new masking and response restoration, remove local restored frames while paused, and preserve the in-memory mapping. Resume restores supported processing and can display previously known markers again. Reset remains an independent deliberate operation.
4. Invalidate processing when pause state changes. Interrupt a pending name-scan wait promptly and never apply a late scan. If the current send contains this gate's unchanged masked replacement, restore that gate's original draft before replay; never overwrite edits or a replacement composer. Preserve best-effort submission and exactly one send.
5. Verify both chat adapters, masked/paused/resumed output, retained markers, reload/default state, automatic expiry, independent tabs, unsupported-tab controls and in-flight scan/composer races. Run existing browser/scanner suites and publish a new versioned alpha ZIP plus guide/checksum.

Pause applies only to the active chat tab, not every website or browser profile. The setting is not approval to send confidential data; existing limits and the AI site's independent handling still apply.

## Local verification

Both supported chat adapters passed pause/resume, retained mappings, independent-tab behavior, expiry/reload and unsupported-tab UI checks. Deterministic isolated-world frame control exercises pauses during editor reconciliation, with and without later edits. A delayed trained-worker fixture proves that pausing releases a pending send promptly and ignores its later result. Existing settings/chat/race/broken-core/28-composer tests and all trained scanner workflows also pass. No model/core assets or permissions change.
