#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-only
# Local development evidence only. No provider calls or public publication.
set -euo pipefail

root_dir="$(cd "$(dirname "$0")/.." && pwd)"
go_bin="${GO_BIN:-go}"
export PATH="$(dirname "$(command -v "$go_bin")"):$PATH"
artifact_dir="${1:-$(mktemp -d /tmp/dmz-agent-review.XXXXXX)}"
mkdir -p -- "$artifact_dir"
artifact_dir="$(cd "$artifact_dir" && pwd)"
cd "$root_dir"

"$go_bin" test ./... > "$artifact_dir/go-test.txt" 2>&1
"$go_bin" test -race ./internal/agentgateway ./internal/companion ./cmd/dmz ./packages/detect ./packages/surrogates > "$artifact_dir/go-race.txt" 2>&1
"$go_bin" vet ./... > "$artifact_dir/go-vet.txt" 2>&1
"$go_bin" test ./internal/agentgateway -run '^$' -fuzz FuzzTransform -fuzztime 10s -parallel 4 > "$artifact_dir/go-fuzz.txt" 2>&1
CGO_ENABLED=0 "$go_bin" build -trimpath -buildvcs=false -o "$artifact_dir/dmz" ./cmd/dmz
mkdir -p "$artifact_dir/tools"
CGO_ENABLED=0 "$go_bin" build -trimpath -buildvcs=false -o "$artifact_dir/tools/companion-testhost" ./internal/companion/testhost
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 "$go_bin" build -trimpath -buildvcs=false -o "$artifact_dir/dmz-darwin-arm64" ./cmd/dmz
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 "$go_bin" build -trimpath -buildvcs=false -o "$artifact_dir/dmz-windows-amd64.exe" ./cmd/dmz

"$go_bin" run ./bench/scorers -baseline bench/results/v0.json -floor bench/gates/selected-v0-recall.json > "$artifact_dir/selected-current.json"
diff -u bench/results/v0.json "$artifact_dir/selected-current.json"
"$go_bin" run ./bench/scorers -corpus bench/corpus/challenge-v0.jsonl -floor bench/gates/challenge-v0-recall.json > "$artifact_dir/challenge-current.json"
diff -u bench/results/challenge-v0.json "$artifact_dir/challenge-current.json"
"$go_bin" run ./bench/scorers -corpus bench/corpus/exploratory-v1.jsonl -floor bench/gates/exploratory-v1-recall.json > "$artifact_dir/exploratory-current.json"
diff -u bench/results/exploratory-v1.json "$artifact_dir/exploratory-current.json"

smoke_status=0
python3 -I scripts/agent-client-smoke.py --dmz "$artifact_dir/dmz" > "$artifact_dir/cli-synthetic.json" || smoke_status=$?
if [[ "$smoke_status" -ne 0 && "$smoke_status" -ne 3 ]]; then
  printf 'Synthetic CLI verification failed; inspect %s\n' "$artifact_dir" >&2
fi
manual_status=0
python3 -I scripts/agent-manual-smoke.py --dmz "$artifact_dir/dmz" > "$artifact_dir/manual-synthetic.json" || manual_status=$?
if [[ "$manual_status" -ne 0 && "$manual_status" -ne 3 ]]; then
  printf 'Manual service verification failed; inspect %s\n' "$artifact_dir" >&2
fi
companion_status=0
python3 -I scripts/agent-companion-smoke.py --dmz "$artifact_dir/dmz" --testhost "$artifact_dir/tools/companion-testhost" --scenario-timeout 60 > "$artifact_dir/companion-synthetic.json" || companion_status=$?
ui_status=3
if [[ -n "${DMZ_TEST_PLAYWRIGHT:-}" ]]; then
  ui_status=0
  node scripts/agent-companion-ui-smoke.mjs --dmz "$artifact_dir/dmz" --playwright "$DMZ_TEST_PLAYWRIGHT" > "$artifact_dir/companion-browser.json" || ui_status=$?
else
  printf '%s\n' '{"status":"not_run","reason":"operator_supplied_playwright_required"}' > "$artifact_dir/companion-browser.json"
fi
overall_status=0
for check_status in "$smoke_status" "$manual_status" "$companion_status" "$ui_status"; do
  if [[ "$check_status" -ne 0 && "$check_status" -ne 3 ]]; then overall_status=1; break; fi
  if [[ "$check_status" -eq 3 ]]; then overall_status=3; fi
done

mkdir -p "$artifact_dir/licenses"
cp LICENSE COPYING-NOTICE.md THIRD_PARTY.md "$artifact_dir/"
for notice in gitleaks-MIT.txt vendor-secrets-NOTICE.txt golang-x-text-LICENSE.txt libphonenumber-Apache-2.0.txt phonenumbers-MIT.txt phone-numbering-NOTICE.txt; do
  cp "apps/dmz/public/licenses/$notice" "$artifact_dir/licenses/"
done
cp "$("$go_bin" env GOROOT)/LICENSE" "$artifact_dir/licenses/Go-LICENSE.txt"

python3 - "$root_dir" "$artifact_dir" <<'PY'
import datetime, hashlib, json, pathlib, subprocess, sys
root, artifacts = map(pathlib.Path, sys.argv[1:])
paths = sorted([*root.glob('cmd/dmz/*.go'), *root.glob('internal/agentgateway/*.go'),
                *[p for p in (root / 'internal/companion').rglob('*') if p.is_file()],
                *root.glob('scripts/agent-*-smoke.py'), root / 'scripts/agent-companion-ui-smoke.mjs',
                root / 'scripts/check-agent-gateway.sh', root / 'scripts/package-companion.sh',
                root / 'scripts/companion-package/README.txt', root / 'docs/local-companion.md',
                *root.glob('docs/local-agent-*.md'),root / '.github/workflows/prototype.yml'])
source = {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest() for p in paths}
outputs = {str(p.relative_to(artifacts)): hashlib.sha256(p.read_bytes()).hexdigest() for p in artifacts.rglob('*') if p.is_file() and p.name != 'manifest.json'}
report = dict(schema=1, evidence_kind='local-development-synthetic-only',
              generated_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),
              base_commit=subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip(),
              worktree_dirty=bool(subprocess.check_output(['git', 'status', '--porcelain'], cwd=root)),
              sources=source, artifacts=outputs,
              existing_selected_challenge_exploratory_baselines_unchanged=True,
              no_provider_or_account_validation=True)
(artifacts / 'manifest.json').write_text(json.dumps(report, indent=2) + '\n')
PY

printf 'Local review artifacts: %s\n' "$artifact_dir"
if [[ "$overall_status" -eq 3 ]]; then
  printf 'Go checks and detector baseline comparisons passed; CLI evidence is incomplete on this host.\n'
fi
exit "$overall_status"
