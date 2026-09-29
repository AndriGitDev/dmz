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

`initial-comparison.json` also retains standalone neural scores. Epoch 2 currently
wins the predeclared precision-floor / lower-recall rule, but its general precision
margin is narrow. A separate, bounded FP32 refinement is running before selection
and freeze. See `../../contextual/full-precision-protocol.md` for the recipe and
development-informed adjustment. All earlier candidates remain eligible.

Each new checkpoint has 123,951,875 FP32 parameters and is 495,831,740 bytes
(472.862 MiB). This is a checkpoint size, not a browser graph or extension download.
No new candidate has been compressed or measured in Chromium yet.

The epoch-2 error audit motivated a fixed-threshold neural-overlap-priority
diagnostic. It did not improve any of the three completed candidates. For epoch 2
on general text, seven recovered gold spans were outweighed by nine newly missed
spans; precision fell to 95.23%. `priority-comparison.json` preserves this failed
alternative. The same diagnostic remains pending for the refinement checkpoint.

Weights, source records, predictions and detailed logs remain outside Git.
Aggregate reports identify input and model hashes. Shield behavior is unchanged.
