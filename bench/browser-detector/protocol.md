# Frozen detector deployment benchmark

Scope: export/quantization parity and local browser feasibility, not another
training round. Do not touch the unlabelled public-document pilot or retune on it.
No model, tokenizer, raw corpus, embeddings or per-record predictions enter Git.
No website changes. Detailed reports remain in the private product repository.

Frozen reference: ../results/error-audit-v2/selected-candidate.json, CRF 0.75,
epoch-12 neural head 0.0, CRF priority on overlap. Do not tune thresholds to
compensate for compression. Run the existing MIM development and synthetic
legal/financial development sets in full. Compare TP/FP/FN and changed predicted
spans against recorded frozen predictions; label all measurements development-only.

Predeclared candidates: ONNX float32 export and dynamic int8 quantization of
constant-weight MatMul/Gather. Keep the head, mean-subword pooling and float16
round-trip from training. No pruning, retraining or threshold search. If operator
compatibility prevents a candidate running, record the failure and any revised
export separately. Accuracy regressions disqualify automatic release.

Browser: local loopback assets only, Chromium WASM CPU, single worker per tab as
a baseline. Fabricated short/medium/long prompts (32/128/512 words), cold session
creation and first run, warm runs, multi-tab concurrency, worker failure and
request deadline handling. Report what timing includes/excludes, actual runtime
versions, model hashes, thread count, process RSS observations and hardware.
Browser accuracy parity uses synthetic vectors; full development scores initially
use native ONNX Runtime. Do not call this full browser corpus validation.

No production Shield integration until deployment feasibility and whole-pipeline
correctness are demonstrated. The browser harness must reject loading failures,
invalid inputs, deadlines and worker crashes. It must not contain a fallback
that sends a raw prompt. Existing Shield interception regression tests remain
separate from this research harness. A headless server is not the owner's Brave
hardware and does not establish battery/low-memory behavior.

## Discrepancy-triggered extension

The initial fabricated replay showed int8 native/WASM absolute logit differences
up to 5.0743, despite matching labels. Therefore expand the same frozen model and
fixed development sets to complete **batch-1 Chromium WASM** replay, with three
threads. Keep tokenization native and merge cached CRF predictions offline. This
is a runtime correctness investigation, with no model/threshold changes or new
validation claim. Preserve the original native results and report browser scores
separately; do not substitute one runtime's accuracy for another's.
