# Auxiliary Person supervision result

The single candidate fails the fixed research comparison. Retain the frozen reference and close this recipe without threshold, priority or auxiliary-weight search.

The [fixed protocol](auxiliary-person-protocol.md) retains the primary three-label Person head and uses a separate nine-label training head with weight 0.25. The complete single epoch used the unchanged 46,196 training rows, seed 45, 1,445 optimizer steps and 824,869 nonpadding word targets in 7074.76 seconds. The auxiliary head is not used in inference. Primary initialization parity was exact before optimization; all saved primary and auxiliary tensors pass the finite-FP32 audit.

## Reused development comparison

| Stratum | Model | Exact excerpts | Accuracy | TP/FP/FN | Recall | Precision |
| --- | --- | --- | --- | --- | --- | --- |
| general | Frozen | 5502/5582 | 98.5668% | 1461/68/45 | 97.0120% | 95.5526% |
| general | Auxiliary trial | 5503/5582 | 98.5847% | 1460/66/46 | 96.9456% | 95.6750% |
| domain | Frozen | 250/256 | 97.6563% | 474/3/6 | 98.7500% | 99.3711% |
| domain | Auxiliary trial | 253/256 | 98.8281% | 477/2/3 | 99.3750% | 99.5825% |

The rule requires both strata to preserve frozen exact excerpt accuracy and precision, and a strict improvement in lower-stratum recall. Failed criteria:

- lower-stratum recall did not improve.

Both strata meet both separate ≥99% point targets: **no**. These are repeatedly reused Person-only development sets, not independent all-entity Shield acceptance. General text is canonical token-spaced text; domain text retains its original formatting. Thresholds remain CRF 0.75 and neural 0.0 with CRF overlap priority. Raw predictions and weights stay private.

The original orchestration runner completed training and the single general evaluation, then stopped because it requested a training-manifest field absent from Person reports. A guarded finisher retained that evaluation, validated checkpoint/code/source/selection/tokenizer/CRF-cache identities and finite confidences, performed domain evaluation once, and applied the unchanged comparison. No inference was repeated and no frozen training/evaluation code changed during the run.

- [Training record](../results/quality-99-v1/auxiliary-training.json).
- [Complete comparison and identity audits](../results/quality-99-v1/auxiliary-comparison.json).
- [General report, including neural-only metrics](../results/quality-99-v1/auxiliary-general.json).
- [Domain report, including neural-only metrics](../results/quality-99-v1/auxiliary-domain.json).
- [Completed run provenance](../results/quality-99-v1/auxiliary-completed.json).

Candidate primary checkpoint SHA-256: `d15385f11674b2c0fd8008bae6db359fe807dc9f406c03350e5ed4f62f5b379c`. The packaged frozen model remains unchanged; this research comparison does not approve a replacement.
