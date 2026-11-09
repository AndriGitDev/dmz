# DMZ macOS menu-bar host (prototype)

Unqualified review prototype. Not signed, not notarized, not a release.

- `DMZMenuBar.m` — Cocoa `NSStatusBar` accessory app (no Dock icon, no initial
  window). It is the bundle executable `DMZ` and supervises the sibling Go core
  `Contents/MacOS/dmz-core` started as
  `dmz-core app --no-open --control-file <private 0700 dir>/control.json`.
- Command-line arguments (`DMZ version`, `DMZ connect …`, `DMZ run …`) are
  passed straight to `dmz-core` with `execv`, so CLI behaviour is unchanged.
- The menu shows what the core's `/api/state` reports. A detected tool is shown
  as "detected, not routed through DMZ" until a DMZ session exists for it.
  Detection is never presented as masking.
- Quit asks the core to stop via `/api/quit`, then escalates SIGTERM → SIGKILL
  after bounded waits, and removes the private descriptor directory.

Build on a Mac: `scripts/build-macos-menubar.sh out [path/to/universal/dmz]`.
