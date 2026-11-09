DMZ companion - operator review prototype
========================================

NOT A RELEASE. Unsigned and unnotarized review build. Use fabricated
text while evaluating. PR #27 may provide this binary as a public review
artifact after its checks pass. Historical private-prototype file/bundle names
do not restrict access to that artifact. Native GUI and live account routes need
qualification; compilation and synthetic fixtures do not establish them.

DMZ attempts to mask selected identifiers in user messages. This basic build
has limited name coverage. Instructions, files, attachments, tool inputs/results
and other network paths are outside masking coverage. Detection can miss data;
failures warn and continue and can send originals. Replies keep visible markers.
Your coding tool keeps its own on-disk history; DMZ does not rewrite it.

Background flow (native Mac or Windows builds)
-----------------------------------------------
  1. Extract the review archive. On Mac, move DMZ.app to Applications.
  2. Open DMZ. Its icon appears in the Mac menu bar or Windows notification area.
  3. Open your coding tool normally. If it was already open, restart it once.

DMZ discovers installed Codex CLI and Claude Code clients and prepares compatible
local model routes. The icon distinguishes detection, waiting and observed model
traffic. Open DMZ Settings from the icon for details or Disconnect/Reconnect.
There is no project picker or separate terminal launch in this background flow.

The tool keeps its sign-in, model, sandbox and permission choices. DMZ never
silently selects an API billing connection. Codex native authentication takes
precedence; an OPENAI_API_KEY available to Codex is used only if Codex supplies
no native Authorization. These request credentials are not saved separately
by DMZ; recovery snapshots can contain sensitive values already in settings.

Existing custom gateways and separate Codex profile files are left unchanged
with a connection-review status. Managed/project settings and command-line
provider overrides can bypass this route. Detection is not masking coverage.

Disconnect or Quit restores routing and preserves unrelated settings edits.
Already-open tools can need a restart after disconnect. DMZ retains a private
recovery journal while connected and uses it after a crash. That journal can
contain existing sensitive settings; never include it in a report. Value
mappings remain in memory only. Their markers change after DMZ restarts.

Keep DMZ open while using its routes. After Quit, newly opened tools connect
directly without DMZ masking. This build does not start at login: open it again
after restarting or signing in to your computer. A crash can leave tools using
a closed local connection until DMZ is reopened and recovers its journal.

"Routing configured, not confirmed" means settings were prepared, not that the
tool has sent traffic. Alternate shell config directories, environment variables,
managed/project settings and command-line overrides can bypass them. Traffic
observed for one tool is not evidence for every concurrent session.

Recent processing/connection warnings affect status for two minutes; lifetime
counts remain available in Settings. Routine mapping eviction is informational,
but older markers may then be unresolved. Mappings are bounded; scope-capacity
warnings mean original text can be forwarded. Start a fresh conversation before
disconnecting/reconnecting if a mapping capacity warning appears.

The native Mac build requires macOS 13+ and contains Intel and Apple Silicon
executables. A "macos-core-only" archive is a cross-compiled legacy browser
launcher checkpoint, not the new menu-bar app. Build on macOS for that app.
Quit an earlier DMZ build before opening its replacement.

First launch and qualification
-------------------------------
OS trust, app relocation, sign-in and tool prompts add actions. This prototype
is not Developer ID signed/notarized; macOS or Windows may block it. Record that
as a review result. No under-eight-action downloaded installation is verified.

Direct vendor desktop applications are not qualified. Claude Desktop needs
separate third-party inference setup and that route cannot reuse its Anthropic
subscription. Real provider/account acceptance, native GUI/recovery, Windows
ACLs/process cleanup and normal Codex sandbox behavior remain qualification
work. Other network routes do not pass through DMZ.

The Linux package retains the optional browser/terminal launcher; no Linux tray
is implemented. Technical usage: run DMZ help. The native background experience
is the Mac/Windows target.

License
-------
Original code is GPL-3.0-only; see LICENSE and COPYING-NOTICE.md. Third-party
notices are in licenses/ and THIRD_PARTY.md. Corresponding source is the dmz
repository revision listed in MANIFEST.txt. This package creates no public
release or distribution approval.
