# Full-precision development comparison (in progress)

All scores are exact Person spans on reused development data. They do not establish
independent quality or production readiness. The blind-review-v2 packet has 42
unlabelled records from nine documents and awaits independent human review.

| Fixed CRF combination | General P / R | Synthetic legal/financial P / R |
| --- | --- | --- |
| Preserved baseline | 95.92% / 95.29% | 98.51% / 96.25% |
| Full FP32 epoch 1 | 96.10% / 96.55% | 99.36% / 97.71% |
| Full FP32 epoch 2 | 95.55% / 97.01% | 99.37% / 98.75% |
| Equal FP32 average, epochs 1 and 2 | 95.80% / 96.88% | 99.58% / 98.54% |
| Bounded refinement from epoch 2 | 95.42% / 96.95% | 99.79% / 99.38% |

`initial-comparison.json` also retains standalone neural scores. Epoch 2 currently
wins the predeclared precision-floor / lower-recall rule, but its general precision
margin is narrow. The completed refinement fails the general precision floor in
combination; its standalone detector passes but has lower general recall.
`final-comparison.json` retains all 13 candidates, including the old baseline.
The reference was frozen at 05:01 UTC on 29 September 2026 after exact batch-8 to
batch-1 span parity on both sets and a passing original UTF-8 domain check. See
`../../contextual/full-precision-protocol.md` for the recipes and development-informed
adjustments.

`selected-candidate.json` is immutable: checkpoint, tokenizer/configuration,
thresholds, data identities, comparison and stability hashes are pinned. Later
browser and compression measurements belong in separate reports. The selected
checkpoint SHA-256 is `ec2ecc9579816fe059b83cc76cd61a18c7a6aad74253565fa98d0654536d8e9a`.

Each new checkpoint has 123,951,875 FP32 parameters and is 495,831,740 bytes
(472.862 MiB). This is a checkpoint size, not a browser graph or extension download.
The FP32 ONNX export is 496,010,299 bytes (473.032 MiB). Native parity is exact on
both datasets. Chromium 153 / ORT Web 1.23.2, three WASM threads and batch one,
also reproduces every selected combined span on both full development sets.
Replay took 764.101 seconds for general text and 81.025 seconds for domain text;
these are corpus replay times, not per-request warm latency measurements.
The browser scope uses native pretokenization and a cached CRF merge and does not
establish end-to-end Shield behavior.

The guarded int4 graph is 88.750 MiB; unsigned-int8 is 119.642 MiB; the mixed
attention-int8/feed-forward-int4 graph is 99.120 MiB. Int4 Chromium exactly matches
its native results: general 95.75% precision / 95.82% recall; synthetic domain
99.15% / 97.08%. Against frozen FP32 it adds 20 general misses (recovering two)
and eight domain misses (recovering none). General replay takes 1596.578 seconds,
versus FP32 764.101 seconds. It is not selected as a quality-preserving replacement.
Int8 and mixed accuracy and isolated runtime measurements remain in progress.
No compressed model is accepted from size alone. `supporting-assets.json` inventories tokenizer, CRF and WASM files separately
and is not a packaged-extension or compressed-transfer measurement.

The epoch-2 error audit motivated a fixed-threshold neural-overlap-priority
diagnostic. It did not improve any of the three completed candidates. For epoch 2
on general text, seven recovered gold spans were outweighed by nine newly missed
spans; precision fell to 95.23%. `priority-comparison.json` preserves this failed
alternative. The refinement's neural-priority alternative also fails to improve
the selection rule.

Weights, source records, predictions and detailed logs remain outside Git.
Aggregate reports identify input and model hashes. Shield behavior is unchanged.
