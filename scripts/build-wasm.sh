#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "$0")/.." && pwd)"
tinygo_bin="${TINYGO_BIN:-tinygo}"
go_bin="${GO_BIN:-go}"
output_dir="$root_dir/apps/shield/public"
mkdir -p "$output_dir"
export PATH="$(dirname "$(command -v "$go_bin")"):$PATH"
build_tags=()
build_temp="$(mktemp -d)"
trap 'rm -rf -- "$build_temp"' EXIT
lexicon_stage="$root_dir/packages/detect/lexicon.generated.json"
if [[ -n "${DMZ_LEXICON_FILE:-}" ]]; then
  if [[ -e "$lexicon_stage" ]]; then
    echo 'A staged lexicon already exists; finish or clean that build first.' >&2
    exit 1
  fi
  cp -- "$DMZ_LEXICON_FILE" "$lexicon_stage"
  trap 'rm -f -- "$lexicon_stage"; rm -rf -- "$build_temp"' EXIT
  build_tags=(-tags dmz_lexicon)
  python3 - "$lexicon_stage" "$build_temp/detector-data.json" <<'PYDATA'
import hashlib, json, pathlib, sys
raw = pathlib.Path(sys.argv[1]).read_bytes()
pack = json.loads(raw)
if pack.get('schema') != 1:
    raise ValueError('invalid lexicon schema')
pathlib.Path(sys.argv[2]).write_text(json.dumps(dict(schema=1, lexicon_sha256=hashlib.sha256(raw).hexdigest(), name_seeds=len(pack.get('name_seeds', [])), name_parts=len(pack.get('name_parts', [])), names=len(pack['names']), addresses=len(pack['addresses'])), indent=2)+'\n')
PYDATA
fi
# Public regex compilation can exceed wasm's default 64 KiB goroutine stack.
# Keep both rules-only and trained builds on the same bounded stack setting.
"$tinygo_bin" build -target wasm -no-debug -stack-size=128kb "${build_tags[@]}" -o "$build_temp/dmz.wasm" "$root_dir/apps/shield/wasm"
mv -- "$build_temp/dmz.wasm" "$output_dir/dmz.wasm"
if [[ -f "$build_temp/detector-data.json" ]]; then
  mv -- "$build_temp/detector-data.json" "$output_dir/detector-data.json"
else
  rm -f -- "$output_dir/detector-data.json"
fi
runtime_root="$("$tinygo_bin" env TINYGOROOT)"
cp "$runtime_root/targets/wasm_exec.js" "$output_dir/wasm_exec.js"
printf 'WASM size: %s bytes\n' "$(wc -c < "$output_dir/dmz.wasm")"
