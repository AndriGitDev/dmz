# Trained Shield preview

This integrates the frozen research name scanner into supported Shield text submissions. It is a local testing build, not an independently validated release. The current default CI build retains the small scanner; the explicit trained build requires private model assets. No weights, tokenizer vocabulary, CRF weights or prompt fixtures belong in Git.

The trained build keeps one extension-origin offscreen worker for all tabs. It loads packaged assets, checks their hashes, tokenizes original text, runs the fixed CRF and neural model, and merges name spans at the frozen thresholds. The quality-first model uses FP16 storage with FP32 computation and preserved test spans; its model file alone is 237.044 MiB. This does not establish accuracy on independent user text. Runtime is single-threaded to avoid requiring cross-origin isolation on chat sites.

Shield uses the owner-requested best-effort masking policy: detected secrets and uncertain identifiers are masked instead of blocking submission. A name contained in an email keeps the complete email mask. Overlapping spans cover their union. Unicode/unknown-marker inputs that cannot safely be represented by individual spans are conservatively masked as a whole. The strict shared-core policy remains unchanged for other callers and baseline evaluation.

The send gate rechecks draft, URL, composer, target and session after asynchronous inference. If the trained scanner is unavailable, corrupt, busy, oversized, returns invalid spans or times out, Shield sends the basic detector's masked result with a visible warning that names may remain unmasked. This fallback cannot provide the trained detector's name coverage. Changed drafts, lost sessions, missing masking core and unsupported send paths still stop the operation. Files and attachments remain unsupported. Model work stays bounded and local; prompts and predictions are not logged or persistently cached.

## Local build

Run from the repository root, with the already installed local toolchains. `prepare-scanner.py` requires python-crfsuite; the native parity preparation additionally requires transformers. Use existing private paths for `--experiment`, `--base` and `--crf`.

```sh
python3 apps/shield/scripts/prepare-scanner.py \
  --experiment /private/full-precision-v1 --base /private/icebert-base \
  --crf /private/initials.crfsuite --output /private/shield-scanner
./scripts/build-wasm.sh
cd apps/shield
npm ci
npm run check
DMZ_SCANNER_ASSETS=/private/shield-scanner npm run build:scanner
npm run test:browser
npm run test:scanner
```

The pinned ONNX Runtime 1.23.2 package must already be installed under `bench/browser-detector/node_modules` from that directory's lockfile. The builder does not download models or runtimes. Generated assets exist only in ignored `.output/`. `npm run build` without the trained build command produces the original small preview, not this model package. Do not use `npm run zip` for the trained build: it rebuilds the ordinary profile. After committing and rebuilding the trained profile, `npm run record:build` records every asset hash; zip the already-built directory into an external private output path.

Load `apps/shield/.output/chrome-mv3` using **Load unpacked** on `chrome://extensions` (or Brave's extensions page), then reload supported ChatGPT/Claude tabs. The extension name includes “trained preview” and Settings identifies the active trained scanner. First use loads the model and can take several seconds. A trained-scan failure uses basic masking with a warning. A changed draft remains for retry. Uninstall or reload the extension to discard its worker/model; reset in Settings clears a tab's mappings.

## Validation scope

Native-vs-JavaScript parity is checked using private fixtures with `prepare-scanner-parity.py` and `scanner-parity.mjs`: original-text token boundaries, UTF-8 offsets, CRF features/labels/marginals, model subword IDs, chunk boundaries and pooling. The CRF JSON export uses python-crfsuite's six-decimal info weights, so parity checks are required after changing an exporter or artifact. Model inference parity and full Shield behavior remain separate checks.

`scanner-smoke.mjs` uses fabricated text in a real loaded Chromium extension. It checks new name masking, isolated restoration, edits during inference, one shared model with separate tab mappings, termination, busy-tab rejection, reset races and corrupt-model rejection. Set `DMZ_SCANNER_DEADLINE_TEST=1` to additionally verify the real 120-second worker deadline. It temporarily changes only the ignored test build's model/worker and restores it in `finally`; do not run concurrently with other browser tests or package that build mid-test.

Before Chrome Web Store publication: complete owner-browser workflow testing, review model/tokenizer/runtime redistribution notices and the actual packaged size, and prepare an accurate store privacy disclosure. A developer account does not publish or approve this preview. No store upload is performed by these build scripts.

Chrome offscreen worker API: https://developer.chrome.com/docs/extensions/reference/api/offscreen

## Integration evidence — 30 September 2026

- Native/JavaScript preprocessing parity: all 5,582 general development records (96,055 words) and 256 original-text synthetic domain records (9,056 words) passed token IDs, chunking, pooling, CRF features and label/threshold decisions. Maximum CRF marginal error from export rounding was below 0.000003.
- Packaged Chromium 153.0.8010.12 worker, including original-text tokenization and fresh CRF inference: all 256 synthetic domain records exactly matched the frozen reference spans. TP/FP/FN 474/3/6, precision 99.3711%, recall 98.75%, 199.589 seconds for the complete replay including startup, zero external requests. This excludes Go policy/masking and is reused-development evidence, not an independent accuracy claim.
- Existing Shield browser suites pass for settings, both supported-site fixtures, isolated restoration, secrets/uploads, network-body checks, composer races and broken core. Added Go tests cover invalid UTF-8 offsets, overlap with other identifiers, preserving old blockers, per-session restoration and issued-marker integrity.
- Actual model workflow tests mask newly detected single names and preserve extension-frame restoration, recheck edited drafts, share a single model while keeping tab mappings separate, and block on scanner termination/corruption. The first measured cold scan of a short fabricated prompt took 8.176 seconds on local CPU; owner-hardware latency is not verified.

The exact input fixtures, model files and individual predictions remain private outside Git. A final build manifest records source commit, scanner profile and every packaged file hash.


## Best-effort policy update — 30 September 2026

Owner explicitly requested masking without content-based blocking, including secrets, and authorized basic masking with a warning when trained inference fails. This supersedes the earlier scanner-failure blocking behavior described in the historical measurements above. Regression checks now cover name/email containment, secret masking, uncertain PII restoration, basic fallback on worker loss/corruption/busy/deadline, and unchanged edit/reset protection. These are integration checks, not new detector accuracy measurements.
