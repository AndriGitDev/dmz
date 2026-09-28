#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
web_root="${1:-$(dirname "$repo_root")/dmz-web}"
if [[ ! -f "$web_root/package.json" ]] || ! grep -q '"name": "dmz-web"' "$web_root/package.json"; then
  echo "Expected a dmz-web checkout at $web_root" >&2
  exit 1
fi
mkdir -p "$web_root/public/benchmark"
cp "$repo_root/bench/corpus/v0.jsonl" "$web_root/public/benchmark/v0.jsonl"
cp "$repo_root/bench/results/v0.json" "$web_root/public/benchmark/v0.json"
cp "$repo_root/bench/results/regex-v0.json" "$web_root/public/benchmark/regex-v0.json"
cp "$repo_root/bench/results/presidio-v0.json" "$web_root/public/benchmark/presidio-v0.json"
echo "Copied synthetic corpus and measured results to $web_root/public/benchmark"
