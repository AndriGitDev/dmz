#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
staged=packages/detect/lexicon.generated.json
if [[ -e "$staged" ]]; then
  echo 'Another lexicon build is active.' >&2
  exit 1
fi
cp bench/datasets/fixture-lexicon.json "$staged"
trap 'rm -f "$staged"' EXIT
"${GO_BIN:-go}" test -tags dmz_lexicon ./...
