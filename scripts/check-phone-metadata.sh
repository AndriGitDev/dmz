#!/usr/bin/env bash
set -euo pipefail
root_dir="$(cd "$(dirname "$0")/.." && pwd)"
go_bin="${GO_BIN:-go}"
task_temp="$(mktemp -d)"
trap 'rm -rf -- "$task_temp"' EXIT
cd "$root_dir/tools/phone-metadata"
"$go_bin" run . "$task_temp/phone-plans.json"
cmp "$task_temp/phone-plans.json" "$root_dir/packages/detect/phone-plans.json"
"$go_bin" run ./audit
