#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "$0")/.." && pwd)"
tinygo_bin="${TINYGO_BIN:-tinygo}"
go_bin="${GO_BIN:-go}"
output_dir="$root_dir/apps/shield/public"
mkdir -p "$output_dir"
export PATH="$(dirname "$(command -v "$go_bin")"):$PATH"
"$tinygo_bin" build -target wasm -no-debug -o "$output_dir/dmz.wasm" "$root_dir/apps/shield/wasm"
runtime_root="$("$tinygo_bin" env TINYGOROOT)"
cp "$runtime_root/targets/wasm_exec.js" "$output_dir/wasm_exec.js"
printf 'WASM size: %s bytes\n' "$(wc -c < "$output_dir/dmz.wasm")"
