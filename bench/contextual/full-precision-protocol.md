# Full-precision training round

Started 28 September 2026, from `c6a8b0d`; PR #10 remains open and unmerged.
The old selected checkpoint, development reports, and browser exports are retained.
No Shield behavior changes are part of this experiment.

## Order and selection

1. Measure CPU/memory feasibility with a ten-optimizer-step training-only probe.
   The probe is not a model candidate and does not inspect development labels.
2. Fine-tune all IceBERT encoder parameters and the existing epoch-12 BIO head
   in FP32. Remove the earlier embedding-cache float16 round-trip. Mean-subword
   pooling and <=256-subword word-boundary chunks remain unchanged.
3. Initial bounded recipe: seed 42, three epochs, AdamW, encoder LR 1e-5,
   head LR 1e-4, weight decay .01, 10% linear warmup then linear decay,
   batch 8 with four-batch gradient accumulation, gradient norm cap 1.
   Length buckets reduce padding; all training chunks appear once per epoch.
   Gradients average per-microbatch token losses; they are not a single
   token-weighted loss across the accumulated batch.
4. CPU time may limit complete epochs. Record partial epochs explicitly and keep
   failed experiments. The training command never reads development input.
5. Evaluate every retained epoch on the two **reused development sets**, separately.
   Keep CRF 0.75, neural 0.0 and CRF overlap priority fixed. Require both datasets'
   precision >=95.5%, then maximize the lower recall; retain the old reference if
   no new candidate improves this rule. No threshold grid search in this round.
6. Freeze the selected full-precision checkpoint, all component hashes, thresholds,
   and input identities before compression. Recheck reproducibility of its scores.
   Only then test compression at unchanged thresholds, including actual Chromium.

Training uses only the existing 46,196-record MIM+synthetic training mixture.
Training, evaluation, weights, individual predictions and research logs are local
outside Git. The implementation refuses wrong row splits and verifies checkpoint
and encoder hashes. Aggregate reports do not imply independently validated quality.

## Independent evidence

The old MIM test split is exposed. The 36 sampled public-document pages remain
unlabelled and have not been sent through a model. Prepare a blind review package;
do not infer negatives from null labels or use agent labels as independent gold.
Whole-document separation, extraction review, leakage review and independent human
annotation are outstanding. The broader quality goal cannot be declared achieved
from development scores alone.

## Runtime and stopping

Use the existing local CPU only, no paid services. Reserve time within the owner's
eight-hour limit for browser evaluation, reports, PR and canonical vault checkpoint.
Record changes to this recipe after the feasibility probe explicitly. Stop jobs at
the deadline and report missing measurements rather than presenting estimates as
observations. No automatic merge, deployment, public research sync, or Shield release.

### Feasibility adjustment before development evaluation

Ten optimizer steps (40 microbatches, 4,869 tokens) took 47.17 seconds after
preparation, with no nonfinite losses or allocation failure. The selected run is
therefore **two epochs**, capped at 18,000 training seconds. The probe's three-epoch
scheduler was only a feasibility measurement. The actual two-epoch run starts from
the unchanged baseline again and uses a two-epoch scheduler. No development scores
were examined to make this adjustment.

## Compression recipes reserved until after freeze

Two bounded candidates are prepared, but must not run until the selected FP32
checkpoint is frozen and exported with its selection hash. Keep the small decision
head in FP32 in both cases. No calibration set or confidence retuning is used.

- Dynamic unsigned 8-bit constant MatMul and Gather weights. This tests a different
  arithmetic path from the rejected signed-weight run; it is not presumed to fix
  the earlier browser recall loss and may remain above the size target.
- Weight-only symmetric 4-bit MatMul, block size 32, FP32 activations and head,
  with unsigned 8-bit Gather weights. Browser operator support and quality are
  measured, not assumed. Failure to load is a recorded failed experiment.

The int4 recipe explicitly requests `accuracy_level=1`. The pinned
[ORT 1.23.2 CPU implementation](https://github.com/microsoft/onnxruntime/blob/v1.23.2/onnxruntime/contrib_ops/cpu/quantization/matmul_nbits.cc)
selects FP32 compute for this setting (level 4 may select activation int8).
Actual WASM support, scores and timings still require measurement.

Report graph bytes, tokenizer/CRF bytes, and runtime asset bytes separately so a
small graph is not confused with a complete extension download. Full development
Chromium replay is required for FP32 and any compressed candidate being considered
acceptable. Native parity alone cannot establish browser accuracy. All such scores
remain reused-development evidence; the independent-review gate is separate.

## Original-text check

Before freezing, also evaluate the chosen model on the synthetic domain set's
`original_text` and `original_entities`, with the existing offset-preserving
tokenizer and matching cached CRF spans. `finetune.py evaluate --text-mode original`
keeps those UTF-8 byte offsets rather than scoring canonical spaced text. This is
a typography/offset regression check on the same generated development examples,
not independent evidence of production tokenization or document accuracy.

## Standalone candidate inclusion before first epoch results

The evaluator already reports both the neural-only detector and CRF combination.
Include both forms of each retained epoch in selection, alongside the preserved
baseline composition. This yields four new candidates for two complete epochs.
Use neural threshold 0.0 in both forms; CRF 0.75 and overlap priority remain fixed
when CRF is used. Apply the same two-dataset precision floor / lower-recall rule.
This inclusion was recorded before any fine-tuned development result was available.
An improved encoder may no longer benefit from CRF, so do not force that component
into the final candidate. Report which composition wins and compare compression
losses for that composition, retaining both neural and combined diagnostics.

## Fixed parameter average before epoch scores

Add one equal FP32 parameter average of the two complete fine-tuned checkpoints,
using `average_finetuned.py`. This is fixed before either epoch's development
scores are available. Do not choose a favorable subset or include partial epochs.
Evaluate the averaged model alone and with the unchanged CRF, retaining failure
results. This adds two composition candidates (six new candidates total for the
two-epoch run); it does not produce new independent evidence. Averaging weights
does not itself establish stability or quality, which must still be measured.

The selected manifest must also pin the local tokenizer/configuration files.
Export validates these before loading the tokenizer and records the exported
tokenizer hashes. Native evaluation and browser-vector preparation check those
hashes; the browser then binds the private token vectors by checksum. Keep the
selection manifest immutable after freeze and put later measurements in separate
reports.

## Bounded FP32 refinement after the two-epoch comparison

The combined detector improved general development recall from 96.55% to 97.01%
and domain recall from 97.71% to 98.75% between epochs 1 and 2. General precision
fell from 96.10% to 95.55%, so another round is an experiment, not an assumed gain.
Before freezing, run one additional full FP32 epoch initialized from epoch 2, with
fresh AdamW state, encoder LR 5e-6, head LR 5e-5, seed 43, the same mixture and
batch/accumulation settings, and a 7,200-second cap. Keep the same 10% warmup and
linear decay. This is a warm start, not continuation of saved optimizer state.

This adjustment is development-informed and adds two candidates (neural-only and
the fixed CRF combination). Retain all earlier checkpoints and the predeclared
epoch-1/2 average. Apply the original selection rule without threshold search;
reject a refinement that does not improve it. Reserve the remaining time for
freezing, compression, actual browser measurements, and the review package.

## Development-informed overlap-priority diagnostic before freeze

The epoch-2 aggregate audit found seven exact general-text neural spans blocked by
CRF overlap priority. Test the deterministic reverse ordering for every new FP32
checkpoint (epochs 1 and 2, their fixed average, and the bounded refinement), with
neural threshold 0.0 and CRF threshold 0.75 unchanged. Neural spans take priority;
only non-overlapping CRF spans are added. Keep the original combinations and
standalone neural candidates eligible under the same selection rule. This adds
four development-selected composition candidates, not independent evidence or a
threshold search. Record failures and only use this ordering for compression if
it wins and is explicitly frozen as part of the selected reference.

## Bounded mixed compression follow-up

After freeze and full Chromium FP32 parity, the initial int4 graph measures
88.750 MiB but native recall falls to 95.82% general and 97.08% domain. Add one
architecture-defined alternative: dynamic unsigned 8-bit attention MatMul and
Gather, weight-only 4-bit feed-forward MatMul (block 32, FP32 feed-forward
activations), and the unchanged FP32 head. This is `mixed`, a separate artifact;
it does not replace the int4 failure/quality evidence. No calibration, retraining,
per-example layer selection, or threshold adjustment is used. Compare it against
the same immutable FP32 Chromium reference and retain size and quality failures.

## Extended bounded int4 comparison

The owner authorized eight additional hours on 29 September 2026, through
15:16:30 UTC. The initial four-graph comparison is complete and retained in
`compression-comparison.json`: no compressed replacement is accepted. The FP32
reference, datasets, thresholds and CRF priority remain immutable.

Before further evaluation, predeclare two weight-only alternatives to int4:
`int4b16` uses symmetric blocks of 16 instead of 32, increasing scale overhead
for potentially lower weight error; `int4a32` uses asymmetric blocks of 32.
Both preserve FP32 activations (`accuracy_level=1`), the FP32 head and the same
U8 Gather recipe. These are development-informed experiments, not independent
validation. No threshold search, calibration or retraining is involved. Preserve
all results; assess native scores first and perform actual Chromium comparisons
for any candidate considered for selection. Do not infer quality from byte size.
ONNX Runtime documents block-wise weight-only options at
https://onnxruntime.ai/docs/performance/model-optimizations/quantization.html;
the implementation and runtime remain pinned to the installed 1.23.2 version.

### Weight-error optimization alternative

Also predeclare one `hqq4` candidate using the installed ONNX Runtime 1.23.2 HQQ
implementation: four-bit MatMul weights, block 32, axis 1, default weight-only
optimization (up to 20 iterations), FP32 floating zero points, unchanged U8
Gather and FP32 head. HQQ consumes no calibration examples or evaluation labels.
Force `accuracy_level=1` on generated MatMulNBits nodes because this installed HQQ
path does not propagate that constructor option. Verify native and actual browser
support for floating zero points before considering selection. This optimizes
weight reconstruction, not an asserted guarantee of task-quality preservation.
Source: https://github.com/microsoft/onnxruntime/blob/v1.23.2/onnxruntime/python/tools/quantization/matmul_nbits_quantizer.py

## Compression-aware correction training, separately gated

If post-training recipes do not preserve the reference quality, prepare one
bounded recovery experiment on the original block-32 symmetric int4 graph. Decode
its fixed backbone weights into FP32 for CPU training and add rank-16 FP32 linear
corrections to all 72 encoder matrix layers (scale 1, random A and zero B). Train
only those corrections and the BIO head; the quantized backbone is frozen.
This adds 2,654,208 correction parameters (10.125 MiB), before graph overhead.
The byte size must be measured rather than inferred from that estimate.

Predeclare two epochs on exactly the original training mixture, seed 44, fresh
AdamW with weight decay 0.01, adapter LR 1e-4, head LR 5e-5, batch 8 with four-way
accumulation, 10% warmup/linear decay, clipping 1.0 and a 14,400-second cap.
Training has no evaluation input or threshold-selection path. Gate startup on a
real ORT-versus-decoded-backbone numerical probe using eight training chunks.
Save only trainable FP32 tensors; incomplete epochs are not eligible for export.
The exported graph attaches additive correction nodes to the preserved int4
matrix operations and replaces the trained head, without requantizing backbone
initializers. Require native and Chromium evaluation after training, with the
same frozen FP32 reference, thresholds, CRF priority and development disclosure.
Apply the existing precision-floor / minimum-recall selection rule to completed
checkpoints; preserve failures. Do not modify the original FP32 selection.

After exporting each completed correction checkpoint, repeat the training-chunk
numerical probe with `--probe --repair-state ... --probe-graph ...`. It compares
the trained PyTorch corrections with their exported ONNX graph and verifies both
artifact hashes. This is an implementation parity check on training inputs,
not a quality evaluation. Development scoring remains in the separate native
and Chromium evaluation commands.

## Original UTF-8 compressed-finalist check

The frozen FP32 PyTorch stability check already covers original domain text.
Also replay original domain text through native ONNX and Chromium for FP32 and
the compressed finalist, at unchanged thresholds and with original-mode CRF
caches. Keep this separate from the canonical-text comparison, with explicit text
mode metadata and a separate FP32 browser span reference. This is reused domain
development data, not an independent legal/financial evaluation.
