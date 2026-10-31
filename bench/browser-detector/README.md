# Private browser detector benchmark

Research harness for contextual name detector exports. **Not wired into Shield**
and not an independent accuracy evaluation. The historical frozen-encoder
comparison is in [protocol.md](protocol.md) and
[results](../results/browser-v1/README.md). The later full-encoder training and
frozen-reference compression comparison is in
[full-precision results](../results/full-precision-v1/README.md).

## Reproduce

Use the existing contextual encoder environment and install the additional pinned
packages in `requirements.txt`. Install browser tools with `npm ci` in this
directory and `npx playwright install chromium`. Model/data paths below are local
research assets; none are downloaded by the benchmark. Weights, tokenizers,
licensed corpus and per-record predictions must stay outside this repository.

From the repository root (adjust paths to your local research directory):

```bash
PY=/root/.local/share/dmz/encoder-venv/bin/python
ASSETS=/root/.local/share/dmz/experiments/browser-v1
RECALL=/root/.local/share/dmz/experiments/recall-v2
$PY bench/browser-detector/export.py \
  --base /root/.local/share/dmz/models/icebert-base \
  --head "$RECALL/head-mlp-16/epoch-12.safetensors" --output "$ASSETS"
$PY bench/browser-detector/evaluate.py \
  --model "$ASSETS/int8.onnx" --tokenizer "$ASSETS/tokenizer" \
  --input /root/.local/share/dmz/datasets/prepared-v2/dev.jsonl \
  --crf "$RECALL/crf-predictions-dev.json" \
  --reference "$RECALL/neural-epoch-12-predictions-dev.json" \
  --output /tmp/int8-general.json --predictions "$ASSETS/new-general-predictions.json"
$PY bench/browser-detector/fixtures.py --assets "$ASSETS"
DMZ_MODEL_ASSETS="$ASSETS" DMZ_BROWSER_REPORT=/tmp/chromium.json \
  node bench/browser-detector/browser.mjs
```

Export refuses to replace an existing asset directory. Evaluation refuses to
replace a prediction cache. Use fresh paths to reproduce. Repeat evaluation for
`fp32.onnx` and `domain-dev.jsonl`, using the matching `*-domain.json` caches.
Use `--batch-size 1` to check unbatched dynamic quantization; batching can change
activation ranges and therefore predictions.

## What each stage measures

- **Export:** encoder, mean subword pooling, the training cache's float16
  round-trip, and frozen nonlinear BIO head. Inputs have dynamic batch, token and
  word dimensions. No source downloads, calibration, retraining or tuning.
- **Native evaluation:** complete fixed development sets, canonical word/UTF-8
  spans, frozen CRF-first merge and thresholds. CRF predictions are reused from
  hash-checked caches. This is not a CRF runtime measurement.
- **Browser:** dedicated worker, ONNX Runtime Web WASM CPU, pretokenized fabricated
  vectors at 32/128/512 words. Includes input transfer, tensor construction,
  inference and output transfer in wall timings. Excludes browser tokenization,
  CRF, masking, UI interception and real website behavior.
- **Load:** local model transfer and session creation in a fresh worker/context,
  with no-store HTTP. Same Chromium process; not an OS-cache cold start or a
  download over the internet.
- **Memory:** sampled sum of Linux RSS across the Chromium process tree, including
  shared pages more than once. It is neither JS heap size nor unique RAM use.
- **Three tabs:** independent model copies, deliberately measuring the simplest
  integration's cost. Does not assume a shared extension inference worker.
- **Failures:** malformed input, corrupt model, inference before readiness,
  request timeout, worker termination and worker crash must reject. This proves
  behavior of this harness only; it is not Shield's fail-closed release gate.

All browser requests are restricted to a loopback server. No hosted inference
fallback exists. Three WASM threads use COOP/COEP; the single-thread cases do not.
A production extension needs its own tested isolation/runtime loading setup.

To repeat on the owner's machine, copy the local assets privately, install this
harness there, and optionally set `DMZ_BROWSER_EXECUTABLE` to the local Brave
binary. Use a fresh report path. This still benchmarks a clean headless browser,
not the owner's live supported AI sites tabs or the installed Shield extension.

## Full browser accuracy replay

The initial int8 native/browser numerical discrepancy triggered a full replay of
both fixed development sets. This additional pass uses the exact same graph and
thresholds. It resolves runtime-specific accuracy, while tokenization and the CRF
merge remain native/offline. It does not establish end-to-end extension quality.

```bash
$PY bench/browser-detector/corpus.py prepare --assets "$ASSETS" \
  --reference-dir "$RECALL" \
  --general-input /root/.local/share/dmz/datasets/prepared-v2/dev.jsonl \
  --vectors "$ASSETS/browser-corpus.json"
DMZ_MODEL_ASSETS="$ASSETS" DMZ_BROWSER_CORPUS="$ASSETS/browser-corpus.json" \
  DMZ_BROWSER_PREDICTIONS="$ASSETS/browser-corpus-predictions.json" \
  node bench/browser-detector/corpus.mjs
$PY bench/browser-detector/corpus.py score --assets "$ASSETS" \
  --reference-dir "$RECALL" \
  --general-input /root/.local/share/dmz/datasets/prepared-v2/dev.jsonl \
  --vectors "$ASSETS/browser-corpus.json" \
  --predictions "$ASSETS/browser-corpus-predictions.json" \
  --report /tmp/chromium-accuracy.json
```

The vector file encodes source text as token IDs and must be treated as private
corpus data. Neither vectors nor predictions belong in Git.

The default timing matrix includes both exports at one and three WASM threads.
To add one missing measurement without repeating the complete matrix, set e.g.
`DMZ_BENCH_CASES=fp32:3 DMZ_BENCH_SINGLE_ONLY=1`. Use a separate report file.
The original report has FP32/1 and int8/1/3; `chromium-fp32-threads3.json` records
the additional FP32/3 run, taken after corpus replay ended to avoid contention.

## Fine-tuned reference comparisons

The new training round is defined in
[`full-precision-protocol.md`](../contextual/full-precision-protocol.md).
`export_finetuned.py --base ... --checkpoint ... --selection ... --output ...`
requires a frozen selection manifest and verifies checkpoint/base hashes. It
exports FP32 only, without the old cache's float16 pooling round-trip.
`compress_frozen.py --assets ... --selection ... --recipe int8|int4|mixed` separately
checks the selection and FP32 graph hashes before creating a new candidate.
The mixed recipe uses int8 attention and int4 feed-forward weights. Acceptance
requires measured quality; graph size alone is insufficient.

Native `evaluate.py` accepts `--selection`. Browser `corpus.py` additionally accepts
`--model fp32|int8|int4|mixed` and `--neural-prefix` for the selected reference caches.
Use the same selection and prediction hashes throughout. Set `DMZ_MODEL_NAME`
for `corpus.mjs`; it defaults to the historical int8 replay. `fixtures.py --models`
selects graphs for fabricated runtime measurements, and `DMZ_MULTI_MODEL` selects
the graph used in the three-tab run. Use new output paths, and perform timing and
memory measurements after training has stopped.

Score FP32 Chromium replay with `--cache-output /outside/fp32-browser-spans.json`.
Pass that file as `--browser-reference` when scoring each compressed Chromium run.
This reports both native-reference differences and compression changes relative
to FP32 in the **same browser/runtime/thread/batch configuration**, including added
misses, recoveries and false-positive changes. A native reference is not a
substitute for this same-runtime comparison. The decoded span cache is private.

The extended recipes `int4b16`, `int4a32` and `hqq4` retain FP32 activations and
are predeclared in the same protocol. To measure additional graphs without
replacing earlier fabricated fixtures, use `fixtures.py --output /outside/new.json`
and `DMZ_BROWSER_FIXTURES=/outside/new.json`. New runtime reports hash that fixture
file; prior fixtures and reports remain intact.

`../contextual/quantized_repair.py --probe` checks the frozen int4 training
backbone against real ORT outputs on training-only chunks before correction
training. `export_repair.py` requires a complete correction checkpoint and adds
its FP32 matrices to the original int4 graph without requantizing its backbone.
Export completed epochs as `repair1.onnx` / `repair2.onnx` for the same native and
Chromium evaluation tools. These are separately trained compression-recovery
candidates, not revisions to the frozen FP32 reference.

For an original-typography domain check, use fresh private asset/reference/output
paths, `evaluate.py --text-mode original`, and `corpus.py --text-mode original
--sets domain`. Supply original-text CRF and frozen prediction caches. First record
a separate FP32 Chromium span reference in that text mode, then compare the
compressed finalist against it. Text-mode metadata prevents canonical/original
cache mixing; old unmarked caches are treated as canonical. This is another reused
development check, not new independent labels.
