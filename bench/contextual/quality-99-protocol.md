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

## Independent evidence

The existing general corpus is sentence-split, not document-disjoint, and the
domain corpus is synthetic. The new web sample needs human review and a complete
entity annotation protocol. Neither more epochs nor added scraped text removes
these limitations. Reserve new source material by document and related-article
cluster before any model-assisted training labels or predictions.
