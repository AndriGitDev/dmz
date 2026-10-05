#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
go_bin="${GO_BIN:-go}"
report_dir="$(mktemp -d)"
trap 'rm -rf "$report_dir"' EXIT
sha256sum --check bench/gates/aws-credentials-v3.sha256
"$go_bin" run ./bench/scorers -corpus bench/corpus/aws-credentials-v3.jsonl > "$report_dir/detection.json"
diff -u bench/results/aws-credentials-v3.json "$report_dir/detection.json"
"$go_bin" run ./bench/scorers -corpus bench/corpus/aws-credentials-v3.jsonl -mode exposure -max-exposed 2 > "$report_dir/exposure.json"
diff -u bench/results/aws-credentials-exposure-v3.json "$report_dir/exposure.json"
