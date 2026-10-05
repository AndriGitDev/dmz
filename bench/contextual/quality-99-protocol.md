# Detector quality experiments for October

The [metric contract](../../docs/detector-quality-contract.md) requires 99% exact
entity-set excerpt accuracy and 99% recall. This round measures the existing
reference first, then tests bounded research candidates. All model selection uses
previously reused development sets; no result from this round is independent
validation. The blind web review packet remains untouched.

## Distinguishing other entity types during training

Predeclared on 4 October 2026, before candidate training or evaluation. The frozen
general Person audit records 29 false positives whose exact spans have another
entity type (6 Location, 10 Organization, 13 Miscellaneous), plus five overlapping
other entities. This motivates explicit other-type supervision rather than treating
every non-Person token as a single background class. It does not prove these errors
are fixable by this change.

Use the exact original 46,196-row training mixture, SHA-256
`9f5020c0ea357b6f85fca5162aa6d01d6328669c189ae67ddf9e27731a22387a`.
Initialize from the frozen FP32 epoch-2 checkpoint. Preserve its encoder and
hidden head layers. Expand the last linear layer from O/B/I to nine labels:
O, B/I-Person, B/I-Organization, B/I-Location, B/I-Miscellaneous. Preserve the
first three learned rows and initialize the other six with the seeded PyTorch
linear initializer. This changes the softmax distribution and is a new candidate,
not a parity-preserving conversion.

Train at most two complete epochs with seed 44, encoder LR 5e-6, head LR 5e-5,
batch 8, four-step gradient accumulation, weight decay .01, clipping 1.0, 10%
warmup and linear decay. Use FP32 and the existing mean-subword pooling.
Three CPU threads; maximum training duration 4.5 hours. Run a two-step training-only
probe first. Preserve partial checkpoints as incomplete experiments; do not select
them as completed candidates. The probe does not read development labels.

Evaluate both complete epoch checkpoints once on general development and original
synthetic domain text. Report Person predictions alone and the fixed CRF-priority
combination at CRF .75 / neural .0. Record exact excerpt accuracy, precision, recall,
counts, negative false detections and changes relative to the frozen reference.
Do not tune thresholds in this trial. Prefer candidates that improve the worse
stratum's recall while maintaining both strata's frozen precision and excerpt
accuracy; otherwise retain the reference. Also show the explicit 99% target result.

Any candidate considered for the extension needs a compatible exporter, original
text tokenization parity, actual browser comparison and all existing per-type
release gates. Nine-label checkpoints are research artifacts; the current
three-label package must remain unchanged until that work passes.

`compare_quality.py` applies this gate to the saved aggregate evaluations. It
checks complete checkpoint identities, fixed input/CRF hashes, text modes,
thresholds, annotation population, and ratios against integer counts. All
training manifests must also match the predeclared training-source hash, row
count, nine-label order, initialization mode and recorded hyperparameters;
duplicate or out-of-range epoch numbers fail. This verifies manifest claims,
not an independent attestation of how weights were produced. All
complete epochs in the supplied training manifest must be compared. A passing
result qualifies only for further research checks; it never approves browser
promotion or independent performance claims.

```sh
python bench/contextual/compare_quality.py \
  --training /outside/entity-training/training.json \
  --candidate /outside/epoch-1-general.json /outside/epoch-1-domain.json \
  --candidate /outside/epoch-2-general.json /outside/epoch-2-domain.json \
  --reference bench/results/full-precision-v1/selected-candidate.json \
  --baseline bench/results/quality-99-v1/general-frozen.json \
             bench/results/quality-99-v1/domain-frozen.json
```

## Completed result — 5 October 2026

**Retain the frozen reference.** Both epochs completed, and both were evaluated
once on each declared development set. Neither passes the unchanged research
gate. The second epoch improves domain recall and precision, but general
accuracy, recall and precision remain below the reference.

| Checkpoint | Stratum | Exact excerpts | Accuracy | TP / FP / FN | Recall | Precision |
| --- | --- | --- | --- | --- | --- | --- |
| Frozen reference | General | 5502/5582 | 98.5668% | 1461 / 68 / 45 | 97.0120% | 95.5526% |
| Epoch 1 | General | 5498/5582 | 98.4952% | 1459 / 72 / 47 | 96.8792% | 95.2972% |
| Epoch 2 | General | 5500/5582 | 98.5310% | 1460 / 71 / 46 | 96.9456% | 95.3625% |
| Frozen reference | Domain | 250/256 | 97.6563% | 474 / 3 / 6 | 98.7500% | 99.3711% |
| Epoch 1 | Domain | 250/256 | 97.6563% | 474 / 1 / 6 | 98.7500% | 99.7895% |
| Epoch 2 | Domain | 252/256 | 98.4375% | 476 / 1 / 4 | 99.1667% | 99.7904% |

The [comparison record](../results/quality-99-v1/entity-comparison.json) retains
integer counts, deltas, failed criteria and source hashes for both candidates.
The [training manifest](../results/quality-99-v1/entity-training.json) records
complete checkpoint identities and settings. Training took 13,855.85 seconds
(about 3 h 51 min), below the 16,200-second cap. Each epoch saw 824,869 labelled
tokens; the checkpoints are 495,834,836 bytes each and remain outside Git.

Separate aggregate evaluations preserve neural-only and combined results:
[epoch 1 general](../results/quality-99-v1/epoch-1-general.json),
[epoch 1 domain](../results/quality-99-v1/epoch-1-domain.json),
[epoch 2 general](../results/quality-99-v1/epoch-2-general.json) and
[epoch 2 domain](../results/quality-99-v1/epoch-2-domain.json).
The legacy `scores.*.target_met` field checks the older 95.5% precision/recall
criterion; only `quality.*` implements the owner's 99% exact-accuracy/recall
contract. Neither checkpoint meets both current targets on either stratum.

Epoch 1's general evaluation was deliberately suspended to give timed training
priority, then the same process resumed. Its elapsed time includes that pause
and must not be used as a latency measurement. The remaining evaluations ran
sequentially. No thresholds were tuned, no blind review data were scored, and no
candidate was exported or substituted into the extension. These results test
this specific training recipe on reused Person-scoped data; they do not rule
out other multi-entity training approaches or establish all-entity quality.

## Independent evidence

The existing general corpus is sentence-split, not document-disjoint, and the
domain corpus is synthetic. The new web sample needs human review and a complete
entity annotation protocol. Neither more epochs nor added scraped text removes
these limitations. Reserve new source material by document and related-article
cluster before any model-assisted training labels or predictions.
