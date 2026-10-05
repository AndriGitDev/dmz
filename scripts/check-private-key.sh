#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
go_bin="${GO_BIN:-go}"
report_dir="$(mktemp -d)"
trap 'rm -rf "$report_dir"' EXIT
sha256sum --check bench/gates/private-key-v3.sha256
"$go_bin" run ./bench/scorers -corpus bench/corpus/private-key-v3.jsonl > "$report_dir/detection.json"
diff -u bench/results/private-key-v3.json "$report_dir/detection.json"
"$go_bin" run ./bench/scorers -corpus bench/corpus/private-key-v3.jsonl -mode exposure -max-exposed 0 > "$report_dir/exposure.json"
diff -u bench/results/private-key-exposure-v3.json "$report_dir/exposure.json"
