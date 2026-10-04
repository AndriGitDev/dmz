# Detector accuracy and recall contract

On 4 October 2026 the owner confirmed **exact entity-set accuracy per excerpt**
as the meaning of accuracy. The research target is accuracy ≥99% and entity
recall ≥99%. The current frozen reference misses these targets on reused
development data. Independently labelled evaluation remains outstanding.

## Metric definitions

An excerpt is correct only when every annotated entity has exactly the predicted
UTF-8 byte start, exclusive end and entity type, and there are no extra predictions.
Accuracy is correct excerpts divided by all fully reviewed excerpts. A reviewed
negative excerpt is correct only when there are no predictions. Null annotations
are incomplete review and must never be counted as negatives.

Recall is TP/(TP+FN), pooled over exact entity matches within the declared scope.
Precision TP/(TP+FP), F1, confusion counts, per-type recall, positive-excerpt
accuracy and the rate of false detections on negative excerpts are required
diagnostics. Undefined denominators produce null, not a perfect score. Ordinary
token accuracy and precision cannot substitute for excerpt accuracy. A wrong
boundary or type produces both a false positive and a false negative.

The initial computational gate uses the two 99% **point estimates**. It does not
establish a population guarantee. Before an independent acceptance run, freeze
the sample design and uncertainty procedure, accounting for excerpts clustered
within documents and related articles. Report sample sizes and uncertainty with
any quality claim. Existing per-entity synthetic recall floors remain unchanged.

## Scope and sampling

The complete Shield target covers names, postal/street addresses, kennitölur,
email, Icelandic phone, IBAN, payment cards, supported secret patterns and explicitly
configured tenant terms. Unicode review is a protective policy signal rather than
a gold entity type. Each dataset must state exactly which types are fully labelled;
Person-only corpora can establish only Person-scoped metrics. Place mentions are
not automatically street addresses, and organization mentions are not automatically
personal names. Write annotation examples and resolve ambiguous boundaries before
freezing an all-entity sample.

The [proposed multi-entity rubric](../bench/holdout/entity-annotation-rubric.md)
includes contextual rural addresses and other currently unsupported forms. It
must not use present detector limitations to define those identifiers away.
Separate blind reviews and explicit adjudication are prepared by
`bench/holdout/entity_review.py`; these structural tools do not prove reviewer
independence or approve a final sample.

Evaluate general Icelandic, legal/financial text and mixed chat separately; do not
pool them into a headline claim to hide a weak stratum. Sampling must be frozen
before predictions, including the ratio of positive and negative excerpts and
excerpt length, because these directly affect exact-set accuracy. The intended
acceptance gate applies both targets to each declared primary stratum, with type
counts and regressions reported separately.

Keep the private 88-document, 142-excerpt web packet unlabelled and model-blind
until independent review. Its existing annotation instructions cover Person only;
it cannot establish all-entity Shield accuracy without a separately frozen,
complete annotation protocol. Require independent annotation, second review and
adjudication, immutable text and label hashes, document and article-cluster split
manifests, and overlap screening. Do not train or choose thresholds on final labels.

## Frozen reference baseline

The following scores replay existing prediction caches at CRF threshold 0.75 and
neural threshold 0.0, with CRF overlap priority. Source, selection, model-cache
identities and gold labels are checked before scoring. No new inference or tuning
is needed for this measurement. These are **Person-only, reused-development**
results; general text is reconstructed token-spaced text, while domain text retains
its original formatting.

| Dataset | Correct excerpts | Accuracy | TP / FP / FN | Recall | Precision |
| --- | --- | --- | --- | --- | --- |
| General Icelandic | 5502 / 5582 | 98.5668% | 1461 / 68 / 45 | 97.0120% | 95.5526% |
| Synthetic legal and financial | 250 / 256 | 97.6563% | 474 / 3 / 6 | 98.7500% | 99.3711% |

General positive-excerpt accuracy is 1046/1094 (95.6124%); 32/4488 negative
excerpts have a false detection. Domain positive-excerpt accuracy is 186/192
(96.8750%); none of its 64 negatives has a false detection. The many general
negatives make overall accuracy look stronger than positive-excerpt accuracy.

Reproduce using `bench/contextual/quality_frozen.py` with the private source,
CRF cache, neural cache and frozen selection; select `--dataset general` or
`--dataset domain`. The scorer prints aggregates and hashes only. It does not
read blind packets or modify weights. `bench/contextual/quality.py` supplies the
shared metric implementation and rejects incomplete labels, duplicate spans and
predictions outside the declared annotation scope.

## System acceptance

Detection scores do not measure the whole extension. Separately measure complete
identifier coverage in actual outbound text, partial residual exposure, benign
false masking, restoration correctness, session isolation, races and fallback.
Run the packaged browser artifact with the approved address vocabulary enabled;
record its hashes. Keep the quality-first full-precision model frozen during rule
experiments, and compare each candidate against that same starting artifact.

Independent quality evidence, model redistribution review and owner-browser
verification remain distinct release requirements. A passing development score
alone does not authorize a general release or a public 99% claim.
