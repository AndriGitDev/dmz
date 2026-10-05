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

A whole-text masking fallback has no individual typed detection set. Count it as
an abstention and an incorrect excerpt, including on reviewed negatives; do not
discard it from the accuracy denominator. Every gold entity in that excerpt is
a false negative for typed detection. Report abstentions separately from extra
typed detections and from the amount of text protected by masking. Successful
whole-text masking can therefore coexist with failed typed detection.

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

The [cluster uncertainty diagnostic](../bench/contextual/quality-clusters.md)
fixes a whole-cluster, within-stratum bootstrap procedure before blind-packet
predictions. It reports approximate descriptive intervals and explicitly withholds
bounds for insufficient, undefined or degenerate resamples. This tooling does not
freeze the still-outstanding acceptance sampling design or establish independence.

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

The [completed nine-label training trial](../bench/contextual/quality-99-protocol.md#completed-result--5-october-2026)
retains this reference. Both candidate epochs fail the predeclared general-text
guard. Epoch two reaches 99.1667% domain recall but only 98.4375% domain exact
accuracy, so even that stratum does not meet both targets. No candidate model was
promoted; the full comparison includes the failed criteria and aggregate counts.

## System acceptance

Detection scores do not measure the whole extension. Separately measure complete
identifier coverage in actual outbound text, partial residual exposure, benign
false masking, restoration correctness, session isolation, races and fallback.
Run the packaged browser artifact with the approved address vocabulary enabled;
record its hashes. Keep the quality-first full-precision model frozen during rule
experiments, and compare each candidate against that same starting artifact.

The core's local result includes `spans_available` and `detected_spans`: final
non-overlapping UTF-8 spans and types after model/rule overlap resolution. No raw
value or reverse mapping is included. Availability says that an individual span
set exists, not that detection is complete or correct. Whole-text fallbacks and
invalid model spans leave it false. Findings remain an explanation of processing;
they may include pre-merge findings and must not substitute for final typed spans.

`apps/shield/scripts/system-corpus.mjs` evaluates both the basic and trained
packaged Chromium paths on committed synthetic corpora. It uses the shared
`quality.py` scorer for exact final span/type sets, alongside independent
restoration-based masking coverage. Reports include the package/core hashes and
the full nine-type scope, keep corpora separate, and contain no text or individual
predictions. Those reused synthetic labels remain development diagnostics.

The first packaged exact-set replay on 4 October used the frozen trained scanner
and the updated address/numeric rules. It produced the following trained-path
results; all requests stayed local and every allowed output restored exactly.

| Reused synthetic corpus | Exact excerpts | Accuracy | Typed recall | Fully masked identifiers | Typed abstentions |
| --- | --- | --- | --- | --- | --- |
| Selected v0 | 42/42 | 100% | 37/37 | 37/37 | 0 |
| Challenge v0 | 17/18 | 94.44% | 15/16 | 15/16 | 0 |
| Exploratory v1 | 23/29 | 79.31% | 20/25 | 22/25 | 3 |
| Formatting v2 | 41/41 | 100% | 23/23 | 23/23 | 0 |
| Later probe v2 | 23/30 | 76.67% | 15/20 | 17/20 | 3 |

The later probe also has one extra typed detection; its typed precision is 15/16.
Full masking and typed detection differ because whole-text fallbacks can cover
identifiers without assigning individual types or spans. These small selected
sets are not independent acceptance strata. The full aggregate record is
[system-typed-v1.json](../bench/results/quality-99-v1/system-typed-v1.json).

The subsequent [canonical Unicode change](detector-unicode-normalization.md)
raises the trained exploratory set to 25/29 exact excerpts (86.21%) with 21/25
typed recall, and the later probe to 25/30 (83.33%) with 16/20 typed recall.
Each retains one typed abstention. Masking coverage is unchanged, and benign
masking decreases by one excerpt in each set. The original replay above remains
preserved; see [the updated aggregate](../bench/results/quality-99-v1/system-typed-after-nfc.json).

Independent quality evidence, model redistribution review and owner-browser
verification remain distinct release requirements. A passing development score
alone does not authorize a general release or a public 99% claim.

Packaged synthetic report schema 3 records SHA-256 for every packaged file except the generated `BUILD.json`, the measurement scripts and shared Python scorer, plus Node and Chromium runtime versions. The harness rechecks artifact and measurement hashes before marking a report complete and rejects a build changed during evaluation. Older reports retain their original schema and narrower provenance. These identities support reproducibility; they do not establish sample independence.

The schema-3 address-hyphen replay preserves all five corpora’s typed quality, masking and restoration counts from the post-NFC replay. Exploratory trained-path scanner fallbacks changed from four to one; this operational count is retained separately and is not attributed to the detector fix. All requests remained local.

Schema 4 adds aggregate `scanner_fallback_reasons` for each path and corpus. The counter uses only fixed reason codes; it never saves exception messages or input text. Input rejection, asset/inference failures, worker failures, busy responses and deadlines remain operational diagnostics rather than alternate detection metrics.

The [email shape and context probes](../bench/email-shape-v3.md) measure complete mailbox spans rather than suffix-only matches. Their selected synthetic improvements preserve preceding corpus results but do not establish representative email accuracy, deliverability, or full RFC/IDNA validation.

A separate [exact other-type veto experiment](../bench/contextual/entity-veto-result.md) reduces general false names but loses true names and fails the unchanged recall guard for both complete checkpoints. It also retains the frozen reference; higher excerpt accuracy alone does not satisfy the joint accuracy/recall target.

The [bounded entity-aware family](../bench/contextual/entity-family-result.md) evaluates all 36 predeclared threshold/priority configurations from verified caches. None passes the unchanged research criterion; the family is closed without model promotion.

The [reused split audit](../bench/contextual/split-overlap.md) finds no exact or NFC token-sequence matches across training and either development set, but seven case-folded general matches, all Person-negative. It does not establish document independence or alter frozen scores; no matching rows are removed.
