# Frozen FP32 and compression review

**Status: development targets met by FP32; overall goal not complete.** All scores are exact Person spans on reused development data. The domain set is synthetic legal/financial text. These are not independent validation or production-readiness results.

## Actual Chromium accuracy

| Graph | MiB | General precision / recall | Domain precision / recall |
| --- | ---: | --- | --- |
| fp32 | 473.032 | 95.5526% / 97.0120% | 99.3711% / 98.7500% |
| int8 | 119.642 | 95.6152% / 97.0120% | 99.3671% / 98.1250% |
| int4 | 88.750 | 95.7532% / 95.8167% | 99.1489% / 97.0833% |
| mixed | 99.120 | 95.1666% / 96.7463% | 99.5798% / 98.7500% |

FP32 epoch 2 was selected from 13 candidates after two full encoder epochs, a fixed parameter average, a bounded lower-rate refinement, and fixed merge-priority alternatives. The refinement and priority alternatives did not improve the predeclared selection rule. `final-comparison.json` preserves every candidate. General FP32 precision has a narrow margin above the floor.

The checkpoint has 123,951,875 parameters, all 203 tensors FP32, and is 472.862 MiB. It was frozen before compression at 05:01 UTC on 29 September 2026. `selected-candidate.json` pins checkpoint, tokenizer/configuration, thresholds, input and stability hashes and remains immutable. Batch-8/batch-1 PyTorch spans, native ONNX spans and FP32 Chromium spans agree exactly on both sets; the separate original UTF-8 domain PyTorch check also agrees. Original-text native and Chromium checks are tracked separately below.

## Compression decision

Retain frozen FP32 reference. Int8 is closest compressed candidate but exceeds 100 MiB and loses domain recall. Int4 meets graph size with material recall loss and slower inference; mixed fails general precision floor.

| Candidate | General added misses / recovered | Domain added misses / recovered | General / domain recall change, pp |
| --- | --- | --- | --- |
| int8 | 1 / 1 | 4 / 1 | +0.0000 / -0.6250 |
| int4 | 20 / 2 | 8 / 0 | -1.1952 / -1.6667 |
| mixed | 4 / 0 | 1 / 1 | -0.2656 / +0.0000 |

Thresholds remain CRF 0.75 / neural 0.0 with CRF priority throughout. The initial four compression recipes used no calibration labels, threshold retuning or retraining. Reports also include added/removed false positives and native/browser differences. Equal aggregate recall does not mean identical predictions.

Tokenizer, CRF and WASM inventory totals 18.598 MiB beyond each graph. Graph-plus-inventory sizes are fp32 491.631 MiB, int8 138.240 MiB, int4 107.349 MiB, mixed 117.718 MiB. These are uncompressed file sums, not packaged extension or network-transfer measurements. No 50 MiB model was obtained.

## Chromium runtime

Chromium 153.0.8010.12 / ONNX Runtime Web 1.23.2, CPU WASM, three threads, batch one. Warm figures are medians of five runs on fabricated pretokenized inputs after an initial run.

| Graph | Load ms | Warm ms, 32 / 128 / 512 words | Peak summed RSS MiB |
| --- | ---: | --- | ---: |
| fp32 | 6121 | 295.5 / 785.6 / 3082.8 | 2112.6 |
| int8 | 2420 | 134.8 / 406.5 / 1744.4 | 928.5 |
| int4 | 1871 | 445.2 / 1038.1 / 3774.9 | 846.5 |
| mixed | 2253 | 306.1 / 822.0 / 3109.0 | 874.8 |

Three independent int8 tabs: peak summed RSS 2051.9 MiB; inference wall times [4512.800000011921, 4580.9000000059605, 4472.800000011921] ms. Failure checks passed: 12/12. All run cases succeeded: True. External requests: 0.

RSS is sampled Linux Chromium process-tree RSS; shared pages may be counted more than once. Load uses a fresh worker/context and no-store loopback HTTP in the same browser process, not a cold OS cache. The three-tab result measures separate model copies, not a shared extension worker.

Accuracy replay uses native pretokenization and cached CRF merge; runtime timings exclude tokenization, CRF, masking and Shield integration. Failure tests cover malformed input, deadline termination, pending-request rejection, not-ready inference, corrupt models and worker crashes. They test this harness, not an end-to-end Shield release gate.

## Review and remaining work

The private blind-review-v2 pilot has 42 unlabelled records from nine documents. It awaits independent human annotation, extraction/source review and a document-disjoint sampling protocol. No candidate predictions were used to create gold labels. See `../../holdout/README.md`.

Weights, data, token vectors, individual predictions and detailed logs remain outside Git. Aggregate reports contain hashes. Baselines and failed experiments are preserved; Shield code is unchanged. Draft PR #11 is stacked on unmerged PR #10; no merge or deployment is authorized. See `../../contextual/full-precision-protocol.md` for the training and compression recipes.

## Extended investigation in progress

The owner authorized up to eight additional hours through 15:16:30 UTC on
29 September 2026. The four-graph comparison above remains preserved.

`int4b16` (symmetric blocks of 16) is 98.875 MiB. Actual Chromium reproduces its
native scores exactly: general 95.8968% precision / 96.2151% recall, synthetic
domain 98.9451% / 97.7083%. Against frozen FP32 it adds 15 general misses and
recovers three; it adds five domain misses and recovers none. General replay took
1804.530 seconds. It improves on the original int4 recall but is not an accepted
replacement. The asymmetric block-32 graph is 90.021 MiB. Chromium exactly matches native:
general 96.1282% precision / 95.6175% recall, domain 99.3603% / 97.0833%.
It adds 23 general misses and recovers two; it adds eight domain misses and
recovers none. General replay takes 2051.638 seconds. It is not selected.
The HQQ block-32 graph is 98.880 MiB. Chromium exactly matches native:
general 96.0265% precision / 96.2815% recall, domain 99.1507% / 97.2917%.
It adds 14 general misses and recovers three; it adds seven domain misses and
recovers none. General/domain replay takes 1576.711/120.143 seconds. It is not
an accepted replacement. The separate original-domain FP32 native/Chromium check
also reproduces all frozen spans exactly: 99.3711% precision / 98.7500% recall,
with Chromium replay taking 85.782 seconds and zero external requests.

The separate rank-16 correction-training job started at approximately 09:34 UTC.
It is bounded to 12,717 seconds to preserve evaluation/reporting time. No trained
checkpoint has been evaluated yet.
It keeps the int4 backbone fixed and trains only additive FP32 matrices plus the
BIO head on the original training mixture. The real initial implementation probe
passes on eight training chunks: maximum PyTorch/ORT logit difference 0.0000401,
zero changed word labels. This probe is not an accuracy evaluation. Hash guards
reject changed references and nontraining inputs; export guards preserve backbone
initializers and require complete checkpoints. See the predeclared protocol for
limits, subsequent export parity checks and separate development evaluation.

The conditional FP16-storage fallback generated successfully at 237.044 MiB.
It converts all 72 encoder matrix constants and three embedding tables, restoring
them to FP32 before computation while retaining the FP32 head. Numerical guard
tests pass. It has not been quality- or runtime-evaluated and is not an accepted
model; lower storage size does not imply lower runtime memory. Its short,
label-free conversion overlapped correction training, not browser measurements.

The optional `repairhalf` follow-up is prepared but not generated or evaluated.
It stores only correction matrices as FP16 with FP32 computation, preserving the
backbone and trained head. A command-level guard requires a parent Chromium
report preserving all reference precision/recall values on both development
sets. Any generated follow-up still requires fresh native and browser evaluation.
