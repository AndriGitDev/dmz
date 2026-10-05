#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
go_bin="${GO_BIN:-go}"
report_dir="$(mktemp -d)"
trap 'rm -rf "$report_dir"' EXIT
sha256sum --check bench/gates/long-context-v3.sha256
for group in short crossing interior nfd; do
  name="long-context-$group-v3"
  "$go_bin" run ./bench/scorers -corpus "bench/corpus/$name.jsonl" -baseline "bench/results/$name.json" > "$report_dir/$name.json"
  diff -u "bench/results/$name.json" "$report_dir/$name.json"
  "$go_bin" run ./bench/scorers -corpus "bench/corpus/$name.jsonl" -mode exposure -max-exposed 2 > "$report_dir/$name-exposure.json"
  diff -u "bench/results/$name-exposure.json" "$report_dir/$name-exposure.json"
done
