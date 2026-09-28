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
