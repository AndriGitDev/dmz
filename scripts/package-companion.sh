#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-only
# Builds PRIVATE REVIEW companion bundles for macOS (arm64, amd64), Windows
# (amd64) and Linux (amd64). Local review artifacts only: no signing,
# notarization, upload, installation, release or public download.
#
# Usage: GO_BIN=/path/to/go scripts/package-companion.sh [OUTPUT_DIR]
# OUTPUT_DIR must be new or empty; default is a fresh /tmp/dmz-companion-package.*
set -euo pipefail

root_dir="$(cd "$(dirname "$0")/.." && pwd -P)"
go_bin="${GO_BIN:-go}"
command -v "$go_bin" >/dev/null || { echo "package-companion: Go toolchain not found (set GO_BIN)" >&2; exit 2; }
command -v python3 >/dev/null || { echo "package-companion: python3 is required to write zip archives" >&2; exit 2; }

out_dir="${1:-$(mktemp -d /tmp/dmz-companion-package.XXXXXX)}"
mkdir -p -- "$out_dir"
out_dir="$(cd -- "$out_dir" && pwd -P)"
if [[ -n "$(ls -A -- "$out_dir")" ]]; then
  echo "package-companion: output directory must be empty" >&2
  exit 2
fi
case "$out_dir" in
  "$root_dir"|"$root_dir"/*) echo "package-companion: output directory must be outside the repository" >&2; exit 2 ;;
esac

cd "$root_dir"
template_dir="$root_dir/scripts/companion-package"
licenses_src="$root_dir/apps/dmz/public/licenses"
# Notices for code compiled into the Go executable (basic detector, phone
# metadata, secret patterns, golang.org/x/text). No trained scanner, model,
# tokenizer or address vocabulary is bundled.
notices=(gitleaks-MIT.txt vendor-secrets-NOTICE.txt golang-x-text-LICENSE.txt libphonenumber-Apache-2.0.txt phonenumbers-MIT.txt phone-numbering-NOTICE.txt)
gateway_notices=(klauspost-compress-LICENSE klauspost-compress-zstd-internal-xxhash-LICENSE.txt klauspost-compress-internal-snapref-LICENSE klauspost-compress-snappy-AUTHORS)
go_license="$("$go_bin" env GOROOT)/LICENSE"
[[ -f "$go_license" ]] || { echo "package-companion: Go LICENSE not found" >&2; exit 2; }

source_rev="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
source_state="clean"
if [[ -n "$(git status --porcelain 2>/dev/null)" ]]; then source_state="modified working tree (not reproducible from the revision alone)"; fi
go_version="$("$go_bin" env GOVERSION)"

build_dir="$out_dir/build"
stage_dir="$out_dir/stage"
mkdir -p "$build_dir" "$stage_dir"

build() { # goos goarch output [extra ldflags]
  CGO_ENABLED=0 GOOS="$1" GOARCH="$2" "$go_bin" build -trimpath -buildvcs=false \
    -ldflags "-s -w ${4:-}" -o "$3" ./cmd/dmz
}

copy_docs() { # destination
  mkdir -p "$1/licenses"
  cp "$template_dir/README.txt" "$1/README.txt"
  cp LICENSE COPYING-NOTICE.md THIRD_PARTY.md "$1/"
  for n in "${notices[@]}"; do cp "$licenses_src/$n" "$1/licenses/"; done
  for n in "${gateway_notices[@]}"; do cp "$root_dir/internal/agentgateway/licenses/$n" "$1/licenses/"; done
  cp "$go_license" "$1/licenses/Go-LICENSE.txt"
  {
    echo "DMZ companion private review prototype - NOT A RELEASE"
    echo "source revision: $source_rev"
    echo "source state: $source_state"
    echo "toolchain: $go_version (CGO_ENABLED=0, -trimpath, -buildvcs=false)"
    echo "detector profile: basic (no trained scanner)"
    echo "target: $2"
  } > "$1/MANIFEST.txt"
}

write_info_plist() { # path
  cat > "$1" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key>
	<string>en</string>
	<key>CFBundleExecutable</key>
	<string>DMZ</string>
	<key>CFBundleIdentifier</key>
	<string>local.dmz.companion.private-prototype</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>DMZ</string>
	<key>CFBundleDisplayName</key>
	<string>DMZ</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>0.0.0</string>
	<key>CFBundleVersion</key>
	<string>0</string>
	<key>LSMinimumSystemVersion</key>
	<string>12.0</string>
	<key>LSUIElement</key>
	<true/>
	<key>NSHumanReadableCopyright</key>
	<string>GPL-3.0-only. Private review prototype, not a release.</string>
</dict>
</plist>
PLIST
}

write_desktop_file() { # path
  # %k is this file's own path; the launcher runs the DMZ executable beside it
  # without word splitting or globbing, so folder names with spaces work.
  cat > "$1" <<'DESKTOP'
[Desktop Entry]
Type=Application
Version=1.5
Name=DMZ
Comment=Private review prototype - not a release
Exec=sh -c "set -f; IFS=; cd -- \\${1%%/*} && exec ./DMZ" dmz-launcher %k
Terminal=false
Categories=Development;
DESKTOP
  chmod 0755 "$1"
}

zip_dir() { # archive source_dir_parent top_entry
  python3 -I - "$1" "$2" "$3" <<'PY'
import os, stat, sys, zipfile
archive, parent, top = sys.argv[1:4]
with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as z:
    for dirpath, dirnames, filenames in os.walk(os.path.join(parent, top)):
        dirnames.sort()
        for name in sorted(filenames) + []:
            full = os.path.join(dirpath, name)
            st = os.lstat(full)
            if not stat.S_ISREG(st.st_mode):
                raise SystemExit("unexpected non-regular file")
            info = zipfile.ZipInfo(os.path.relpath(full, parent), date_time=(2026, 1, 1, 0, 0, 0))
            info.external_attr = (stat.S_IFREG | (st.st_mode & 0o777)) << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            with open(full, "rb") as f:
                z.writestr(info, f.read())
PY
}

archives=()

for arch in arm64 amd64; do
  name="DMZ-private-prototype-macos-$arch"
  top="$stage_dir/$name"
  app="$top/DMZ.app/Contents"
  mkdir -p "$app/MacOS"
  build darwin "$arch" "$app/MacOS/DMZ"
  chmod 0755 "$app/MacOS/DMZ"
  write_info_plist "$app/Info.plist"
  printf 'APPL????' > "$app/PkgInfo"
  copy_docs "$top" "darwin/$arch"
  zip_dir "$out_dir/$name.zip" "$stage_dir" "$name"
  archives+=("$name.zip")
done

name="DMZ-private-prototype-windows-amd64"
top="$stage_dir/$name"
mkdir -p "$top"
build windows amd64 "$top/DMZ.exe" "-H=windowsgui"
copy_docs "$top" "windows/amd64"
zip_dir "$out_dir/$name.zip" "$stage_dir" "$name"
archives+=("$name.zip")

name="DMZ-private-prototype-linux-amd64"
top="$stage_dir/$name"
mkdir -p "$top"
build linux amd64 "$top/DMZ"
chmod 0755 "$top/DMZ"
write_desktop_file "$top/DMZ.desktop"
copy_docs "$top" "linux/amd64"
python3 -I - "$stage_dir" "$name" "$out_dir/$name.tar.gz" <<'PYARCHIVE'
import gzip, os, pathlib, stat, sys, tarfile
parent, name, output = sys.argv[1:]
root = pathlib.Path(parent) / name
with open(output, 'wb') as raw, gzip.GzipFile(filename='', mode='wb', fileobj=raw, mtime=1767225600) as compressed:
    with tarfile.open(fileobj=compressed, mode='w') as archive:
        for path in [root, *sorted(root.rglob('*'))]:
            st = path.lstat()
            if not (stat.S_ISREG(st.st_mode) or stat.S_ISDIR(st.st_mode)):
                raise SystemExit('unexpected non-regular package file')
            info = archive.gettarinfo(str(path), arcname=str(path.relative_to(parent)))
            info.uid = info.gid = 0
            info.uname = info.gname = ''
            info.mtime = 1767225600
            if path.is_file():
                with path.open('rb') as body:
                    archive.addfile(info, body)
            else:
                archive.addfile(info)
PYARCHIVE
archives+=("$name.tar.gz")

python3 -I - "$out_dir" "${archives[@]}" <<'PYHASH'
import hashlib, pathlib, sys
root = pathlib.Path(sys.argv[1])
with (root / 'SHA256SUMS.txt').open('w', encoding='utf-8') as output:
    for name in sys.argv[2:]:
        output.write(hashlib.file_digest((root / name).open('rb'), 'sha256').hexdigest() + '  ' + name + '\n')
PYHASH

# ---------------------------------------------------------------------------
# Validation (no GUI is started; the companion's no-argument launch is never run).
report="$out_dir/validation.txt"
validate_status=0
python3 -I - "$out_dir" "${archives[@]}" > "$report" <<'PY' || validate_status=$?
import plistlib, struct, sys, tarfile, zipfile, io
out, archives = sys.argv[1], sys.argv[2:]
common = {"README.txt", "LICENSE", "COPYING-NOTICE.md", "THIRD_PARTY.md", "MANIFEST.txt",
          "licenses/Go-LICENSE.txt", "licenses/golang-x-text-LICENSE.txt", "licenses/gitleaks-MIT.txt",
          "licenses/vendor-secrets-NOTICE.txt", "licenses/libphonenumber-Apache-2.0.txt",
          "licenses/phonenumbers-MIT.txt", "licenses/phone-numbering-NOTICE.txt",
          "licenses/klauspost-compress-LICENSE", "licenses/klauspost-compress-zstd-internal-xxhash-LICENSE.txt",
          "licenses/klauspost-compress-internal-snapref-LICENSE", "licenses/klauspost-compress-snappy-AUTHORS"}
forbidden = (".onnx", ".bin", ".safetensors", ".jsonl", ".wasm", "tokenizer", "model")
ok = True
def check(cond, msg):
    global ok
    print(("PASS " if cond else "FAIL ") + msg)
    ok = ok and cond
for name in archives:
    path = f"{out}/{name}"
    top = name.rsplit(".zip", 1)[0].rsplit(".tar.gz", 1)[0]
    files = {}
    if name.endswith(".zip"):
        with zipfile.ZipFile(path) as z:
            for i in z.infolist():
                files[i.filename] = ((i.external_attr >> 16) & 0o777, z.read(i))
    else:
        with tarfile.open(path) as t:
            members = t.getmembers()
            check(all(m.isfile() or m.isdir() for m in members), f"{name}: only regular files and directories")
            for m in members:
                if m.isfile():
                    files[m.name] = (m.mode & 0o777, t.extractfile(m).read())
    rel = {k[len(top) + 1:]: v for k, v in files.items()}
    check(all(k.startswith(top + "/") for k in files), f"{name}: single top-level folder {top}")
    check(not [k for k in files if ".." in k.split("/") or k.startswith("/")], f"{name}: no absolute or parent paths")
    check(common <= set(rel), f"{name}: README, GPL license and notices present")
    check(not [k for k in rel if k.lower().endswith(forbidden) or "scanner" in k.lower()], f"{name}: no trained scanner/model/corpus files")
    check(b"NOT A RELEASE" in rel["README.txt"][1], f"{name}: README states private prototype")
    steps = [l for l in rel["README.txt"][1].decode().splitlines() if l[:4].strip().rstrip(".").isdigit() and l.startswith("  ")]
    check(len(steps) <= 7, f"{name}: README documents {len(steps)} core steps (native prompts unqualified)")
    if "macos" in name:
        exe = rel.get("DMZ.app/Contents/MacOS/DMZ")
        check(exe is not None and exe[0] & 0o111 == 0o111, f"{name}: DMZ.app/Contents/MacOS/DMZ executable")
        magic = struct.unpack("<I", exe[1][:4])[0]
        cpu = struct.unpack("<I", exe[1][4:8])[0]
        want = 0x0100000C if "arm64" in name else 0x01000007
        check(magic == 0xFEEDFACF and cpu == want, f"{name}: Mach-O 64-bit for expected CPU")
        info = plistlib.loads(rel["DMZ.app/Contents/Info.plist"][1])
        check(info["CFBundleExecutable"] == "DMZ" and info["CFBundlePackageType"] == "APPL", f"{name}: Info.plist executable/package type")
        check("prototype" in info["CFBundleIdentifier"] and "not a release" in info["NSHumanReadableCopyright"], f"{name}: Info.plist private prototype label")
    elif "windows" in name:
        exe = rel.get("DMZ.exe")
        check(exe is not None and exe[1][:2] == b"MZ", f"{name}: DMZ.exe is a PE image")
        pe = struct.unpack("<I", exe[1][0x3C:0x40])[0]
        check(exe[1][pe:pe + 4] == b"PE\0\0", f"{name}: PE signature")
        machine = struct.unpack("<H", exe[1][pe + 4:pe + 6])[0]
        subsystem = struct.unpack("<H", exe[1][pe + 24 + 68:pe + 24 + 70])[0]
        check(machine == 0x8664, f"{name}: AMD64 machine")
        check(subsystem == 2, f"{name}: Windows GUI subsystem (no console window on double-click)")
    else:
        exe = rel.get("DMZ")
        check(exe is not None and exe[0] & 0o111 == 0o111 and exe[1][:4] == b"\x7fELF", f"{name}: DMZ ELF executable")
        desk = rel.get("DMZ.desktop")
        check(desk is not None and desk[0] & 0o111 == 0o111, f"{name}: DMZ.desktop launcher executable")
        text = desk[1].decode()
        check("Exec=sh -c" in text and "%k" in text and "Terminal=false" in text, f"{name}: desktop launcher entry")
print("RESULT", "PASS" if ok else "FAIL")
sys.exit(0 if ok else 1)
PY

# Native helper commands of the host build only (never the GUI launch).
host_goos="$("$go_bin" env GOHOSTOS)"; host_arch="$("$go_bin" env GOHOSTARCH)"
if [[ "$host_goos/$host_arch" == "linux/amd64" ]]; then
  native="$stage_dir/DMZ-private-prototype-linux-amd64/DMZ"
  {
    printf 'native linux/amd64 version: '; "$native" version
    "$native" help >/dev/null && echo "PASS native help command"
  } >> "$report" 2>&1
  # Desktop launcher: parse the Exec line as a desktop launcher would and run
  # it against a stub executable in a folder with spaces and shell characters.
  probe="$(mktemp -d)"
  launch_dir="$probe/My \$(id) 'DMZ' folder"
  mkdir -p "$launch_dir"
  printf '#!/bin/sh\npwd > "%s/ran"\n' "$probe" > "$launch_dir/DMZ"; chmod 0755 "$launch_dir/DMZ"
  cp "$stage_dir/DMZ-private-prototype-linux-amd64/DMZ.desktop" "$launch_dir/"
  python3 -I - "$launch_dir/DMZ.desktop" <<'PY' && [[ "$(cat "$probe/ran")" == "$launch_dir" ]] && echo "PASS desktop Exec runs DMZ beside the launcher (spaces, quotes, \$())" >> "$report" || { echo "FAIL desktop Exec probe" >> "$report"; validate_status=1; }
import subprocess, sys
path = sys.argv[1]
line = next(l for l in open(path, encoding="utf-8") if l.startswith("Exec="))[5:].rstrip("\n")
line = line.replace("\\\\", "\\")  # desktop-entry string escape layer
# Field codes, then Exec quoting: inside double quotes a backslash escapes
# one of " ` $ \ ; outside quotes arguments are split on spaces.
argv, cur, i, quoted, have = [], "", 0, False, False
while i < len(line):
    c = line[i]
    if c == "%" and not quoted:
        code = line[i + 1]
        if code == "k":
            argv.append(path); i += 2; continue
        if code == "%":
            cur += "%"; have = True; i += 2; continue
        raise SystemExit("unexpected field code")
    if c == "%" and quoted and line[i + 1] == "%":
        cur += "%"; i += 2; continue
    if c == '"':
        quoted = not quoted; have = True; i += 1; continue
    if c == "\\" and quoted and line[i + 1] in '"`$\\':
        cur += line[i + 1]; i += 2; continue
    if c == " " and not quoted:
        if have: argv.append(cur)
        cur, have = "", False; i += 1; continue
    cur += c; have = True; i += 1
if have: argv.append(cur)
assert argv[:2] == ["sh", "-c"] and argv[2] == "set -f; IFS=; cd -- ${1%/*} && exec ./DMZ", argv
subprocess.run(argv, check=True)
PY
  rm -rf -- "$probe"
else
  echo "SKIP native helper commands: host is $host_goos/$host_arch (cross-built outputs are compile-only)" >> "$report"
fi
echo "NOTE macOS and Windows bundles are cross-compiled and unsigned; they have not been run natively." >> "$report"

rm -rf -- "$build_dir"
cat "$report"
printf 'Private review bundles written to %s\n' "$out_dir"
exit "$validate_status"
