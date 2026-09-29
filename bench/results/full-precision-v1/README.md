# Frozen FP32 and compression review

**Status: reused-development results only; overall goal not complete.** The domain set is synthetic legal/financial text. Independent human labels and representative real-document validation remain unavailable. No model is approved for deployment.

For review, `fp16storage` is the smallest measured graph with zero observed FP32 span changes: 237.044 MiB. `repair2` is the highest-ranked sub-100-MiB candidate at 98.945 MiB; it meets both development floors but loses one net general true span versus FP32. Neither result establishes independent quality.

## Actual Chromium exact-span accuracy

| Graph | Graph MiB | General precision / recall | Domain precision / recall | Development floors |
| --- | ---: | --- | --- | --- |
| fp32 | 473.032 | 95.5526% / 97.0120% | 99.3711% / 98.7500% | pass |
| int8 | 119.642 | 95.6152% / 97.0120% | 99.3671% / 98.1250% | pass |
| int4 | 88.750 | 95.7532% / 95.8167% | 99.1489% / 97.0833% | pass |
| mixed | 99.120 | 95.1666% / 96.7463% | 99.5798% / 98.7500% | fail |
| int4b16 | 98.875 | 95.8968% / 96.2151% | 98.9451% / 97.7083% | pass |
| int4a32 | 90.021 | 96.1282% / 95.6175% | 99.3603% / 97.0833% | pass |
| hqq4 | 98.880 | 96.0265% / 96.2815% | 99.1507% / 97.2917% | pass |
| repair1 | 98.945 | 95.3625% / 96.9456% | 99.5816% / 99.1667% | fail |
| repair2 | 98.945 | 95.7377% / 96.9456% | 99.5798% / 98.7500% | pass |
| fp16storage | 237.044 | 95.5526% / 97.0120% | 99.3711% / 98.7500% | pass |
| int8fp16 | 156.639 | 95.6094% / 96.8792% | 99.3711% / 98.7500% | pass |

The frozen FP32 epoch-2 reference remains unchanged. It was selected from 13 full-precision composition candidates after full encoder training, a fixed average and a bounded refinement. Its checkpoint is 472.862 MiB; the ONNX graph is 473.032 MiB. See [selected-candidate.json](selected-candidate.json), [final-comparison.json](final-comparison.json) and [stability.json](stability.json) for immutable identities and selection evidence. General FP32 precision has a narrow margin above the floor.

Thresholds are unchanged at CRF 0.75 / neural 0.0, using CRF priority with non-overlapping neural spans added. Training and evaluation inputs remain separate. Compression correction training uses the original training mixture; it does not make these repeatedly used development sets independent.

Measured compressed graphs preserving every reference aggregate precision/recall value: fp16storage. Equal aggregate scores do not prove identical predictions; span changes follow below.

The minimum-recall ranking among precision-qualified sub-100-MiB graphs is: repair2, hqq4, int4b16, int4, int4a32. This is a development ranking, not release acceptance.

## Compression losses against frozen FP32

| Graph | Set | TP / FP / FN | Precision / recall delta, pp | Added misses / recovered | Added FP / removed | Native/browser changed records |
| --- | --- | --- | --- | --- | --- | ---: |
| int8 | general | 1461 / 67 / 45 | +0.0625 / +0.0000 | 1 / 1 | 5 / 6 | 4 |
| int8 | domain | 471 / 3 / 9 | -0.0040 / -0.6250 | 4 / 1 | 2 / 2 | 2 |
| int4 | general | 1443 / 64 / 63 | +0.2005 / -1.1952 | 20 / 2 | 10 / 14 | 0 |
| int4 | domain | 466 / 4 / 14 | -0.2221 / -1.6667 | 8 / 0 | 4 / 3 | 0 |
| mixed | general | 1457 / 74 / 49 | -0.3861 / -0.2656 | 4 / 0 | 11 / 5 | 3 |
| mixed | domain | 474 / 2 / 6 | +0.2088 / +0.0000 | 1 / 1 | 1 / 2 | 1 |
| int4b16 | general | 1449 / 62 / 57 | +0.3441 / -0.7968 | 15 / 3 | 5 / 11 | 0 |
| int4b16 | domain | 469 / 5 / 11 | -0.4259 / -1.0417 | 5 / 0 | 4 / 2 | 0 |
| int4a32 | general | 1440 / 58 / 66 | +0.5755 / -1.3944 | 23 / 2 | 6 / 16 | 0 |
| int4a32 | domain | 466 / 3 / 14 | -0.0107 / -1.6667 | 8 / 0 | 2 / 2 | 0 |
| hqq4 | general | 1450 / 60 / 56 | +0.4738 / -0.7304 | 14 / 3 | 9 / 17 | 0 |
| hqq4 | domain | 467 / 4 / 13 | -0.2203 / -1.4583 | 7 / 0 | 2 / 1 | 0 |
| repair1 | general | 1460 / 71 / 46 | -0.1901 / -0.0664 | 3 / 2 | 9 / 6 | 0 |
| repair1 | domain | 476 / 2 / 4 | +0.2105 / +0.4167 | 2 / 4 | 0 / 1 | 0 |
| repair2 | general | 1460 / 65 / 46 | +0.1851 / -0.0664 | 3 / 2 | 4 / 7 | 0 |
| repair2 | domain | 474 / 2 / 6 | +0.2088 / +0.0000 | 1 / 1 | 1 / 2 | 0 |
| fp16storage | general | 1461 / 68 / 45 | +0.0000 / +0.0000 | 0 / 0 | 0 / 0 | 0 |
| fp16storage | domain | 474 / 3 / 6 | +0.0000 / +0.0000 | 0 / 0 | 0 / 0 | 0 |
| int8fp16 | general | 1459 / 67 / 47 | +0.0568 / -0.1328 | 3 / 1 | 8 / 9 | 5 |
| int8fp16 | domain | 474 / 3 / 6 | +0.0000 / +0.0000 | 1 / 1 | 1 / 1 | 3 |

## Storage scope

Tokenizer, CRF and WASM inventory adds 18.598 MiB to each graph. These are uncompressed file sums, not a packaged extension or network-transfer measurement. No model below 50 MiB was obtained.

| Graph | Graph plus supporting inventory MiB |
| --- | ---: |
| fp32 | 491.631 |
| int8 | 138.240 |
| int4 | 107.349 |
| mixed | 117.718 |
| int4b16 | 117.474 |
| int4a32 | 108.619 |
| hqq4 | 117.479 |
| repair1 | 117.544 |
| repair2 | 117.544 |
| fp16storage | 255.643 |
| int8fp16 | 175.238 |

## Extended Chromium runtime


Report: [extended-runtime.json](extended-runtime.json).

Chromium 153.0.8010.12 / onnxruntime-web 1.23.2; CPU WASM, batch one. Each warm latency is the median of five fabricated pretokenized runs. See [extended-runtime.json](extended-runtime.json) for individual cases and failures.

| Graph | Threads | Load wall ms | Warm ms, 32 / 128 / 512 words | Peak summed RSS MiB |
| --- | ---: | ---: | --- | ---: |
| fp32 | 3 | 6370 | 261.6 / 927.2 / 3135.8 | 2093.1 |
| int8 | 3 | 2151 | 144.6 / 425.6 / 1687.0 | 922.6 |
| int4b16 | 3 | 2141 | 481.4 / 986.4 / 3875.1 | 849.9 |
| int4a32 | 3 | 1950 | 487.8 / 999.6 / 4126.1 | 842.8 |
| hqq4 | 3 | 2346 | 402.5 / 984.9 / 3818.5 | 870.6 |
| repair1 | 3 | 2288 | 430.7 / 969.6 / 3938.8 | 964.9 |
| repair2 | 3 | 2897 | 546.7 / 1071.8 / 4184.4 | 869.1 |
| fp16storage | 3 | 5986 | 244.4 / 835.7 / 3205.8 | 1976.2 |

Failure checks: 20/20 passed. All run cases succeeded: True. External requests: 0.

Three independent repair2 tab workers, one thread each: peak summed RSS 2044.5 MiB; inference wall times [11530.6, 11369.6, 11554.1] ms. This measures separate model copies, not a shared extension worker.

Report: [int8fp16-runtime.json](int8fp16-runtime.json).

Chromium 153.0.8010.12 / onnxruntime-web 1.23.2; CPU WASM, batch one. Each warm latency is the median of five fabricated pretokenized runs. See [int8fp16-runtime.json](int8fp16-runtime.json) for individual cases and failures.

| Graph | Threads | Load wall ms | Warm ms, 32 / 128 / 512 words | Peak summed RSS MiB |
| --- | ---: | ---: | --- | ---: |
| fp32 | 3 | 6307 | 257.1 / 777.8 / 3129.6 | 2093.7 |
| int8fp16 | 3 | 3962 | 141.4 / 435.0 / 1814.6 | 1599.0 |

Failure checks: 8/8 passed. All run cases succeeded: True. External requests: 0.

Three independent int8fp16 tab workers, one thread each: peak summed RSS 4017.4 MiB; inference wall times [5603.3, 5090.0, 5425.7] ms. This measures separate model copies, not a shared extension worker.

Memory is sampled Linux Chromium process-tree RSS; shared pages may be counted more than once. Load uses a fresh worker/context and no-store loopback HTTP in one browser process, not a cold OS cache. Accuracy replay uses native pretokenization and cached CRF merging. Timings exclude tokenization, CRF, masking and Shield integration. Failure tests exercise the harness, not an end-to-end Shield release gate. The original four-graph runtime and comparison reports remain preserved.

## Original UTF-8 domain checks

| Graph | Domain precision / recall | Changed records from native |
| --- | --- | ---: |
| fp32 | 99.3711% / 98.7500% | 0 |
| repair2 | 99.5798% / 98.7500% | 0 |
| fp16storage | 99.3711% / 98.7500% | 0 |
| int8fp16 | 99.3711% / 98.7500% | 3 |

These use the same reused synthetic domain examples with original text retained, not an independent corpus.

## Remaining evidence and review

- Independent human labels and document-disjoint real legal/financial validation.
- End-to-end tokenizer/CRF/Shield integration and owner-hardware validation.
- No measured sub-100-MiB graph preserving all frozen-reference aggregate precision/recall values.

The private blind-review-v2 annotation pilot contains 42 unlabelled records from nine documents. Source/text hashes are intact; extraction, source-use and human annotation review remain pending. Null labels are not negatives. See [../../holdout/README.md](../../holdout/README.md).

Weights, datasets, token vectors, individual predictions and detailed logs stay outside Git. Aggregate reports pin hashes. Both correction checkpoints and failed exports/experiments are preserved. The commuted ONNX head-bias export bug was fixed and both trained exports passed their training-chunk implementation probes before evaluation. Shield code remains unchanged. Draft PR #11 is stacked on open, unmerged PR #10; no merge or deployment was performed.

The optional `repairhalf` trial was skipped because neither corrected parent preserved every frozen-reference precision/recall value; [followup-decision.json](followup-decision.json) pins the actual guard decisions. [final-integrity.json](final-integrity.json) rechecks 19 baseline, checkpoint, input and tokenizer artifacts, the unchanged freeze, repository boundaries and unlabelled review-packet integrity.

See [extended-comparison.json](extended-comparison.json) for artifact identities, ranking and pending/skipped stages, and [../../contextual/full-precision-protocol.md](../../contextual/full-precision-protocol.md) for recipes.
