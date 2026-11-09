#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-only
# Build the native macOS menu-bar host (prototype; not a release, not signed).
#
#   scripts/build-macos-menubar.sh <out-dir> [dmz-core-universal-binary]
#
# Produces <out-dir>/DMZ (universal arm64+x86_64, macOS 13+). When a Go core
# binary is given, also assembles <out-dir>/DMZ.app with Contents/MacOS/DMZ
# (host) and Contents/MacOS/dmz-core (core). On non-Darwin hosts it only
# checks the source files are present and well formed, and exits 0.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
src="$root/native/macos/DMZMenuBar.m"
plist="$root/native/macos/Info.plist"
out="${1:?usage: build-macos-menubar.sh <out-dir> [dmz-core]}"
core="${2:-}"

if [ "$(uname -s)" != "Darwin" ]; then
  test -s "$src" && test -s "$plist"
  if command -v python3 >/dev/null; then
    python3 -I -c 'import plistlib,sys; d=plistlib.load(open(sys.argv[1],"rb")); assert d["CFBundleExecutable"]=="DMZ" and d["LSUIElement"] is True' "$plist"
  fi
  echo "build-macos-menubar: non-Darwin host; source/plist checked, native compile skipped"
  exit 0
fi

mkdir -p "$out"
obj="$(mktemp -d)"
trap 'rm -rf "$obj"' EXIT
flags=(-fobjc-arc -fmodules -O2 -Wall -Wextra -Werror -Wno-unused-parameter
       -mmacosx-version-min=13.0 -framework Cocoa -framework Foundation)
for arch in arm64 x86_64; do
  clang -arch "$arch" "${flags[@]}" "$src" -o "$obj/DMZ-$arch"
done
lipo -create "$obj/DMZ-arm64" "$obj/DMZ-x86_64" -output "$out/DMZ"
chmod 0755 "$out/DMZ"
lipo -info "$out/DMZ"

if [ -n "$core" ]; then
  app="$out/DMZ.app/Contents"
  rm -rf "$out/DMZ.app"
  mkdir -p "$app/MacOS"
  cp "$out/DMZ" "$app/MacOS/DMZ"
  cp "$core" "$app/MacOS/dmz-core"
  chmod 0755 "$app/MacOS/DMZ" "$app/MacOS/dmz-core"
  cp "$plist" "$app/Info.plist"
  plutil -lint "$app/Info.plist"
  # Ad-hoc signature only so Gatekeeper on the build Mac can run it locally.
  codesign --force --sign - "$app/MacOS/dmz-core"
  codesign --force --sign - "$out/DMZ.app"
  codesign --verify --deep --strict "$out/DMZ.app"
  echo "build-macos-menubar: assembled $out/DMZ.app (unsigned prototype)"
fi
