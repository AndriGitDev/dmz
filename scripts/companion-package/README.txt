DMZ companion - private review prototype
========================================

NOT A RELEASE. This package is for private review only. It is not signed,
notarized or published, and it is not a supported download.

DMZ runs on this computer. It masks selected identifiers in the text you type
to Codex CLI or Claude Code, on a best-effort basis. Instructions, tool
results, files, attachments and client history are outside masking coverage.
Detection is not complete. Use fabricated data while evaluating.

The macOS archive is universal (Apple Silicon and Intel), requires macOS 13
or newer, and still needs native installation and signing qualification.

Before you start: install Codex CLI or Claude Code and sign in with it the way
you normally do. DMZ does not read your client authentication store. You can optionally enter
an API key in the local DMZ window; it stays in memory for that session.

Start a session
---------------
  1. Download the package for your computer from the private review handoff.
  2. Extract the archive.
  3. Open DMZ:
       macOS:   move DMZ.app to Applications, then open it
       Windows: double-click DMZ.exe
       Linux:   double-click DMZ.desktop (or DMZ)
  4. Choose your project folder in the local DMZ window.
  5. If needed, enter an API key in the field for your tool. Otherwise DMZ uses the tool's existing
     account sign-in. A key from DMZ's environment requires explicit selection.
  6. Press Open Codex CLI or Open Claude Code. Continue in the terminal window that opens.

Existing-sign-in users skip action 5. Tool choice and launch share one
button: five core actions with existing sign-in or six with a pasted key. This counts the intended flow,
not a native-platform acceptance result: OS trust prompts, moving a macOS app,
client login and project-trust prompts can add actions in this prototype.
Nothing has yet established a universal under-eight-action installation.

Stop a session from the DMZ window. Closing only the browser window keeps
DMZ running; reopen DMZ to reopen its window. Use Quit DMZ to stop all sessions
and exit. The macOS app has no Dock icon in this prototype.

First-run system prompts
------------------------
This prototype is unsigned. Your operating system may block the first launch:
  macOS:   System Settings > Privacy & Security > Open Anyway.
  Windows: "Windows protected your PC" > More info > Run anyway.
  Linux:   right-click DMZ.desktop > Allow Launching.
Only do this for a package you received directly from the DMZ maintainers.

Limits
------
- Routes that reuse an existing account sign-in are experimental and not yet
  qualified. Desktop apps (Codex app, Claude Desktop) are not supported.
- DMZ needs a desktop session with a web browser and a terminal application.
- If DMZ stops, the coding client loses its connection. Start it again
  without DMZ to leave this route.
- Your coding client keeps its own history and settings as usual.

License
-------
DMZ is free software under the GNU GPL version 3 only (see LICENSE and
COPYING-NOTICE.md). Third-party notices are in the licenses folder; see
THIRD_PARTY.md. Corresponding source: the dmz repository revision listed in
MANIFEST.txt.
