#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
go_bin="${GO_BIN:-go}"
report_dir="$(mktemp -d)"
trap 'rm -rf "$report_dir"' EXIT
sha256sum --check bench/gates/corpus-v2.sha256
for corpus in format holdout; do
  "$go_bin" run ./bench/scorers -corpus "bench/corpus/${corpus}-v2.jsonl" > "$report_dir/detection.json"
  diff -u "bench/results/${corpus}-v2.json" "$report_dir/detection.json"
  "$go_bin" run ./bench/scorers -corpus "bench/corpus/${corpus}-v2.jsonl" -mode exposure > "$report_dir/exposure.json"
  diff -u "bench/results/${corpus}-exposure-v2.json" "$report_dir/exposure.json"
done
